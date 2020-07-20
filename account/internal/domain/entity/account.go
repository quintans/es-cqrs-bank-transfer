package entity

import "github.com/quintans/eventstore"

type Status string

const (
	OPEN   Status = "OPEN"
	CLOSED Status = "CLOSED"
	FROZEN Status = "FROZEN"
)

type AccountCreated struct {
	ID    string `json:"id,omitempty"`
	Money int64  `json:"money,omitempty"`
	Owner string `json:"owner,omitempty"`
}

type MoneyWithdrawn struct {
	Money int64 `json:"money,omitempty"`
}

type MoneyDeposited struct {
	Money int64 `json:"money,omitempty"`
}

type OwnerUpdated struct {
	Owner string `json:"owner,omitempty"`
}

func CreateAccount(owner string, id string, money int64) *Account {
	a := &Account{
		Status:  OPEN,
		Balance: money,
		Owner:   owner,
	}
	a.RootAggregate = eventstore.NewRootAggregate(a, id, 0)
	a.ApplyChange(AccountCreated{
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
	Status  Status `json:"status,omitempty"`
	Balance int64  `json:"balance,omitempty"`
	Owner   string `json:"owner,omitempty"`
}

func (a *Account) HandleAccountCreated(event AccountCreated) {
	a.ID = event.ID
	a.Balance = event.Money
	a.Owner = event.Owner
	// this reflects that we are handling domain events and NOT property events
	a.Status = OPEN
}

func (a *Account) HandleMoneyDeposited(event MoneyDeposited) {
	a.Balance += event.Money
}

func (a *Account) HandleMoneyWithdrawn(event MoneyWithdrawn) {
	a.Balance -= event.Money
}

func (a *Account) HandleOwnerUpdated(event OwnerUpdated) {
	a.Owner = event.Owner
}

func (a *Account) Withdraw(money int64) bool {
	if a.Balance >= money {
		a.ApplyChange(MoneyWithdrawn{Money: money})
		return true
	}
	return false
}

func (a *Account) Deposit(money int64) {
	a.ApplyChange(MoneyDeposited{Money: money})
}

func (a *Account) UpdateOwner(owner string) {
	a.ApplyChange(OwnerUpdated{Owner: owner})
}
