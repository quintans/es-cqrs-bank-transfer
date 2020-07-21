package main

import (
	"fmt"
	"log"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/caarlos0/env/v6"
)

type Config struct {
	PulsarAddress string `env:"PULSAR_ADDRESS" envDefault:"localhost:6650"`
	Topic         string `env:"TOPIC" envDefault:"accounts"`
	Subscription  string `env:"SUBSCRIPTION" envDefault:"accounts-subscription"`
}

func main() {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

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
		fmt.Printf("Received message  msgId: %v -- content: '%s'\n",
			msg.ID(), string(msg.Payload()))

		consumer.Ack(msg)
	}
}
