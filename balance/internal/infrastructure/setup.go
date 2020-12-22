package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/eventstore/subscriber"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

const (
	NotificationTopic = "notifications"
)

type Config struct {
	ApiPort           int      `env:"API_PORT" envDefault:"8030"`
	ElasticAddresses  []string `env:"ELASTIC_ADDRESSES" envSeparator:","`
	Partitions        int      `env:"PARTITIONS"`
	BalancePartitions []string `env:"BALANCE_PARTITIONS" envSeparator:","`
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

	repo := gateway.BalanceRepository{
		Client: es,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// es player
	esRepo := player.NewGrpcRepository(cfg.EsAddress)

	// the clientID could be suffixed with low partition ID
	natsSub, err := subscriber.NewNatsSubscriber(ctx, cfg.NatsAddress, "test-cluster", "balance-0", cfg.Topic, NotificationTopic)
	if err != nil {
		log.Fatalf("Error creating NATS subscriber: %s", err)
	}

	// if we used partitioned topic, we would not need a locker, since each instance would be the only one responsible for a partion range
	pool, err := locks.NewConsulLockPool(cfg.ConsulAddress)
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}

	balanceRebuild := pool.NewLock("balance-freeze", cfg.LockExpiry)
	restarter := projection.NewNotifierLockRestarter(
		balanceRebuild,
		natsSub,
	)
	balanceUC := usecase.NewBalanceUsecase(repo, restarter, cfg.Partitions)

	prjCtrl := controller.ProjectionBalance{
		BalanceUsecase: balanceUC,
	}

	balancePartitions, err := common.ParseSlots(cfg.BalancePartitions)
	if err != nil {
		log.Fatal(err)
	}
	if len(balancePartitions) != cfg.Partitions {
		log.Fatal("Wrong size for Balance Partitions ranges. Expected %d, got %d", cfg.Partitions, len(balancePartitions))
	}

	workers := make([]common.LockWorker, len(balancePartitions))
	for i, v := range balancePartitions {
		workers[i] = common.LockWorker{
			Lock: pool.NewLock("balance-worker", cfg.LockExpiry),
			Worker: common.NewRunWorker("Balance Projection", projection.NewProjectionPartition(
				balanceRebuild,
				prjCtrl,
				natsSub,
				projection.BootStage{
					AggregateTypes: []string{event.AggregateType_Account},
					Subscriber:     natsSub,
					Repository:     esRepo,
					PartitionLo:    v.From,
					PartitionHi:    v.To,
				},
			)),
		}
	}

	memberlist, err := common.NewConsulMemberList(cfg.ConsulAddress, "balance-member", cfg.LockExpiry)
	if err != nil {
		log.Fatal(err)
	}
	go common.BalanceWorkers(ctx, memberlist, workers, cfg.LockExpiry/2)

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
