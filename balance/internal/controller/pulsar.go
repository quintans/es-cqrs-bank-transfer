package controller

import (
	"context"
	"encoding/json"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	log "github.com/sirupsen/logrus"
)

const (
	NotificationHandlerName  = "NotificationHandler"
	NotificationSubscription = "notifications-balance"
)

type NotificationController struct {
	PulsarRegistry *PulsarRegistry
}

func (p NotificationController) GetName() string {
	return NotificationHandlerName
}

func (p NotificationController) GetTopic() string {
	return gateway.NotificationTopic
}

func (p NotificationController) GetSubscription() string {
	return NotificationSubscription
}

func (p NotificationController) Handler(ctx context.Context, msg pulsar.Message) error {
	b := msg.Payload()
	n := domain.Notification{}
	json.Unmarshal(b, &n)
	reg := p.PulsarRegistry.Get(n.Projection)
	if reg == nil {
		log.WithField("projection", n.Projection).
			Info("Projection not found while handling notification")
		return nil
	}
	if n.Action == domain.Start {
		go reg.Start(ctx)
	} else {
		reg.Stop()
	}
	return nil
}
