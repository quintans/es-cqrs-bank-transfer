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
	Topic             string
	Ctrl              PulsarController
	BalanceRepository domain.BalanceRepository
	EsRepo            poller.Repository
	Messenger         domain.Messenger
}

func NewProjectionBalance(topic string, repo domain.BalanceRepository, EsRepo poller.Repository, Messenger domain.Messenger) ProjectionBalance {
	return ProjectionBalance{}
}

func (p ProjectionBalance) GetName() string {
	return "ProjectionBalance"
}

func (p ProjectionBalance) GetTopic() string {
	return p.Topic
}

func (p ProjectionBalance) Boot(ctx context.Context) (pulsar.MessageID, error) {
	// get the latest event ID from the eventstore
	eventID1, err := p.BalanceRepository.GetMaxEventID(ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get last event ID: %w", err)
	}

	// get the last messageID from the MQ
	messageID, eventID2, err := p.Messenger.GetLastMessageID(ctx, p.Topic)
	if err != nil {
		return nil, fmt.Errorf("Could not get last message ID: %w", err)
	}

	// get all events from the ES in between
	esPoller := poller.New(p.EsRepo, poller.WithFilter(common.Filter{
		AggregateTypes: []string{event.AggregateType_Account},
	}))

	_, err = esPoller.ReplayFromUntil(ctx, func(ctx context.Context, e common.Event) error {
		m := domain.Metadata{AggregateID: e.AggregateID, EventID: e.ID}
		return routeEventBalanceProjection(ctx, m, e, p.Ctrl)
	}, eventID1, eventID2)
	if err != nil {
		return nil, fmt.Errorf("Could not replay all events: %w", err)
	}

	// resume reading from the MQ
	id, err := pulsar.DeserializeMessageID(messageID)
	if err != nil {
		return nil, fmt.Errorf("Could not deserialize message ID: %w", err)
	}
	return id, nil

}

func (p ProjectionBalance) Handler(c context.Context, rm pulsar.ReaderMessage) error {
	return routeMessageBalanceProjection(c, rm, p.Ctrl)
}

func routeMessageBalanceProjection(ctx context.Context, rm pulsar.ReaderMessage, ctrl PulsarController) error {
	e, m := messageToEvent(rm)
	return routeEventBalanceProjection(ctx, m, e, ctrl)
}

func routeEventBalanceProjection(ctx context.Context, m domain.Metadata, e common.Event, ctrl PulsarController) error {
	var err error
	switch e.Kind {
	case event.Event_AccountCreated:
		err = ctrl.AccountCreated(context.Background(), m, e)
	case event.Event_MoneyDeposited:
		err = ctrl.MoneyDeposited(context.Background(), m, e)
	case event.Event_MoneyWithdrawn:
		err = ctrl.MoneyWithdrawn(context.Background(), m, e)
	default:
		log.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}

func messageToEvent(rm pulsar.ReaderMessage) (common.Event, domain.Metadata) {
	msg := rm.Message
	p := msg.Payload()
	e := common.Event{}
	json.Unmarshal(p, &e)
	m := domain.Metadata{
		AggregateID: e.AggregateID,
		EventID:     e.ID,
	}
	return e, m
}
