package event

type Status string

const (
	AggregateType_Account = "Account"
	Event_AccountCreated  = "AccountCreated"
	Event_MoneyWithdrawn  = "MoneyWithdrawn"
	Event_MoneyDeposited  = "MoneyDeposited"
	Event_OwnerUpdated    = "OwnerUpdated"

	OPEN   Status = "OPEN"
	CLOSED Status = "CLOSED"
	FROZEN Status = "FROZEN"
)

type AccountCreated struct {
	ID    string `json:"id,omitempty"`
	Money int64  `json:"money,omitempty"`
	Owner string `json:"owner,omitempty"`
}

func (_ AccountCreated) GetType() string {
	return Event_AccountCreated
}

type MoneyWithdrawn struct {
	Money         int64  `json:"money,omitempty"`
	TransactionID string `transactionID:"money,omitempty"`
}

func (_ MoneyWithdrawn) GetType() string {
	return Event_MoneyWithdrawn
}

type MoneyDeposited struct {
	Money         int64  `json:"money,omitempty"`
	TransactionID string `transactionID:"money,omitempty"`
}

func (_ MoneyDeposited) GetType() string {
	return Event_MoneyDeposited
}

type OwnerUpdated struct {
	Owner string `json:"owner,omitempty"`
}

func (_ OwnerUpdated) GetType() string {
	return Event_OwnerUpdated
}
