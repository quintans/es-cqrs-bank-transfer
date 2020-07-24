package main

import (
	"log"

	"github.com/caarlos0/env/v6"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/infrastructure"
)

func main() {
	cfg := &infrastructure.Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	infrastructure.Setup(cfg)
}
