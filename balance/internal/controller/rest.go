package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventstore/common"
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
	var afterEventID string

	epoch := c.QueryParams().Get("epoch")
	if epoch != "" {
		secs, err := strconv.Atoi(epoch)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		ts := time.Unix(int64(secs), 0)
		afterEventID = common.NewEventID(ts, "", 0)
	}

	err := ctl.BalanceUsecase.RebuildBalance(c.Request().Context(), afterEventID)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}
