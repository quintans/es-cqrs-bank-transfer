module github.com/quintans/es-cqrs-bank-transfer/balance

go 1.15

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/elastic/go-elasticsearch/v7 v7.8.0
	github.com/go-redsync/redsync v1.4.2
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/kr/pretty v0.2.0 // indirect
	github.com/labstack/echo/v4 v4.1.16
	github.com/nats-io/nats-server/v2 v2.1.7 // indirect
	github.com/nats-io/nats.go v1.10.0
	github.com/nats-io/stan.go v0.7.0
	github.com/quintans/es-cqrs-bank-transfer/account/shared v0.0.0
	github.com/quintans/eventstore v0.4.0
	github.com/quintans/toolkit v0.0.3
	github.com/sirupsen/logrus v1.6.0
	golang.org/x/sys v0.0.0-20200720211630-cb9d2d5c5666 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ../account/shared
