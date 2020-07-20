package usecase

import (
	"context"

	"github.com/quintans/es-cqrs-money-transfer/account/internal/domain/entity"
	"github.com/quintans/eventstore"
)

type AccountUsecase struct {
	es eventstore.EventStore
}

func (uc AccountUsecase) Create(ctx context.Context, acc *entity.Account) (string, error) {
	uc.es.Save(ctx, acc, eventstore.Options{})
	return "", nil
}

func (uc AccountUsecase) Deposit(ctx context.Context, id string, amount uint64) error {
	return nil
}

func (uc AccountUsecase) Withdraw(ctx context.Context, id string, amount uint64) error {
	return nil
}
