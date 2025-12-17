#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

PID_DIR="./.data/pids"

echo "=== Stack Down: Stopping Full Stack ==="
echo ""

# Function to kill process by PID file
kill_by_pid_file() {
	local pid_file=$1
	local name=$2
	
	if [ -f "$pid_file" ]; then
		local pid=$(cat "$pid_file")
		if ps -p "$pid" > /dev/null 2>&1; then
			echo "Stopping $name (PID: $pid)..."
			kill "$pid" 2>/dev/null || true
			sleep 1
			# Force kill if still running
			if ps -p "$pid" > /dev/null 2>&1; then
				kill -9 "$pid" 2>/dev/null || true
			fi
			rm -f "$pid_file"
			echo "PASS: $name stopped"
		else
			rm -f "$pid_file"
			echo "PASS: $name already stopped"
		fi
	else
		echo "WARNING: $name PID file not found"
	fi
}

# Kill all services
if [ -d "$PID_DIR" ]; then
	for pid_file in "$PID_DIR"/*.pid; do
		if [ -f "$pid_file" ]; then
			name=$(basename "$pid_file" .pid)
			kill_by_pid_file "$pid_file" "$name"
		fi
	done
fi

# Also kill by process name (fallback)
echo ""
echo "Cleaning up any remaining processes..."
pkill -f "raft-node" 2>/dev/null || true
pkill -f "order-executor" 2>/dev/null || true
pkill -f "stream-processor" 2>/dev/null || true
pkill -f "market-ingestor" 2>/dev/null || true
pkill -f "verifier" 2>/dev/null || true
pkill -f "producer" 2>/dev/null || true

sleep 2

# Stop Redpanda
echo ""
echo "Stopping Redpanda..."
make dev-down > /dev/null 2>&1 || true
echo "PASS: Redpanda stopped"

# Clean up PID directory
rm -rf "$PID_DIR"

echo ""
echo "=== Stack Down Complete ==="

