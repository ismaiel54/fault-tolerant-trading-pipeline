# Local Ports Reference

This document lists all ports used by the system when running locally.

## Raft Nodes

| Node | gRPC Port | HTTP Port | Raft Transport Port |
|------|-----------|-----------|---------------------|
| node1 | 50070 | 8080 | 7000 |
| node2 | 50071 | 8081 | 7001 |
| node3 | 50072 | 8082 | 7002 |

## Services

| Service | gRPC Port | HTTP Port |
|---------|-----------|------------|
| market-ingestor | 50051 | 8080 |
| stream-processor | 50052 | 8080 |
| order-executor | 50053 | 8080 |
| raft-node | 50070-50072 | 8080-8082 |

**Note**: HTTP ports may conflict if multiple services run on the same machine. In practice:
- Each service should use a different HTTP port when running simultaneously
- Default HTTP port is 8080, but can be overridden via `PORT_HTTP` env var

## Infrastructure

| Component | Port | Description |
|-----------|------|-------------|
| Redpanda Kafka API | 9092 | Kafka protocol |
| Redpanda Admin | 9644 | Admin API |
| Redpanda Console | 8080 | Web UI (if enabled) |

## Port Conflicts

If you encounter port conflicts:
1. Check what's using the port: `lsof -i :PORT` (macOS) or `netstat -tulpn | grep PORT` (Linux)
2. Stop conflicting services or change ports via environment variables
3. For services, set `PORT_GRPC` and `PORT_HTTP` environment variables
4. For Raft nodes, modify the scripts in `scripts/raft-*.sh`

