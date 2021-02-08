package controller

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore"
	log "github.com/sirupsen/logrus"
)

type Listener struct {
	txUC    domain.TransactionUsecaser
	factory eventstore.Factory
	codec   eventstore.Codec
}

func NewListener(transactionUsecase domain.TransactionUsecaser, factory eventstore.Factory, codec eventstore.Codec) Listener {
	return Listener{
		txUC:    transactionUsecase,
		factory: factory,
		codec:   codec,
	}
}

func (p Listener) Handler(ctx context.Context, e eventstore.Event) error {
	logger := log.WithFields(log.Fields{
		"event": e,
	})

	evt, err := eventstore.RehydrateEvent(p.factory, p.codec, nil, e.Kind, e.Body)
	if err != nil {
		return err
	}

	switch t := evt.(type) {
	case event.TransactionCreated:
		err = p.txUC.TransactionCreated(ctx, t)
	case event.TransactionFailed:
		err = p.txUC.TransactionFailed(ctx, e.AggregateID, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}
