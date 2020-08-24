package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/nats-io/stan.go"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/gateway"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/player"
	log "github.com/sirupsen/logrus"
)

type ProjectionBalance struct {
	BalanceUsecase domain.BalanceUsecase
	EsRepo         player.Repository
	EsBatchSize    int
	Topic          string
	Messenger      gateway.Messenger
	Stan           stan.Conn
}

func (p ProjectionBalance) GetName() string {
	return domain.ProjectionBalance
}

func (p ProjectionBalance) GetResumeEventID(ctx context.Context) (string, error) {
	return p.BalanceUsecase.GetLastEventID(ctx)
}

func (p ProjectionBalance) Boot(ctx context.Context) error {
	// get the latest event ID from the DB
	prjEventID, err := p.BalanceUsecase.GetLastEventID(ctx)
	if err != nil {
		return fmt.Errorf("Could not get last event ID from the projection: %w", err)
	}
	filter := player.WithAggregateTypes(event.AggregateType_Account)
	logger := log.WithFields(log.Fields{
		"projection": domain.ProjectionBalance,
		"from":       prjEventID,
	})
	logger.Info("Booting...")

	// To avoid the creation of  potential massive buffer size
	// and to ensure that events are not lost, between the switch to the consumer,
	// we execute the fetch in several steps.
	// 1) Process all events from the ES from the begginning
	// 2) start the consumer to track new events from now on
	// 3) process any event that may have arrived between the switch
	// 4) start consuming events from the last position
	replayer := player.New(p.EsRepo, p.EsBatchSize)
	handler := func(ctx context.Context, e eventstore.Event) error {
		err := p.Handler(ctx, e)
		if err != nil {
			return err
		}
		return err
	}
	// replay oldest events
	lastEventID, err := replayer.ReplayFromUntil(ctx, handler, prjEventID, "", filter)
	if err != nil {
		return fmt.Errorf("Could not replay all events (first part): %w", err)
	}

	// start tracking events
	token, err := p.Messenger.GetResumeToken(ctx, p.Topic)
	if err != nil {
		return fmt.Errorf("Could not retrieve resume token: %w", err)
	}

	// consume potential missed events events between the switch to the consumer
	lastEventID, err = replayer.ReplayFromUntil(ctx, handler, lastEventID, "", filter)
	if err != nil {
		return fmt.Errorf("Could not replay all events (second part): %w", err)
	}
	logger.WithField("to", lastEventID).Info("Finished booting")

	// start consuming events from the last available position
	start := stan.DeliverAllAvailable()
	if token != "" {
		seq, err := strconv.ParseUint(token, 10, 64)
		if err != nil {
			logger.WithError(err).Errorf("Unable to parse resume token '%s'", token)
		} else {
			start = stan.StartAtSequence(seq)
		}
	}
	sub, err := p.Stan.Subscribe(p.Topic, func(m *stan.Msg) {
		e := eventstore.Event{}
		err := json.Unmarshal(m.Data, &e)
		if err != nil {
			logger.WithError(err).Errorf("Unable to unmarshal event '%s'", string(m.Data))
		}
		err = p.Handler(ctx, e)
		if err != nil {
			logger.WithError(err).Errorf("Error when handling event with ID '%s'", e.ID)
		}
	}, start)

	go func() {
		<-ctx.Done()
		sub.Close()
	}()

	return nil
}

func (p ProjectionBalance) Handler(ctx context.Context, e eventstore.Event) error {
	logger := log.WithFields(log.Fields{
		"projection": domain.ProjectionBalance,
		"event":      e,
	})
	logger.Info("routing event")

	m := domain.Metadata{
		AggregateID: e.AggregateID,
		EventID:     e.ID,
	}

	var err error
	switch e.Kind {
	case event.Event_AccountCreated:
		err = p.AccountCreated(context.Background(), m, e)
	case event.Event_MoneyDeposited:
		err = p.MoneyDeposited(context.Background(), m, e)
	case event.Event_MoneyWithdrawn:
		err = p.MoneyWithdrawn(context.Background(), m, e)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}

func (p ProjectionBalance) AccountCreated(ctx context.Context, m domain.Metadata, e eventstore.Event) error {
	ac := event.AccountCreated{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.AccountCreated(ctx, m, ac)
}

func (p ProjectionBalance) MoneyDeposited(ctx context.Context, m domain.Metadata, e eventstore.Event) error {
	ac := event.MoneyDeposited{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyDeposited(ctx, m, ac)
}

func (p ProjectionBalance) MoneyWithdrawn(ctx context.Context, m domain.Metadata, e eventstore.Event) error {
	ac := event.MoneyWithdrawn{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyWithdrawn(ctx, m, ac)
}
