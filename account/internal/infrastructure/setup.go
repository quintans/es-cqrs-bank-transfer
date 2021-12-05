package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	mg "github.com/golang-migrate/migrate/v4/database/mongodb"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/lock"
	"github.com/quintans/eventsourcing/lock/consullock"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/player"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/eventsourcing/store"
	"github.com/quintans/eventsourcing/store/mongodb"
	"github.com/quintans/eventsourcing/stream/nats"
	"github.com/quintans/eventsourcing/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/locks"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/gateway/esdb"
)

type Config struct {
	DBMigrationsURL   string `env:"DB_MIGRATIONS_URL" envDefault:"file:///migrations"`
	ApiPort           int    `env:"API_PORT" envDefault:"8000"`
	SnapshotThreshold uint32 `env:"SNAPSHOT_THRESHOLD" envDefault:"50"`
	GrpcAddress       string `env:"ES_GRPC_ADDRESS" envDefault:":3000"`

	ConsulURL  string        `env:"CONSUL_URL"`
	LockExpiry time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
	NatsURL    string        `env:"NATS_URL"`

	Partitions uint32 `env:"PARTITIONS"`
	Topic      string `env:"TOPIC" envDefault:"accounts"`

	ConfigEs
	Forwarder ConfigForwarder
}

type ConfigForwarder struct {
	PartitionSlots []string `env:"PARTITION_SLOTS" envSeparator:","`
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
		logger.Fatal(err)
	}

	// evenstore
	esRepo, err := mongodb.NewStore(connStr, cfg.EsName)
	if err != nil {
		logger.Fatal(err)
	}
	es := eventsourcing.NewEventStore(esRepo, entity.AggregateFactory{}, eventsourcing.WithSnapshotThreshold(cfg.SnapshotThreshold))

	ctx, cancel := context.WithCancel(context.Background())

	// Repository
	accRepo := esdb.NewAccountRepository(es)

	// Usecases
	accUC := usecase.NewAccountUsecase(logger, accRepo)

	txRepo := esdb.NewTransactionRepository(es)
	txUC := usecase.NewTransactionUsecase(logger, txRepo, accRepo)
	reactor := controller.NewListener(logger, txUC, entity.AggregateFactory{}, eventsourcing.JSONCodec{})

	c := controller.NewRestController(accUC, txUC)

	memberlist, err := worker.NewConsulMemberList(cfg.ConsulURL, "forwarder-member", cfg.LockExpiry)
	if err != nil {
		logger.Fatal(err)
	}

	latch := locks.NewCountDownLatch()

	// event forwarding
	workers := eventForwarderWorkers(ctx, logger, latch, connStr, cfg)
	balancer := worker.NewMembersBalancer(logger, "account", memberlist, workers, cfg.LockExpiry/2)
	if err := reactorConsumerWorkers(ctx, logger, latch, cfg, reactor.Handler); err != nil {
		logger.Fatal(err)
	}

	latch.Add(1)
	go func() {
		<-balancer.Start(ctx)
		latch.Done()
	}()

	latch.Add(1)
	go func() {
		player.StartGrpcServer(ctx, cfg.GrpcAddress, esRepo)
		latch.Done()
	}()

	// Rest server
	latch.Add(1)
	go func() {
		startRestServer(ctx, logger, c, cfg.ApiPort)
		latch.Done()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	latch.WaitWithTimeout(3 * time.Second)
}

// eventForwarderWorkers creates workers that listen to database changes,
// transform them to events and publish them into the message bus.
// Partitions will be grouped in slots, and that will determine the number of workers.
func eventForwarderWorkers(ctx context.Context, logger log.Logger, latch *locks.CountDownLatch, connStr string, cfg *Config) []worker.Worker {
	lockPool, err := consullock.NewPool(cfg.ConsulURL)
	if err != nil {
		logger.Fatalf("Error instantiating Locker: %+v", err)
	}

	partitionSlots, err := worker.ParseSlots(cfg.Forwarder.PartitionSlots)
	if err != nil {
		logger.Fatal(err)
	}
	var partitions uint32
	for _, v := range partitionSlots {
		if v.To > partitions {
			partitions = v.To
		}
	}
	lockFact := func(lockName string) lock.Locker {
		return lockPool.NewLock(lockName, cfg.LockExpiry)
	}
	feederFact := func(partitionLow, partitionHi uint32) store.Feeder {
		return mongodb.NewFeed(logger, connStr, cfg.EsName, mongodb.WithPartitions(partitions, partitionLow, partitionHi))
	}

	const name = "forwarder"
	// sinker provider
	clientID := name + "-" + uuid.New().String()
	sinker, err := nats.NewSink(logger, cfg.Topic, partitions, cfg.NatsURL)
	if err != nil {
		logger.Fatalf("Error initialising NATS (%s) Sink '%s' on boot: %+v", cfg.NatsURL, clientID, err)
	}
	latch.Add(1)
	go func() {
		<-ctx.Done()
		sinker.Close()
		latch.Done()
	}()

	return projection.EventForwarderWorkers(ctx, logger, name, lockFact, feederFact, sinker, partitionSlots)
}

// reactorConsumerWorkers creates workers that listen to events coming through the event bus,
// forwarding them to an handler. This is the same approache used for projections, but for the write side of things.
// The number of workers will be equal to the number of partitions.
func reactorConsumerWorkers(ctx context.Context, logger log.Logger, latch *locks.CountDownLatch, cfg *Config, handler projection.EventHandlerFunc) error {
	const streamName = "accounts-reactor"
	done, err := nats.NewReactor(ctx, logger, cfg.NatsURL, streamName, cfg.Topic, cfg.Partitions, handler)
	if err != nil {
		return err
	}

	latch.Add(1)
	go func() {
		<-done
		latch.Done()
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
