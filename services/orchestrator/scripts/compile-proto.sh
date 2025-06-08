#!/bin/bash

# Script to compile proto files for the orchestrator service

set -e

# Change to the orchestrator directory
cd "$(dirname "$0")/.."

# Ensure Go bin directory is in PATH
export PATH="$(go env GOPATH)/bin:$PATH"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed."
    echo "Please install protoc:"
    echo "  macOS: brew install protobuf"
    echo "  Linux: apt-get install -y protobuf-compiler"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Create output directory if it doesn't exist
mkdir -p internal/proto/intent

# Compile the proto file
echo "Compiling intent.proto..."
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    internal/proto/intent/intent.proto

echo "Proto compilation complete!"

# Make the generated files use the correct import path
echo "Fixing import paths in generated files..."
if [ -f "internal/proto/intent/intent.pb.go" ]; then
    sed -i '' 's|github.com/quantumlayer/uos/services/orchestrator/internal/proto/intent|orchestrator/internal/proto/intent|g' internal/proto/intent/intent.pb.go
fi

if [ -f "internal/proto/intent/intent_grpc.pb.go" ]; then
    sed -i '' 's|github.com/quantumlayer/uos/services/orchestrator/internal/proto/intent|orchestrator/internal/proto/intent|g' internal/proto/intent/intent_grpc.pb.go
fi

echo "Import paths fixed in generated proto files!"