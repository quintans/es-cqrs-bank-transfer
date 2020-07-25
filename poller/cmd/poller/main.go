package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/caarlos0/env/v6"
	_ "github.com/lib/pq"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/poller"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ConfigDb
	ConfigPulsar
	ConfigPoller
}

type ConfigDb struct {
	DbUser     string `env:"DB_USER" envDefault:"root"`
	DbPassword string `env:"DB_PASSWORD" envDefault:"password"`
	DbHost     string `env:"DB_HOST"`
	DbPort     int    `env:"DB_PORT" envDefault:"5432"`
	DbName     string `env:"DB_NAME" envDefault:"accounts"`
}

type ConfigPulsar struct {
	PulsarAddress string `env:"PULSAR_ADDRESS"`
	Topic         string `env:"TOPIC" envDefault:"accounts"`
}

type ConfigPoller struct {
	PollInterval time.Duration `env:"POLL_INTERVAL" envDefault:"500ms"`
}

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	p, err := NewPulsarSink(cfg.ConfigPulsar)
	if err != nil {
		log.Fatal(err)
	}

	defer p.Close()

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DbUser, cfg.DbPassword, cfg.DbHost, cfg.DbPort, cfg.DbName)
	tracker, err := poller.NewPgRepository(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	lm := poller.New(tracker, poller.WithPollInterval(cfg.PollInterval))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Printf("Polling every: %s\n", cfg.PollInterval)
		lm.Forward(ctx, p)
	}()

	<-quit
	cancel()

}

const (
	EventIDKey = "EventID"
)

type PulsarSink struct {
	topic    string
	client   pulsar.Client
	producer pulsar.Producer
}

// NewPulsarSink instantiate PulsarSink
func NewPulsarSink(cfg ConfigPulsar) (*PulsarSink, error) {
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://" + cfg.PulsarAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate Pulsar client: %w", err)
	}

	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: cfg.Topic,
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("Could not instantiate Pulsar producer: %w", err)
	}

	return &PulsarSink{
		topic:    cfg.Topic,
		client:   client,
		producer: producer,
	}, nil
}

// Close releases resources blocking until
func (p *PulsarSink) Close() {
	if p.producer != nil {
		p.producer.Close()
	}
	if p.client != nil {
		p.client.Close()
	}
}

// LastEventID gets the last event sent to pulsar
func (p *PulsarSink) LastEventID(ctx context.Context) (string, error) {
	reader, err := p.client.CreateReader(pulsar.ReaderOptions{
		Topic:                   p.topic,
		StartMessageID:          pulsar.LatestMessageID(),
		StartMessageIDInclusive: true,
	})
	if err != nil {
		return "", fmt.Errorf("Unable to create reader for topic %s: %w", p.topic, err)
	}
	defer reader.Close()

	if reader.HasNext() {
		msg, err := reader.Next(ctx)
		if err != nil {
			return "", fmt.Errorf("Unable to read last message in topic %s: %w", p.topic, err)
		}
		eid := msg.Properties()[EventIDKey]

		log.Printf("Will start polling from last event ID: %s\n", eid)
		return eid, nil
	}

	log.Println("Will start polling from the begginning")
	return "", nil
}

// Send sends the event to pulsar
func (p *PulsarSink) Send(ctx context.Context, e common.Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return nil
	}

	log.WithFields(log.Fields{
		"method":     "PulsarSink.Send",
		"EventIDKey": EventIDKey,
	}).Infof("Sending message to pulsar: %s", string(b))
	_, err = p.producer.Send(context.Background(), &pulsar.ProducerMessage{
		Payload: b,
		Properties: map[string]string{
			EventIDKey: e.ID,
		},
		Key:       e.AggregateID,
		EventTime: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("Failed to send message: %w", err)
	}
	return nil
}
