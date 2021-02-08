# es-cqrs-bank-transfer
PoC for CQRS and Event Sourcing

This project is a way to play around with event sourcing and CQRS.
Here I followed the more complex path, where I tried to have an architecture to handle a high throughput.
This is achieved by partitioning events and having workers handling them. These workers are then balanced over the service replicas.

This project has several moving pieces
* MongoDB: the event store database
* Account Service: the write side of things. This service writes into the event store. Additionally listens the database for new event records and publish them into a MQ. Publishing is partitioned over several topics with the same prefix. eg: `balance.1` and `balance.2`. Finally it also exposes gRPC endpoints for projections in other services to be able to replay events and rebuild themselves.
* Balance Service: reads the MQ and updates its projection(s). Several listeners are created according to number of partitions.
* NATS: the message bus
* Elasticsearch: the projection database
* Redis: for distributed locking.

## Running

To build and run
```sh
docker-compose up --build
```

> The elasticsearch commands can also be run in http://localhost:5601/app/kibana#/dev_tools/console


before run the examples bellow we need to create an index in elasticsearch.
```sh
curl -XPUT "http://elastic:9200/balance" \
-H 'Content-Type: application/json' -d'
{  
  "mappings": {    
    "properties": {      
      "event_id": {        
        "type": "keyword"      
      },   
      "owner": {        
        "type": "keyword"      
      }    
    }  
  }
}'
```

> If you want to start from a clean index, just delete the existing index
> 
> ```sh
> curl -X DELETE http://localhost:9200/balance
> ```
> and then recreate the index by running the previous command

List all indexes
```sh
curl http://localhost:9200/_cat/indices
```

create a user account
```sh
curl http://localhost:8000/accounts\
  -H "Content-Type: application/json" \
  -d '{"owner":"Paulo"}' 
```

The previous returns an ID. Use that for ID for the next calls.

retrieve account
```sh
curl http://localhost:8000/accounts/{id}
```

deposit money
```sh
curl http://localhost:8000/transactions\
  -H "Content-Type: application/json" \
  -d '{"to":"<ID>", "money": 100}' 
```

withdraw money
```sh
curl http://localhost:8000/transactions\
  -H "Content-Type: application/json" \
  -d '{"from":"<ID>", "money": 20}'
```

transfer money from one account to another

```sh
curl http://localhost:8000/transactions\
  -H "Content-Type: application/json" \
  -d '{"from":"<ID>", "to":"<ID>", "money": 20}'
```

Elasticsearch API

List all docs by owner
```sh
curl -XGET "http://elastic:9200/balance/_search?pretty" \
-H 'Content-Type: application/json' -d'
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
}'
```


Get doc by ID
```sh
curl http://localhost:9200/balance/_doc/<ID>?pretty
```

Max event ID
```sh
curl -XGET http://localhost:9200/balance/_search?pretty&size=1 \
-H 'Content-Type: application/json' -d'
{
  "sort": [
    {
      "event_id": { "order": "desc"}
    }
  ]
}'
```

Delete index
```sh
curl -X DELETE http://localhost:9200/balance
```

### API

List all
```sh
curl http://localhost:8030/
```

Rebuild Balance projection
```sh
curl http://localhost:8030/balance/rebuild
```
