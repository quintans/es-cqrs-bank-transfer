package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	log "github.com/sirupsen/logrus"
)

type AccountUsecase struct {
	repo domain.AccountRepository
}

func NewAccountUsecase(repo domain.AccountRepository) AccountUsecase {
	return AccountUsecase{
		repo: repo,
	}
}

func (uc AccountUsecase) Create(ctx context.Context, createAccount domain.CreateCommand) (string, error) {
	id := uuid.New().String()
	log.WithFields(log.Fields{
		"method": "AccountUsecase.Create",
	}).Infof("Creating account with owner:%s, id: %s, money: %d", createAccount.Owner, id, createAccount.Money)

	acc := entity.CreateAccount(createAccount.Owner, id, createAccount.Money)
	if err := uc.repo.Save(ctx, acc); err != nil {
		return "", err
	}
	return id, nil
}

func (uc AccountUsecase) Deposit(ctx context.Context, cmd domain.DepositCommand) error {
	log.WithFields(log.Fields{
		"method": "AccountUsecase.Deposit",
	}).Infof("Depositing id: %s, money: %d", cmd.ID, cmd.Money)

	return uc.repo.Exec(ctx, cmd.ID, func(acc *entity.Account) (*entity.Account, error) {
		acc.Deposit(cmd.Money)

		return acc, nil
	})
}

func (uc AccountUsecase) Withdraw(ctx context.Context, cmd domain.WithdrawCommand) error {
	log.WithFields(log.Fields{
		"method": "AccountUsecase.Withdraw",
	}).Infof("Depositing id: %s, money: %d", cmd.ID, cmd.Money)

	return uc.repo.Exec(ctx, cmd.ID, func(acc *entity.Account) (*entity.Account, error) {
		if acc.Withdraw(cmd.Money) {
			err := uc.repo.Save(ctx, acc)
			if err != nil {
				return nil, err
			}
		}

		return acc, domain.ErrNotEnoughFunds
	})
}

func (uc AccountUsecase) Transfer(ctx context.Context, cmd domain.TransferCommand) error {
	return errors.New("Not implemented")
}

func (uc AccountUsecase) Balance(ctx context.Context, id string) (domain.AccountDTO, error) {
	var dto domain.AccountDTO

	acc, err := uc.repo.Get(ctx, id)
	if err != nil {
		return domain.AccountDTO{}, err
	}
	dto = domain.AccountDTO{
		Owner:   acc.Owner,
		Balance: acc.Balance,
		Status:  string(acc.Status),
	}

	return dto, nil
}
