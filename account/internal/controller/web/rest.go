package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Controller struct {
}

// Handler
func (ctl Controller) Hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}
