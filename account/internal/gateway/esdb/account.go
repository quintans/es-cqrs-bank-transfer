package esdb

import (
	"context"
	"errors"

	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

var accountType = (&entity.Account{}).GetType()

type AccountRepository struct {
	es eventsourcing.EventStore
}

func NewAccountRepository(es eventsourcing.EventStore) AccountRepository {
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

func (r AccountRepository) Exec(ctx context.Context, id string, do func(*entity.Account) (*entity.Account, error), idempotencyKey string) error {
	has, err := r.es.HasIdempotencyKey(ctx, accountType, idempotencyKey)
	if has || err != nil {
		return faults.Wrap(err)
	}
	return r.es.Exec(ctx, id, func(a eventsourcing.Aggregater) (eventsourcing.Aggregater, error) {
		acc, err := do(a.(*entity.Account))
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	}, eventsourcing.WithIdempotencyKey(idempotencyKey))
}

func (r AccountRepository) New(ctx context.Context, agg *entity.Account) error {
	return r.es.Save(ctx, agg)
}

func errorMap(err error) error {
	if errors.Is(err, eventsourcing.ErrUnknownAggregateID) {
		return faults.Wrap(domain.ErrEntityNotFound)
	}
	return err
}
