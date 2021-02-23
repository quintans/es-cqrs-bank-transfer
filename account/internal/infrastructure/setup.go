package infrastructure

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	mg "github.com/golang-migrate/migrate/v4/database/mongodb"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nats-io/stan.go"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/gateway/esdb"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/eventstore/projection/resumestore"
	"github.com/quintans/eventstore/sink"
	"github.com/quintans/eventstore/store"
	"github.com/quintans/eventstore/store/mongodb"
	"github.com/quintans/eventstore/subscriber"
	"github.com/quintans/eventstore/worker"
	"github.com/quintans/faults"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func Setup(cfg *Config) {
	log.Info("doing migration")
	connStr := fmt.Sprintf("mongodb://%s:%d/%s?replicaSet=rs0", cfg.EsHost, cfg.EsPort, cfg.EsName)
	err := migration(connStr, cfg.DBMigrationsURL)
	if err != nil {
		log.Fatal(err)
	}

	// evenstore
	esRepo, err := mongodb.NewStore(connStr, cfg.EsName)
	if err != nil {
		log.Fatal(err)
	}
	es := eventstore.NewEventStore(esRepo, cfg.SnapshotThreshold, entity.AggregateFactory{})

	ctx := context.Background()

	go player.StartGrpcServer(ctx, cfg.GrpcAddress, esRepo)

	pool, err := locks.NewConsulLockPool(cfg.ConsulURL)
	if err != nil {
		log.Fatalf("Error instantiating Locker: %v", err)
	}

	// Repository
	accRepo := esdb.NewAccountRepository(es)

	// Usecases
	accUC := usecase.NewAccountUsecase(accRepo)

	txRepo := esdb.NewTransactionRepository(es)
	txUC := usecase.NewTransactionUsecase(txRepo, accRepo)
	reactor := controller.NewListener(txUC, event.EventFactory{}, eventstore.JSONCodec{})

	c := controller.NewRestController(accUC, txUC)

	// event forwarding
	workers := startEventForwarder(ctx, connStr, cfg.EsName, pool, cfg)

	workers = append(workers, startTransactionConsumers(ctx, pool, connStr, cfg, reactor.Handler)...)

	memberlist, err := worker.NewConsulMemberList(cfg.ConsulURL, "forwarder-member", cfg.LockExpiry)
	if err != nil {
		log.Fatal(err)
	}
	go worker.BalanceWorkers(ctx, memberlist, workers, cfg.LockExpiry/2)

	// Rest server
	startRestServer(c, cfg.ApiPort)
}

func startEventForwarder(ctx context.Context, connStr, database string, lockPool locks.ConsulLockPool, cfg *Config) []worker.Worker {
	partitionSlots, err := worker.ParseSlots(cfg.Forwarder.PartitionSlots)
	if err != nil {
		log.Fatal(err)
	}
	var partitions uint32
	for _, v := range partitionSlots {
		if v.To > partitions {
			partitions = v.To
		}
	}

	clientID := "forwarder-id-" + uuid.New().String()
	sinker, err := sink.NewNatsSink(cfg.Topic, partitions, "test-cluster", clientID, stan.NatsURL(cfg.NatsURL))
	if err != nil {
		log.Fatalf("Error initialising NATS (%s) Sink '%s' on boot: %v", cfg.NatsURL, clientID, err)
	}

	workers := make([]worker.Worker, len(partitionSlots))
	for i, v := range partitionSlots {
		listener, err := mongodb.NewFeed(connStr, database, mongodb.WithPartitions(partitions, v.From, v.To))
		if err != nil {
			log.Fatalf("Error instantiating store listener: %v", err)
		}

		slots := fmt.Sprintf("%d-%d", v.From, v.To)
		workers[i] = worker.NewRunWorker(
			"forwarder-worker-"+slots,
			lockPool.NewLock("forwarder-lock-"+slots, cfg.LockExpiry),
			store.NewForwarder(
				"forwarder-"+slots,
				listener,
				sinker,
			))
	}

	return workers
}

func startTransactionConsumers(ctx context.Context, lockPool locks.ConsulLockPool, dbURL string, cfg *Config, handler projection.EventHandlerFunc) []worker.Worker {
	streamResumer, err := resumestore.NewMongoDBStreamResumer(dbURL, cfg.EsName, "stream_resume")
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}
	streamName := "accounts-reactor"
	natsSub, err := subscriber.NewNatsSubscriber(ctx, cfg.NatsURL, "test-cluster", streamName+"-"+uuid.New().String(), streamResumer)
	if err != nil {
		log.Fatalf("Error creating NATS subscriber: %s", err)
	}

	workers := make([]worker.Worker, cfg.Partitions)
	var i uint32
	for i = 0; i < cfg.Partitions; i++ {
		x := i
		name := streamName + "-lock-" + strconv.Itoa(int(x))
		workers[x] = worker.NewRunWorker(
			name,
			lockPool.NewLock(name, cfg.LockExpiry),
			worker.NewTask(func(ctx context.Context) (<-chan struct{}, error) {
				done, err := natsSub.StartConsumer(
					ctx,
					projection.StreamResume{
						Topic:  common.TopicWithPartition(cfg.Topic, x+1),
						Stream: streamName,
					},
					handler,
					projection.WithFilter(func(e eventstore.Event) bool {
						return common.In(e.Kind, event.Event_TransactionCreated, event.Event_TransactionFailed)
					}),
				)
				if err != nil {
					return nil, faults.Errorf("Unable to start consumer for %s: %w", name, err)
				}

				return done, nil
			}),
		)
	}

	return workers
}

func startRestServer(c controller.RestController, port int) {
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

	// Start server
	address := fmt.Sprintf(":%d", port)
	e.Logger.Fatal(e.Start(address))
}

func newDB(dbURL string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := options.Client().ApplyURI(dbURL)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, faults.Wrap(err)
	}

	return client, nil
}

func migration(dbURL string, sourceURL string) error {
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
