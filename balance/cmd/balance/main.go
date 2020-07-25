package main

import (
	"os"

	"github.com/caarlos0/env/v6"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/infrastructure"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	cfg := &infrastructure.Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	infrastructure.Setup(cfg)
}
