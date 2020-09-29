module github.com/quintans/es-cqrs-bank-transfer/account

go 1.14

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/google/uuid v1.1.1
	github.com/labstack/echo/v4 v4.1.16
	github.com/quintans/es-cqrs-bank-transfer/account/shared v0.0.0-00010101000000-000000000000
	github.com/quintans/eventstore v0.7.0
	github.com/sirupsen/logrus v1.4.2
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ./shared
