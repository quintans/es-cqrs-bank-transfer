package event

type Status string

const (
	OPEN   Status = "OPEN"
	CLOSED Status = "CLOSED"
	FROZEN Status = "FROZEN"
)

const (
	Event_AccountCreated = "AccountCreated"
	Event_MoneyWithdrawn = "MoneyWithdrawn"
	Event_MoneyDeposited = "MoneyDeposited"
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
