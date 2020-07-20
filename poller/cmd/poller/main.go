package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v6"
	_ "github.com/lib/pq"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/poller"
)

type Config struct {
	DbUser     string `env:"DB_USER" envDefault:"root"`
	DbPassword string `env:"DB_PASSWORD" envDefault:"password"`
	DbHost     string `env:"DB_HOST"`
	DbPort     int    `env:"DB_PORT" envDefault:"5432"`
	DbName     string `env:"DB_NAME" envDefault:"accounts"`
}

func main() {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DbUser, cfg.DbPassword, cfg.DbHost, cfg.DbPort, cfg.DbName)
	tracker, err := poller.NewPgRepository(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	lm := poller.New(tracker, poller.WithStartFrom(poller.BEGINNING))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := <-sigs
		fmt.Println(sig)
		cancel()
	}()

	lm.Handle(ctx, func(ctx context.Context, e common.Event) {
		b, err := json.Marshal(e)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(b))
	})
}
