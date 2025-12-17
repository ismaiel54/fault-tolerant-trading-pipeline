#!/bin/bash
set -e

export RAFT_NODE_ID=node1
export PORT_GRPC=50070
export PORT_HTTP=8080
export RAFT_BIND_ADDR=127.0.0.1:7000
export RAFT_ADVERTISE_ADDR=127.0.0.1:7000
export RAFT_DATA_DIR=./.data/raft/node1
export RAFT_BOOTSTRAP=true

exec ./bin/raft-node

