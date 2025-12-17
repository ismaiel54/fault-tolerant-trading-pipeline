#!/bin/bash
# List Kafka topics

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/deployments"

# Check if redpanda is running
if ! docker compose ps redpanda | grep -q "Up"; then
  echo "Error: Redpanda is not running. Run 'make dev-up' first."
  exit 1
fi

echo "Listing topics..."
docker compose exec -T redpanda rpk topic list

