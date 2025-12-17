.PHONY: fmt vet test build run-market-ingestor run-stream-processor run-order-executor run-raft-node ci clean

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Service binaries
SERVICES=market-ingestor stream-processor order-executor raft-node
BIN_DIR=bin

# Build all services
build: $(SERVICES)

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

# CI pipeline: fmt + vet + test + build
ci: fmt vet test build
	@echo "CI pipeline completed successfully"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)

# Help
help:
	@echo "Available targets:"
	@echo "  fmt                  - Format code"
	@echo "  vet                  - Run go vet"
	@echo "  test                 - Run tests"
	@echo "  build                - Build all services"
	@echo "  run-<service>        - Run a specific service"
	@echo "  ci                   - Run CI pipeline (fmt+vet+test+build)"
	@echo "  clean                - Clean build artifacts"

