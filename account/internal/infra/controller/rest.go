package controller

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
	"github.com/quintans/es-cqrs-bank-transfer/shared/utils"
	"github.com/quintans/eventsourcing/log"
)

type RestController struct {
	logger     log.Logger
	accService domain.AccountService
	txService  domain.TransactionService
}

func NewRestController(logger log.Logger, accountService domain.AccountService, txService domain.TransactionService) RestController {
	return RestController{
		logger:     logger,
		accService: accountService,
		txService:  txService,
	}
}

func (ctl RestController) Ping(c echo.Context) error {
	return c.String(http.StatusOK, "ready to server")
}

// Create calls the usecase to crete a new account
func (ctl RestController) Create(c echo.Context) error {
	cmd := domain.CreateAccountCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	ctx := utils.LogToCtx(c.Request().Context(), ctl.logger)
	id, err := ctl.accService.Create(ctx, cmd)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, id)
}

func (ctl RestController) Transaction(c echo.Context) error {
	cmd := domain.CreateTransactionCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	ctx := utils.LogToCtx(c.Request().Context(), ctl.logger)
	id, err := ctl.txService.Create(ctx, cmd)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, id)
}

func (ctl RestController) Balance(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return err
	}
	ctx := utils.LogToCtx(c.Request().Context(), ctl.logger)
	dto, err := ctl.accService.Balance(ctx, id)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.JSON(http.StatusOK, dto)
}

func resolveError(c echo.Context, err error) (bool, error) {
	if errors.Is(err, domain.ErrEntityNotFound) {
		e := c.String(http.StatusNotFound, err.Error())
		return true, e
	}
	return false, err
}
