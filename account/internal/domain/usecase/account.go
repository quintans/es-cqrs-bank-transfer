package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing/log"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

type AccountUsecase struct {
	logger log.Logger
	repo   domain.AccountRepository
}

func NewAccountUsecase(logger log.Logger, repo domain.AccountRepository) AccountUsecase {
	return AccountUsecase{
		logger: logger,
		repo:   repo,
	}
}

func (uc AccountUsecase) Create(ctx context.Context, createAccount domain.CreateAccountCommand) (uuid.UUID, error) {
	id := uuid.New()
	uc.logger.WithTags(log.Tags{
		"method": "AccountUsecase.Create",
	}).Infof("Creating account with owner:%s, id: %s", createAccount.Owner, id)

	acc := entity.CreateAccount(createAccount.Owner, id)
	if err := uc.repo.New(ctx, acc); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (uc AccountUsecase) Balance(ctx context.Context, id uuid.UUID) (domain.AccountDTO, error) {
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
