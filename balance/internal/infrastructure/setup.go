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
	ApiPort          int      `env:"API_PORT" envDefault:"3000"`
	PulsarAddress    string   `env:"PULSAR_ADDRESS" envDefault:"localhost:6650"`
	Topic            string   `env:"TOPIC" envDefault:"accounts"`
	Subscription     string   `env:"SUBSCRIPTION" envDefault:"accounts-subscription"`
	ElasticAddresses []string `env:"ES_ADDRESSES" envSeparator:","`
	ConfigEs
	ConfigPoller
	ConfigRedis
	LockExpiry time.Duration `env:"LOCK_EXPIRY" envDefault:"20s"`
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

	balanceUC := usecase.BalanceUsecase{
		BalanceRepository: repo,
	}

	pulsarCtrl := controller.PulsarController{
		BalanceUsecase: balanceUC,
	}

	restCtrl := controller.RestController{
		BalanceUsecase: balanceUC,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// poller
	esRepo := poller.NewGrpcRepository(cfg.EsAddress)

	mess := gateway.Messenger{
		PulsarAddress: cfg.PulsarAddress,
	}

	latch := locks.NewCountDownLatch()
	latch.Add(2)
	go StartRestServer(ctx, latch, restCtrl, cfg.ApiPort)
	registry := StartPulsarListener(ctx, latch, cfg, pulsarCtrl)

	projectionBalance := controller.ProjectionBalance{
		Topic:             cfg.Topic,
		Ctrl:              pulsarCtrl,
		BalanceRepository: repo,
		EsRepo:            esRepo,
		Messenger:         mess,
	}
	regPB := registry.Register(projectionBalance)

	// acquire distributed lock
	pool, err := redisPool(cfg.RedisAddresses)
	if err != nil {
		log.Fatal(err)
	}
	lock := redsync.New(pool)

	go monitor(ctx, regPB, lock, cfg.LockExpiry)

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

func monitor(ctx context.Context, ph *controller.PulsarHandler, lock *redsync.Redsync, expiry time.Duration) {
	latch := Latch{
		Name:   ph.GetName(),
		Expiry: expiry,
		Lock:   lock,
		OnLock: ph.Start,
	}
	go latch.Start(ctx)
}

func StartPulsarListener(
	ctx context.Context,
	latch *locks.CountDownLatch,
	cfg Config,
	ctrl controller.PulsarController,
) *controller.PulsarRegistry {
	logger := log.WithFields(log.Fields{
		"method": "PulsarListener",
	})

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://" + cfg.PulsarAddress,
	})
	if err != nil {
		log.Fatalf("Could not instantiate Pulsar client: %v", err)
	}

	pr := controller.NewPulsarRegistry(client)

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down PulsarListener")
		client.Close()
		latch.Done()
	}()

	return pr
}

func StartRestServer(ctx context.Context, latch *locks.CountDownLatch, c controller.RestController, port int) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", c.ListAll)

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
	ticker := time.NewTicker(l.Expiry / 2)
	defer ticker.Stop()
	for {
		mu := l.Lock.NewMutex(l.Name, redsync.SetExpiry(l.Expiry), redsync.SetTries(2))
		err := mu.Lock()
		if err == nil {
			ctx2, cancel := context.WithCancel(ctx)
			l.OnLock(ctx2)
			ok := true
			for ok {
				select {
				// happens when the latch was cancelled
				case <-ctx.Done():
					cancel()
					return
				case <-ticker.C:
					ok, _ = mu.Extend()
				}
			}
			cancel() // happens when where not able to extend the lock
		} else {
			select {
			// happens when the latch was cancelled
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}
