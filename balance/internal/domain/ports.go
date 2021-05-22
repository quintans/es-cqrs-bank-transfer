package domain

import (
	"context"
	"time"

	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventsourcing"
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
	RebuildBalance(ctx context.Context, after time.Time) func(ctx context.Context) (string, error)
	RebuildWrapUp(ctx context.Context, afterEventID string) (string, error)
}

type BalanceUsecase interface {
	GetOne(ctx context.Context, id string) (entity.Balance, error)
	ListAll(ctx context.Context) ([]entity.Balance, error)
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
	Handle(ctx context.Context, e eventsourcing.Event) error
}

const (
	ProjectionBalance string = "ProjectionBalance"
)
