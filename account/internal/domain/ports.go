package domain

import (
	"context"
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

type AccountUsecaser interface {
	Create(ctx context.Context, cmd CreateCommand) (string, error)
	Deposit(ctx context.Context, cmd DepositCommand) error
	Withdraw(ctx context.Context, cmd WithdrawCommand) error
	Transfer(ctx context.Context, cmd TransferCommand) error
	Balance(ctx context.Context, id string) (AccountDTO, error)
}
