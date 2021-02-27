package usecase

import (
	"context"
	"time"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventstore/eventid"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/projection"
	"github.com/quintans/eventstore/store"
	"github.com/quintans/faults"
	log "github.com/sirupsen/logrus"
)

type BalanceUsecase struct {
	balanceRepository domain.BalanceRepository
	restarter         projection.Rebuilder
	listenerCount     uint32
	esRepo            player.Repository
	handler           domain.EventHandler
}

func NewBalanceUsecase(
	balanceRepository domain.BalanceRepository,
	restarter projection.Rebuilder,
	listenerCount uint32,
	esRepo player.Repository,
	handler domain.EventHandler,
) BalanceUsecase {
	return BalanceUsecase{
		balanceRepository: balanceRepository,
		restarter:         restarter,
		listenerCount:     listenerCount,
		esRepo:            esRepo,
		handler:           handler,
	}
}

func (b BalanceUsecase) GetOne(ctx context.Context, id string) (entity.Balance, error) {
	return b.balanceRepository.GetByID(ctx, id)
}

func (b BalanceUsecase) ListAll(ctx context.Context) ([]entity.Balance, error) {
	return b.balanceRepository.GetAllOrderByOwnerAsc(ctx)
}

func (b BalanceUsecase) RebuildBalance(ctx context.Context, after time.Time) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.RebuildBalance",
	})

	p := player.New(b.esRepo)

	return b.restarter.Rebuild(ctx, domain.ProjectionBalance, int(b.listenerCount),
		func(ctx context.Context) (string, error) {
			if !after.IsZero() {
				return eventid.NewEventID(after, "", 0), nil
			}

			logger.Info("Cleaning all balance data")
			err := b.balanceRepository.ClearAllData(ctx)
			if err != nil {
				return "", faults.Errorf("Unable to clean balance data: %w", err)
			}
			afterEventID, err := p.Replay(ctx, b.handler.Handle, "", store.WithAggregateTypes(event.AggregateType_Account))
			if err != nil {
				return "", faults.Errorf("Unable to replay ALL balance data: %w", err)
			}

			return afterEventID, nil
		},
		func(ctx context.Context, afterEventID string) (string, error) {
			afterEventID, err := p.Replay(ctx, b.handler.Handle, afterEventID, store.WithAggregateTypes(event.AggregateType_Account))
			if err != nil {
				return "", faults.Errorf("Unable to replay events after cleaning balance data: %w", err)
			}
			return afterEventID, nil
		},
	)
}
