package esdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

type TransactionRepository struct {
	es eventsourcing.EventStorer[*entity.Transaction]
}

func NewTransactionRepository(es eventsourcing.EventStorer[*entity.Transaction]) TransactionRepository {
	return TransactionRepository{
		es: es,
	}
}

func (r TransactionRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Transaction, error) {
	agg, err := r.es.Retrieve(ctx, id.String())
	if err != nil {
		return nil, errorMap(err)
	}
	return agg, nil
}

func (r TransactionRepository) CreateIfNew(ctx context.Context, agg *entity.Transaction) (bool, error) {
	a, err := r.es.Retrieve(ctx, agg.ID.String())
	if a != nil || err != nil {
		return false, err
	}
	err = r.es.Create(ctx, agg)

	return err == nil, errorMap(err)
}

func (r TransactionRepository) Exec(ctx context.Context, id uuid.UUID, do func(*entity.Transaction) (*entity.Transaction, error), idempotencyKey string) error {
	has, err := r.es.HasIdempotencyKey(ctx, idempotencyKey)
	if has || err != nil {
		return faults.Wrap(err)
	}
	return r.es.Update(ctx, id.String(), func(acc *entity.Transaction) (*entity.Transaction, error) {
		acc, err := do(acc)
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	}, eventsourcing.WithIdempotencyKey(idempotencyKey))
}
