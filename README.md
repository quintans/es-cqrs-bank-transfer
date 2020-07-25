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

We use Pulsar consumers with `key_shared` to load balance into as many consumers has we would wish.
> With `key_shared`, the same aggregate is always handled by the same instance
