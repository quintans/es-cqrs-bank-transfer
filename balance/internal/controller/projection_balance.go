package controller

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore"
	log "github.com/sirupsen/logrus"
)

type ProjectionBalance struct {
	BalanceUsecase domain.BalanceUsecase
	factory        eventstore.Factory
	codec          eventstore.Codec
	upcaster       eventstore.Upcaster
}

func NewProjectionBalance(balanceUsecase domain.BalanceUsecase, factory eventstore.Factory, codec eventstore.Codec, upcaster eventstore.Upcaster) ProjectionBalance {
	return ProjectionBalance{
		BalanceUsecase: balanceUsecase,
		factory:        factory,
		codec:          codec,
		upcaster:       upcaster,
	}
}

func (p ProjectionBalance) GetName() string {
	return domain.ProjectionBalance
}

func (p ProjectionBalance) GetResumeEventIDs(ctx context.Context) (map[string]string, error) {
	lastEventID, err := p.BalanceUsecase.GetLastEventID(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		event.AggregateType_Account: lastEventID,
	}, nil
}

func (p ProjectionBalance) Handler(ctx context.Context, e eventstore.Event) error {
	logger := log.WithFields(log.Fields{
		"projection": domain.ProjectionBalance,
		"event":      e,
	})
	logger.Info("routing event")

	m := domain.Metadata{
		AggregateID: e.AggregateID,
		EventID:     e.ID,
	}

	evt, err := eventstore.Decode(p.factory, p.codec, p.upcaster, e.Kind, e.Body)
	if err != nil {
		return err
	}

	switch t := evt.(type) {
	case event.AccountCreated:
		err = p.BalanceUsecase.AccountCreated(ctx, m, t)
	case event.MoneyDeposited:
		err = p.BalanceUsecase.MoneyDeposited(ctx, m, t)
	case event.MoneyWithdrawn:
		err = p.BalanceUsecase.MoneyWithdrawn(ctx, m, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}
