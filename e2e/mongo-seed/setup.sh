#!/bin/bash

if [ -f /replicated.txt ]; then
  echo "Mongo is already set up"
else
  echo "Setting up mongo replication..."
  # Wait for few seconds until the mongo server is up
  sleep 10
  mongo m1:27017 replicate.js
  echo "Replication done..."
  # Wait for few seconds until replication takes effect
  sleep 15
  touch /replicated.txt
fi
