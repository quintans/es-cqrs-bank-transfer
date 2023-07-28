package main

import (
	"os"

	"github.com/caarlos0/env/v6"
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
	cfg := Config{}
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	Setup(cfg)
}
