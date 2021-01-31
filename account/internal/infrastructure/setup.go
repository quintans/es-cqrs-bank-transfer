package infrastructure

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	mg "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nats-io/stan.go"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/gateway"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/sink"
	"github.com/quintans/eventstore/store"
	"github.com/quintans/eventstore/store/mongodb"
	"github.com/quintans/eventstore/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	ApiPort           int    `env:"API_PORT" envDefault:"8000"`
	SnapshotThreshold uint32 `env:"SNAPSHOT_THRESHOLD" envDefault:"50"`
	GrpcAddress       string `env:"ES_GRPC_ADDRESS" envDefault:":3000"`

	ConfigEs
	ConfigForwarder
}

type ConfigForwarder struct {
	ConsulAddress  string        `env:"CONSUL_ADDRESS"`
	LockExpiry     time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
	PartitionSlots []string      `env:"PARTITION_SLOTS" envSeparator:","`
	NatsAddress    string        `env:"NATS_ADDRESS"`
	Topic          string        `env:"TOPIC" envDefault:"accounts"`
}

type ConfigEs struct {
	EsUser     string `env:"ES_USER" envDefault:"root"`
	EsPassword string `env:"ES_PASSWORD" envDefault:"password"`
	EsHost     string `env:"ES_HOST"`
	EsPort     int    `env:"ES_PORT" envDefault:"27017"`
	EsName     string `env:"ES_NAME" envDefault:"accounts"`
}

func Setup(cfg *Config) {
	dbURL := fmt.Sprintf("mongodb://%s:%d/%s?replicaSet=rs0", cfg.EsHost, cfg.EsPort, cfg.EsName)
	err := migration(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	client, err := newDB(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// evenstore
	esRepo := mongodb.NewStoreDB(client, cfg.EsName)
	es := eventstore.NewEventStore(esRepo, cfg.SnapshotThreshold, entity.EventFactory{})

	go player.StartGrpcServer(context.Background(), cfg.GrpcAddress, esRepo)

	// event forwarding
	startEventForwarder(context.Background(), client, cfg)

	// Repository
	repo := gateway.NewAccountRepository(es)

	// Usecases
	uc := usecase.NewAccountUsecase(repo)

	// Controllers
	c := controller.NewController(uc)

	// Rest server
	startRestServer(c, cfg.ApiPort)
}

func startEventForwarder(ctx context.Context, client *mongo.Client, cfg *Config) {
	partitionSlots, err := worker.ParseSlots(cfg.PartitionSlots)
	if err != nil {
		log.Fatal(err)
	}
	var partitions uint32
	for _, v := range partitionSlots {
		if v.To > partitions {
			partitions = v.To
		}
	}

	pool, err := locks.NewConsulLockPool(cfg.ConsulAddress)
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}

	slotLen := len(partitionSlots)
	sinkers := make([]*sink.NatsSink, slotLen)
	for i := 0; i < slotLen; i++ {
		idx := strconv.Itoa(i)
		sinkers[i] = sink.NewNatsSink(cfg.Topic, partitions, "test-cluster", "forwarder-id-"+idx, stan.NatsURL(cfg.NatsAddress))
	}

	workers := make([]worker.Worker, slotLen)
	for i, v := range partitionSlots {
		listener, err := mongodb.NewFeedDB(client, cfg.EsName, mongodb.WithPartitions(partitions, v.From, v.To))
		if err != nil {
			log.Fatalf("Error instantiating store listener: %v", err)
		}

		slots := fmt.Sprintf("%d-%d", v.From, v.To)
		workers[i] = worker.NewRunWorker(
			"forwarder-worker-"+slots,
			pool.NewLock("forwarder-lock-"+slots, cfg.LockExpiry),
			store.NewForwarder(
				"forwarder-"+slots,
				listener,
				sinkers[i],
			))
	}

	memberlist, err := worker.NewConsulMemberList(cfg.ConsulAddress, "forwarder-member", cfg.LockExpiry)
	if err != nil {
		log.Fatal(err)
	}
	go worker.BalanceWorkers(ctx, memberlist, workers, cfg.LockExpiry/2)
}

func startRestServer(c controller.Controller, port int) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/create", c.Create)
	e.POST("/deposit", c.Deposit)
	e.POST("/withdraw", c.Withdraw)
	e.POST("/transfer", c.Transfer)
	e.GET("/:id", c.Balance)

	// Start server
	address := fmt.Sprintf(":%d", port)
	e.Logger.Fatal(e.Start(address))
}

func newDB(dbURL string) (*mongo.Client, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	opts := options.Client().ApplyURI(dbURL)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func migration(dbURL string) error {
	p := &mg.Mongo{}
	d, err := p.Open(dbURL)
	if err != nil {
		return faults.Errorf("failed to connect to '%s': %w", dbURL, err)
	}

	defer func() {
		if err := d.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	m, err := migrate.NewWithDatabaseInstance("file://../../migrations", "", d)
	if err != nil {
		return faults.Errorf("unable to find migration scripts: %w", err)
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return faults.Errorf("failed to migrate database: %w", err)
	}

	return nil
}
