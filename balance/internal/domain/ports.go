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
	GetLastIDs(ctx context.Context) (LastIDs, error)
	RebuildBalance(ctx context.Context) error
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
	Start Action = iota + 1
	Stop
)

const (
	ProjectionBalance string = "ProjectionBalance"
)

type Notification struct {
	Projection string `json:"projection"`
	Action
}

type Messenger interface {
	// GetLastMessageID returns the serialized message ID and the event ID, both from the MQ
	GetLastMessageID(ctx context.Context, topic string) ([]byte, string, error)
	// NotifyProjectionRegistry signals the registry to stop or start a listener
	NotifyProjectionRegistry(ctx context.Context, notification Notification) error
}
