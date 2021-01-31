package controller

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore"
)

type ProjectionBalance struct {
	balanceUsecase domain.BalanceUsecase
}

func NewProjectionBalance(balanceUsecase domain.BalanceUsecase) ProjectionBalance {
	return ProjectionBalance{
		balanceUsecase: balanceUsecase,
	}
}

func (p ProjectionBalance) GetName() string {
	return domain.ProjectionBalance
}

func (p ProjectionBalance) GetResumeEventIDs(ctx context.Context, aggregateTypes []string, partition uint32) (string, error) {
	lastEventID, err := p.balanceUsecase.GetLastEventID(ctx, int(partition))
	if err != nil {
		return "", err
	}
	return lastEventID, nil
}

func (p ProjectionBalance) Handler(ctx context.Context, e eventstore.Event) error {
	return p.balanceUsecase.Handler(ctx, e)
}
