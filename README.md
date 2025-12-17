# Fault-Tolerant Trading Pipeline

A production-quality distributed trading pipeline built with Go, featuring 4 microservices with gRPC communication, health checks, and graceful shutdown.

## Architecture

The system consists of 4 microservices:

- **market-ingestor**: Ingests market data from external sources
- **stream-processor**: Processes market data streams in real-time
- **order-executor**: Executes trading orders
- **raft-node**: Provides consensus and replication for fault tolerance

## Features

- ✅ gRPC + Protocol Buffers for inter-service communication
- ✅ Structured logging with zap
- ✅ Health checks (HTTP `/healthz` + gRPC health service)
- ✅ Graceful shutdown handling
- ✅ Environment-based configuration
- ✅ Docker Compose for local development
- ✅ CI/CD pipeline

## Prerequisites

- Go 1.21 or later
- Make
- Docker (optional, for docker-compose)

## Quick Start

### Build all services

```bash
make build
```

### Run a service

```bash
make run-market-ingestor
make run-stream-processor
make run-order-executor
make run-raft-node
```

### Run CI pipeline

```bash
make ci  # Runs fmt + vet + test + build
```

## Configuration

Services can be configured via environment variables:

- `PORT_GRPC`: gRPC server port (default: 50051)
- `PORT_HTTP`: HTTP server port (default: 8080)
- `LOG_LEVEL`: Log level - debug, info, warn, error (default: info)

Example:

```bash
PORT_GRPC=50051 PORT_HTTP=8080 LOG_LEVEL=debug make run-market-ingestor
```

## Health Checks

Each service exposes:

- **HTTP**: `http://localhost:8080/healthz`
- **gRPC**: Standard gRPC health service

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
│   └── observability/     # Health checks and metrics
├── proto/                 # Protocol Buffer definitions
├── deployments/           # Docker and deployment configs
├── scripts/               # Utility scripts
└── docs/                  # Documentation
```

### Available Make Targets

- `make fmt` - Format code
- `make vet` - Run go vet
- `make test` - Run tests
- `make build` - Build all services
- `make run-<service>` - Run a specific service
- `make ci` - Run CI pipeline (fmt+vet+test+build)
- `make clean` - Clean build artifacts

## Docker

### Build and run with Docker Compose

```bash
cd deployments
docker-compose up
```

Services will be available on:
- market-ingestor: gRPC :50051, HTTP :8080
- stream-processor: gRPC :50052, HTTP :8081
- order-executor: gRPC :50053, HTTP :8082
- raft-node: gRPC :50054, HTTP :8083

## License

MIT
