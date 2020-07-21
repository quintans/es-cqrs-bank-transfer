module github.com/quintans/es-cqrs-bank-transfer/poller

go 1.14

require (
	github.com/apache/pulsar-client-go v0.1.1
	github.com/caarlos0/env/v6 v6.3.0
	github.com/lib/pq v1.3.0
	github.com/quintans/eventstore v0.0.0-20200721134350-7b18ecd69d0a
)

// replace github.com/quintans/eventstore => ../../eventstore
