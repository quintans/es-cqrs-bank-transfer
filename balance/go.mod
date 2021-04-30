module github.com/quintans/es-cqrs-bank-transfer/balance

go 1.15

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/elastic/go-elasticsearch/v7 v7.10.0
	github.com/google/uuid v1.1.2
	github.com/labstack/echo/v4 v4.1.16
	github.com/quintans/es-cqrs-bank-transfer/account/shared v0.0.0
	github.com/quintans/eventsourcing v0.14.1
	github.com/quintans/faults v1.3.0
	github.com/quintans/toolkit v0.1.1
	github.com/sirupsen/logrus v1.6.0
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ../account/shared

// replace github.com/quintans/eventsourcing => ../../eventsourcing

// replace github.com/quintans/toolkit => ../../toolkit
