package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain"
)

type Controller struct {
	accountUc domain.AccountUsecaser
}

func NewController(accountUc domain.AccountUsecaser) Controller {
	return Controller{
		accountUc: accountUc,
	}
}

// Create calls the usecase to crete a new account
func (ctl Controller) Create(c echo.Context) error {
	cmd := domain.CreateCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	id, err := ctl.accountUc.Create(c.Request().Context(), cmd)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, id)
}

func (ctl Controller) Deposit(c echo.Context) error {
	cmd := domain.DepositCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	err := ctl.accountUc.Deposit(c.Request().Context(), cmd)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func (ctl Controller) Withdraw(c echo.Context) error {
	cmd := domain.WithdrawCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	err := ctl.accountUc.Withdraw(c.Request().Context(), cmd)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func (ctl Controller) Transfer(c echo.Context) error {
	return errors.New("Not implemented")
}

func (ctl Controller) Balance(c echo.Context) error {
	id := c.Param("id")
	dto, err := ctl.accountUc.Balance(c.Request().Context(), id)
	ok, err := resolveError(c, err)
	if ok || err != nil {
		return err
	}
	return c.JSON(http.StatusOK, dto)
}

func resolveError(c echo.Context, err error) (bool, error) {
	if errors.Is(err, domain.ErrUnknownAccount) {
		e := c.String(http.StatusNotFound, err.Error())
		return true, e
	}
	return false, err
}
