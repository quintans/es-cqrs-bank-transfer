package esdb

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
)

var accountType = (&entity.Account{}).GetKind()

type AccountRepository struct {
	es eventsourcing.EventStore[*entity.Account]
}

func NewAccountRepository(es eventsourcing.EventStore[*entity.Account]) AccountRepository {
	return AccountRepository{
		es: es,
	}
}

func (r AccountRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Account, error) {
	agg, err := r.es.Retrieve(ctx, id.String())
	if err != nil {
		return nil, errorMap(err)
	}
	return agg, nil
}

func (r AccountRepository) Exec(ctx context.Context, id uuid.UUID, do func(*entity.Account) (*entity.Account, error), idempotencyKey string) error {
	has, err := r.es.HasIdempotencyKey(ctx, idempotencyKey)
	if has || err != nil {
		return faults.Wrap(err)
	}
	return r.es.Update(ctx, id.String(), func(acc *entity.Account) (*entity.Account, error) {
		acc, err := do(acc)
		if err != nil {
			return nil, errorMap(err)
		}
		return acc, nil
	}, eventsourcing.WithIdempotencyKey(idempotencyKey))
}

func (r AccountRepository) New(ctx context.Context, agg *entity.Account) error {
	return r.es.Create(ctx, agg)
}

func errorMap(err error) error {
	if errors.Is(err, eventsourcing.ErrUnknownAggregateID) {
		return faults.Wrap(domain.ErrEntityNotFound)
	}
	return err
}
