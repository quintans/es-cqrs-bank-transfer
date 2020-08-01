# es-cqrs-bank-transfer
PoC for CQRS and Event Sourcing

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
