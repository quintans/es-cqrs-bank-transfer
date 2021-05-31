package controller

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
)

type RestController struct {
	BalanceUsecase domain.BalanceUsecase
}

func (ctl RestController) Ping(c echo.Context) error {
	return c.String(http.StatusOK, "ready to server")
}

func (ctl RestController) ListAll(c echo.Context) error {
	docs, err := ctl.BalanceUsecase.ListAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, docs)
}

func (ctl RestController) GetOne(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return err
	}
	doc, err := ctl.BalanceUsecase.GetOne(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if doc.IsZero() {
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, doc)
}
