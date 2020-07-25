module github.com/quintans/es-cqrs-bank-transfer/balance

go 1.14

require (
	github.com/apache/pulsar-client-go v0.1.1
	github.com/caarlos0/env/v6 v6.3.0
	github.com/elastic/go-elasticsearch/v7 v7.8.0
	github.com/frankban/quicktest v1.10.0 // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/klauspost/compress v1.10.10 // indirect
	github.com/labstack/echo/v4 v4.1.16
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/quintans/es-cqrs-bank-transfer/account/shared v0.0.0
	github.com/quintans/eventstore v0.0.0-20200723150040-ddcf1134a535
	github.com/sirupsen/logrus v1.6.0
	github.com/yahoo/athenz v1.9.11 // indirect
	golang.org/x/sys v0.0.0-20200720211630-cb9d2d5c5666 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
)

replace github.com/quintans/es-cqrs-bank-transfer/account/shared => ../account/shared
