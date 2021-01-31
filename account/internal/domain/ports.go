package domain

import (
	"context"
	"errors"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

type CreateCommand struct {
	Owner string `json:"owner"`
	Money int64  `json:"money"`
}

type DepositCommand struct {
	ID    string `json:"id"`
	Money int64  `json:"money"`
}

type WithdrawCommand struct {
	ID    string `json:"id"`
	Money int64  `json:"money"`
}

type TransferCommand struct {
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
	ErrUnknownAccount = errors.New("Unknown account")
	ErrNotEnoughFunds = errors.New("Not enough funds")
)

type AccountUsecaser interface {
	Create(ctx context.Context, cmd CreateCommand) (string, error)
	Deposit(ctx context.Context, cmd DepositCommand) error
	Withdraw(ctx context.Context, cmd WithdrawCommand) error
	Transfer(ctx context.Context, cmd TransferCommand) error
	Balance(ctx context.Context, id string) (AccountDTO, error)
}

type AccountRepository interface {
	Get(ctx context.Context, id string) (*entity.Account, error)
	Save(ctx context.Context, agg *entity.Account) error
	Exec(ctx context.Context, id string, do func(*entity.Account) (*entity.Account, error)) error
}
