package controller

import (
	"net/http"
	"strconv"
	"time"

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
	id := c.Param("id")
	doc, err := ctl.BalanceUsecase.GetOne(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if doc.IsZero() {
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, doc)
}

func (ctl RestController) RebuildBalance(c echo.Context) error {
	var ts time.Time

	epoch := c.QueryParams().Get("epoch")
	if epoch != "" {
		secs, err := strconv.Atoi(epoch)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		ts = time.Unix(int64(secs), 0)
	}

	err := ctl.BalanceUsecase.RebuildBalance(c.Request().Context(), ts)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}
