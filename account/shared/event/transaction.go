package event

import "github.com/google/uuid"

type TxStatus string

const (
	AggregateType_Transaction  = "Transaction"
	Event_TransactionCreated   = "TransactionCreated"
	Event_TransactionFailed    = "TransactionFailed"
	Event_TransactionSucceeded = "TransactionSucceeded"

	PENDING   TxStatus = "PENDING"
	FAILED    TxStatus = "FAILED"
	SUCCEEDED TxStatus = "SUCCEEDED"
)

type TransactionCreated struct {
	ID    uuid.UUID `json:"id,omitempty"`
	Money int64     `json:"money,omitempty"`
	From  uuid.UUID `json:"from,omitempty"`
	To    uuid.UUID `json:"to,omitempty"`
}

func (TransactionCreated) GetType() string {
	return Event_TransactionCreated
}

type TransactionFailed struct {
	Rollback bool   `json:"rollback,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func (TransactionFailed) GetType() string {
	return Event_TransactionFailed
}

type TransactionSucceeded struct{}

func (TransactionSucceeded) GetType() string {
	return Event_TransactionSucceeded
}
