package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/eventid"
)

type Metadata struct {
	EventID     eventid.EventID
	AggregateID uuid.UUID
}

type LastIDs struct {
	DbEventID   string
	MqEventID   string
	MqMessageID []byte
}

type ProjectionUsecase interface {
	CatchUp(ctx context.Context) (eventid.EventID, error)
	AfterCatchUp(ctx context.Context, afterEventID eventid.EventID) (eventid.EventID, error)
}

type BalanceUsecase interface {
	GetOne(ctx context.Context, id uuid.UUID) (entity.Balance, error)
	ListAll(ctx context.Context) ([]entity.Balance, error)
}

type BalanceRepository interface {
	GetAllOrderByOwnerAsc(ctx context.Context) ([]entity.Balance, error)
	GetEventID(ctx context.Context, aggregateID uuid.UUID) (eventid.EventID, error)
	GetMaxEventID(ctx context.Context) (eventid.EventID, error)
	CreateAccount(ctx context.Context, balance entity.Balance) error
	GetByID(ctx context.Context, aggregateID uuid.UUID) (entity.Balance, error)
	Update(ctx context.Context, balance entity.Balance) error
	ClearAllData(ctx context.Context) error
}

type EventHandler interface {
	Handle(ctx context.Context, e eventsourcing.Event) error
}

const (
	ProjectionBalance string = "ProjectionBalance"
)
