package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/shared/event"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/faults"
	"github.com/sirupsen/logrus"
)

var ErrAggregateNotFound = errors.New("aggregate Not Found")

// gog:aspect
type BalanceService struct {
	projection.WriteResumeStore

	balanceRepository domain.BalanceRepository
}

func NewBalanceService(
	wrs projection.WriteResumeStore,
	balanceRepository domain.BalanceRepository,
) BalanceService {
	return BalanceService{
		WriteResumeStore:  wrs,
		balanceRepository: balanceRepository,
	}
}

// gog:@monitor
func (b BalanceService) GetOne(ctx context.Context, id uuid.UUID) (entity.Balance, error) {
	return b.balanceRepository.GetByID(ctx, id)
}

// gog:@monitor
func (b BalanceService) ListAll(ctx context.Context) ([]entity.Balance, error) {
	return b.balanceRepository.GetAllOrderByOwnerAsc(ctx)
}

// gog:@monitor
func (b BalanceService) AccountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error {
	logger := logrus.WithFields(logrus.Fields{
		"method": "ProjectionUsecase.AccountCreated",
	})

	e := entity.Balance{
		ID:       ac.ID,
		Sequence: m.ResumeToken.Sequence(),
		Kind:     m.ResumeToken.Kind(),
		Owner:    ac.Owner,
		Status:   event.OPEN,
		Balance:  ac.Money,
	}
	logger.Infof("Creating account: ID: %s, Owner: %s, Balance: %d", ac.ID, ac.Owner, ac.Money)
	err := b.balanceRepository.CreateAccount(ctx, e)
	if err != nil {
		return faults.Wrap(err)
	}

	err = b.SetStreamResumeToken(ctx, m.ResumeKey, m.ResumeToken)
	return faults.Wrapf(err, "saving resume key on account create (%s=%s)", m.ResumeKey, m.ResumeToken)
}

// gog:@monitor
func (b BalanceService) MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error {
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if err != nil {
		return faults.Wrap(err)
	}
	if agg.IsZero() {
		return faults.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:       agg.ID,
		Version:  agg.Version,
		Sequence: m.ResumeToken.Sequence(),
		Kind:     m.ResumeToken.Kind(),
		Balance:  agg.Balance + ac.Money,
	}
	err = b.balanceRepository.Update(ctx, update)
	if err != nil {
		return faults.Wrap(err)
	}

	err = b.SetStreamResumeToken(ctx, m.ResumeKey, m.ResumeToken)
	return faults.Wrapf(err, "saving resume key on money deposited (%s=%s)", m.ResumeKey, m.ResumeToken)
}

// gog:@monitor
func (b BalanceService) MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error {
	agg, err := b.balanceRepository.GetByID(ctx, m.AggregateID)
	if err != nil {
		return faults.Wrap(err)
	}
	if agg.IsZero() {
		return faults.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:       agg.ID,
		Version:  agg.Version,
		Sequence: m.ResumeToken.Sequence(),
		Kind:     m.ResumeToken.Kind(),
		Balance:  agg.Balance - ac.Money,
	}
	err = b.balanceRepository.Update(ctx, update)
	if err != nil {
		return faults.Wrap(err)
	}

	err = b.SetStreamResumeToken(ctx, m.ResumeKey, m.ResumeToken)
	return faults.Wrapf(err, "saving resume key on money withdrawn (%s=%s)", m.ResumeKey, m.ResumeToken)
}
