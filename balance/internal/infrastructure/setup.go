package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/go-redsync/redsync"
	"github.com/gomodule/redigo/redis"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore/poller"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiPort          int      `env:"API_PORT" envDefault:"8030"`
	PulsarAddress    string   `env:"PULSAR_ADDRESS" envDefault:"localhost:6650"`
	Topic            string   `env:"TOPIC" envDefault:"accounts"`
	Subscription     string   `env:"SUBSCRIPTION" envDefault:"accounts-subscription"`
	ElasticAddresses []string `env:"ELASTIC_ADDRESSES" envSeparator:","`
	ConfigEs
	ConfigPoller
	ConfigRedis
	LockExpiry time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
}

type ConfigEs struct {
	EsAddress string `env:"ES_ADDRESS"`
}

type ConfigPoller struct {
	PollInterval time.Duration `env:"POLL_INTERVAL" envDefault:"500ms"`
}

type ConfigRedis struct {
	RedisAddresses []string `env:"REDIS_ADDRESSES" envSeparator:","`
}

type RedisPool struct {
	Conn redis.Conn
}

func (rp RedisPool) Get() redis.Conn {
	return rp.Conn
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
	// poller
	esRepo := poller.NewGrpcRepository(cfg.EsAddress)

	latch := locks.NewCountDownLatch()
	latch.Add(1)
	client := StartPulsarClient(ctx, latch, cfg)
	mess := gateway.Messenger{
		Client:        client,
		PulsarAddress: cfg.PulsarAddress,
	}

	balanceUC := usecase.BalanceUsecase{
		BalanceRepository: repo,
		Messenger:         mess,
		Topic:             cfg.Topic,
	}

	registry := controller.NewPulsarRegistry(client)

	// acquire distributed lock
	pool, err := redisPool(cfg.RedisAddresses)
	if err != nil {
		log.Fatal(err)
	}
	lock := redsync.New(pool)

	regPB := registry.RegisterReader(controller.ProjectionBalance{
		Topic:          cfg.Topic,
		EsRepo:         esRepo,
		BalanceUsecase: balanceUC,
	})
	go monitorLock(ctx, regPB, lock, cfg.LockExpiry)

	notif := registry.RegisterSubscription(controller.NotificationController{
		PulsarRegistry: registry,
	})
	err = notif.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

	restCtrl := controller.RestController{
		BalanceUsecase: balanceUC,
	}

	latch.Add(1)
	go StartRestServer(ctx, latch, restCtrl, cfg.ApiPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	latch.WaitWithTimeout(3 * time.Second)
}

func redisPool(addrs []string) ([]redsync.Pool, error) {
	pool := make([]redsync.Pool, len(addrs))
	for k, v := range addrs {
		conn, err := redis.Dial("tcp", v)
		if err != nil {
			return nil, err
		}
		pool[k] = RedisPool{Conn: conn}
	}
	return pool, nil
}

func monitorLock(ctx context.Context, ph *controller.PulsarHandler, lock *redsync.Redsync, expiry time.Duration) {
	latch := Latch{
		Name:   ph.GetName(),
		Expiry: expiry,
		Lock:   lock,
		OnLock: ph.Start,
	}
	go latch.Start(ctx)
}

func StartPulsarClient(
	ctx context.Context,
	latch *locks.CountDownLatch,
	cfg Config,
) pulsar.Client {
	logger := log.WithFields(log.Fields{
		"method": "PulsarListener",
	})

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://" + cfg.PulsarAddress,
	})
	if err != nil {
		log.Fatalf("Could not instantiate Pulsar client: %v", err)
	}

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down PulsarListener")
		client.Close()
		latch.Done()
	}()

	return client
}

func StartRestServer(ctx context.Context, latch *locks.CountDownLatch, c controller.RestController, port int) {
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

type Latch struct {
	Name   string
	Expiry time.Duration
	Lock   *redsync.Redsync
	OnLock func(context.Context) error
}

func (l Latch) Start(ctx context.Context) {
	logger := log.WithField("latch", l.Name)
	halfTime := l.Expiry / 2
	for {
		mu := l.Lock.NewMutex(l.Name, redsync.SetExpiry(l.Expiry), redsync.SetTries(2))
		err := mu.Lock()
		if err == redsync.ErrFailed {
			if ok := waitWithCancel(ctx, halfTime); !ok {
				return
			}
		} else if err == nil {
			// acquired lock
			ctx2, cancel := context.WithCancel(ctx)
			err := l.OnLock(ctx2)
			if err != nil {
				logger.WithError(err).Error("Unexpected error while running OnLock")
			}
			loop := true
			for loop {
				if ok := waitWithCancel(ctx, halfTime); !ok {
					cancel()
					return
				}
				loop, _ = mu.Extend()
			}
			cancel() // happens when where not able to extend the lock
		} else {
			logger.WithError(err).Error("Unexpected error while acquiring lock")
			if ok := waitWithCancel(ctx, halfTime); !ok {
				return
			}
		}
	}
}

func waitWithCancel(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	timer.Stop()
	select {
	// happens when the latch was cancelled
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
