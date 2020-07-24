package entity

import (
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore"
)

func CreateAccount(owner string, id string, money int64) *Account {
	a := &Account{
		Status:  event.OPEN,
		Balance: money,
		Owner:   owner,
	}
	a.RootAggregate = eventstore.NewRootAggregate(a, id, 0)
	a.ApplyChange(event.AccountCreated{
		ID:    id,
		Money: money,
		Owner: owner,
	})
	return a
}

func NewAccount() *Account {
	a := &Account{}
	a.RootAggregate = eventstore.NewRootAggregate(a, "", 0)
	return a
}

type Account struct {
	eventstore.RootAggregate
	Status  event.Status `json:"status,omitempty"`
	Balance int64        `json:"balance,omitempty"`
	Owner   string       `json:"owner,omitempty"`
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
