# Fault-Tolerant Trading Pipeline

Distributed trading pipeline built with Go. Four microservices using gRPC for communication and Kafka topics for streaming, with health checks and graceful shutdown.

## Architecture

The system consists of 4 microservices:

- **market-ingestor**: Ingests market data and publishes ticks to Kafka
- **stream-processor**: Consumes ticks, processes them, and produces order commands
- **order-executor**: Consumes order commands and produces order events
- **raft-node**: Provides consensus and replication for fault tolerance

## Features

- gRPC + Protocol Buffers for inter-service communication
- Kafka (Redpanda) for streaming market data and orders
- Structured logging with zap
- Health checks (HTTP `/healthz` + gRPC health service)
- Graceful shutdown handling
- Environment-based configuration
- Docker Compose for local development
- CI/CD pipeline

## Prerequisites

- Go 1.21 or later
- Make
- Docker (for Redpanda stack)

## Quick Start

### Start Redpanda Stack

```bash
make dev-up
make topics
```

This starts Redpanda and creates the required topics:
- `market.ticks` (3 partitions, 1 replica)
- `orders.commands` (3 partitions, 1 replica)
- `orders.events` (3 partitions, 1 replica)

### Build all services

```bash
make build
```

### Run the pipeline locally

In separate terminals:

```bash
# Terminal 1: Stream processor
make run-stream-processor

# Terminal 2: Order executor
make run-order-executor

# Terminal 3: Market ingestor (produces ticks)
make run-market-ingestor
```

### Observe the pipeline

- **Redpanda Console**: http://localhost:8080 (if enabled)
- **List topics**: `make topics-list`
- **Stop Redpanda**: `make dev-down`

## Configuration

Services can be configured via environment variables:

- `PORT_GRPC`: gRPC server port (default: 50051)
- `PORT_HTTP`: HTTP server port (default: 8080)
- `LOG_LEVEL`: Log level - debug, info, warn, error (default: info)
- `KAFKA_BROKERS`: Kafka broker addresses (default: 127.0.0.1:9092)

Example:

```bash
KAFKA_BROKERS=127.0.0.1:9092 PORT_GRPC=50051 PORT_HTTP=8080 LOG_LEVEL=debug make run-market-ingestor
```

## Health Checks

Each service exposes:

- **HTTP**: `http://localhost:8080/healthz`
- **gRPC**: Standard gRPC health service

Health checks verify that Kafka clients are connected and consumers are running.

## Development

### Project Structure

```
.
├── cmd/                    # Service entry points
│   ├── market-ingestor/
│   ├── stream-processor/
│   ├── order-executor/
│   └── raft-node/
├── internal/               # Internal packages
│   ├── config/            # Configuration management
│   ├── logging/           # Structured logging
│   ├── msg/               # Kafka messaging (franz-go)
│   └── observability/     # Health checks and metrics
├── proto/                 # Protocol Buffer definitions
├── gen/proto/             # Generated protobuf code
├── deployments/           # Docker and deployment configs
├── scripts/               # Utility scripts
└── docs/                  # Documentation
```

### Available Make Targets

- `make fmt` - Format code
- `make vet` - Run go vet
- `make test` - Run tests
- `make proto` - Generate protobuf code
- `make build` - Build all services
- `make run-<service>` - Run a specific service
- `make ci` - Run CI pipeline (fmt+vet+test+proto+build)
- `make clean` - Clean build artifacts
- `make dev-up` - Start Redpanda stack
- `make dev-down` - Stop Redpanda stack
- `make topics` - Create Kafka topics
- `make topics-list` - List Kafka topics

## Message Flow

1. **market-ingestor** produces `TickMsg` to `market.ticks` topic every 1 second
2. **stream-processor** consumes from `market.ticks`, processes ticks, and produces `OrderCmdMsg` to `orders.commands`
3. **order-executor** consumes from `orders.commands`, validates orders, and produces `OrderEventMsg` to `orders.events`

## License

MIT
