package controller

import (
	"context"
	"encoding/json"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore/common"
)

type PulsarController struct {
	BalanceUsecase domain.BalanceUsecase
}

func (p PulsarController) AccountCreated(ctx context.Context, e common.Event) error {
	ac := event.AccountCreated{}
	if err := json.Unmarshal(e.Body, &ac); err != nil {
		return err
	}
	return p.BalanceUsecase.AccountCreated(ctx, e.ID, ac)
}
