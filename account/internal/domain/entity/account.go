package entity

import (
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore"
	"github.com/quintans/faults"
)

func CreateAccount(owner string, id string, money int64) *Account {
	a := &Account{
		Status:  event.OPEN,
		Balance: money,
		Owner:   owner,
	}
	a.RootAggregate = eventstore.NewRootAggregate(a)
	a.ApplyChange(event.AccountCreated{
		ID:    id,
		Money: money,
		Owner: owner,
	})
	return a
}

func NewAccount() *Account {
	a := &Account{}
	a.RootAggregate = eventstore.NewRootAggregate(a)
	return a
}

type Account struct {
	eventstore.RootAggregate
	Status  event.Status `json:"status,omitempty"`
	Balance int64        `json:"balance,omitempty"`
	Owner   string       `json:"owner,omitempty"`
}

func (a Account) GetType() string {
	return event.AggregateType_Account
}

func (a *Account) HandleEvent(e eventstore.Eventer) {
	switch t := e.(type) {
	case event.AccountCreated:
		a.HandleAccountCreated(t)
	case event.MoneyDeposited:
		a.HandleMoneyDeposited(t)
	case event.MoneyWithdrawn:
		a.HandleMoneyWithdrawn(t)
	case event.OwnerUpdated:
		a.HandleOwnerUpdated(t)
	}
}

func (a *Account) HandleAccountCreated(e event.AccountCreated) {
	a.ID = e.ID
	a.Balance = e.Money
	a.Owner = e.Owner
	// this reflects that we are handling domain events and NOT property events
	a.Status = event.OPEN
}

func (a *Account) HandleMoneyDeposited(event event.MoneyDeposited) {
	a.Balance += event.Money
}

func (a *Account) HandleMoneyWithdrawn(event event.MoneyWithdrawn) {
	a.Balance -= event.Money
}

func (a *Account) HandleOwnerUpdated(event event.OwnerUpdated) {
	a.Owner = event.Owner
}

func (a *Account) Withdraw(money int64) bool {
	if a.Balance >= money {
		a.ApplyChange(event.MoneyWithdrawn{Money: money})
		return true
	}
	return false
}

func (a *Account) Deposit(money int64) {
	a.ApplyChange(event.MoneyDeposited{Money: money})
}

func (a *Account) UpdateOwner(owner string) {
	a.ApplyChange(event.OwnerUpdated{Owner: owner})
}

type EventFactory struct{}

func (_ EventFactory) New(kind string) (eventstore.Typer, error) {
	var e eventstore.Typer
	switch kind {
	case "Account":
		e = NewAccount()
	case "AccountCreated":
		e = &event.AccountCreated{}
	case "MoneyDeposited":
		e = &event.MoneyDeposited{}
	case "MoneyWithdrawn":
		e = &event.MoneyWithdrawn{}
	case "OwnerUpdated":
		e = &event.OwnerUpdated{}
	default:
		return nil, faults.Errorf("Unknown event kind: %s", kind)
	}

	return e, nil
}
