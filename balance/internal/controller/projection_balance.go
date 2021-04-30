package controller

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventsourcing"
	log "github.com/sirupsen/logrus"
)

type ProjectionBalance struct {
	projectionUC domain.ProjectionUsecase
	factory      eventsourcing.Factory
	codec        eventsourcing.Codec
	upcaster     eventsourcing.Upcaster
}

func NewProjectionBalance(
	projectionUC domain.ProjectionUsecase,
	factory eventsourcing.Factory,
	codec eventsourcing.Codec,
	upcaster eventsourcing.Upcaster,
) ProjectionBalance {
	return ProjectionBalance{
		projectionUC: projectionUC,
		factory:      factory,
		codec:        codec,
		upcaster:     upcaster,
	}
}

func (p ProjectionBalance) GetName() string {
	return domain.ProjectionBalance
}

func (p ProjectionBalance) GetResumeEventIDs(ctx context.Context, aggregateTypes []string) (string, error) {
	lastEventID, err := p.projectionUC.GetLastEventID(ctx)
	if err != nil {
		return "", err
	}
	return lastEventID, nil
}

func (p ProjectionBalance) Handle(ctx context.Context, e eventsourcing.Event) error {
	logger := log.WithFields(log.Fields{
		"projection": domain.ProjectionBalance,
		"event":      e,
	})

	m := domain.Metadata{
		AggregateID: e.AggregateID,
		EventID:     e.ID,
	}

	evt, err := eventsourcing.RehydrateEvent(p.factory, p.codec, p.upcaster, e.Kind, e.Body)
	if err != nil {
		return err
	}

	switch t := evt.(type) {
	case event.AccountCreated:
		err = p.projectionUC.AccountCreated(ctx, m, t)
	case event.MoneyDeposited:
		err = p.projectionUC.MoneyDeposited(ctx, m, t)
	case event.MoneyWithdrawn:
		err = p.projectionUC.MoneyWithdrawn(ctx, m, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}
