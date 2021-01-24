package main

import (
	"os"

	"github.com/caarlos0/env/v6"
	"github.com/quintans/es-cqrs-bank-transfer/account/internal/infrastructure"
	log "github.com/sirupsen/logrus"
)

func init() {
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
