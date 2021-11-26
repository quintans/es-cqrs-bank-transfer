module github.com/quintans/es-cqrs-bank-transfer/account

go 1.15

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/golang-migrate/migrate/v4 v4.15.1
	github.com/google/uuid v1.3.0
	github.com/labstack/echo/v4 v4.1.16
	github.com/nats-io/stan.go v0.10.0
	github.com/quintans/es-cqrs-bank-transfer/account/shared v0.0.0-00010101000000-000000000000
	github.com/quintans/eventsourcing v0.22.1
	github.com/quintans/faults v1.4.0
	github.com/quintans/toolkit v0.1.1
	github.com/sirupsen/logrus v1.8.1
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ./shared

// replace github.com/quintans/eventsourcing => ../../eventsourcing

// replace github.com/quintans/toolkit => ../../toolkit
