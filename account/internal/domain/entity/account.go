package entity

import (
	"errors"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore"
)

var (
	ErrNotEnoughFunds       = errors.New("not enough funds")
	ErrTransactionsDisabled = errors.New("transactions disabled")
)

type Account struct {
	eventstore.RootAggregate
	Status  event.Status `json:"status,omitempty"`
	Balance int64        `json:"balance,omitempty"`
	Owner   string       `json:"owner,omitempty"`
}

func NewAccount() *Account {
	a := &Account{}
	a.RootAggregate = eventstore.NewRootAggregate(a)
	return a
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

func CreateAccount(owner string, id string) *Account {
	a := &Account{
		Status: event.OPEN,
		Owner:  owner,
	}
	a.RootAggregate = eventstore.NewRootAggregate(a)
	a.ApplyChange(event.AccountCreated{
		ID:    id,
		Owner: owner,
	})
	return a
}

func (a *Account) HandleAccountCreated(e event.AccountCreated) {
	a.ID = e.ID
	a.Balance = e.Money
	a.Owner = e.Owner
	// this reflects that we are handling domain events and NOT property events
	a.Status = event.OPEN
}

func (a *Account) Withdraw(txID string, money int64) error {
	if a.Balance < money {
		return ErrNotEnoughFunds
	}
	if a.Status != event.OPEN {
		return ErrTransactionsDisabled
	}
	a.ApplyChange(event.MoneyWithdrawn{
		Money:         money,
		TransactionID: txID,
	})
	return nil
}

func (a *Account) HandleMoneyWithdrawn(event event.MoneyWithdrawn) {
	a.Balance -= event.Money
}

func (a *Account) Deposit(txID string, money int64) error {
	if a.Status != event.OPEN {
		return ErrTransactionsDisabled
	}
	a.ApplyChange(event.MoneyDeposited{
		Money:         money,
		TransactionID: txID,
	})
	return nil
}

func (a *Account) HandleMoneyDeposited(event event.MoneyDeposited) {
	a.Balance += event.Money
}

func (a *Account) UpdateOwner(owner string) {
	a.ApplyChange(event.OwnerUpdated{Owner: owner})
}

func (a *Account) HandleOwnerUpdated(event event.OwnerUpdated) {
	a.Owner = event.Owner
}
