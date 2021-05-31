package controller

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/common"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

type Listener struct {
	logger  log.Logger
	txUC    domain.TransactionUsecaser
	factory eventsourcing.Factory
	codec   eventsourcing.Codec
}

func NewListener(logger log.Logger, transactionUsecase domain.TransactionUsecaser, factory eventsourcing.Factory, codec eventsourcing.Codec) Listener {
	return Listener{
		logger:  logger,
		txUC:    transactionUsecase,
		factory: factory,
		codec:   codec,
	}
}

func (p Listener) Handler(ctx context.Context, e eventsourcing.Event) error {
	if !common.In(e.Kind.String(), event.Event_TransactionCreated, event.Event_TransactionFailed) {
		return nil
	}

	logger := p.logger.WithTags(log.Tags{
		"event": e,
	})

	evt, err := eventsourcing.RehydrateEvent(p.factory, p.codec, nil, e.Kind, e.Body)
	if err != nil {
		return err
	}

	switch t := evt.(type) {
	case event.TransactionCreated:
		err = p.txUC.TransactionCreated(ctx, t)
	case event.TransactionFailed:
		var aggID uuid.UUID
		aggID, err = uuid.Parse(e.AggregateID)
		if err != nil {
			return faults.Errorf("unable to parse aggregate ID: %w", err)
		}
		err = p.txUC.TransactionFailed(ctx, aggID, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}
