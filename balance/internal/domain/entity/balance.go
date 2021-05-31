package entity

import (
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventsourcing/eventid"
)

type Balance struct {
	ID        uuid.UUID       `json:"id"`
	Version   int64           `json:"version"`
	EventID   eventid.EventID `json:"event_id"`
	Partition uint32          `json:"partition"`
	Status    event.Status    `json:"status,omitempty"`
	Balance   int64           `json:"balance,omitempty"`
	Owner     string          `json:"owner,omitempty"`
}

func (b Balance) IsZero() bool {
	return b.ID == uuid.Nil
}

type Base64 []byte

func (b Base64) MarshalJSON() ([]byte, error) {
	sEnc := base64.StdEncoding.EncodeToString(b)
	return []byte(`"` + sEnc + `"`), nil
}

func (b *Base64) UnmarshalJSON(data []byte) (err error) {
	// strip quotes
	data = data[1 : len(data)-1]
	sDec, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return err
	}
	*b = sDec
	return nil
}
