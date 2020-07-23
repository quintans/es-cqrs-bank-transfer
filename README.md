# es-cqrs-bank-transfer
PoC for CQRS and Event Sourcing

To build and run
```sh
docker-compose up --build
```

create a user account with some money
```sh
curl --H "Content-Type: application/json" \
  --d '{"owner":"Paulo", "amount": 50}' \
  http://localhost:8000/create
```

The previous returns an ID. Use that for ID for the next calls.

deposit money
```sh
curl --H "Content-Type: application/json" \
  --d '{"id":"8f0cf478-1be5-4660-aaa2-650b186ecbbb", "amount": 100}' \
  http://localhost:8000/deposit
```

withdraw money
```sh
curl --H "Content-Type: application/json" \
  --d '{"id":"8f0cf478-1be5-4660-aaa2-650b186ecbbb", "amount": 20}' \
  http://localhost:8000/withdraw
```
