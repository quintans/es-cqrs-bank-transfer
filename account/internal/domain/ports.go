package domain

import (
	"context"
	"errors"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

type CreateAccountCommand struct {
	Owner string `json:"owner"`
}

type DepositCommand struct {
	ID            string `json:"id"`
	Money         int64  `json:"money"`
	TransactionID string `json:"transaction"`
}

type WithdrawCommand struct {
	ID            string `json:"id"`
	Money         int64  `json:"money"`
	TransactionID string `json:"transaction"`
}

type CreateTransactionCommand struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Money int64  `json:"money"`
}

type AccountDTO struct {
	Status  string `json:"status,omitempty"`
	Balance int64  `json:"balance,omitempty"`
	Owner   string `json:"owner,omitempty"`
}

var (
	ErrEntityNotFound = errors.New("entity not found")
)

type AccountUsecaser interface {
	Create(ctx context.Context, cmd CreateAccountCommand) (string, error)
	Balance(ctx context.Context, id string) (AccountDTO, error)
}

type AccountRepository interface {
	Get(ctx context.Context, id string) (*entity.Account, error)
	New(ctx context.Context, agg *entity.Account) error
	Exec(ctx context.Context, id string, do func(*entity.Account) (*entity.Account, error), idempotencyKey string) error
}

type Metadata struct {
	EventID     string
	AggregateID string
}
type TransactionUsecaser interface {
	Create(ctx context.Context, cmd CreateTransactionCommand) (string, error)
	TransactionCreated(ctx context.Context, e event.TransactionCreated) error
	TransactionFailed(ctx context.Context, aggregateID string, e event.TransactionFailed) error
}

type TransactionRepository interface {
	Get(ctx context.Context, id string) (*entity.Transaction, error)
	CreateIfNew(ctx context.Context, agg *entity.Transaction) (bool, error)
	Exec(ctx context.Context, id string, do func(*entity.Transaction) (*entity.Transaction, error)) error
}
