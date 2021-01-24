module github.com/quintans/es-cqrs-bank-transfer/forwarder

go 1.15

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/nats-io/stan.go v0.7.0
	github.com/quintans/eventstore v0.7.0
	github.com/quintans/toolkit v0.0.3
	github.com/sirupsen/logrus v1.7.0
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ../account/shared

replace github.com/quintans/eventstore => ../../eventstore

replace github.com/quintans/toolkit => ../../toolkit
