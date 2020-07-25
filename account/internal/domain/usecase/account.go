package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/eventstore"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNotEnoughFunds = errors.New("Not enough funds")
)

type AccountUsecase struct {
	es eventstore.EventStore
}

func NewAccountUsecase(es eventstore.EventStore) AccountUsecase {
	return AccountUsecase{
		es: es,
	}
}

func (uc AccountUsecase) Create(ctx context.Context, createAccount domain.CreateCommand) (string, error) {
	id := uuid.New().String()
	log.WithFields(log.Fields{
		"method": "AccountUsecase.Create",
	}).Infof("Creating account with owner:%s, id: %s, money: %d", createAccount.Owner, id, createAccount.Money)
	acc := entity.CreateAccount(createAccount.Owner, id, createAccount.Money)
	if err := uc.es.Save(ctx, acc, eventstore.Options{}); err != nil {
		return "", err
	}
	return id, nil
}

func (uc AccountUsecase) Deposit(ctx context.Context, cmd domain.DepositCommand) error {
	acc := entity.NewAccount()
	if err := uc.es.GetByID(ctx, cmd.ID, acc); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"method": "AccountUsecase.Deposit",
	}).Infof("Depositing id: %s, money: %d", cmd.ID, cmd.Money)
	acc.Deposit(cmd.Money)

	if err := uc.es.Save(ctx, acc, eventstore.Options{}); err != nil {
		return err
	}

	return nil
}

func (uc AccountUsecase) Withdraw(ctx context.Context, cmd domain.WithdrawCommand) error {
	acc := entity.NewAccount()
	if err := uc.es.GetByID(ctx, cmd.ID, acc); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"method": "AccountUsecase.Withdraw",
	}).Infof("Depositing id: %s, money: %d", cmd.ID, cmd.Money)
	if acc.Withdraw(cmd.Money) {
		return uc.es.Save(ctx, acc, eventstore.Options{})
	}

	return ErrNotEnoughFunds
}

func (uc AccountUsecase) Transfer(ctx context.Context, cmd domain.TransferCommand) error {
	return errors.New("Not implemented")
}
