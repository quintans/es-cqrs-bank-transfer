module github.com/quintans/es-cqrs-bank-transfer/poller

go 1.14

require (
	github.com/caarlos0/env/v6 v6.3.0
	github.com/lib/pq v1.3.0
	github.com/quintans/eventstore v0.0.0-20200720205834-a4b40139f841
)

// replace github.com/quintans/eventstore => ../../eventstore
