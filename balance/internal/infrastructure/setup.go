package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/elastic/go-elasticsearch/v7"
	controller "github.com/quintans/es-cqrs-bank-transfer/balance/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/usecase"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore/common"
)

type Config struct {
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

	StartPulsarListener(cfg, pulsarCtrl)
}

func StartPulsarListener(cfg *Config, ctrl controller.PulsarController) {
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

	for cm := range channel {
		msg := cm.Message
		p := msg.Payload()
		e := common.Event{}
		json.Unmarshal(p, &e)

		fmt.Printf("Received message  msgId: %v -- content: '%s'\n", msg.ID(), string(msg.Payload()))

		switch e.Kind {
		case "AccountCreated":
			err = ctrl.AccountCreated(context.Background(), e)
		default:
			log.Printf("Unknown event type: %s\n", e.Kind)
		}

		if err == nil {
			consumer.Ack(msg)
		} else {
			consumer.Nack(msg)
		}
	}

}
