#!/bin/bash
set -e

# Simple script to join a node to the cluster via gRPC
# Usage: ./scripts/raft-join.sh <node_id> <raft_address> <grpc_addr_of_leader>

if [ $# -lt 3 ]; then
	echo "Usage: $0 <node_id> <raft_address> <grpc_addr_of_leader>"
	echo "Example: $0 node2 127.0.0.1:7001 127.0.0.1:50070"
	exit 1
fi

NODE_ID=$1
RAFT_ADDR=$2
GRPC_ADDR=$3

# Use grpcurl if available, otherwise print instructions
if command -v grpcurl &> /dev/null; then
	grpcurl -plaintext -d "{\"node_id\": \"$NODE_ID\", \"raft_address\": \"$RAFT_ADDR\"}" \
		$GRPC_ADDR trading.v1.RaftNodeService/Join
else
	echo "grpcurl not found. Install it or use a gRPC client to call:"
	echo "  RaftNodeService.Join"
	echo "  node_id: $NODE_ID"
	echo "  raft_address: $RAFT_ADDR"
	echo "  grpc_addr: $GRPC_ADDR"
fi

