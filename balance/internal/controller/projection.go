package controller

import (
	"context"

	"github.com/apache/pulsar-client-go/pulsar"
	log "github.com/sirupsen/logrus"
)

type PulsarReader interface {
	GetName() string
	GetTopic() string
	Boot(context.Context) (pulsar.MessageID, error)
	Handler(context.Context, pulsar.ReaderMessage) error
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

func (m *PulsarRegistry) Register(reader PulsarReader) *PulsarHandler {
	p := &PulsarHandler{
		name:    reader.GetName(),
		client:  m.client,
		topic:   reader.GetTopic(),
		handler: reader.Handler,
		boot:    reader.Boot,
	}
	m.handlers[reader.GetName()] = p
	return p
}

func (m *PulsarRegistry) Get(name string) *PulsarHandler {
	return m.handlers[name]
}

type PulsarHandler struct {
	name    string
	boot    func(context.Context) (pulsar.MessageID, error)
	handler func(context.Context, pulsar.ReaderMessage) error
	client  pulsar.Client
	topic   string
	cancel  context.CancelFunc
}

func (bp *PulsarHandler) GetName() string {
	return bp.name
}

func (bp *PulsarHandler) Start(ctx context.Context) error {
	ctx, bp.cancel = context.WithCancel(ctx)
	msgID, err := bp.boot(ctx)
	if err != nil {
		return err
	}
	channel := make(chan pulsar.ReaderMessage, 100)
	reader, err := bp.client.CreateReader(pulsar.ReaderOptions{
		Topic:          bp.topic,
		StartMessageID: msgID,
		MessageChannel: channel,
	})
	if err != nil {
		return err
	}

	defer reader.Close()

	go func() {
		<-ctx.Done()
		close(channel)
	}()

	// Listen on the topic for incoming messages
	for rm := range channel {
		err := bp.handler(ctx, rm)
		if err != nil {
			log.WithError(err).Errorf("Failed handling message")
		}
	}

	return nil
}

func (bp *PulsarHandler) Stop() {
	bp.cancel()
}
