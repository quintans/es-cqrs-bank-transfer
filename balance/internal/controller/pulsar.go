package controller

import (
	"context"
	"encoding/json"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore/common"
)

type PulsarController struct {
	BalanceUsecase domain.BalanceUsecase
}

func (p PulsarController) AccountCreated(ctx context.Context, e common.Event) error {
	m := domain.Metadata{AggregateID: e.AggregateID, EventID: e.ID}
	ac := event.AccountCreated{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.AccountCreated(ctx, m, ac)
}

func (p PulsarController) MoneyDeposited(ctx context.Context, e common.Event) error {
	m := domain.Metadata{AggregateID: e.AggregateID, EventID: e.ID}
	ac := event.MoneyDeposited{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyDeposited(ctx, m, ac)
}

func (p PulsarController) MoneyWithdrawn(ctx context.Context, e common.Event) error {
	m := domain.Metadata{AggregateID: e.AggregateID, EventID: e.ID}
	ac := event.MoneyWithdrawn{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyWithdrawn(ctx, m, ac)
}
