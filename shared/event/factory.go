package event

import (
	"github.com/quintans/eventsourcing"
	"github.com/quintans/eventsourcing/encoding/jsoncodec"
)

func NewJSONCodec() *jsoncodec.Codec {
	c := jsoncodec.New()
	c.RegisterFactory(Event_AccountCreated, func() eventsourcing.Kinder {
		return &AccountCreated{}
	})
	c.RegisterFactory(Event_MoneyDeposited, func() eventsourcing.Kinder {
		return &MoneyDeposited{}
	})
	c.RegisterFactory(Event_MoneyWithdrawn, func() eventsourcing.Kinder {
		return &MoneyWithdrawn{}
	})
	c.RegisterFactory(Event_OwnerUpdated, func() eventsourcing.Kinder {
		return &OwnerUpdated{}
	})
	c.RegisterFactory(Event_TransactionCreated, func() eventsourcing.Kinder {
		return &TransactionCreated{}
	})
	c.RegisterFactory(Event_TransactionFailed, func() eventsourcing.Kinder {
		return &TransactionFailed{}
	})
	c.RegisterFactory(Event_TransactionSucceeded, func() eventsourcing.Kinder {
		return &TransactionSucceeded{}
	})
	return c
}
