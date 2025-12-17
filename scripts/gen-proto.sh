#!/bin/bash
# Generate Go code from Protocol Buffer definitions

set -e

PROTO_DIR="proto"
OUT_DIR="proto"

# Ensure protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed. Please install Protocol Buffers compiler."
    exit 1
fi

# Ensure protoc-gen-go and protoc-gen-go-grpc are installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

echo "Generating Go code from .proto files..."

# Generate code for each proto file
for proto_file in "$PROTO_DIR"/*.proto; do
    if [ -f "$proto_file" ]; then
        echo "Processing $proto_file..."
        protoc \
            --go_out="$OUT_DIR" \
            --go_opt=paths=source_relative \
            --go-grpc_out="$OUT_DIR" \
            --go-grpc_opt=paths=source_relative \
            "$proto_file"
    fi
done

echo "Proto code generation complete!"

