package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
)

type RestController struct {
	BalanceUsecase domain.BalanceUsecase
}

func (ctl RestController) ListAll(c echo.Context) error {
	docs, err := ctl.BalanceUsecase.ListAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, docs)
}

func (ctl RestController) RebuildBalance(c echo.Context) error {
	err := ctl.BalanceUsecase.RebuildBalance(c.Request().Context())
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}
