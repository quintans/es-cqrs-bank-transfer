package usecase

import (
	"context"
)

type CreateCommand struct {
	Owner string `json:"owner"`
	Money int64  `json:"money"`
}

type DepositCommand struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount"`
}

type WithdrawCommand struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount"`
}

type TransferCommand struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount int64  `json:"amount"`
}

type AccountUsecaser interface {
	Create(ctx context.Context, cmd CreateCommand) (string, error)
	Deposit(ctx context.Context, cmd DepositCommand) error
	Withdraw(ctx context.Context, cmd WithdrawCommand) error
	Transfer(ctx context.Context, cmd TransferCommand) error
}
