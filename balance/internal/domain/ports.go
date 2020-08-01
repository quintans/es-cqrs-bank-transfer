package domain

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
)

type Metadata struct {
	EventID     string
	AggregateID string
}

type BalanceUsecase interface {
	ListAll(ctx context.Context) ([]entity.Balance, error)
	AccountCreated(ctx context.Context, metadata Metadata, e event.AccountCreated) error
	MoneyDeposited(ctx context.Context, metadata Metadata, e event.MoneyDeposited) error
	MoneyWithdrawn(ctx context.Context, metadata Metadata, e event.MoneyWithdrawn) error
}

type BalanceRepository interface {
	GetAllOrderByOwnerAsc(ctx context.Context) ([]entity.Balance, error)
	GetEventID(ctx context.Context, aggregateID string) (string, error)
	GetMaxEventID(ctx context.Context) (string, error)
	CreateAccount(ctx context.Context, balance entity.Balance) error
	GetByID(ctx context.Context, aggregateID string) (entity.Balance, error)
	Update(ctx context.Context, balance entity.Balance) error
}

type Messenger interface {
	GetLastMessageID(ctx context.Context, topic string) ([]byte, string, error)
}
