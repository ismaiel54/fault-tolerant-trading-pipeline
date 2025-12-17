#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

LOG_DIR="./.data/logs"
PID_DIR="./.data/pids"

# Cleanup function
cleanup() {
	echo ""
	echo "=== Cleanup ==="
	./scripts/stack-down.sh || true
}

trap cleanup EXIT

echo "=== Smoke Test ==="
echo ""

# Step 1: Doctor checks
echo "Step 1: Running doctor checks..."
if ! ./scripts/doctor.sh; then
	echo "FAIL: Prerequisites check failed"
	exit 1
fi
echo ""

# Step 2: Generate protobuf code
echo "Step 2: Generating protobuf code..."
make proto > /dev/null 2>&1
if [ $? -ne 0 ]; then
	echo "FAIL: Proto generation failed"
	exit 1
fi
echo "PASS: Proto code generated"
echo ""

# Step 3: Build services
echo "Step 3: Building services..."
make build > /dev/null 2>&1
if [ $? -ne 0 ]; then
	echo "FAIL: Build failed"
	exit 1
fi
echo "PASS: Services built"
echo ""

# Step 4: Start stack
echo "Step 4: Starting stack..."
./scripts/stack-up.sh
if [ $? -ne 0 ]; then
	echo "FAIL: Stack startup failed"
	echo "Check logs in $LOG_DIR/"
	exit 1
fi
echo ""

# Step 5: Wait for services to stabilize
echo "Step 5: Waiting for services to stabilize..."
sleep 5
echo "PASS: Services stabilized"
echo ""

# Step 6: Produce test orders
echo "Step 6: Producing test orders..."
export KAFKA_BROKERS="127.0.0.1:9092"
./bin/producer -count=50 -dup-pct=30 -seed=42 -topic=orders.commands > "$LOG_DIR/producer.log" 2>&1
if [ $? -ne 0 ]; then
	echo "FAIL: Producer failed"
	echo "Check logs: $LOG_DIR/producer.log"
	exit 1
fi
echo "PASS: Orders produced"
echo ""

# Step 7: Wait for processing
echo "Step 7: Waiting for order processing..."
sleep 10
echo "PASS: Processing complete"
echo ""

# Step 8: Verify no duplicates
echo "Step 8: Verifying no duplicates..."
./bin/verifier -window=10 -expected=35 -brokers=127.0.0.1:9092 > "$LOG_DIR/verifier.log" 2>&1
VERIFIER_EXIT=$?

if [ $VERIFIER_EXIT -ne 0 ]; then
	echo "FAIL: Verification failed"
	echo "Check logs: $LOG_DIR/verifier.log"
	echo ""
	echo "=== Verifier Output ==="
	cat "$LOG_DIR/verifier.log"
	exit 1
fi

echo "PASS: Verification passed"
echo ""

# Step 9: Cleanup
echo "Step 9: Cleaning up..."
./scripts/stack-down.sh > /dev/null 2>&1 || true
echo "PASS: Cleanup complete"
echo ""

echo "=== Smoke Test PASSED ==="
echo "Logs available in: $LOG_DIR/"
exit 0

