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

type LastIDs struct {
	DbEventID   string
	MqEventID   string
	MqMessageID []byte
}

type BalanceUsecase interface {
	ListAll(ctx context.Context) ([]entity.Balance, error)
	AccountCreated(ctx context.Context, metadata Metadata, e event.AccountCreated) error
	MoneyDeposited(ctx context.Context, metadata Metadata, e event.MoneyDeposited) error
	MoneyWithdrawn(ctx context.Context, metadata Metadata, e event.MoneyWithdrawn) error
	RebuildBalance(ctx context.Context) error
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

type Action int

const (
	Freeze Action = iota + 1
	Unfreeze
)

const (
	ProjectionBalance string = "ProjectionBalance"
)

type Notification struct {
	Projection string `json:"projection"`
	Action
}

type Messenger interface {
	// GetResumeToken returns the resume token for a specific topic
	GetResumeToken(ctx context.Context, topic string) (string, error)
	// FreezeProjection signals the registry to STOP processing the projection
	FreezeProjection(ctx context.Context, projectionName string) error
	// UnfreezeProjection signals the registry to START processing the projection
	UnfreezeProjection(ctx context.Context, projectionName string) error
}
