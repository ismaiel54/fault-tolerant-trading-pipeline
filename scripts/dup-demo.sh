#!/bin/bash
# Demo script to show idempotency: publish duplicate order commands and verify only one event is emitted

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Phase 4 Idempotency Demo"
echo "========================"
echo ""
echo "This script demonstrates that duplicate order commands produce only one event."
echo ""

# Check if Redpanda is running
if ! docker compose -f "$PROJECT_ROOT/deployments/docker-compose.yml" ps redpanda | grep -q "Up"; then
  echo "Error: Redpanda is not running. Run 'make dev-up' first."
  exit 1
fi

# Build order-executor if needed
if [ ! -f "$PROJECT_ROOT/bin/order-executor" ]; then
  echo "Building order-executor..."
  cd "$PROJECT_ROOT"
  make build
fi

echo "1. Starting order-executor in background..."
cd "$PROJECT_ROOT"
./bin/order-executor > /tmp/order-executor.log 2>&1 &
ORDER_EXECUTOR_PID=$!
echo "   PID: $ORDER_EXECUTOR_PID"

# Wait for it to start
sleep 3

echo ""
echo "2. Publishing duplicate order commands to orders.commands topic..."
echo "   (Using rpk to publish directly)"

# Create a test order command JSON
ORDER_CMD='{
  "event_id": "evt-demo-123",
  "order_id": "ord-demo-123",
  "symbol": "AAPL",
  "side": "BUY",
  "qty": 10,
  "price": 150.0,
  "ts_unix_millis": '$(date +%s000)'
}'

# Publish first time
echo "$ORDER_CMD" | docker compose -f "$PROJECT_ROOT/deployments/docker-compose.yml" exec -T redpanda rpk topic produce orders.commands --key "ord-demo-123"
echo "   Published command #1"

sleep 1

# Publish duplicate (same order_id)
echo "$ORDER_CMD" | docker compose -f "$PROJECT_ROOT/deployments/docker-compose.yml" exec -T redpanda rpk topic produce orders.commands --key "ord-demo-123"
echo "   Published command #2 (duplicate)"

sleep 2

echo ""
echo "3. Checking orders.events topic for emitted events..."
echo "   (Should see only ONE event for order_id=ord-demo-123)"
echo ""

# Consume events
docker compose -f "$PROJECT_ROOT/deployments/docker-compose.yml" exec -T redpanda rpk topic consume orders.events --num 10 --format '%k %v\n' | grep -A 5 "ord-demo-123" || echo "   (No events found yet - may need to wait a bit longer)"

echo ""
echo "4. Checking order-executor logs for duplicate detection..."
echo ""
tail -20 /tmp/order-executor.log | grep -E "(duplicate|order command processed|order_id.*ord-demo)" || echo "   (Check /tmp/order-executor.log for details)"

echo ""
echo "5. Stopping order-executor..."
kill $ORDER_EXECUTOR_PID 2>/dev/null || true
wait $ORDER_EXECUTOR_PID 2>/dev/null || true

echo ""
echo "Demo complete!"
echo ""
echo "Expected behavior:"
echo "  - First command: processed, outbox event created"
echo "  - Second command: detected as duplicate, skipped"
echo "  - Only ONE event published to orders.events"

