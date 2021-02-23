package esdb

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/eventstore"
)

type TransactionRepository struct {
	es eventstore.EventStore
}

func NewTransactionRepository(es eventstore.EventStore) TransactionRepository {
	return TransactionRepository{
		es: es,
	}
}

func (r TransactionRepository) Get(ctx context.Context, id string) (*entity.Transaction, error) {
	agg, err := r.es.GetByID(ctx, id)
	if err != nil {
		return nil, errorMap(err)
	}
	return agg.(*entity.Transaction), nil
}

func (r TransactionRepository) CreateIfNew(ctx context.Context, agg *entity.Transaction) (bool, error) {
	a, err := r.es.GetByID(ctx, agg.ID)
	if a != nil || err != nil {
		return false, err
	}
	err = r.es.Save(ctx, agg)

	return err == nil, errorMap(err)
}

func (r TransactionRepository) Exec(ctx context.Context, id string, do func(*entity.Transaction) (*entity.Transaction, error)) error {
	return r.es.Exec(ctx, id, func(a eventstore.Aggregater) (eventstore.Aggregater, error) {
		acc, err := do(a.(*entity.Transaction))
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	})
}