package entity

import (
	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

type AggregateFactory struct {
	event.EventFactory
}

func (f AggregateFactory) NewAggregate(kind eventsourcing.AggregateType) (eventsourcing.Aggregater, error) {
	switch kind {
	case event.AggregateType_Account:
		return NewAccount(), nil
	case event.AggregateType_Transaction:
		return NewTransaction(), nil
	default:
		return nil, faults.Errorf("unknown aggregate type: %s", kind)
	}
}
