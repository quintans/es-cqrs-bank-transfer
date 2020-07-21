module github.com/quintans/es-cqrs-bank-transfer/account

go 1.14

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/google/uuid v1.1.1
	github.com/labstack/echo/v4 v4.1.16
	github.com/quintans/eventstore v0.0.0-20200721134350-7b18ecd69d0a
)

// replace github.com/quintans/eventstore => ../../eventstore
