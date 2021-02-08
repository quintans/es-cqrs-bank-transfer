package domain

import (
	"context"
	"time"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventstore"
)

type Metadata struct {
	EventID     string
	AggregateID string
}

type LastIDs struct {
	DbEventID   string
	MqEventID   string
	MqMessageID []byte
}

type ProjectionUsecase interface {
	AccountCreated(ctx context.Context, m Metadata, ac event.AccountCreated) error
	MoneyDeposited(ctx context.Context, m Metadata, ac event.MoneyDeposited) error
	MoneyWithdrawn(ctx context.Context, m Metadata, ac event.MoneyWithdrawn) error
	GetLastEventID(ctx context.Context) (string, error)
}

type BalanceUsecase interface {
	ListAll(ctx context.Context) ([]entity.Balance, error)
	RebuildBalance(ctx context.Context, after time.Time) error
}

type BalanceRepository interface {
	GetAllOrderByOwnerAsc(ctx context.Context) ([]entity.Balance, error)
	GetEventID(ctx context.Context, aggregateID string) (string, error)
	GetMaxEventID(ctx context.Context) (string, error)
	CreateAccount(ctx context.Context, balance entity.Balance) error
	GetByID(ctx context.Context, aggregateID string) (entity.Balance, error)
	Update(ctx context.Context, balance entity.Balance) error
	ClearAllData(ctx context.Context) error
}

type EventHandler interface {
	Handle(ctx context.Context, e eventstore.Event) error
}

const (
	ProjectionBalance string = "ProjectionBalance"
)
