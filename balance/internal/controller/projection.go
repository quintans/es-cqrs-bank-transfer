package controller

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/eventsourcing/sink"
	"github.com/quintans/eventsourcing/util"
	"github.com/quintans/faults"
)

type ProjectionService interface {
	AccountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error
	MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error
	MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error
}

type Projection struct {
	projection.ReadResumeStore

	logger  log.Logger
	service ProjectionService
	codec   eventsourcing.Codec
}

func NewProjection(
	logger log.Logger,
	rrs projection.ReadResumeStore,
	service ProjectionService,
	codec eventsourcing.Codec,
) Projection {
	return Projection{
		ReadResumeStore: rrs,
		logger:          logger,
		service:         service,
		codec:           codec,
	}
}

func (p Projection) Name() string {
	return "balance"
}

func (p Projection) Options() projection.Options {
	return projection.Options{}
}

func (p Projection) Handle(ctx context.Context, meta projection.MetaData, e *sink.Message) error {
	if !util.In(e.AggregateKind, event.AggregateType_Account) {
		return nil
	}

	logger := p.logger.WithTags(log.Tags{
		"projection": domain.ProjectionBalance,
		"event":      e,
	})

	aggID, err := uuid.Parse(e.AggregateID)
	if err != nil {
		return faults.Errorf("unable to parse aggregate ID: %w", err)
	}

	topic, err := util.NewPartitionedTopic(meta.Topic, meta.Partition)
	if err != nil {
		return faults.Errorf("creating partitioned topic: %s:%d", meta.Topic, meta.Partition)
	}
	key, err := projection.NewResume(topic, p.Name())
	if err != nil {
		return faults.Errorf("creating resume key: %s:%s", topic, p.Name())
	}
	m := domain.Metadata{
		AggregateID: aggID,
		EventID:     e.ID,
		ResumeKey:   key,
		ResumeToken: meta.Token,
	}

	evt, err := eventsourcing.RehydrateEvent(p.codec, e.Kind, e.Body)
	if err != nil {
		return err
	}

	switch t := evt.(type) {
	case event.AccountCreated:
		err = p.service.AccountCreated(ctx, m, t)
	case event.MoneyDeposited:
		err = p.service.MoneyDeposited(ctx, m, t)
	case event.MoneyWithdrawn:
		err = p.service.MoneyWithdrawn(ctx, m, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}
