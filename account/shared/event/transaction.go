package event

import (
	"github.com/google/uuid"
	"github.com/quintans/eventsourcing"
)

type TxStatus string

const (
	AggregateType_Transaction  eventsourcing.Kind = "Transaction"
	Event_TransactionCreated   eventsourcing.Kind = "TransactionCreated"
	Event_TransactionFailed    eventsourcing.Kind = "TransactionFailed"
	Event_TransactionSucceeded eventsourcing.Kind = "TransactionSucceeded"

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

func (TransactionCreated) GetKind() eventsourcing.Kind {
	return Event_TransactionCreated
}

type TransactionFailed struct {
	Rollback bool   `json:"rollback,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func (TransactionFailed) GetKind() eventsourcing.Kind {
	return Event_TransactionFailed
}

type TransactionSucceeded struct{}

func (TransactionSucceeded) GetKind() eventsourcing.Kind {
	return Event_TransactionSucceeded
}
