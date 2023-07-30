package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/app"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/infra/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/infra/gateway"
	"github.com/quintans/eventsourcing/lock"
	"github.com/quintans/eventsourcing/lock/consullock"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/projection"
	pnats "github.com/quintans/eventsourcing/projection/nats"
	"github.com/quintans/eventsourcing/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/latch"
	"github.com/sirupsen/logrus"
)

const (
	NotificationTopic     = "notifications"
	BalanceProjectionName = "balance"
)

var logger = log.NewLogrus(logrus.StandardLogger())

type Config struct {
	ApiPort    int      `env:"API_PORT" envDefault:"8030"`
	ElasticURL []string `env:"ELASTIC_URL" envSeparator:","`
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

	ltx := latch.NewCountDownLatch()

	// repositories
	pr := gateway.NewProjectionResume(es, "stream_resume")
	repo := gateway.NewBalanceRepository(logger, es)

	ctx, cancel := context.WithCancel(context.Background())
	// es player
	esRepo := projection.NewGrpcRepository(cfg.EsAddress)

	lockPool, err := consullock.NewPool(cfg.ConsulURL)
	if err != nil {
		logger.Fatalf("Error instantiating Locker: %+v", err)
	}
	lockFact := func(lockName string) lock.Locker {
		return lockPool.NewLock(lockName, cfg.LockExpiry)
	}

	// services
	codec := event.NewJSONCodec()
	balanceService := app.NewBalanceService(pr, repo)

	// controllers
	proj := controller.NewProjection(logger, repo, balanceService, codec)
	restCtrl := controller.RestController{
		BalanceUsecase: balanceService,
	}

	// servers
	startProjection(ctx, logger, ltx, &cfg, lockFact, esRepo, proj)

	ltx.Add(1)
	go startRestServer(ctx, ltx, restCtrl, cfg.ApiPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	ltx.WaitWithTimeout(3 * time.Second)
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
	logger log.Logger,
	ltx *latch.CountDownLatch,
	cfg *Config,
	lockFact projection.LockerFactory,
	esRepo projection.EventsRepository,
	proj projection.Projection,
) error {
	sub, err := pnats.NewSubscriberWithURL(ctx, logger, cfg.NatsURL, cfg.Topic)
	if err != nil {
		return faults.Errorf("creating NATS subscriber for '%s': %w", cfg.Topic, err)
	}

	projector := projection.Project(ctx, logger, lockFact, esRepo, sub, proj)
	balancer := worker.NewSingleBalancer(logger, projector, cfg.LockExpiry/2)

	ltx.Add(1)
	go func() {
		<-balancer.Start(ctx)
		ltx.Done()
	}()

	return nil
}

func startRestServer(ctx context.Context, ltx *latch.CountDownLatch, restCtrl controller.RestController, port int) {
	defer ltx.Done()

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", restCtrl.Ping)
	e.GET("/balance/:id", restCtrl.GetOne)
	e.GET("/balance/", restCtrl.ListAll)

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
