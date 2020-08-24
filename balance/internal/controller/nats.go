package controller

import (
	"encoding/json"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore/projection"
	log "github.com/sirupsen/logrus"
)

const (
	NotificationHandlerName  = "NotificationHandler"
	NotificationSubscription = "notifications-balance"
)

type NotificationController struct {
	Registry map[string]*projection.BootableManager
	mu       sync.RWMutex
	Client   *nats.Conn
}

func (p NotificationController) Handler(msg *nats.Msg) {
	n := domain.Notification{}
	err := json.Unmarshal(msg.Data, &n)
	if err != nil {
		log.Errorf("Unable to unmarshal %v", err)
		return
	}
	p.mu.RLock()
	reg := p.Registry[n.Projection]
	p.mu.RUnlock()
	if reg == nil {
		log.WithField("projection", n.Projection).
			Info("Projection not found while handling notification")
		return
	}

	switch n.Action {
	case domain.Freeze:
		if reg.IsLocked() {
			reg.Freeze()
			err := msg.Respond([]byte("..."))
			if err != nil {
				log.WithError(err).Error("Unable to respond to notification")
			}
		}
	case domain.Unfreeze:
		reg.Unfreeze()
	default:
		log.WithField("notification", n).Error("Unknown notification")
	}
}
