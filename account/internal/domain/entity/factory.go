package entity

import (
	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

type AggregateFactory struct {
	event.EventFactory
}

func (f AggregateFactory) New(kind string) (eventsourcing.Typer, error) {
	var e eventsourcing.Typer
	switch kind {
	case event.AggregateType_Account:
		e = NewAccount()
	case event.AggregateType_Transaction:
		e = NewTransaction()
	default:
		evt, err := f.EventFactory.New(kind)
		if err != nil {
			return nil, err
		}
		return evt, nil
	}
	if e == nil {
		return nil, faults.Errorf("Unknown aggregate kind: %s", kind)
	}
	return e, nil
}
