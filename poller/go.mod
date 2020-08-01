module github.com/quintans/es-cqrs-bank-transfer/poller

go 1.14

require (
	github.com/apache/pulsar-client-go v0.1.1
	github.com/caarlos0/env/v6 v6.3.0
	github.com/lib/pq v1.3.0
	github.com/quintans/eventstore v0.0.0-20200731002243-a2d48b790649
	github.com/sirupsen/logrus v1.4.2
)

replace (
	github.com/apache/pulsar-client-go => github.com/quintans/pulsar-client-go v0.1.2-0.20200723162447-5ee5bb2e794d
	github.com/quintans/eventstore => ../../eventstore
)
