#!/bin/bash
set -e

export RAFT_NODE_ID=node3
export PORT_GRPC=50072
export PORT_HTTP=8082
export RAFT_BIND_ADDR=127.0.0.1:7002
export RAFT_ADVERTISE_ADDR=127.0.0.1:7002
export RAFT_DATA_DIR=./.data/raft/node3
export RAFT_BOOTSTRAP=false
export RAFT_JOIN_ADDR=127.0.0.1:50070

exec ./bin/raft-node

