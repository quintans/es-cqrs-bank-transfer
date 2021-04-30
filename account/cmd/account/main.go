package main

import (
	"os"

	"github.com/caarlos0/env/v6"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/quintans/eventsourcing/log"
	"github.com/sirupsen/logrus"

	"github.com/quintans/es-cqrs-bank-transfer/account/internal/infrastructure"
)

func init() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		lvl = "debug"
	}
	// parse string, this is built-in feature of logrus
	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.DebugLevel
	}
	// set global log level
	logrus.SetLevel(ll)

	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: true,
	})
}

func main() {
	cfg := &infrastructure.Config{}
	err := env.Parse(cfg)
	if err != nil {
		logrus.Fatal(err)
	}

	logger := log.NewLogrus(logrus.StandardLogger())
	infrastructure.Setup(cfg, logger)
}
