package controller

import (
	"context"
	"encoding/json"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/projection"
	log "github.com/sirupsen/logrus"
)

type ProjectionBalance struct {
	projection.Subscriber

	BalanceUsecase domain.BalanceUsecase
}

func (p ProjectionBalance) GetName() string {
	return domain.ProjectionBalance
}

func (p ProjectionBalance) GetResumeEventID(ctx context.Context) (string, error) {
	return p.BalanceUsecase.GetLastEventID(ctx)
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

	var err error
	switch e.Kind {
	case event.Event_AccountCreated:
		err = p.AccountCreated(context.Background(), m, e)
	case event.Event_MoneyDeposited:
		err = p.MoneyDeposited(context.Background(), m, e)
	case event.Event_MoneyWithdrawn:
		err = p.MoneyWithdrawn(context.Background(), m, e)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}

func (p ProjectionBalance) AccountCreated(ctx context.Context, m domain.Metadata, e eventstore.Event) error {
	ac := event.AccountCreated{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.AccountCreated(ctx, m, ac)
}

func (p ProjectionBalance) MoneyDeposited(ctx context.Context, m domain.Metadata, e eventstore.Event) error {
	ac := event.MoneyDeposited{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyDeposited(ctx, m, ac)
}

func (p ProjectionBalance) MoneyWithdrawn(ctx context.Context, m domain.Metadata, e eventstore.Event) error {
	ac := event.MoneyWithdrawn{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyWithdrawn(ctx, m, ac)
}
