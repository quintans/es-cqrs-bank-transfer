package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/controller"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/entity"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/domain/usecase"
	"github.com/quintans/eventstore"
	"github.com/quintans/eventstore/store/mongodb"
	log "github.com/sirupsen/logrus"

	"github.com/golang-migrate/migrate/v4"
	mg "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	ApiPort           int    `env:"API_PORT" envDefault:"8000"`
	EsUser            string `env:"ES_USER" envDefault:"root"`
	EsPassword        string `env:"ES_PASSWORD" envDefault:"password"`
	EsHost            string `env:"ES_HOST"`
	EsPort            int    `env:"ES_PORT" envDefault:"27017"`
	EsName            string `env:"ES_NAME" envDefault:"accounts"`
	SnapshotThreshold uint32 `env:"SNAPSHOT_THRESHOLD" envDefault:"50"`
}

func Setup(cfg *Config) {
	dbURL := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", cfg.EsUser, cfg.EsPassword, cfg.EsHost, cfg.EsPort, cfg.EsName)
	err := migration(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	client, err := newDB(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// evenstore
	esRepo := mongodb.NewStoreDB(client, cfg.EsName, entity.Factory{})
	es := eventstore.NewEventStore(esRepo, cfg.SnapshotThreshold)

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

func newDB(dbURL string) (*mongo.Client, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	opts := options.Client().ApplyURI(dbURL)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func migration(dbURL string) error {
	p := &mg.Mongo{}
	d, err := p.Open(dbURL)
	if err != nil {
		return err
	}

	defer func() {
		if err := d.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	m, err := migrate.NewWithDatabaseInstance("file://../../migrations", "", d)
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}
