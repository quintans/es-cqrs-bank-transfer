package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
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

// TransactionCreated processes a transaction.
// This demonstrates how we can use the idempotency key to guard against duplicated events.
// Since all aggregates belong to the same service, there is no reason to split into several event handlers.
// Another restriction for the split is that a transaction origin and destination are optional, making it a bit more trickier to split.
// If the aggregates belonged to different services, then we would have no choice but to break it down into
// several chained event handlers: TransactionCreated, MoneyWithdrawn, MoneyDeposited, TransactionFailed
func (uc TransactionUsecase) TransactionCreated(ctx context.Context, e event.TransactionCreated) error {
	logger := uc.logger.WithTags(log.Tags{
		"method": "TransactionUsecase.TransactionCreated",
		"event":  e,
	})

	ok, err := uc.whitdraw(ctx, logger, e.From, e.Money, e.ID)
	if !ok || err != nil {
		return err
	}

	ok, err = uc.deposit(ctx, logger, e.To, e.Money, e.ID)
	if !ok || err != nil {
		return err
	}

	// complete transaction
	return uc.txRepo.Exec(ctx, e.ID, func(t *entity.Transaction) (*entity.Transaction, error) {
		t.Succeeded()
		return t, nil
	}, eventsourcing.EmptyIdempotencyKey)
}

func (uc TransactionUsecase) whitdraw(ctx context.Context, logger log.Logger, accID uuid.UUID, money int64, txID uuid.UUID) (bool, error) {
	if accID == uuid.Nil {
		return true, nil
	}

	var failed bool
	logger.Infof("Withdrawing from %s, money: %d", accID, money)
	idempotencyKey := txID.String() + "/withdraw"
	err := uc.accRepo.Exec(ctx, accID, func(acc *entity.Account) (*entity.Account, error) {
		err := acc.Withdraw(txID, money)
		if err == nil {
			return acc, nil
		}
		logger.WithError(err).Warn("failed to withdraw")

		failed = true
		// transaction failed
		errTx := uc.txRepo.Exec(ctx, txID, func(tx *entity.Transaction) (*entity.Transaction, error) {
			tx.WithdrawFailed("From account: " + err.Error())
			return tx, nil
		}, idempotencyKey)

		return nil, errTx
	}, idempotencyKey)
	if failed || err != nil {
		return false, err
	}

	return true, nil
}

func (uc TransactionUsecase) deposit(ctx context.Context, logger log.Logger, accID uuid.UUID, money int64, txID uuid.UUID) (bool, error) {
	if accID == uuid.Nil {
		return true, nil
	}

	var failed bool
	logger.Infof("Depositing from %s, money: %d", accID, money)
	idempotencyKey := txID.String() + "/deposit"
	err := uc.accRepo.Exec(ctx, accID, func(acc *entity.Account) (*entity.Account, error) {
		err := acc.Deposit(txID, money)
		if err == nil {
			return acc, nil
		}
		logger.WithError(err).Warn("failed to deposit")

		failed = true
		// transaction failed. Need to rollback withdraw
		errTx := uc.txRepo.Exec(ctx, txID, func(tx *entity.Transaction) (*entity.Transaction, error) {
			tx.DepositFailed("To account: " + err.Error())
			return tx, nil
		}, idempotencyKey)

		return nil, errTx
	}, idempotencyKey)
	if failed || err != nil {
		return false, err
	}

	return true, nil
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
