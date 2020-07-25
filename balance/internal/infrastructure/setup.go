package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore/common"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiPort          int      `env:"API_PORT" envDefault:"3000"`
	PulsarAddress    string   `env:"PULSAR_ADDRESS" envDefault:"localhost:6650"`
	Topic            string   `env:"TOPIC" envDefault:"accounts"`
	Subscription     string   `env:"SUBSCRIPTION" envDefault:"accounts-subscription"`
	ElasticAddresses []string `env:"ES_ADDRESSES" envSeparator:","`
}

func Setup(cfg *Config) {
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go StartPulsarListener(ctx, wg, cfg, pulsarCtrl)
	go StartRestServer(ctx, wg, restCtrl, cfg.ApiPort)

	<-quit
	cancel()
	Wait(wg, 3*time.Second)
}

func Wait(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func StartPulsarListener(ctx context.Context, wg *sync.WaitGroup, cfg *Config, ctrl controller.PulsarController) {
	logger := log.WithFields(log.Fields{
		"method": "PulsarListener",
	})

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://" + cfg.PulsarAddress,
	})
	if err != nil {
		log.Fatalf("Could not instantiate Pulsar client: %v", err)
	}

	defer client.Close()

	channel := make(chan pulsar.ConsumerMessage, 100)

	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            cfg.Topic,
		SubscriptionName: cfg.Subscription,
		Type:             pulsar.KeyShared,
		MessageChannel:   channel,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	go func() {
		<-ctx.Done()
		close(channel)
	}()

	for cm := range channel {
		msg := cm.Message
		p := msg.Payload()
		e := common.Event{}
		json.Unmarshal(p, &e)

		logger.Infof("Received message: %s\n", string(msg.Payload()))

		switch e.Kind {
		case event.Event_AccountCreated:
			err = ctrl.AccountCreated(context.Background(), e)
		case event.Event_MoneyDeposited:
			err = ctrl.MoneyDeposited(context.Background(), e)
		case event.Event_MoneyWithdrawn:
			err = ctrl.MoneyWithdrawn(context.Background(), e)
		default:
			log.Printf("Unknown event type: %s\n", e.Kind)
		}

		if err == nil {
			consumer.Ack(msg)
		} else {
			logger.WithError(err).Errorf("Failed handling message")
			consumer.Nack(msg)
		}
	}
	log.Info("Shutting down PulsarListener")
	wg.Done()
}

func StartRestServer(ctx context.Context, wg *sync.WaitGroup, c controller.RestController, port int) {
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
	wg.Done()
}
