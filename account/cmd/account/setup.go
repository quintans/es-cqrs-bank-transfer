package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	mg "github.com/golang-migrate/migrate/v4/database/mongodb"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/lock"
	"github.com/quintans/eventsourcing/lock/consullock"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/projection"
	pnats "github.com/quintans/eventsourcing/projection/nats"
	"github.com/quintans/eventsourcing/sink/nats"
	"github.com/quintans/eventsourcing/store/mongodb"
	"github.com/quintans/eventsourcing/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/latch"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/app"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/infra/controller"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/infra/gateway/esdb"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

type Config struct {
	DBMigrationsURL   string `env:"DB_MIGRATIONS_URL" envDefault:"file:///migrations"`
	ApiPort           int    `env:"API_PORT" envDefault:"8000"`
	SnapshotThreshold uint32 `env:"SNAPSHOT_THRESHOLD" envDefault:"50"`
	GrpcAddress       string `env:"ES_GRPC_ADDRESS" envDefault:":3000"`

	ConsulURL  string        `env:"CONSUL_URL"`
	LockExpiry time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
	NatsURL    string        `env:"NATS_URL"`

	Topic string `env:"TOPIC" envDefault:"accounts"`

	ConfigEs
}

type ConfigEs struct {
	EsHost     string `env:"ES_HOST" envDefault:"localhost"`
	EsPort     int    `env:"ES_PORT" envDefault:"27017"`
	EsUser     string `env:"ES_USER" envDefault:"root"`
	EsPassword string `env:"ES_PASSWORD" envDefault:"password"`
	EsName     string `env:"ES_NAME" envDefault:"accounts"`
}

func Setup(cfg *Config, logger log.Logger) {
	logger.Info("doing migration")
	connStr := fmt.Sprintf("mongodb://%s:%d/%s?replicaSet=rs0", cfg.EsHost, cfg.EsPort, cfg.EsName)
	err := migration(logger, connStr, cfg.DBMigrationsURL)
	if err != nil {
		logger.Fatalf("%+v", err)
	}

	// evenstore
	esRepo, err := mongodb.NewStoreWithURI(connStr, cfg.EsName)
	if err != nil {
		logger.Fatalf("%+v", err)
	}

	codec := event.NewJSONCodec()
	esTx := eventsourcing.NewEventStore[*entity.Transaction](esRepo, codec, &eventsourcing.EsOptions{SnapshotThreshold: cfg.SnapshotThreshold})
	esAcc := eventsourcing.NewEventStore[*entity.Account](esRepo, codec, &eventsourcing.EsOptions{SnapshotThreshold: cfg.SnapshotThreshold})

	// Repository
	pr := mongodb.NewProjectionResume(esRepo.Client(), cfg.EsName, "resumes")
	txRepo := esdb.NewTransactionRepository(esTx)
	accRepo := esdb.NewAccountRepository(esAcc)

	// Usecases
	accUC := app.NewAccountService(logger, accRepo)
	txUC := app.NewTransactionService(logger, pr, txRepo, accRepo, esRepo.WithTx)

	// controllers
	reactor := controller.NewReactor(logger, pr, txUC, codec)
	rest := controller.NewRestController(accUC, txUC)

	ltx := latch.NewCountDownLatch()
	ctx, cancel := context.WithCancel(context.Background())

	// event forwarding
	lockPool, err := consullock.NewPool(cfg.ConsulURL)
	if err != nil {
		logger.Fatalf("Error instantiating Locker: %+v", err)
	}
	lockFact := func(lockName string) lock.Locker {
		return lockPool.NewLock(lockName, cfg.LockExpiry)
	}

	if err := eventForwarderWorkers(ctx, logger, ltx, connStr, cfg, lockFact, esRepo); err != nil {
		logger.Fatalf("%+v", err)
	}

	// reactor
	if err := reactorConsumerWorkers(ctx, logger, ltx, cfg, lockFact, esRepo, reactor); err != nil {
		logger.Fatalf("%+v", err)
	}

	// Servers

	// grpc
	ltx.Add(1)
	go func() {
		projection.StartGrpcServer(ctx, cfg.GrpcAddress, esRepo)
		ltx.Done()
	}()

	// rest server
	ltx.Add(1)
	go func() {
		startRestServer(ctx, logger, rest, cfg.ApiPort)
		ltx.Done()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	ltx.WaitWithTimeout(3 * time.Second)
}

// eventForwarderWorkers creates workers that listen to database changes,
// transform them to events and publish them into the message bus.
// Partitions will be grouped in slots, and that will determine the number of workers.
func eventForwarderWorkers(ctx context.Context, logger log.Logger, ltx *latch.CountDownLatch, connStr string, cfg *Config, lockFact projection.LockerFactory, setSinkRepo mongodb.SetSeqRepository) error {
	// sinker provider
	sinker, err := nats.NewSink(logger, cfg.Topic, 1, cfg.NatsURL)
	if err != nil {
		return faults.Errorf("initialising NATS (%s) Sink '%s' on boot: %w", cfg.NatsURL, cfg.Topic, err)
	}

	feed := mongodb.NewFeed(logger, connStr, cfg.EsName, sinker, setSinkRepo)

	ltx.Add(1)
	go func() {
		<-ctx.Done()
		sinker.Close()
		ltx.Done()
	}()

	forwarder := projection.EventForwarderWorker(logger, "account-forwarder", lockFact, feed.Run)
	balancer := worker.NewSingleBalancer(logger, forwarder, cfg.LockExpiry/2)
	ltx.Add(1)
	go func() {
		<-balancer.Start(ctx)
		ltx.Done()
	}()

	return nil
}

// reactorConsumerWorkers creates workers that listen to events coming through the event bus,
// forwarding them to an handler. This is the same approach used for projections, but for the write side of things.
// The number of workers will be equal to the number of partitions.
func reactorConsumerWorkers(
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

	reactor := projection.Project(ctx, logger, lockFact, esRepo, sub, proj)
	balancer := worker.NewSingleBalancer(logger, reactor, cfg.LockExpiry/2)

	ltx.Add(1)
	go func() {
		<-balancer.Start(ctx)
		ltx.Done()
	}()

	return nil
}

func startRestServer(ctx context.Context, logger log.Logger, c controller.RestController, port int) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", c.Ping)
	e.POST("/accounts", c.Create)
	e.POST("/transactions", c.Transaction)
	e.GET("/accounts/:id", c.Balance)

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

func migration(logger log.Logger, dbURL string, sourceURL string) error {
	p := &mg.Mongo{}
	d, err := p.Open(dbURL)
	if err != nil {
		return faults.Errorf("failed to connect to '%s': %w", dbURL, err)
	}

	defer func() {
		if err := d.Close(); err != nil {
			logger.Fatal(err)
		}
	}()

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "", d)
	if err != nil {
		return faults.Errorf("unable to execute migration scripts: %w", err)
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return faults.Errorf("failed to migrate database: %w", err)
	}

	return nil
}
