package domain

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
)

type BalanceUsecase interface {
	AccountCreated(ctx context.Context, eventID string, e event.AccountCreated) error
}

type BalanceRepository interface {
	GetEventID(ctx context.Context, aggregateID string) (string, error)
	CreateAccount(ctx context.Context, balance entity.Balance) error
	Update(ctx context.Context, balance entity.Balance) error
}
