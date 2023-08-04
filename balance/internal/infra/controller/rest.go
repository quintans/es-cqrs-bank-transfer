package controller

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/shared/utils"
	"github.com/quintans/eventsourcing/log"
)

type RestController struct {
	logger         log.Logger
	balanceService domain.BalanceService
}

func NewRestController(logger log.Logger, balanceService domain.BalanceService) RestController {
	return RestController{
		logger:         logger,
		balanceService: balanceService,
	}
}

func (ctl RestController) Ping(c echo.Context) error {
	return c.String(http.StatusOK, "ready to server")
}

func (ctl RestController) ListAll(c echo.Context) error {
	ctx := utils.LogToCtx(c.Request().Context(), ctl.logger)
	docs, err := ctl.balanceService.ListAll(ctx)
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
	ctx := utils.LogToCtx(c.Request().Context(), ctl.logger)
	doc, err := ctl.balanceService.GetOne(ctx, id)
	if err != nil {
		return err
	}
	if doc.IsZero() {
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, doc)
}
