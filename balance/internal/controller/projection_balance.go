package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/labstack/gommon/log"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/poller"
)

type ProjectionBalance struct {
	Topic          string
	EsRepo         poller.Repository
	BalanceUsecase domain.BalanceUsecase
}

func (p ProjectionBalance) GetName() string {
	return domain.ProjectionBalance
}

func (p ProjectionBalance) GetTopic() string {
	return p.Topic
}

func (p ProjectionBalance) Boot(ctx context.Context) (pulsar.MessageID, error) {
	// get the latest event ID from the DB
	// get the last messageID from the MQ
	lastIds, err := p.BalanceUsecase.GetLastIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get last IDs: %w", err)
	}

	if lastIds.MqEventID > lastIds.DbEventID {
		// process all events from the ES in between
		esPoller := poller.New(p.EsRepo, poller.WithFilter(common.Filter{
			AggregateTypes: []string{event.AggregateType_Account},
		}))

		_, err = esPoller.ReplayFromUntil(ctx, func(ctx context.Context, e common.Event) error {
			m := domain.Metadata{AggregateID: e.AggregateID, EventID: e.ID}
			return p.routeEventBalanceProjection(ctx, m, e)
		}, lastIds.DbEventID, lastIds.MqEventID)
		if err != nil {
			return nil, fmt.Errorf("Could not replay all events: %w", err)
		}
	}

	// resume reading from the MQ
	id, err := pulsar.DeserializeMessageID(lastIds.MqMessageID)
	if err != nil {
		return nil, fmt.Errorf("Could not deserialize message ID: %w", err)
	}
	return id, nil

}

func (p ProjectionBalance) Handler(c context.Context, rm pulsar.Message) error {
	return p.routeMessageBalanceProjection(c, rm)
}

func (p ProjectionBalance) routeMessageBalanceProjection(ctx context.Context, rm pulsar.Message) error {
	e, m := messageToEvent(rm)
	return p.routeEventBalanceProjection(ctx, m, e)
}

func (p ProjectionBalance) routeEventBalanceProjection(ctx context.Context, m domain.Metadata, e common.Event) error {
	var err error
	switch e.Kind {
	case event.Event_AccountCreated:
		err = p.AccountCreated(context.Background(), m, e)
	case event.Event_MoneyDeposited:
		err = p.MoneyDeposited(context.Background(), m, e)
	case event.Event_MoneyWithdrawn:
		err = p.MoneyWithdrawn(context.Background(), m, e)
	default:
		log.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}

func messageToEvent(msg pulsar.Message) (common.Event, domain.Metadata) {
	p := msg.Payload()
	e := common.Event{}
	json.Unmarshal(p, &e)
	m := domain.Metadata{
		AggregateID: e.AggregateID,
		EventID:     e.ID,
	}
	return e, m
}

func (p ProjectionBalance) AccountCreated(ctx context.Context, m domain.Metadata, e common.Event) error {
	ac := event.AccountCreated{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.AccountCreated(ctx, m, ac)
}

func (p ProjectionBalance) MoneyDeposited(ctx context.Context, m domain.Metadata, e common.Event) error {
	ac := event.MoneyDeposited{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyDeposited(ctx, m, ac)
}

func (p ProjectionBalance) MoneyWithdrawn(ctx context.Context, m domain.Metadata, e common.Event) error {
	ac := event.MoneyWithdrawn{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.MoneyWithdrawn(ctx, m, ac)
}
