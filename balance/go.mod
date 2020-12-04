module github.com/quintans/es-cqrs-bank-transfer/balance

go 1.15

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/elastic/go-elasticsearch/v7 v7.8.0
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/labstack/echo/v4 v4.1.16
	github.com/quintans/es-cqrs-bank-transfer/account/shared v0.0.0
	github.com/quintans/eventstore v0.7.0
	github.com/quintans/toolkit v0.0.3
	github.com/sirupsen/logrus v1.6.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ../account/shared

replace github.com/quintans/eventstore => ../../eventstore
replace github.com/quintans/toolkit => ../../toolkit