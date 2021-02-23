#!/bin/bash
sleep 10

mongo --host m1:27017 <<EOF
rs.initiate({
      _id: "rs0",
      version: 1,
      members: [
         { _id: 0, host : "m1:27017" }
      ]
   }
)
EOF
