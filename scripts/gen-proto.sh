#!/bin/bash
# Generate Go code from Protocol Buffer definitions

set -e

PROTO_DIR="proto"
OUT_DIR="gen/proto"

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

# Create output directory if it doesn't exist
mkdir -p "$OUT_DIR"

# Find all .proto files recursively and generate code
find "$PROTO_DIR" -name "*.proto" -type f | while read -r proto_file; do
    echo "Processing $proto_file..."
    
    # Get relative path from proto_dir to preserve directory structure
    rel_path="${proto_file#$PROTO_DIR/}"
    proto_dir=$(dirname "$rel_path")
    
    # Create corresponding directory in output
    if [ "$proto_dir" != "." ]; then
        mkdir -p "$OUT_DIR/$proto_dir"
    fi
    
    protoc \
        --go_out="$OUT_DIR" \
        --go_opt=paths=source_relative \
        --go-grpc_out="$OUT_DIR" \
        --go-grpc_opt=paths=source_relative \
        --proto_path="$PROTO_DIR" \
        "$proto_file"
done

echo "Proto code generation complete!"
