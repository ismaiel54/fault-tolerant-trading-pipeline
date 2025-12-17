# Architecture Overview

## System Architecture

This is a fault-tolerant distributed trading pipeline built with Go, consisting of 4 microservices:

### Services

1. **market-ingestor** - Ingests market data from external sources
2. **stream-processor** - Processes market data streams in real-time
3. **order-executor** - Executes trading orders
4. **raft-node** - Provides consensus and replication for fault tolerance

### Communication

- **gRPC**: All inter-service communication uses gRPC with Protocol Buffers
- **HTTP**: Health check endpoints exposed via HTTP on `/healthz`
- **gRPC Health Service**: Standard gRPC health checking protocol

### Observability

- **Structured Logging**: All services use zap for structured JSON logging
- **Health Checks**: Both HTTP and gRPC health endpoints
- **Graceful Shutdown**: All services handle SIGINT/SIGTERM gracefully

### Configuration

- Environment-based configuration with sensible defaults
- Configurable ports (PORT_GRPC, PORT_HTTP)
- Configurable log levels (LOG_LEVEL)

### Deployment

- Docker Compose for local development
- Multi-stage Docker builds for optimized images
- Health checks configured for all services

## Development

### Prerequisites

- Go 1.21+
- Make
- Docker (optional, for docker-compose)

### Building

```bash
make build
```

### Running Services

```bash
make run-market-ingestor
make run-stream-processor
make run-order-executor
make run-raft-node
```

### CI Pipeline

```bash
make ci  # Runs fmt + vet + test + build
```

