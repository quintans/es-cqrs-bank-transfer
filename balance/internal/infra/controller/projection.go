package controller

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/shared/event"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/eventsourcing/sink"
	"github.com/quintans/eventsourcing/util"
	"github.com/quintans/faults"
)

type Sequencer interface {
	GetMaxSequence(ctx context.Context) (projection.Token, error)
}

type ProjectionService interface {
	AccountCreated(ctx context.Context, m domain.Metadata, ac event.AccountCreated) error
	MoneyDeposited(ctx context.Context, m domain.Metadata, ac event.MoneyDeposited) error
	MoneyWithdrawn(ctx context.Context, m domain.Metadata, ac event.MoneyWithdrawn) error
}

type Projection struct {
	logger    log.Logger
	sequencer Sequencer
	service   ProjectionService
	codec     eventsourcing.Codec
}

func NewProjection(
	logger log.Logger,
	sequencer Sequencer,
	service ProjectionService,
	codec eventsourcing.Codec,
) Projection {
	return Projection{
		sequencer: sequencer,
		logger:    logger,
		service:   service,
		codec:     codec,
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

	ignore, err := p.ignoreEvent(ctx, m)
	if err != nil || ignore {
		return err
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

func (b Projection) GetStreamResumeToken(ctx context.Context, key projection.ResumeKey) (projection.Token, error) {
	return b.sequencer.GetMaxSequence(ctx)
}

func (b Projection) ignoreEvent(ctx context.Context, m domain.Metadata) (bool, error) {
	dbToken, err := b.GetStreamResumeToken(ctx, m.ResumeKey)
	if err != nil {
		return false, err
	}
	// guarding against redelivery
	ignore := m.ResumeToken.Sequence() <= dbToken.Sequence()
	return ignore, nil
}
