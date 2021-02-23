package entity

import (
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore"
)

func CreateTransaction(id string, from string, to string, money int64) *Transaction {
	tx := &Transaction{}
	tx.RootAggregate = eventstore.NewRootAggregate(tx)
	tx.ApplyChange(event.TransactionCreated{
		ID:    id,
		Money: money,
		From:  from,
		To:    to,
	})
	return tx
}

func NewTransaction() *Transaction {
	tx := &Transaction{}
	tx.RootAggregate = eventstore.NewRootAggregate(tx)
	return tx
}

type Transaction struct {
	eventstore.RootAggregate
	Money         int64          `json:"balance,omitempty"`
	From          string         `json:"from,omitempty"`
	To            string         `json:"to,omitempty"`
	Status        event.TxStatus `json:"status,omitempty"`
	FailureReason string         `json:"failureReason,omitempty"`
}

func (Transaction) GetType() string {
	return event.AggregateType_Transaction
}

func (tx *Transaction) HandleEvent(e eventstore.Eventer) {
	switch t := e.(type) {
	case event.TransactionCreated:
		tx.HandleTransactionCreated(t)
	case event.TransactionFailed:
		tx.HandleTransactionFailed(t)
	case event.TransactionSucceeded:
		tx.HandleTransactionSucceeded(t)
	}
}

func (tx *Transaction) HandleTransactionCreated(e event.TransactionCreated) {
	tx.ID = e.ID
	tx.Money = e.Money
	tx.From = e.From
	tx.To = e.To
	tx.Status = event.PENDING
}

func (tx *Transaction) WithdrawFailed(reason string) bool {
	if tx.Status != event.PENDING {
		return false
	}
	tx.ApplyChange(event.TransactionFailed{
		Reason: reason,
	})
	return true
}

func (tx *Transaction) DepositFailed(reason string) bool {
	if tx.Status != event.PENDING {
		return false
	}
	tx.ApplyChange(event.TransactionFailed{
		Reason:   reason,
		Rollback: tx.From != "",
	})
	return true
}

func (tx *Transaction) HandleTransactionFailed(e event.TransactionFailed) {
	tx.Status = event.FAILED
	tx.FailureReason = e.Reason
}

func (tx *Transaction) Succeeded() bool {
	if tx.Status != event.PENDING {
		return false
	}
	tx.ApplyChange(event.TransactionSucceeded{})
	return true
}

func (tx *Transaction) HandleTransactionSucceeded(e event.TransactionSucceeded) {
	tx.Status = event.SUCCEEDED
	tx.FailureReason = ""
}