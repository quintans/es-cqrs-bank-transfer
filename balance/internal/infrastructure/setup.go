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
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/eventstore/projection/resumestore"
	"github.com/quintans/eventstore/subscriber"
	"github.com/quintans/eventstore/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

const (
	NotificationTopic = "notifications"
)

type Config struct {
	ApiPort          int      `env:"API_PORT" envDefault:"8030"`
	ElasticAddresses []string `env:"ELASTIC_ADDRESSES" envSeparator:","`
	Partitions       uint32   `env:"PARTITIONS"`
	ConfigEs
	ConfigRedis
	ConfigNats
}

type ConfigEs struct {
	EsAddress   string `env:"ES_ADDRESS"`
	EsBatchSize int    `env:"ES_BATCH_SIZE" envDefault:"20"`
}

type ConfigRedis struct {
	ConsulAddress string        `env:"CONSUL_ADDRESS"`
	LockExpiry    time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
}

type ConfigNats struct {
	NatsAddress  string `env:"NATS_ADDRESS" envDefault:"localhost:4222"`
	Topic        string `env:"TOPIC" envDefault:"accounts"`
	Subscription string `env:"SUBSCRIPTION" envDefault:"accounts-subscription"`
}

func Setup(cfg Config) {
	escfg := elasticsearch.Config{
		Addresses: cfg.ElasticAddresses,
	}
	es, err := elasticsearch.NewClient(escfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	res, err := es.Info()
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}

	defer res.Body.Close()
	log.Println("Elasticsearch info: ", res)

	repo := gateway.NewBalanceRepository(es)

	ctx, cancel := context.WithCancel(context.Background())
	// es player
	esRepo := player.NewGrpcRepository(cfg.EsAddress)

	natsNotifier, err := subscriber.NewNatsProjectionSubscriber(ctx, cfg.NatsAddress, NotificationTopic)
	if err != nil {
		log.Fatalf("Error creating NATS subscriber: %s", err)
	}

	// if we used partitioned topic, we would not need a locker, since each instance would be the only one responsible for a partion range
	pool, err := locks.NewConsulLockPool(cfg.ConsulAddress)
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}

	streamResumer, err := resumestore.NewElasticSearchStreamResumer(cfg.ElasticAddresses, "stream_resume")
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}
	natsSub, err := subscriber.NewNatsSubscriber(ctx, cfg.NatsAddress, "test-cluster", "balance-"+uuid.New().String(), streamResumer)
	if err != nil {
		log.Fatalf("Error creating NATS subscriber: %s", err)
	}

	tokenStreams := []projection.StreamResume{}
	unlockWaiter := pool.NewLock("balance-freeze", cfg.LockExpiry)
	restarter := projection.NewNotifierLockRestarter(
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
			"balance-projection-"+idx,
			pool.NewLock("balance-lock-"+idx, cfg.LockExpiry),
			runner,
		)
	}

	memberlist, err := worker.NewConsulMemberList(cfg.ConsulAddress, "balance-member", cfg.LockExpiry)
	if err != nil {
		log.Fatal(err)
	}
	go worker.BalanceWorkers(ctx, memberlist, workers, cfg.LockExpiry/2)

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

func startRestServer(ctx context.Context, c controller.RestController, port int) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", c.ListAll)
	e.GET("/balance/rebuild", c.RebuildBalance)

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
		log.Info("shutting down the server")
	}
}
