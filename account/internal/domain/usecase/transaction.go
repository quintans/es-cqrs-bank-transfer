package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	log "github.com/sirupsen/logrus"
)

type TransactionUsecase struct {
	txRepo  domain.TransactionRepository
	accRepo domain.AccountRepository
}

func NewTransactionUsecase(
	txRepo domain.TransactionRepository,
	accRepo domain.AccountRepository,
) TransactionUsecase {
	return TransactionUsecase{
		txRepo:  txRepo,
		accRepo: accRepo,
	}
}

func (uc TransactionUsecase) Create(ctx context.Context, cmd domain.CreateTransactionCommand) (string, error) {
	id := uuid.New().String()
	log.WithFields(log.Fields{
		"method": "TransactionUsecase.Create",
	}).Infof("Creating transaction %s from: %s, to: %s, money: %d", id, cmd.From, cmd.To, cmd.Money)

	tx := entity.CreateTransaction(id, cmd.From, cmd.To, cmd.Money)
	ok, err := uc.txRepo.CreateIfNew(ctx, tx)
	if !ok || err != nil {
		return "", err
	}
	return id, nil
}

func (uc TransactionUsecase) TransactionCreated(ctx context.Context, e event.TransactionCreated) error {
	logger := log.WithFields(log.Fields{
		"method": "TransactionUsecase.transactionCreated",
	})

	if e.From != "" {
		logger.Infof("Withdrawing from %s, money: %d", e.From, e.Money)
		err := uc.accRepo.Exec(ctx, e.From, func(acc *entity.Account) (*entity.Account, error) {
			err := acc.Withdraw(e.ID, e.Money)
			if err == nil {
				return acc, nil
			}

			// transaction failed
			err = uc.txRepo.Exec(ctx, e.ID, func(t *entity.Transaction) (*entity.Transaction, error) {
				t.WithdrawFailed("From account: " + err.Error())
				return t, nil
			})

			return nil, err
		}, e.ID+"_withdraw")
		if err != nil {
			return err
		}
	}
	if e.To != "" {
		logger.Infof("Depositing from %s, money: %d", e.From, e.Money)
		err := uc.accRepo.Exec(ctx, e.To, func(acc *entity.Account) (*entity.Account, error) {
			err := acc.Deposit(e.ID, e.Money)
			if err == nil {
				return acc, nil
			}

			// transaction failed. Need to rollback withdraw
			err = uc.txRepo.Exec(ctx, e.ID, func(t *entity.Transaction) (*entity.Transaction, error) {
				t.DepositFailed("To account: " + err.Error())
				return t, nil
			})

			return nil, err
		}, e.ID+"_deposit")
		if err != nil {
			return err
		}
	}

	// complete transaction
	return uc.txRepo.Exec(ctx, e.ID, func(t *entity.Transaction) (*entity.Transaction, error) {
		t.Succeeded()
		return t, nil
	})
}

func (uc TransactionUsecase) TransactionFailed(ctx context.Context, aggregateID string, e event.TransactionFailed) error {
	if !e.Rollback {
		return nil
	}

	tx, err := uc.txRepo.Get(ctx, aggregateID)
	if err != nil {
		return err
	}
	err = uc.accRepo.Exec(ctx, tx.From, func(a *entity.Account) (*entity.Account, error) {
		err := a.Deposit(tx.ID, tx.Money)
		return a, err
	}, tx.ID+"/rollback")

	return err
}
