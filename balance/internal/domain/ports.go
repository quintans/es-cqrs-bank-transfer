package domain

import (
	"context"

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

type BalanceUsecase interface {
	ListAll(ctx context.Context) ([]entity.Balance, error)
	Handler(ctx context.Context, e eventstore.Event) error
	RebuildBalance(ctx context.Context, afterEventID string) error
	GetLastEventID(ctx context.Context) (string, error)
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

const (
	ProjectionBalance string = "ProjectionBalance"
)
