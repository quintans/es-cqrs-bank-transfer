package event

import (
	"fmt"

	"github.com/quintans/eventstore"
)

type Status string

const (
	OPEN   Status = "OPEN"
	CLOSED Status = "CLOSED"
	FROZEN Status = "FROZEN"
)

const (
	AggregateType_Account = "Account"
	Event_AccountCreated  = "AccountCreated"
	Event_MoneyWithdrawn  = "MoneyWithdrawn"
	Event_MoneyDeposited  = "MoneyDeposited"
	Event_OwnerUpdated    = "OwnerUpdated"
)

type AccountCreated struct {
	ID    string `json:"id,omitempty"`
	Money int64  `json:"money,omitempty"`
	Owner string `json:"owner,omitempty"`
}

func (_ AccountCreated) GetType() string {
	return "AccountCreated"
}

type MoneyWithdrawn struct {
	Money int64 `json:"money,omitempty"`
}

func (_ MoneyWithdrawn) GetType() string {
	return "MoneyWithdrawn"
}

type MoneyDeposited struct {
	Money int64 `json:"money,omitempty"`
}

func (_ MoneyDeposited) GetType() string {
	return "MoneyDeposited"
}

type OwnerUpdated struct {
	Owner string `json:"owner,omitempty"`
}

func (_ OwnerUpdated) GetType() string {
	return "OwnerUpdated"
}

type EventFactory struct{}

func (_ EventFactory) New(kind string) (eventstore.Typer, error) {
	var e eventstore.Typer
	switch kind {
	case Event_AccountCreated:
		e = &AccountCreated{}
	case Event_MoneyDeposited:
		e = &MoneyDeposited{}
	case Event_MoneyWithdrawn:
		e = &MoneyWithdrawn{}
	case Event_OwnerUpdated:
		e = &OwnerUpdated{}
	}
	if e == nil {
		return nil, fmt.Errorf("Unknown event kind: %s", kind)
	}
	return e, nil
}
