package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing/log"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

// gog:aspect
type AccountService struct {
	logger log.Logger
	repo   domain.AccountRepository
}

func NewAccountService(logger log.Logger, repo domain.AccountRepository) AccountService {
	return AccountService{
		logger: logger,
		repo:   repo,
	}
}

// gog:@monitor
func (s AccountService) Create(ctx context.Context, createAccount domain.CreateAccountCommand) (uuid.UUID, error) {
	id := uuid.New()
	s.logger.WithTags(log.Tags{
		"method": "AccountUsecase.Create",
	}).Infof("Creating account with owner:%s, id: %s", createAccount.Owner, id)

	acc := entity.CreateAccount(createAccount.Owner, id)
	if err := s.repo.New(ctx, acc); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// gog:@monitor
func (s AccountService) Balance(ctx context.Context, id uuid.UUID) (domain.AccountDTO, error) {
	var dto domain.AccountDTO

	acc, err := s.repo.Get(ctx, id)
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
