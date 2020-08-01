# es-cqrs-bank-transfer
PoC for CQRS and Event Sourcing

This project is a way to play around with event sourcing and CQRS.
Here I followed the more complex path, where I tried to have an architecture to handle a high throughput.

This project has several moving pieces
* Account Service: the right side of things. This writes into the event store.
* Poller Service: periodically pools (less 1s) the event store for new events and publish them into a MQ. Only one instance will be running at a given time.
* Balance Service: reads the MQ and updates its projection(s). The projections listeners will only be active in one of the instances. To guarantee that, we distributed locking.
* Pulsar: the message queue
* Elasticsearch: the projection database
* Redis: for distributed locking.

In a future project I will make it as simple as possible, reusing many of the things developed here, but removing the Poller service and Pulsar for delivering events.

## Running

To build and run
```sh
docker-compose up --build
```

before run  the examples bellow we need to create an index in elasticsearch.
```sh
curl -X PUT http://localhost:9200/balance
```

List all indexes
```sh
curl http://localhost:9200/_cat/indices
```

create a user account with some money
```sh
curl -H "Content-Type: application/json" \
  -d '{"owner":"Paulo", "money": 50}' \
  http://localhost:8000/create
```

The previous returns an ID. Use that for ID for the next calls.

deposit money
```sh
curl -H "Content-Type: application/json" \
  -d '{"id":"<ID>", "money": 100}' \
  http://localhost:8000/deposit
```

withdraw money
```sh
curl -H "Content-Type: application/json" \
  -d '{"id":"<ID>", "money": 20}' \
  http://localhost:8000/withdraw
```

Elasticsearch API

List all docs
```sh
curl -H "Content-Type: application/json" -d '
{
    "query": {
        "match_all": {}
    },
    "sort": [
      {
        "owner.keyword": {
          "order": "asc"
        }
      }
    ]
}' \
http://localhost:9200/balance/_search?pretty
```

Get doc by ID
```sh
curl http://localhost:9200/balance/_doc/<ID>?pretty
```

## Balance Service

### API

List all
```sh
curl http://localhost:3000/
```

### Observations

To be able to able to rebuild a projection, each projection will only be running in one instance. When a projection needs to be rebuild, a notification will be sent to the MQ. The projection that the notification refers to, will stop and will replay all the events, by requesting them to the event store.

Since the poller still keeps producing messages, when need to make sure that we don't lose messages when we switch to listening to messages from the MQ. For that, when all the events from the ES for the projection are all processed, we will ask the MQ for the last message, process once more the events from the last event of the replay, process them and then switch to the MQ. Since the projectors are idempotent it will be ok.
