package event

import (
	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
)

type Status string

const (
	AggregateType_Account eventsourcing.Kind = "Account"
	Event_AccountCreated  eventsourcing.Kind = "AccountCreated"
	Event_MoneyWithdrawn  eventsourcing.Kind = "MoneyWithdrawn"
	Event_MoneyDeposited  eventsourcing.Kind = "MoneyDeposited"
	Event_OwnerUpdated    eventsourcing.Kind = "OwnerUpdated"

	OPEN   Status = "OPEN"
	CLOSED Status = "CLOSED"
	FROZEN Status = "FROZEN"
)

type AccountCreated struct {
	ID    uuid.UUID `json:"id,omitempty"`
	Money int64     `json:"money,omitempty"`
	Owner string    `json:"owner,omitempty"`
}

func (_ AccountCreated) GetKind() eventsourcing.Kind {
	return Event_AccountCreated
}

type MoneyWithdrawn struct {
	Money         int64     `json:"money,omitempty"`
	TransactionID uuid.UUID `transactionID:"money,omitempty"`
}

func (_ MoneyWithdrawn) GetKind() eventsourcing.Kind {
	return Event_MoneyWithdrawn
}

type MoneyDeposited struct {
	Money         int64     `json:"money,omitempty"`
	TransactionID uuid.UUID `transactionID:"money,omitempty"`
}

func (_ MoneyDeposited) GetKind() eventsourcing.Kind {
	return Event_MoneyDeposited
}

type OwnerUpdated struct {
	Owner string `json:"owner,omitempty"`
}

func (_ OwnerUpdated) GetKind() eventsourcing.Kind {
	return Event_OwnerUpdated
}
