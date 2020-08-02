package controller

import (
	"context"
	"sync"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	log "github.com/sirupsen/logrus"
)

type PulsarKind int

const (
	Reader PulsarKind = iota + 1
	Consumer
)

type PulsarReader interface {
	GetName() string
	GetTopic() string
	Boot(context.Context) (pulsar.MessageID, error)
	Handler(context.Context, pulsar.Message) error
}

type PulsarConsumer interface {
	GetName() string
	GetTopic() string
	GetSubscription() string
	Handler(context.Context, pulsar.Message) error
}

type PulsarRegistry struct {
	client   pulsar.Client
	handlers map[string]*PulsarHandler
}

func NewPulsarRegistry(client pulsar.Client) *PulsarRegistry {
	return &PulsarRegistry{
		client:   client,
		handlers: map[string]*PulsarHandler{},
	}
}

func (m *PulsarRegistry) RegisterReader(reader PulsarReader) *PulsarHandler {
	p := &PulsarHandler{
		name:    reader.GetName(),
		client:  m.client,
		topic:   reader.GetTopic(),
		handler: reader.Handler,
		boot:    reader.Boot,
		kind:    Reader,
	}
	m.handlers[reader.GetName()] = p
	return p
}

func (m *PulsarRegistry) RegisterSubscription(s PulsarConsumer) *PulsarHandler {
	p := &PulsarHandler{
		name:         s.GetName(),
		client:       m.client,
		topic:        s.GetTopic(),
		subscription: s.GetSubscription(),
		handler:      s.Handler,
		kind:         Consumer,
	}
	m.handlers[s.GetName()] = p
	return p
}

func (m *PulsarRegistry) Get(name string) *PulsarHandler {
	return m.handlers[name]
}

type PulsarHandler struct {
	name         string
	boot         func(context.Context) (pulsar.MessageID, error)
	handler      func(context.Context, pulsar.Message) error
	client       pulsar.Client
	topic        string
	subscription string
	cancel       context.CancelFunc
	kind         PulsarKind
	mu           sync.Mutex
}

func (bp *PulsarHandler) GetName() string {
	return bp.name
}

// Start starts a pulsar listener.
// It will cancel previous running goroutine.
func (bp *PulsarHandler) Start(ctx context.Context) error {
	bp.mu.Lock()
	if bp.cancel != nil {
		bp.cancel()
	}
	ctx, bp.cancel = context.WithCancel(ctx)
	bp.mu.Unlock()

	if bp.kind == Reader {
		return bp.startReader(ctx)
	}
	return bp.startConsumer(ctx)
}

func (bp *PulsarHandler) startReader(ctx context.Context) error {
	// channel := make(chan pulsar.ReaderMessage)
	msgID, err := bp.boot(ctx)
	if err != nil {
		return err
	}
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://localhost:6650",
	})
	reader, err := client.CreateReader(pulsar.ReaderOptions{
		Topic:          bp.topic,
		StartMessageID: msgID,
		// MessageChannel: channel,
	})
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		reader.Close()
		// close(channel)
	}()

	// Listen on the topic for incoming messages
	go func() {
		logger := log.WithFields(log.Fields{
			"listener": bp.name,
			"topic":    reader.Topic(),
		})
		logger.Info("Listening...")
		// for rm := range channel {
		// 	err := bp.handler(ctx, rm.Message)
		// 	if err != nil {
		// 		logger.WithError(err).Errorf("Failed handling message")
		// 	}
		// }
		for {
			m, err := reader.Next(ctx)
			if err != nil {
				break
			}
			err = bp.handler(ctx, m)
			if err != nil {
				logger.WithError(err).Errorf("Failed handling message")
			}
		}
		logger.Info("Stop listening...")
	}()

	return nil
}

func (bp *PulsarHandler) startConsumer(ctx context.Context) error {
	channel := make(chan pulsar.ConsumerMessage)
	consumer, err := bp.client.Subscribe(pulsar.ConsumerOptions{
		Topic:            gateway.NotificationTopic,
		SubscriptionName: bp.subscription,
		Type:             pulsar.Shared,
		MessageChannel:   channel,
	})
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		close(channel)
	}()

	// Listen on the topic for incoming messages
	go func() {
		defer consumer.Close()
		logger := log.WithFields(log.Fields{
			"listener":     bp.name,
			"topic":        gateway.NotificationTopic,
			"subscription": bp.subscription,
		})
		logger.Info("Listening...")
		for rm := range channel {
			err := bp.handler(ctx, rm.Message)
			if err != nil {
				logger.WithError(err).Errorf("Failed handling message")
			}
			if err == nil {
				consumer.Ack(rm)
			} else {
				consumer.Nack(rm)
			}
		}
		logger.Info("Stop listening...")

	}()

	return nil
}

// Stop stops the listener
func (bp *PulsarHandler) Stop() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if bp.cancel != nil {
		bp.cancel()
		bp.cancel = nil
	}
}
