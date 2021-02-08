package main

import (
	"os"

	"github.com/caarlos0/env/v6"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/infrastructure"
	log "github.com/sirupsen/logrus"
)

func init() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		lvl = "debug"
	}
	// parse string, this is built-in feature of logrus
	ll, err := log.ParseLevel(lvl)
	if err != nil {
		ll = log.DebugLevel
	}
	// set global log level
	log.SetLevel(ll)

	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		DisableQuote: true,
	})
}

func main() {
	cfg := &infrastructure.Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	infrastructure.Setup(cfg)
}
