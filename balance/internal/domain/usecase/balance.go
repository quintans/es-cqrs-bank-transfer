package usecase

import (
	"context"

	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
)

type BalanceUsecase struct {
	balanceRepository domain.BalanceRepository
}

func NewBalanceUsecase(
	balanceRepository domain.BalanceRepository,
) BalanceUsecase {
	return BalanceUsecase{
		balanceRepository: balanceRepository,
	}
}

func (b BalanceUsecase) GetOne(ctx context.Context, id string) (entity.Balance, error) {
	return b.balanceRepository.GetByID(ctx, id)
}

func (b BalanceUsecase) ListAll(ctx context.Context) ([]entity.Balance, error) {
	return b.balanceRepository.GetAllOrderByOwnerAsc(ctx)
}
