package entity

import (
	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
)

func CreateTransaction(id uuid.UUID, from uuid.UUID, to uuid.UUID, money int64) *Transaction {
	tx := &Transaction{}
	tx.RootAggregate = eventsourcing.NewRootAggregate(tx)
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
	tx.RootAggregate = eventsourcing.NewRootAggregate(tx)
	return tx
}

type Transaction struct {
	eventsourcing.RootAggregate
	ID            uuid.UUID      `json:"id"`
	Money         int64          `json:"balance,omitempty"`
	From          uuid.UUID      `json:"from,omitempty"`
	To            uuid.UUID      `json:"to,omitempty"`
	Status        event.TxStatus `json:"status,omitempty"`
	FailureReason string         `json:"failureReason,omitempty"`
}

func (Transaction) GetType() string {
	return event.AggregateType_Transaction
}

func (tx Transaction) GetID() string {
	return tx.ID.String()
}

func (tx *Transaction) HandleEvent(e eventsourcing.Eventer) {
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
		Rollback: tx.From != uuid.Nil,
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
