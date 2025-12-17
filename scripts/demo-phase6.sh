#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

LOG_DIR="./.data/logs"
mkdir -p "$LOG_DIR"

echo "=== Phase 6 Demo: Chaos Testing ==="
echo ""

# Cleanup function
cleanup() {
	echo ""
	echo "=== Cleaning up ==="
	pkill -f "raft-node" || true
	pkill -f "order-executor" || true
	pkill -f "stream-processor" || true
	pkill -f "market-ingestor" || true
	pkill -f "verifier" || true
	sleep 2
}

trap cleanup EXIT

# Step 1: Start Redpanda
echo "Step 1: Starting Redpanda..."
make dev-up > /dev/null 2>&1
sleep 3
make topics > /dev/null 2>&1
echo "✓ Redpanda started"

# Step 2: Build services
echo "Step 2: Building services..."
make build > /dev/null 2>&1
echo "✓ Services built"

# Step 3: Start Raft nodes
echo "Step 3: Starting Raft cluster..."
export RAFT_NODE_GRPC_ADDRS="127.0.0.1:50070,127.0.0.1:50071,127.0.0.1:50072"

./scripts/raft-1.sh > "$LOG_DIR/raft-1.log" 2>&1 &
RAFT1_PID=$!
sleep 2

./scripts/raft-2.sh > "$LOG_DIR/raft-2.log" 2>&1 &
RAFT2_PID=$!
sleep 2

./scripts/raft-3.sh > "$LOG_DIR/raft-3.log" 2>&1 &
RAFT3_PID=$!
sleep 5

echo "✓ Raft cluster started (PIDs: $RAFT1_PID, $RAFT2_PID, $RAFT3_PID)"

# Step 4: Start services
echo "Step 4: Starting trading pipeline..."
export RAFT_NODE_GRPC_ADDR="127.0.0.1:50070"
export KAFKA_BROKERS="127.0.0.1:9092"

./bin/stream-processor > "$LOG_DIR/stream-processor.log" 2>&1 &
STREAM_PROCESSOR_PID=$!
sleep 2

./bin/order-executor > "$LOG_DIR/order-executor.log" 2>&1 &
ORDER_EXECUTOR_PID=$!
sleep 2

./bin/market-ingestor > "$LOG_DIR/market-ingestor.log" 2>&1 &
MARKET_INGESTOR_PID=$!
sleep 3

echo "✓ Trading pipeline started"

# Scenario 1: Partition leader during Apply
echo ""
echo "=== Scenario 1: Partition leader during Apply ==="
echo "Enabling chaos on leader (drop 100% of Apply requests)..."

# Find leader by checking logs
LEADER_NODE="node1"
if grep -q "became leader" "$LOG_DIR/raft-2.log" 2>/dev/null; then
	LEADER_NODE="node2"
elif grep -q "became leader" "$LOG_DIR/raft-3.log" 2>/dev/null; then
	LEADER_NODE="node3"
fi

echo "Leader detected: $LEADER_NODE"

# Kill the leader process temporarily (simulate partition)
if [ "$LEADER_NODE" = "node1" ]; then
	kill -STOP $RAFT1_PID
	echo "Paused leader node1"
	sleep 2
	kill -CONT $RAFT1_PID
	echo "Resumed leader node1"
elif [ "$LEADER_NODE" = "node2" ]; then
	kill -STOP $RAFT2_PID
	echo "Paused leader node2"
	sleep 2
	kill -CONT $RAFT2_PID
	echo "Resumed leader node2"
else
	kill -STOP $RAFT3_PID
	echo "Paused leader node3"
	sleep 2
	kill -CONT $RAFT3_PID
	echo "Resumed leader node3"
fi

sleep 5

# Scenario 2: Kill leader mid-run
echo ""
echo "=== Scenario 2: Kill leader mid-run ==="
echo "Enabling CHAOS_EXIT_ON_LEADER for node1..."

# Restart node1 with exit-on-leader
kill $RAFT1_PID 2>/dev/null || true
sleep 2

export CHAOS_ENABLED=true
export CHAOS_EXIT_ON_LEADER=true
export RAFT_NODE_ID=node1
export PORT_GRPC=50070
export PORT_HTTP=8080
export RAFT_BIND_ADDR=127.0.0.1:7000
export RAFT_ADVERTISE_ADDR=127.0.0.1:7000
export RAFT_DATA_DIR=./.data/raft/node1
export RAFT_BOOTSTRAP=true

./bin/raft-node > "$LOG_DIR/raft-1.log" 2>&1 &
RAFT1_PID=$!
sleep 5

echo "✓ Node1 restarted with exit-on-leader"
sleep 5

# Scenario 3: Flapping network
echo ""
echo "=== Scenario 3: Flapping network (delay + partial drops) ==="
echo "Enabling chaos on node2 and node3..."

# Restart node2 with chaos
kill $RAFT2_PID 2>/dev/null || true
sleep 2

export CHAOS_ENABLED=true
export CHAOS_PROFILE="drop-pct=30,delay=50-250"
export CHAOS_TARGET_NODE_ID=node2
export RAFT_NODE_ID=node2
export PORT_GRPC=50071
export PORT_HTTP=8081
export RAFT_BIND_ADDR=127.0.0.1:7001
export RAFT_ADVERTISE_ADDR=127.0.0.1:7001
export RAFT_DATA_DIR=./.data/raft/node2
export RAFT_BOOTSTRAP=false
export RAFT_JOIN_ADDR=127.0.0.1:50070

./bin/raft-node > "$LOG_DIR/raft-2.log" 2>&1 &
RAFT2_PID=$!
sleep 2

# Restart node3 with chaos
kill $RAFT3_PID 2>/dev/null || true
sleep 2

export CHAOS_TARGET_NODE_ID=node3
export RAFT_NODE_ID=node3
export PORT_GRPC=50072
export PORT_HTTP=8082
export RAFT_BIND_ADDR=127.0.0.1:7002
export RAFT_ADVERTISE_ADDR=127.0.0.1:7002
export RAFT_DATA_DIR=./.data/raft/node3

./bin/raft-node > "$LOG_DIR/raft-3.log" 2>&1 &
RAFT3_PID=$!
sleep 5

echo "✓ Chaos enabled on node2 and node3"

# Let system run for a bit
echo ""
echo "Running pipeline for 30 seconds..."
sleep 30

# Step 5: Verify no duplicates
echo ""
echo "=== Verification: Checking for duplicates ==="
echo "Consuming events for 10 seconds..."

timeout 10s ./bin/verifier 10 127.0.0.1:9092 || VERIFIER_EXIT=$?

if [ "${VERIFIER_EXIT:-0}" = "0" ]; then
	echo ""
	echo "✅ VERIFICATION PASSED: No duplicates detected!"
	exit 0
else
	echo ""
	echo "❌ VERIFICATION FAILED: Duplicates detected!"
	exit 1
fi

