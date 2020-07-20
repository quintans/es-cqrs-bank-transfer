package infrastructure

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-money-transfer/account/internal/controller/web"
)

func StartRestServer() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	c := web.Controller{}
	e.GET("/", c.Hello)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
