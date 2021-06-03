package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing/log"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

type TransactionUsecase struct {
	logger  log.Logger
	txRepo  domain.TransactionRepository
	accRepo domain.AccountRepository
}

func NewTransactionUsecase(
	logger log.Logger,
	txRepo domain.TransactionRepository,
	accRepo domain.AccountRepository,
) TransactionUsecase {
	return TransactionUsecase{
		logger:  logger,
		txRepo:  txRepo,
		accRepo: accRepo,
	}
}

func (uc TransactionUsecase) Create(ctx context.Context, cmd domain.CreateTransactionCommand) (uuid.UUID, error) {
	id := uuid.New()
	uc.logger.WithTags(log.Tags{
		"method": "TransactionUsecase.Create",
	}).Infof("Creating transaction %s from: %s, to: %s, money: %d", id, cmd.From, cmd.To, cmd.Money)

	tx := entity.CreateTransaction(id, cmd.From, cmd.To, cmd.Money)
	ok, err := uc.txRepo.CreateIfNew(ctx, tx)
	if !ok || err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (uc TransactionUsecase) TransactionCreated(ctx context.Context, e event.TransactionCreated) error {
	logger := uc.logger.WithTags(log.Tags{
		"method": "TransactionUsecase.transactionCreated",
	})

	if e.From != uuid.Nil {
		var failed bool
		logger.Infof("Withdrawing from %s, money: %d", e.From, e.Money)
		err := uc.accRepo.Exec(ctx, e.From, func(acc *entity.Account) (*entity.Account, error) {
			err := acc.Withdraw(e.ID, e.Money)
			if err == nil {
				return acc, nil
			}

			failed = true
			// transaction failed
			errTx := uc.txRepo.Exec(ctx, e.ID, func(tx *entity.Transaction) (*entity.Transaction, error) {
				tx.WithdrawFailed("From account: " + err.Error())
				return tx, nil
			})

			return nil, errTx
		}, e.ID.String()+"/withdraw")
		if failed || err != nil {
			return err
		}
	}

	if e.To != uuid.Nil {
		var failed bool
		logger.Infof("Depositing from %s, money: %d", e.From, e.Money)
		err := uc.accRepo.Exec(ctx, e.To, func(acc *entity.Account) (*entity.Account, error) {
			err := acc.Deposit(e.ID, e.Money)
			if err == nil {
				return acc, nil
			}

			failed = true
			// transaction failed. Need to rollback withdraw
			errTx := uc.txRepo.Exec(ctx, e.ID, func(tx *entity.Transaction) (*entity.Transaction, error) {
				tx.DepositFailed("To account: " + err.Error())
				return tx, nil
			})

			return nil, errTx
		}, e.ID.String()+"/deposit")
		if failed || err != nil {
			return err
		}
	}

	// complete transaction
	return uc.txRepo.Exec(ctx, e.ID, func(t *entity.Transaction) (*entity.Transaction, error) {
		t.Succeeded()
		return t, nil
	})
}

func (uc TransactionUsecase) TransactionFailed(ctx context.Context, aggregateID uuid.UUID, e event.TransactionFailed) error {
	if !e.Rollback {
		return nil
	}

	tx, err := uc.txRepo.Get(ctx, aggregateID)
	if err != nil {
		return err
	}
	err = uc.accRepo.Exec(ctx, tx.From, func(acc *entity.Account) (*entity.Account, error) {
		err := acc.Deposit(tx.ID, tx.Money)
		return acc, err
	}, tx.ID.String()+"/rollback")

	return err
}
