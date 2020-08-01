package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var (
	ErrAggregateNotFound = errors.New("Aggregate Not Found")
)

type BalanceUsecase struct {
	BalanceRepository domain.BalanceRepository
	Messenger         domain.Messenger
	Topic             string
}

func (b BalanceUsecase) ListAll(ctx context.Context) ([]entity.Balance, error) {
	return b.BalanceRepository.GetAllOrderByOwnerAsc(ctx)
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
	return b.BalanceRepository.CreateAccount(ctx, e)
}

func (b BalanceUsecase) MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.MoneyDeposited",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.BalanceRepository.GetByID(ctx, m.AggregateID)
	if agg.IsZero() {
		return fmt.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance + ac.Money,
	}
	return b.BalanceRepository.Update(ctx, update)
}

func (b BalanceUsecase) MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error {
	logger := log.WithFields(log.Fields{
		"method": "BalanceUsecase.MoneyWithdrawn",
	})
	ignore, err := b.ignoreEvent(ctx, logger, m)
	if err != nil || ignore {
		return err
	}
	agg, err := b.BalanceRepository.GetByID(ctx, m.AggregateID)
	if agg.IsZero() {
		return fmt.Errorf("Unknown aggregate with ID %s: %w", m.AggregateID, ErrAggregateNotFound)
	}
	update := entity.Balance{
		ID:      agg.ID,
		Version: agg.Version,
		EventID: m.EventID,
		Balance: agg.Balance - ac.Money,
	}
	return b.BalanceRepository.Update(ctx, update)
}

func (b BalanceUsecase) ignoreEvent(ctx context.Context, logger *logrus.Entry, m domain.Metadata) (bool, error) {
	lastEventID, err := b.BalanceRepository.GetEventID(ctx, m.AggregateID)
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

func (b BalanceUsecase) GetLastIDs(ctx context.Context) (domain.LastIDs, error) {
	// get the latest event ID from the eventstore
	eventID1, err := b.BalanceRepository.GetMaxEventID(ctx)
	if err != nil {
		return domain.LastIDs{}, fmt.Errorf("Could not get last event ID: %w", err)
	}

	// get the last messageID from the MQ
	messageID, eventID2, err := b.Messenger.GetLastMessageID(ctx, b.Topic)
	if err != nil {
		return domain.LastIDs{}, fmt.Errorf("Could not get last message ID: %w", err)
	}

	return domain.LastIDs{
		DbEventID:   eventID1,
		MqEventID:   eventID2,
		MqMessageID: messageID,
	}, nil
}

func (b BalanceUsecase) RebuildBalance(ctx context.Context) error {
	// signals to stop the projection listener
	err := b.Messenger.NotifyProjectionRegistry(ctx, domain.Notification{
		Projection: domain.ProjectionBalance,
		Action:     domain.Stop,
	})
	if err != nil {
		return err
	}

	// recreate table
	err = b.BalanceRepository.ClearAllData(ctx)
	if err != nil {
		return err
	}

	// signals to stop the projection listener
	return b.Messenger.NotifyProjectionRegistry(ctx, domain.Notification{
		Projection: domain.ProjectionBalance,
		Action:     domain.Start,
	})
}
