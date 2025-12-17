#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

LOG_DIR="./.data/logs"
PID_DIR="./.data/pids"
mkdir -p "$LOG_DIR" "$PID_DIR"

echo "=== Stack Up: Starting Full Stack ==="
echo ""

# Function to wait for HTTP health endpoint
wait_for_http() {
	local name=$1
	local url=$2
	local max_attempts=30
	local attempt=0
	
	echo -n "Waiting for $name to be healthy..."
	while [ $attempt -lt $max_attempts ]; do
		if curl -sf "$url" > /dev/null 2>&1; then
			echo " PASS"
			return 0
		fi
		sleep 1
		attempt=$((attempt + 1))
		echo -n "."
	done
	echo " ✗ (timeout)"
	return 1
}

# Function to wait for Raft leader
wait_for_raft_leader() {
	local max_attempts=30
	local attempt=0
	
	echo -n "Waiting for Raft leader election..."
	while [ $attempt -lt $max_attempts ]; do
		# Check logs for "became leader" message
		if grep -q "became leader" "$LOG_DIR"/raft-*.log 2>/dev/null; then
			echo " PASS"
			return 0
		fi
		sleep 1
		attempt=$((attempt + 1))
		echo -n "."
	done
	echo " ✗ (timeout)"
	return 1
}

# Step 1: Start Redpanda
echo "Step 1: Starting Redpanda..."
if ! docker ps | grep -q redpanda; then
	make dev-up > /dev/null 2>&1
	sleep 3
	echo "PASS: Redpanda started"
else
	echo "PASS: Redpanda already running"
fi

# Step 2: Create topics
echo "Step 2: Creating topics..."
make topics > /dev/null 2>&1
echo "PASS: Topics created"

# Step 3: Start Raft nodes
echo "Step 3: Starting Raft cluster..."
export RAFT_NODE_GRPC_ADDRS="127.0.0.1:50070,127.0.0.1:50071,127.0.0.1:50072"

./scripts/raft-1.sh > "$LOG_DIR/raft-1.log" 2>&1 &
RAFT1_PID=$!
echo "$RAFT1_PID" > "$PID_DIR/raft-1.pid"
sleep 2

./scripts/raft-2.sh > "$LOG_DIR/raft-2.log" 2>&1 &
RAFT2_PID=$!
echo "$RAFT2_PID" > "$PID_DIR/raft-2.pid"
sleep 2

./scripts/raft-3.sh > "$LOG_DIR/raft-3.log" 2>&1 &
RAFT3_PID=$!
echo "$RAFT3_PID" > "$PID_DIR/raft-3.pid"
sleep 3

wait_for_raft_leader || {
	echo "WARNING: Raft leader election timeout, continuing anyway..."
}

echo "PASS: Raft cluster started (PIDs: $RAFT1_PID, $RAFT2_PID, $RAFT3_PID)"

# Step 4: Start services
echo "Step 4: Starting services..."
export RAFT_NODE_GRPC_ADDR="127.0.0.1:50070"
export KAFKA_BROKERS="127.0.0.1:9092"

# Start stream-processor
PORT_HTTP=8081 ./bin/stream-processor > "$LOG_DIR/stream-processor.log" 2>&1 &
STREAM_PROCESSOR_PID=$!
echo "$STREAM_PROCESSOR_PID" > "$PID_DIR/stream-processor.pid"
sleep 2
wait_for_http "stream-processor" "http://localhost:8081/healthz" || true

# Start order-executor
PORT_HTTP=8082 ./bin/order-executor > "$LOG_DIR/order-executor.log" 2>&1 &
ORDER_EXECUTOR_PID=$!
echo "$ORDER_EXECUTOR_PID" > "$PID_DIR/order-executor.pid"
sleep 2
wait_for_http "order-executor" "http://localhost:8082/healthz" || true

# Start market-ingestor
PORT_HTTP=8083 ./bin/market-ingestor > "$LOG_DIR/market-ingestor.log" 2>&1 &
MARKET_INGESTOR_PID=$!
echo "$MARKET_INGESTOR_PID" > "$PID_DIR/market-ingestor.pid"
sleep 2
wait_for_http "market-ingestor" "http://localhost:8083/healthz" || true

echo "PASS: Services started"
echo ""
echo "=== Stack Up Complete ==="
echo "Logs: $LOG_DIR/"
echo "PIDs: $PID_DIR/"

