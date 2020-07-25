package infrastructure

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/usecase"
	"github.com/quintans/eventstore"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiPort           int    `env:"API_PORT" envDefault:"8000"`
	DbUser            string `env:"DB_USER" envDefault:"root"`
	DbPassword        string `env:"DB_PASSWORD" envDefault:"password"`
	DbHost            string `env:"DB_HOST"`
	DbPort            int    `env:"DB_PORT" envDefault:"5432"`
	DbName            string `env:"DB_NAME" envDefault:"accounts"`
	SnapshotThreshold int    `env:"SNAPSHOT_THRESHOLD" envDefault:"5"`
}

func Setup(cfg *Config) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DbUser, cfg.DbPassword, cfg.DbHost, cfg.DbPort, cfg.DbName)

	// evenstore
	r, err := eventstore.NewPgEsRepository(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	es := eventstore.NewESPostgreSQL(r, cfg.SnapshotThreshold)

	// Usecases
	uc := usecase.NewAccountUsecase(es)

	// Controllers
	c := controller.NewController(uc)

	// Rest server
	StartRestServer(c, cfg.ApiPort)
}

func StartRestServer(c controller.Controller, port int) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/create", c.Create)
	e.POST("/deposit", c.Deposit)
	e.POST("/withdraw", c.Withdraw)
	e.POST("/transfer", c.Transfer)

	// Start server
	address := fmt.Sprintf(":%d", port)
	e.Logger.Fatal(e.Start(address))
}
