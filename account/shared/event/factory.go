package event

import (
	"github.com/quintans/eventsourcing"
	"github.com/quintans/faults"
)

type EventFactory struct{}

func (EventFactory) NewEvent(kind eventsourcing.EventKind) (eventsourcing.Typer, error) {
	var e eventsourcing.Typer
	switch kind {
	case Event_AccountCreated:
		e = &AccountCreated{}
	case Event_MoneyDeposited:
		e = &MoneyDeposited{}
	case Event_MoneyWithdrawn:
		e = &MoneyWithdrawn{}
	case Event_OwnerUpdated:
		e = &OwnerUpdated{}
	case Event_TransactionCreated:
		e = &TransactionCreated{}
	case Event_TransactionFailed:
		e = &TransactionFailed{}
	case Event_TransactionSucceeded:
		e = &TransactionSucceeded{}
	}
	if e == nil {
		return nil, faults.Errorf("Unknown event kind: %s", kind)
	}
	return e, nil
}
