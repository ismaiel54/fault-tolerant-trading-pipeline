.PHONY: fmt vet test test-integration proto build run-market-ingestor run-stream-processor run-order-executor run-raft-node run-raft-1 run-raft-2 run-raft-3 ci clean dev-up dev-down topics topics-list

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Service binaries
SERVICES=market-ingestor stream-processor order-executor raft-node
BIN_DIR=bin

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@./scripts/gen-proto.sh

# Build all services
build: proto $(SERVICES)

$(SERVICES):
	@echo "Building $@..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$@ ./cmd/$@

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-integration:
	@echo "Running integration tests..."
	@INTEGRATION=1 $(GOTEST) -v ./internal/it/...

# Run individual services
run-market-ingestor: market-ingestor
	@echo "Running market-ingestor..."
	./$(BIN_DIR)/market-ingestor

run-stream-processor: stream-processor
	@echo "Running stream-processor..."
	./$(BIN_DIR)/stream-processor

run-order-executor: order-executor
	@echo "Running order-executor..."
	./$(BIN_DIR)/order-executor

run-raft-node: raft-node
	@echo "Running raft-node..."
	./$(BIN_DIR)/raft-node

run-raft-1: raft-node
	@echo "Running raft-node 1..."
	@./scripts/raft-1.sh

run-raft-2: raft-node
	@echo "Running raft-node 2..."
	@./scripts/raft-2.sh

run-raft-3: raft-node
	@echo "Running raft-node 3..."
	@./scripts/raft-3.sh

# CI pipeline: fmt + vet + test + proto + build
ci: fmt vet test proto build
	@echo "CI pipeline completed successfully"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)

# Development environment
dev-up:
	@echo "Starting Redpanda stack..."
	@./scripts/dev-up.sh

dev-down:
	@echo "Stopping Redpanda stack..."
	@./scripts/dev-down.sh

topics:
	@echo "Creating Kafka topics..."
	@./scripts/topics-create.sh

topics-list:
	@echo "Listing Kafka topics..."
	@./scripts/topics-list.sh

# Help
help:
	@echo "Available targets:"
	@echo "  fmt                  - Format code"
	@echo "  vet                  - Run go vet"
	@echo "  test                 - Run tests"
	@echo "  proto                - Generate protobuf code"
	@echo "  build                - Build all services"
	@echo "  run-<service>        - Run a specific service"
	@echo "  ci                   - Run CI pipeline (fmt+vet+test+proto+build)"
	@echo "  clean                - Clean build artifacts"
	@echo "  dev-up               - Start Redpanda stack"
	@echo "  dev-down             - Stop Redpanda stack"
	@echo "  topics               - Create Kafka topics"
	@echo "  topics-list          - List Kafka topics"

