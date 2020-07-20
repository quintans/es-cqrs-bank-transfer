package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/eventstore"
)

type AccountUsecase struct {
	es eventstore.EventStore
}

func NewAccountUsecase(es eventstore.EventStore) AccountUsecase {
	return AccountUsecase{
		es: es,
	}
}

func (uc AccountUsecase) Create(ctx context.Context, createAccount CreateCommand) (string, error) {
	id := uuid.New().String()
	acc := entity.CreateAccount(createAccount.Owner, id, createAccount.Money)
	if err := uc.es.Save(ctx, acc, eventstore.Options{}); err != nil {
		return "", err
	}
	return id, nil
}

func (uc AccountUsecase) Deposit(ctx context.Context, cmd DepositCommand) error {
	acc := entity.NewAccount()
	if err := uc.es.GetByID(ctx, cmd.ID, acc); err != nil {
		return err
	}

	acc.Deposit(cmd.Amount)

	if err := uc.es.Save(ctx, acc, eventstore.Options{}); err != nil {
		return err
	}

	return nil
}

func (uc AccountUsecase) Withdraw(ctx context.Context, cmd WithdrawCommand) error {
	acc := entity.NewAccount()
	if err := uc.es.GetByID(ctx, cmd.ID, acc); err != nil {
		return err
	}

	acc.Withdraw(cmd.Amount)

	if err := uc.es.Save(ctx, acc, eventstore.Options{}); err != nil {
		return err
	}

	return nil
}

func (uc AccountUsecase) Transfer(ctx context.Context, cmd TransferCommand) error {
	return errors.New("Not implemented")
}
