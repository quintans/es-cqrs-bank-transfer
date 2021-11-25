package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/lock"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/player"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/eventsourcing/projection/resumestore"
	"github.com/quintans/eventsourcing/subscriber"
	"github.com/quintans/eventsourcing/worker"
	"github.com/quintans/toolkit/locks"
	"github.com/sirupsen/logrus"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

const (
	NotificationTopic     = "notifications"
	BalanceProjectionName = "balance"
)

var logger = log.NewLogrus(logrus.StandardLogger())

type Config struct {
	ApiPort    int      `env:"API_PORT" envDefault:"8030"`
	ElasticURL []string `env:"ELASTIC_URL" envSeparator:","`
	Partitions uint32   `env:"PARTITIONS"`
	ConfigEs
	ConfigRedis
	ConfigNats
}

type ConfigEs struct {
	EsAddress   string `env:"ES_ADDRESS"`
	EsBatchSize int    `env:"ES_BATCH_SIZE" envDefault:"20"`
}

type ConfigRedis struct {
	ConsulURL  string        `env:"CONSUL_URL"`
	LockExpiry time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
}

type ConfigNats struct {
	NatsURL      string `env:"NATS_URL" envDefault:"localhost:4222"`
	Topic        string `env:"TOPIC" envDefault:"accounts"`
	Subscription string `env:"SUBSCRIPTION" envDefault:"accounts-subscription"`
}

func Setup(cfg Config) {
	escfg := elasticsearch.Config{
		Addresses: cfg.ElasticURL,
	}
	es, err := elasticsearch.NewClient(escfg)
	if err != nil {
		logger.Fatalf("Error creating the client: %s", err)
	}

	waitForElastic(es)

	repo := gateway.NewBalanceRepository(es)

	ctx, cancel := context.WithCancel(context.Background())
	// es player
	esRepo := player.NewGrpcRepository(cfg.EsAddress)

	latch := locks.NewCountDownLatch()

	prjCtrl := startProjection(ctx, cfg, latch, repo, esRepo)

	balanceUC := usecase.NewBalanceUsecase(repo)

	restCtrl := controller.RestController{
		BalanceUsecase: balanceUC,
	}

	latch.Add(1)
	go startRestServer(ctx, latch, restCtrl, prjCtrl, cfg.ApiPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	latch.WaitWithTimeout(3 * time.Second)
}

func waitForElastic(es *elasticsearch.Client) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var err error
	var res *esapi.Response
	for i := 0; i < 5; i++ {
		res, err = es.Info()
		if err != nil {
			logger.Infof("Unable to get info elasticsearch about: %v", err)
			<-ticker.C
		}
	}

	if err != nil {
		logger.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()
	logger.Info("Elasticsearch info: ", res)
}

// startProjection creates workers that listen to events coming through the event bus,
// forwarding them to the projection handler.
// The number of workers will be equal to the number of partitions.
// These workers manage the connection with the partition subscribers and the recording of the stream resumer value.
func startProjection(
	ctx context.Context,
	cfg Config,
	latch *locks.CountDownLatch,
	repo domain.BalanceRepository,
	esRepo player.Repository,
) controller.ProjectionBalance {
	// if we used partitioned topic, we would not need a locker, since each instance would be the only one responsible for a partion range
	lockPool, err := lock.NewConsulLockPool(cfg.ConsulURL)
	if err != nil {
		logger.Fatal("Error instantiating Locker: %v", err)
	}

	natsCanceller, err := subscriber.NewNatsProjectionSubscriber(ctx, logger, cfg.NatsURL, NotificationTopic)
	if err != nil {
		logger.Fatalf("Error creating NATS subscriber: %s", err)
	}

	lockFact := func(lockName string) lock.Locker {
		return lockPool.NewLock(lockName, cfg.LockExpiry)
	}

	streamResumer, err := resumestore.NewElasticSearchStreamResumer(cfg.ElasticURL, "stream_resume")
	if err != nil {
		logger.Fatal("Error instantiating Locker: %v", err)
	}

	natsPartitionSubscriber, err := subscriber.NewNatsSubscriber(ctx, logger, cfg.NatsURL, "test-cluster", BalanceProjectionName+"-"+uuid.New().String(), streamResumer)
	if err != nil {
		logger.Fatalf("Error creating NATS subscriber: %+v", err)
	}

	prjUC := usecase.NewProjectionUsecase(repo, event.EventFactory{}, eventsourcing.JSONCodec{}, nil, esRepo)
	// create workers according to the topic that we want to listen
	workers, tokenStreams := projection.ReactorConsumerWorkers(ctx, logger, BalanceProjectionName, lockFact, cfg.Topic, cfg.Partitions, natsPartitionSubscriber, prjUC.Handle)
	memberlist, err := worker.NewConsulMemberList(cfg.ConsulURL, BalanceProjectionName+"-member", cfg.LockExpiry)
	if err != nil {
		logger.Fatal(err)
	}
	balancer := worker.NewBalancer(BalanceProjectionName, logger, memberlist, workers, cfg.LockExpiry/2)

	unlockWaiter := lockPool.NewLock(BalanceProjectionName+"-freeze", cfg.LockExpiry)
	startStop := projection.NewStartStopBalancer(logger, unlockWaiter, natsCanceller, *balancer)

	rebuilder := projection.NewNotifierLockRestarter(
		logger,
		unlockWaiter,
		natsCanceller,
		natsPartitionSubscriber,
		streamResumer,
		tokenStreams,
		memberlist,
	)
	prjCtrl := controller.NewProjectionBalance(prjUC, rebuilder)

	// work balancer
	latch.Add(1)
	go func() {
		startStop.Run(context.Background())
		latch.Done()
	}()

	return prjCtrl
}

func startRestServer(ctx context.Context, latch *locks.CountDownLatch, restCtrl controller.RestController, prjCtrl controller.ProjectionBalance, port int) {
	defer latch.Done()

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", restCtrl.Ping)
	e.GET("/balance/:id", restCtrl.GetOne)
	e.GET("/balance/", restCtrl.ListAll)
	e.GET("/rebuild/balance", prjCtrl.RebuildBalance)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(c); err != nil {
			logger.Fatal(err)
		}
	}()

	// Start server
	address := fmt.Sprintf(":%d", port)
	if err := e.Start(address); err != nil {
		logger.WithError(err).Error("failing shutting down the server")
	} else {
		logger.Info("shutting down the server")
	}
}
