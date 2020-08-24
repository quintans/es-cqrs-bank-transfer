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
	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiPort          int      `env:"API_PORT" envDefault:"8030"`
	ElasticAddresses []string `env:"ELASTIC_ADDRESSES" envSeparator:","`
	ConfigEs
	ConfigRedis
	ConfigNats
}

type ConfigEs struct {
	EsAddress   string `env:"ES_ADDRESS"`
	EsBatchSize int    `env:"ES_BATCH_SIZE" envDefault:"20"`
}

type ConfigRedis struct {
	RedisAddresses []string      `env:"REDIS_ADDRESSES" envSeparator:","`
	LockExpiry     time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
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
	log.Println(res)

	repo := gateway.BalanceRepository{
		Client: es,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// es player
	esRepo := player.NewGrpcRepository(cfg.EsAddress)

	latch := locks.NewCountDownLatch()
	latch.Add(1)
	mq, streamer := startMQ(ctx, latch, cfg)
	mess := gateway.Messenger{
		Nats: mq,
		Stan: streamer,
	}

	balanceUC := usecase.BalanceUsecase{
		BalanceRepository: repo,
		Messenger:         mess,
	}

	registry := map[string]*projection.BootableManager{}
	registry[domain.ProjectionBalance] = projection.NewBootableManager(func() projection.Bootable {
		return controller.ProjectionBalance{
			BalanceUsecase: balanceUC,
			EsRepo:         esRepo,
			EsBatchSize:    cfg.EsBatchSize,
			Topic:          cfg.Topic,
			Messenger:      mess,
			Stan:           streamer,
		}
	})
	for _, reg := range registry {
		refreshLock := projection.NewLooper(reg)
		go refreshLock.Start(ctx)
	}

	natsCtrl := controller.NotificationController{
		Registry: registry,
	}
	mq.Subscribe(gateway.NotificationTopic, natsCtrl.Handler)

	restCtrl := controller.RestController{
		BalanceUsecase: balanceUC,
	}

	latch.Add(1)
	go startRestServer(ctx, latch, restCtrl, cfg.ApiPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	latch.WaitWithTimeout(3 * time.Second)
}

func startMQ(
	ctx context.Context,
	latch *locks.CountDownLatch,
	cfg Config,
) (*nats.Conn, stan.Conn) {
	logger := log.WithFields(log.Fields{
		"method": "MQListener",
	})

	nc, err := nats.Connect("nats://" + cfg.NatsAddress)
	if err != nil {
		log.Fatalf("Could not instantiate NATS client: %v", err)
	}

	streamer, err := stan.Connect("test-cluster", "my-id", stan.NatsURL(cfg.NatsAddress))
	if err != nil {
		log.Fatalf("Could not instantiate NATS stream connection: %v", err)
	}

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down MQ")
		nc.Close()
		streamer.Close()
		latch.Done()
	}()

	return nc, streamer
}

func startRestServer(ctx context.Context, latch *locks.CountDownLatch, c controller.RestController, port int) {
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
	latch.Done()
}
