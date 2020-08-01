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
	GrpcAddress string `env:"GRPC_ADDRESS" envDefault:":3000"`
	ConfigEs
	ConfigPulsar
	ConfigPoller
}

type ConfigEs struct {
	EsUser     string `env:"ES_USER" envDefault:"root"`
	EsPassword string `env:"ES_PASSWORD" envDefault:"password"`
	EsHost     string `env:"ES_HOST"`
	EsPort     int    `env:"ES_PORT" envDefault:"5432"`
	EsName     string `env:"ES_NAME" envDefault:"accounts"`
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

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.EsUser, cfg.EsPassword, cfg.EsHost, cfg.EsPort, cfg.EsName)
	repo, err := poller.NewPgRepository(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	lm := poller.New(repo, poller.WithPollInterval(cfg.PollInterval))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Printf("Polling every: %s", cfg.PollInterval)
		lm.Forward(ctx, p)
	}()

	go poller.StartGrpcServer(ctx, cfg.GrpcAddress, repo)

	<-quit
	cancel()
}

const (
	EventIDKey = "EventID"
)

type PulsarSink struct {
	config   ConfigPulsar
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
		config:   cfg,
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
	// 2020-07-26: A separate client is created for reading, otherwise the producer will hang
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://" + p.config.PulsarAddress,
	})
	if err != nil {
		return "", fmt.Errorf("Could not instantiate Pulsar client for reading: %w", err)
	}
	defer client.Close()

	reader, err := client.CreateReader(pulsar.ReaderOptions{
		Topic:                   p.config.Topic,
		StartMessageID:          pulsar.LatestMessageID(),
		StartMessageIDInclusive: true,
	})
	if err != nil {
		return "", fmt.Errorf("Unable to create reader for topic %s: %w", p.config.Topic, err)
	}
	defer reader.Close()

	if reader.HasNext() {
		msg, err := reader.Next(ctx)
		if err != nil {
			return "", fmt.Errorf("Unable to read last message in topic %s: %w", p.config.Topic, err)
		}
		eid := msg.Properties()[EventIDKey]

		log.Printf("Will start polling from last event ID: %s", eid)
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
	_, err = p.producer.Send(ctx, &pulsar.ProducerMessage{
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
