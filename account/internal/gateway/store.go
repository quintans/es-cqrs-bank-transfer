package gateway

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/eventstore"
)

type AccountRepository struct {
	es eventstore.EventStore
}

func NewAccountRepository(es eventstore.EventStore) AccountRepository {
	return AccountRepository{
		es: es,
	}
}

func (r AccountRepository) Get(ctx context.Context, id string) (*entity.Account, error) {
	agg, err := r.es.GetByID(ctx, id)
	if err != nil {
		return nil, errorMap(err)
	}
	return agg.(*entity.Account), nil
}

func (r AccountRepository) Save(ctx context.Context, agg *entity.Account) error {
	return r.es.Save(ctx, agg)
}

func (r AccountRepository) Exec(ctx context.Context, id string, do func(*entity.Account) (*entity.Account, error)) error {
	return r.es.Exec(ctx, id, func(a eventstore.Aggregater) (eventstore.Aggregater, error) {
		acc, err := do(a.(*entity.Account))
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	})
}

func errorMap(err error) error {
	if err == eventstore.ErrUnknownAggregateID {
		return domain.ErrUnknownAccount
	}
	return err
}
