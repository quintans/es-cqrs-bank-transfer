package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/faults"
	"github.com/sirupsen/logrus"
)

var ErrAggregateNotFound = errors.New("aggregate Not Found")

type BalanceUsecase struct {
	balanceRepository domain.BalanceRepository
}

func NewBalanceUsecase(
	balanceRepository domain.BalanceRepository,
) BalanceUsecase {
	return BalanceUsecase{
		balanceRepository: balanceRepository,
	}
}

func (b BalanceUsecase) GetOne(ctx context.Context, id uuid.UUID) (entity.Balance, error) {
	return b.balanceRepository.GetByID(ctx, id)
}

func (b BalanceUsecase) ListAll(ctx context.Context) ([]entity.Balance, error) {
	return b.balanceRepository.GetAllOrderByOwnerAsc(ctx)
}

func (b BalanceUsecase) AccountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error {
	logger := logrus.WithFields(logrus.Fields{
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
	return b.balanceRepository.CreateAccount(ctx, m.ResumeKey, m.ResumeToken, e)
}

func (b BalanceUsecase) MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error {
	logger := logrus.WithFields(logrus.Fields{
		"method": "ProjectionUsecase.MoneyDeposited",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if err != nil {
		return nil
	}
	if agg.IsZero() {
		return faults.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance + ac.Money,
	}
	return b.balanceRepository.Update(ctx, m.ResumeKey, m.ResumeToken, update)
}

func (b BalanceUsecase) MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error {
	logger := logrus.WithFields(logrus.Fields{
		"method": "ProjectionUsecase.MoneyWithdrawn",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if err != nil {
		return nil
	}
	if agg.IsZero() {
		return faults.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance - ac.Money,
	}
	return b.balanceRepository.Update(ctx, m.ResumeKey, m.ResumeToken, update)
}

func (b BalanceUsecase) ignoreEvent(ctx context.Context, logger *logrus.Entry, m domain.Metadata) (bool, error) {
	lastEventID, err := b.balanceRepository.GetEventID(ctx, m.AggregateID)
	if err != nil {
		return false, err
	}
	// guarding against redelivery
	if m.EventID.Compare(lastEventID) <= 0 {
		logger.Warnf("Ignoring current event %s, since it is less or equal than last event %s", m.EventID, lastEventID)
		return true, nil
	}

	return false, nil
}
