package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/log"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/eventstore/projection/resumestore"
	"github.com/quintans/eventstore/subscriber"
	"github.com/quintans/eventstore/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/locks"
	"github.com/sirupsen/logrus"
)

const (
	NotificationTopic = "notifications"
)

var (
	logger = log.NewLogrus(logrus.StandardLogger())
)

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

	natsNotifier, err := subscriber.NewNatsProjectionSubscriber(ctx, logger, cfg.NatsURL, NotificationTopic)
	if err != nil {
		logger.Fatalf("Error creating NATS subscriber: %s", err)
	}

	// if we used partitioned topic, we would not need a locker, since each instance would be the only one responsible for a partion range
	pool, err := locks.NewConsulLockPool(cfg.ConsulURL)
	if err != nil {
		logger.Fatal("Error instantiating Locker: %v", err)
	}

	streamResumer, err := resumestore.NewElasticSearchStreamResumer(cfg.ElasticURL, "stream_resume")
	if err != nil {
		logger.Fatal("Error instantiating Locker: %v", err)
	}
	natsSub, err := subscriber.NewNatsSubscriber(ctx, logger, cfg.NatsURL, "test-cluster", "balance-"+uuid.New().String(), streamResumer)
	if err != nil {
		logger.Fatalf("Error creating NATS subscriber: %s", err)
	}

	tokenStreams := []projection.StreamResume{}
	unlockWaiter := pool.NewLock("balance-freeze", cfg.LockExpiry)
	restarter := projection.NewNotifierLockRestarter(
		logger,
		unlockWaiter,
		natsNotifier,
		func(ctx context.Context) error {
			for _, ts := range tokenStreams {
				token, err := natsSub.GetResumeToken(ctx, ts.Topic)
				if err != nil {
					return faults.Wrap(err)
				}
				err = streamResumer.SetStreamResumeToken(ctx, ts.String(), token)
				if err != nil {
					return faults.Wrap(err)
				}
			}
			return nil
		},
	)

	prjUC := usecase.NewProjectionUsecase(repo)

	prjCtrl := controller.NewProjectionBalance(prjUC, event.EventFactory{}, eventstore.JSONCodec{}, nil)

	balanceUC := usecase.NewBalanceUsecase(repo, restarter, cfg.Partitions, esRepo, prjCtrl)

	workers := make([]worker.Worker, cfg.Partitions)
	var i uint32
	for i = 1; i <= cfg.Partitions; i++ {
		idx := strconv.Itoa(int(i))
		resume := projection.StreamResume{
			Topic:  common.TopicWithPartition(cfg.Topic, i),
			Stream: "balance-projection",
		}
		tokenStreams = append(tokenStreams, resume)
		runner := projection.NewProjectionPartition(
			logger,
			unlockWaiter,
			natsNotifier,
			natsSub,
			resume,
			func(e eventstore.Event) bool {
				// just to demo filter capability
				return common.In(e.AggregateType, event.AggregateType_Account)
			},
			prjCtrl.Handle,
		)
		workers[i-1] = worker.NewRunWorker(
			logger,
			"balance-projection-"+idx,
			pool.NewLock("balance-lock-"+idx, cfg.LockExpiry),
			runner,
		)
	}

	memberlist, err := worker.NewConsulMemberList(cfg.ConsulURL, "balance-member", cfg.LockExpiry)
	if err != nil {
		logger.Fatal(err)
	}
	go worker.BalanceWorkers(ctx, logger, memberlist, workers, cfg.LockExpiry/2)

	restCtrl := controller.RestController{
		BalanceUsecase: balanceUC,
	}

	go startRestServer(ctx, restCtrl, cfg.ApiPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	time.Sleep(3 * time.Second)
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

func startRestServer(ctx context.Context, c controller.RestController, port int) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", c.Ping)
	e.GET("/balance/:id", c.GetOne)
	e.GET("/balance/", c.ListAll)
	e.GET("/rebuild/balance", c.RebuildBalance)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(c); err != nil {
			e.Logger.Fatal(err)
		}
	}()

	// Start server
	address := fmt.Sprintf(":%d", port)
	if err := e.Start(address); err != nil {
		logger.Info("shutting down the server")
	}
}
