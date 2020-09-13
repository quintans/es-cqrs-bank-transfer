module github.com/quintans/es-cqrs-bank-transfer/poller

go 1.15

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/go-redsync/redsync v1.4.2
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/lib/pq v1.7.0
	github.com/nats-io/nats-streaming-server v0.18.0 // indirect
	github.com/nats-io/stan.go v0.7.0
	github.com/quintans/eventstore v0.5.0
	github.com/quintans/toolkit v0.0.3
	github.com/sirupsen/logrus v1.4.2
	google.golang.org/protobuf v1.25.0 // indirect
)
