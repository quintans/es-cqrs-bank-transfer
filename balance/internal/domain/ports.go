package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/eventid"
	"github.com/quintans/eventsourcing/projection"
)

type Metadata struct {
	EventID     eventid.EventID
	AggregateID uuid.UUID
	ResumeKey   projection.ResumeKey
	ResumeToken projection.Token
}

type LastIDs struct {
	DbEventID   string
	MqEventID   string
	MqMessageID []byte
}

type BalanceService interface {
	GetOne(ctx context.Context, id uuid.UUID) (entity.Balance, error)
	ListAll(ctx context.Context) ([]entity.Balance, error)
}

type BalanceRepository interface {
	GetAllOrderByOwnerAsc(ctx context.Context) ([]entity.Balance, error)
	CreateAccount(ctx context.Context, balance entity.Balance) error
	GetByID(ctx context.Context, aggregateID uuid.UUID) (entity.Balance, error)
	Update(ctx context.Context, balance entity.Balance) error
}

type EventHandler interface {
	Handle(ctx context.Context, e eventsourcing.Event) error
}

const (
	ProjectionBalance string = "ProjectionBalance"
)
