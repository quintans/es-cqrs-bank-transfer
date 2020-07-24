package usecase

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
)

type BalanceUsecase struct {
	BalanceRepository domain.BalanceRepository
}

func (b BalanceUsecase) AccountCreated(ctx context.Context, eventID string, ac event.AccountCreated) error {
	lastEventID, err := b.BalanceRepository.GetEventID(ctx, ac.ID)
	if err != nil {
		return err
	}
	// guarding against redelivery
	if eventID <= lastEventID {
		return nil
	}
	e := entity.Balance{
		ID:      ac.ID,
		EventID: eventID,
		Owner:   ac.Owner,
		Status:  event.OPEN,
		Balance: ac.Money,
	}
	return b.BalanceRepository.CreateAccount(ctx, e)
}
