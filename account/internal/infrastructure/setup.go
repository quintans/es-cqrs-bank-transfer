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
	EsUser            string `env:"ES_USER" envDefault:"root"`
	EsPassword        string `env:"ES_PASSWORD" envDefault:"password"`
	EsHost            string `env:"ES_HOST"`
	EsPort            int    `env:"ES_PORT" envDefault:"5432"`
	EsName            string `env:"ES_NAME" envDefault:"accounts"`
	SnapshotThreshold int    `env:"SNAPSHOT_THRESHOLD" envDefault:"5"`
}

func Setup(cfg *Config) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.EsUser, cfg.EsPassword, cfg.EsHost, cfg.EsPort, cfg.EsName)

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
