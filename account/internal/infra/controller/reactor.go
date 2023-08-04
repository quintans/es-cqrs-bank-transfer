package controller

import (
	"context"

	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/eventsourcing/sink"
	"github.com/quintans/eventsourcing/util"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/shared/utils"
)

type Reactor struct {
	projection.ReadResumeStore

	logger    log.Logger
	txService domain.TransactionService
	codec     eventsourcing.Codec
}

func NewReactor(logger log.Logger, rrs projection.ReadResumeStore, transactionUsecase domain.TransactionService, codec eventsourcing.Codec) Reactor {
	return Reactor{
		ReadResumeStore: rrs,
		logger:          logger,
		txService:       transactionUsecase,
		codec:           codec,
	}
}

func (r Reactor) Name() string {
	return "accounts-reactor"
}

func (r Reactor) Options() projection.Options {
	return projection.Options{}
}

func (r Reactor) Handle(ctx context.Context, meta projection.MetaData, e *sink.Message) error {
	if !util.In(e.Kind, event.Event_TransactionCreated, event.Event_TransactionFailed) {
		return nil
	}

	logger := r.logger.WithTags(log.Tags{
		"event": e,
	})

	evt, err := eventsourcing.RehydrateEvent(r.codec, e.Kind, e.Body)
	if err != nil {
		return err
	}

	topic, err := util.NewPartitionedTopic(meta.Topic, meta.Partition)
	if err != nil {
		return faults.Errorf("creating partitioned topic: %s:%d", meta.Topic, meta.Partition)
	}
	key, err := projection.NewResume(topic, r.Name())
	if err != nil {
		return faults.Errorf("creating resume key: %s:%s", topic, r.Name())
	}

	ctx = utils.LogToCtx(ctx, r.logger)
	switch t := evt.(type) {
	case event.TransactionCreated:
		err = r.txService.TransactionCreated(ctx, key, meta.Token, t)
	case event.TransactionFailed:
		var aggID uuid.UUID
		aggID, err = uuid.Parse(e.AggregateID)
		if err != nil {
			return faults.Errorf("unable to parse aggregate ID: %w", err)
		}
		err = r.txService.TransactionFailed(ctx, key, meta.Token, aggID, t)
	default:
		logger.Warnf("Unknown event type: %s\n", e.Kind)
	}
	return err
}
