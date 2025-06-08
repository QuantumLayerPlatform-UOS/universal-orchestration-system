# Orchestrator Service Scripts

This directory contains utility scripts for the orchestrator service.

## Available Scripts

### compile-proto.sh
Compiles the proto files for the orchestrator service and generates Go code.

```bash
./scripts/compile-proto.sh
```

This script:
- Checks for required dependencies (protoc, protoc-gen-go, protoc-gen-go-grpc)
- Installs missing Go protoc plugins if needed
- Compiles the intent.proto file
- Fixes import paths in generated files to use local module paths

### fix-imports.sh
Fixes all import paths in Go files to use the local module name instead of the GitHub path.

```bash
./scripts/fix-imports.sh
```

This script:
- Finds all Go files in the orchestrator service
- Replaces "github.com/quantumlayer/uos/services/orchestrator/" with "orchestrator/"
- Skips vendor directory if it exists

## Prerequisites

- Go 1.21 or higher
- protoc (Protocol Buffers compiler)
  - macOS: `brew install protobuf`
  - Linux: `apt-get install -y protobuf-compiler`

The scripts will automatically install the required Go protoc plugins if they're not already installed.