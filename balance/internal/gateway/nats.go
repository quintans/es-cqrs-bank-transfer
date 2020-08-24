package gateway

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	log "github.com/sirupsen/logrus"
)

const (
	EventIDKey        = "EventID"
	NotificationTopic = "notifications"
)

type Messenger struct {
	Nats *nats.Conn
	Stan stan.Conn
}

func (m Messenger) GetResumeToken(ctx context.Context, topic string) (string, error) {
	ch := make(chan uint64)
	sub, err := m.Stan.Subscribe(topic, func(m *stan.Msg) {
		ch <- m.Sequence
	}, stan.StartWithLastReceived())
	if err != nil {
		return "", err
	}
	defer sub.Close()
	ctx, _ = context.WithTimeout(ctx, 100*time.Millisecond)
	var sequence uint64
	select {
	case sequence = <-ch:
	case <-ctx.Done():
		return "", nil
	}
	return strconv.FormatUint(sequence, 10), nil
}

func (m Messenger) FreezeProjection(ctx context.Context, projectionName string) error {
	log.WithField("projection", projectionName).Info("Freezing projection")
	payload, err := json.Marshal(domain.Notification{
		Projection: projectionName,
		Action:     domain.Freeze,
	})
	if err != nil {
		return err
	}
	_, err = m.Nats.Request(NotificationTopic, payload, 500*time.Millisecond)
	return err
}

func (m Messenger) UnfreezeProjection(ctx context.Context, projectionName string) error {
	log.WithField("projection", projectionName).Info("Unfreezing projection")
	payload, err := json.Marshal(domain.Notification{
		Projection: projectionName,
		Action:     domain.Unfreeze,
	})
	if err != nil {
		return err
	}
	return m.Nats.Publish(NotificationTopic, payload)
}
