#!/bin/bash
# Start Redpanda stack for local development

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/deployments"

echo "Starting Redpanda stack..."
docker compose up -d

echo "Waiting for Redpanda to be healthy..."
timeout=30
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if docker compose ps redpanda | grep -q "healthy"; then
    echo "Redpanda is healthy!"
    exit 0
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done

echo "Redpanda started (may still be initializing)"
docker compose ps

