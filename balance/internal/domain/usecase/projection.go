package usecase

import (
	"context"
	"errors"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/faults"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var ErrAggregateNotFound = errors.New("Aggregate Not Found")

type ProjectionUsecase struct {
	balanceRepository domain.BalanceRepository
}

func NewProjectionUsecase(
	balanceRepository domain.BalanceRepository,
) ProjectionUsecase {
	return ProjectionUsecase{
		balanceRepository: balanceRepository,
	}
}

func (b ProjectionUsecase) AccountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error {
	logger := log.WithFields(log.Fields{
		"method": "ProjectionUsecase.AccountCreated",
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

func (b ProjectionUsecase) MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error {
	logger := log.WithFields(log.Fields{
		"method": "ProjectionUsecase.MoneyDeposited",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if agg.IsZero() {
		return faults.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance + ac.Money,
	}
	return b.balanceRepository.Update(ctx, update)
}

func (b ProjectionUsecase) MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error {
	logger := log.WithFields(log.Fields{
		"method": "ProjectionUsecase.MoneyWithdrawn",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if agg.IsZero() {
		return faults.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance - ac.Money,
	}
	return b.balanceRepository.Update(ctx, update)
}

func (b ProjectionUsecase) ignoreEvent(ctx context.Context, logger *logrus.Entry, m domain.Metadata) (bool, error) {
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

func (b ProjectionUsecase) GetLastEventID(ctx context.Context) (string, error) {
	// get the latest event ID from the eventsourcing
	eventID, err := b.balanceRepository.GetMaxEventID(ctx)
	if err != nil {
		return "", faults.Errorf("Could not get last event ID: %w", err)
	}

	return eventID, nil
}
