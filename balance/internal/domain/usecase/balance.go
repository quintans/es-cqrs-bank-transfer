package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventstore/projection"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var (
	ErrAggregateNotFound = errors.New("Aggregate Not Found")
)

type BalanceUsecase struct {
	balanceRepository domain.BalanceRepository
	restarter         projection.Restarter
	partitions        int
}

func NewBalanceUsecase(balanceRepository domain.BalanceRepository, restarter projection.Restarter, partitions int) BalanceUsecase {
	return BalanceUsecase{
		balanceRepository: balanceRepository,
		restarter:         restarter,
		partitions:        partitions,
	}
}

func (b BalanceUsecase) ListAll(ctx context.Context) ([]entity.Balance, error) {
	return b.balanceRepository.GetAllOrderByOwnerAsc(ctx)
}

func (b BalanceUsecase) AccountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.AccountCreated",
	})

	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}

	e := entity.Balance{
		ID:      ac.ID,
		EventID: m.EventID,
		Owner:   ac.Owner,
		Status:  event.OPEN,
		Balance: ac.Money,
	}
	logger.Infof("Creating account: ID: %s, Owner: %s, Balance: %d", ac.ID, ac.Owner, ac.Money)
	return b.balanceRepository.CreateAccount(ctx, e)
}

func (b BalanceUsecase) MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.MoneyDeposited",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if agg.IsZero() {
		return fmt.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance + ac.Money,
	}
	return b.balanceRepository.Update(ctx, update)
}

func (b BalanceUsecase) MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.MoneyWithdrawn",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if agg.IsZero() {
		return fmt.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance - ac.Money,
	}
	return b.balanceRepository.Update(ctx, update)
}

func (b BalanceUsecase) ignoreEvent(ctx context.Context, logger *logrus.Entry, m domain.Metadata) (bool, error) {
	lastEventID, err := b.balanceRepository.GetEventID(ctx, m.AggregateID)
	if err != nil {
		return false, err
	}
	// guarding against redelivery
	if m.EventID <= lastEventID {
		logger.Warnf("Ignoring current event %s, since it is less or equal than last event %s", m.EventID, lastEventID)
		return true, nil
	}

	return false, nil
}

func (b BalanceUsecase) RebuildBalance(ctx context.Context) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.RebuildBalance",
	})

	return b.restarter.Restart(ctx, domain.ProjectionBalance, b.partitions, func(ctx context.Context) error {
		logger.Info("Cleaning all balance data")
		// TODO after cleaning all data, we could replay all events here, rather than wait for the boot process, that is slower.
		return b.balanceRepository.ClearAllData(ctx)
	})

}

func (b BalanceUsecase) GetLastEventID(ctx context.Context) (string, error) {
	// get the latest event ID from the eventstore
	eventID, err := b.balanceRepository.GetMaxEventID(ctx)
	if err != nil {
		return "", fmt.Errorf("Could not get last event ID: %w", err)
	}

	return eventID, nil
}
