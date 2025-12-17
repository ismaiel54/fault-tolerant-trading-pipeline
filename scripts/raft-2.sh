#!/bin/bash
set -e

export RAFT_NODE_ID=node2
export PORT_GRPC=50071
export PORT_HTTP=8081
export RAFT_BIND_ADDR=127.0.0.1:7001
export RAFT_ADVERTISE_ADDR=127.0.0.1:7001
export RAFT_DATA_DIR=./.data/raft/node2
export RAFT_BOOTSTRAP=false
export RAFT_JOIN_ADDR=127.0.0.1:50070

exec ./bin/raft-node

