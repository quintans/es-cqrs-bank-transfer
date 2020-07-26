package entity

import "github.com/quintans/es-cqrs-bank-transfer/account/shared/event"

type Balance struct {
	ID      string       `json:"id,omitempty"`
	Version int64        `json:"version,omitempty"`
	EventID string       `json:"event_id,omitempty"`
	Status  event.Status `json:"status,omitempty"`
	Balance int64        `json:"balance,omitempty"`
	Owner   string       `json:"owner,omitempty"`
}

func (b Balance) IsZero() bool {
	return b == Balance{}
}
