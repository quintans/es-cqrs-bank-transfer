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
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/store/mongodb"
	"github.com/quintans/faults"
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
	GrpcAddress       string `env:"ES_GRPC_ADDRESS" envDefault:":3000"`
}

func Setup(cfg *Config) {
	dbURL := fmt.Sprintf("mongodb://%s:%d/%s?replicaSet=rs0", cfg.EsHost, cfg.EsPort, cfg.EsName)
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
	esRepo := mongodb.NewStoreDB(client, cfg.EsName)
	es := eventstore.NewEventStore(esRepo, cfg.SnapshotThreshold, entity.Factory{})

	go player.StartGrpcServer(context.Background(), cfg.GrpcAddress, esRepo)

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
	e.GET("/:id", c.Balance)

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
		return faults.Errorf("failed to connect to '%s': %w", dbURL, err)
	}

	defer func() {
		if err := d.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	m, err := migrate.NewWithDatabaseInstance("file://../../migrations", "", d)
	if err != nil {
		return faults.Errorf("unable to find migration scripts: %w", err)
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return faults.Errorf("failed to migrate database: %w", err)
	}

	return nil
}
