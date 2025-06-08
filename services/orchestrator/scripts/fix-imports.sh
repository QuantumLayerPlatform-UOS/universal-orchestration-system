#!/bin/bash

# Script to fix import paths in Go files

set -e

# Change to the orchestrator directory
cd "$(dirname "$0")/.."

echo "Fixing import paths in Go files..."

# Find all Go files and fix import paths
find . -name "*.go" -type f | while read -r file; do
    # Skip vendor directory if it exists
    if [[ "$file" == *"/vendor/"* ]]; then
        continue
    fi
    
    # Replace import paths
    sed -i '' 's|"github.com/quantumlayer/uos/services/orchestrator/internal/|"orchestrator/internal/|g' "$file"
    sed -i '' 's|"github.com/quantumlayer/uos/services/orchestrator/cmd/|"orchestrator/cmd/|g' "$file"
    
    echo "Fixed imports in: $file"
done

echo "Import path fixes complete!"