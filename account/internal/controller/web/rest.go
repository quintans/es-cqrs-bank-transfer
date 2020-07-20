package web

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/usecase"
)

type Controller struct {
	accountUc usecase.AccountUsecaser
}

func NewController(accountUc usecase.AccountUsecaser) Controller {
	return Controller{
		accountUc: accountUc,
	}
}

// Hello is the greeting handler
func (ctl Controller) Hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

// Create calls the usecase to crete a new account
func (ctl Controller) Create(c echo.Context) error {
	cmd := usecase.CreateCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	id, err := ctl.accountUc.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, id)
}

func (ctl Controller) Deposit(c echo.Context) error {
	cmd := usecase.DepositCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	err := ctl.accountUc.Deposit(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func (ctl Controller) Withdraw(c echo.Context) error {
	cmd := usecase.WithdrawCommand{}
	if err := c.Bind(&cmd); err != nil {
		return err
	}
	err := ctl.accountUc.Withdraw(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

func (ctl Controller) Transfer(c echo.Context) error {
	return errors.New("Not implemented")
}
