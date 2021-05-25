package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/common"
	"github.com/quintans/eventsourcing/eventid"
	"github.com/quintans/eventsourcing/player"
	"github.com/quintans/eventsourcing/store"
	"github.com/quintans/faults"
	"github.com/sirupsen/logrus"
)

var ErrAggregateNotFound = errors.New("aggregate Not Found")

type ProjectionUsecase struct {
	balanceRepository domain.BalanceRepository
	factory           eventsourcing.Factory
	codec             eventsourcing.Codec
	upcaster          eventsourcing.Upcaster
	esRepo            player.Repository
}

func NewProjectionUsecase(
	balanceRepository domain.BalanceRepository,
	factory eventsourcing.Factory,
	codec eventsourcing.Codec,
	upcaster eventsourcing.Upcaster,
	esRepo player.Repository,
) ProjectionUsecase {
	return ProjectionUsecase{
		balanceRepository: balanceRepository,
		factory:           factory,
		codec:             codec,
		upcaster:          upcaster,
		esRepo:            esRepo,
	}
}

func (p ProjectionUsecase) Handle(ctx context.Context, e eventsourcing.Event) error {
	if !common.In(e.AggregateType, event.AggregateType_Account) {
		return nil
	}

	logger := logrus.WithFields(logrus.Fields{
		"projection": domain.ProjectionBalance,
		"event":      e,
	})

	m := domain.Metadata{
		AggregateID: e.AggregateID,
		EventID:     e.ID,
	}

	evt, err := eventsourcing.RehydrateEvent(p.factory, p.codec, p.upcaster, e.Kind, e.Body)
	if err != nil {
		return err
	}

	switch t := evt.(type) {
	case event.AccountCreated:
		err = p.accountCreated(ctx, m, t)
	case event.MoneyDeposited:
		err = p.moneyDeposited(ctx, m, t)
	case event.MoneyWithdrawn:
		err = p.moneyWithdrawn(ctx, m, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}

func (b ProjectionUsecase) accountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error {
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
	return b.balanceRepository.CreateAccount(ctx, e)
}

func (b ProjectionUsecase) moneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error {
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
	return b.balanceRepository.Update(ctx, update)
}

func (b ProjectionUsecase) moneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error {
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

func (b ProjectionUsecase) RebuildBalance(ctx context.Context, after time.Time) (string, error) {
	logger := logrus.WithFields(logrus.Fields{
		"method": "BalanceUsecase.RebuildBalance",
	})

	if !after.IsZero() {
		return eventid.NewEventID(after, "", 0), nil
	}

	logger.Info("Cleaning all balance data")
	err := b.balanceRepository.ClearAllData(ctx)
	if err != nil {
		return "", faults.Errorf("Unable to clean balance data: %w", err)
	}

	return b.RebuildWrapUp(ctx, "")
}

func (b ProjectionUsecase) RebuildWrapUp(ctx context.Context, afterEventID string) (string, error) {
	p := player.New(b.esRepo)
	afterEventID, err := p.Replay(ctx, b.Handle, afterEventID, store.WithAggregateTypes(event.AggregateType_Account))
	if err != nil {
		return "", faults.Errorf("Unable to replay events after '%s': %w", afterEventID, err)
	}
	return afterEventID, nil
}
