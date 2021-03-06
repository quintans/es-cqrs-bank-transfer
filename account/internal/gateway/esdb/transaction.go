package esdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

type TransactionRepository struct {
	es eventsourcing.EventStore
}

func NewTransactionRepository(es eventsourcing.EventStore) TransactionRepository {
	return TransactionRepository{
		es: es,
	}
}

func (r TransactionRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Transaction, error) {
	agg, err := r.es.GetByID(ctx, id.String())
	if err != nil {
		return nil, errorMap(err)
	}
	return agg.(*entity.Transaction), nil
}

func (r TransactionRepository) CreateIfNew(ctx context.Context, agg *entity.Transaction) (bool, error) {
	a, err := r.es.GetByID(ctx, agg.ID.String())
	if a != nil || err != nil {
		return false, err
	}
	err = r.es.Save(ctx, agg)

	return err == nil, errorMap(err)
}

func (r TransactionRepository) Exec(ctx context.Context, id uuid.UUID, do func(*entity.Transaction) (*entity.Transaction, error), idempotencyKey string) error {
	has, err := r.es.HasIdempotencyKey(ctx, idempotencyKey)
	if has || err != nil {
		return faults.Wrap(err)
	}
	return r.es.Exec(ctx, id.String(), func(a eventsourcing.Aggregater) (eventsourcing.Aggregater, error) {
		acc, err := do(a.(*entity.Transaction))
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	}, eventsourcing.WithIdempotencyKey(idempotencyKey))
}
