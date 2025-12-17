#!/bin/bash
# Create required Kafka topics (idempotent)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/deployments"

echo "Creating topics..."

# Check if redpanda is running
if ! docker compose ps redpanda | grep -q "Up"; then
  echo "Error: Redpanda is not running. Run 'make dev-up' first."
  exit 1
fi

# Create topics using rpk inside the container
docker compose exec -T redpanda rpk topic create market.ticks --partitions 3 --replicas 1 || true
docker compose exec -T redpanda rpk topic create orders.commands --partitions 3 --replicas 1 || true
docker compose exec -T redpanda rpk topic create orders.events --partitions 3 --replicas 1 || true

echo "Topics created (or already exist):"
echo "  - market.ticks (3 partitions, 1 replica)"
echo "  - orders.commands (3 partitions, 1 replica)"
echo "  - orders.events (3 partitions, 1 replica)"

