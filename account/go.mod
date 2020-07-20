module github.com/quintans/es-cqrs-bank-transfer/account

go 1.14

require (
	github.com/labstack/echo/v4 v4.1.16
)

replace github.com/quintans/eventstore => ../../eventstore
