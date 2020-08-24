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
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/eventstore/subscriber"
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

	sub, err := subscriber.NewNatsSubscriber(ctx, cfg.NatsAddress, cfg.Topic, "test-cluster", "my-id")
	if err != nil {
		log.Fatalf("Error creating NATS subscriber: %s", err)
	}
	mess := gateway.Messenger{
		Nats: sub.GetQueue(),
		Stan: sub.GetStream(),
	}

	balanceUC := usecase.BalanceUsecase{
		BalanceRepository: repo,
		Messenger:         mess,
	}

	prjCtrl := controller.ProjectionBalance{
		Subscriber:     sub,
		BalanceUsecase: balanceUC,
	}
	manager := projection.NewBootableManager(
		domain.ProjectionBalance,
		prjCtrl,
		[]string{event.AggregateType_Account},
		esRepo,
		cfg.Topic,
		1, 2,
		gateway.NotificationTopic,
		prjCtrl.Handler,
	)
	refreshLock := projection.NewLooper(manager)
	go refreshLock.Start(ctx)

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
