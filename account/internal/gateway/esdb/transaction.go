package esdb

import (
	"context"

	"github.com/quintans/eventsourcing"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

var transactionType = (&entity.Transaction{}).GetType()

type TransactionRepository struct {
	es eventsourcing.EventStore
}

func NewTransactionRepository(es eventsourcing.EventStore) TransactionRepository {
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
	return r.es.Exec(ctx, id, func(a eventsourcing.Aggregater) (eventsourcing.Aggregater, error) {
		acc, err := do(a.(*entity.Transaction))
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	})
}
