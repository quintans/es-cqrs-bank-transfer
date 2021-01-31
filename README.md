# es-cqrs-bank-transfer
PoC for CQRS and Event Sourcing

This project is a way to play around with event sourcing and CQRS.
Here I followed the more complex path, where I tried to have an architecture to handle a high throughput.
This is achieved by partitioning events and having workers handling them. These workers are then balanced over the service replicas.

This project has several moving pieces
* MongoDB: the event store database
* Account Service: the write side of things. This writes into the event store.
* Forwarder Service: Listens the event store for new events and publish them into a MQ. Publishing is partitioned over several topics with the same prefix. eg: `balance.2`.
* Balance Service: reads the MQ and updates its projection(s). Several listeners are created according to number of partitions.
* NATS: the message queue
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

create a user account with some money
```sh
curl http://localhost:8000/create\
  -H "Content-Type: application/json" \
  -d '{"owner":"Paulo", "money": 50}' 
```

The previous returns an ID. Use that for ID for the next calls.

retrieve account
```sh
curl http://localhost:8000/{id}
```

deposit money
```sh
curl http://localhost:8000/deposit\
  -H "Content-Type: application/json" \
  -d '{"id":"<ID>", "money": 100}' 
```

withdraw money
```sh
curl -H "Content-Type: application/json" \
  -d '{"id":"<ID>", "money": 20}' \
  http://localhost:8000/withdraw
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

## Balance Service

On start up it will synchronise with the event store by calling `poller` service so it is important that the `poller` service is up and running.

### API

List all
```sh
curl http://localhost:8030/
```

Rebuild Balance projection
```sh
curl http://localhost:8030/balance/rebuild
```

### Observations

To be able to able to rebuild a projection, each projection will only be running in one instance. When a projection needs to be rebuild, a notification will be sent to the MQ. The projection that the notification refers to, will stop and will replay all the events, by requesting them to the event store.

Since the poller still keeps producing messages, when need to make sure that we don't lose messages when we switch to listening to messages from the MQ. For that, when all the events from the ES for the projection are all processed, we will ask the MQ for the last message, process once more the events from the last event of the replay, process them and then switch to the MQ. Since the projectors are idempotent it will be ok.
