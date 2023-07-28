package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventsourcing/projection"
)

var ErrEntityNotFound = errors.New("entity not found")

type AccountService interface {
	Create(ctx context.Context, cmd CreateAccountCommand) (uuid.UUID, error)
	Balance(ctx context.Context, id uuid.UUID) (AccountDTO, error)
}

type CreateAccountCommand struct {
	Owner string `json:"owner"`
}

type AccountDTO struct {
	Status  string `json:"status,omitempty"`
	Balance int64  `json:"balance,omitempty"`
	Owner   string `json:"owner,omitempty"`
}

type AccountRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*entity.Account, error)
	New(ctx context.Context, agg *entity.Account) error
	Exec(ctx context.Context, id uuid.UUID, do func(*entity.Account) (*entity.Account, error), idempotencyKey string) error
}

type TransactionService interface {
	Create(ctx context.Context, cmd CreateTransactionCommand) (uuid.UUID, error)
	TransactionCreated(ctx context.Context, resumeKey projection.ResumeKey, resumeToken projection.Token, e event.TransactionCreated) error
	TransactionFailed(ctx context.Context, resumeKey projection.ResumeKey, resumeToken projection.Token, aggregateID uuid.UUID, e event.TransactionFailed) error
}

type CreateTransactionCommand struct {
	From  uuid.UUID `json:"from"`
	To    uuid.UUID `json:"to"`
	Money int64     `json:"money"`
}

type TransactionRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*entity.Transaction, error)
	CreateIfNew(ctx context.Context, agg *entity.Transaction) (bool, error)
	Exec(ctx context.Context, id uuid.UUID, do func(*entity.Transaction) (*entity.Transaction, error), idempotencyKey string) error
}
