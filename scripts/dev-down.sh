#!/bin/bash
# Stop Redpanda stack

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/deployments"

echo "Stopping Redpanda stack..."
docker compose down

echo "Redpanda stack stopped"

