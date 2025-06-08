#!/bin/bash

# Generate documentation for QuantumLayer Platform

set -e

echo "Generating documentation for QuantumLayer Platform..."

# Create docs directory if it doesn't exist
mkdir -p docs/api

# Generate Go documentation
echo "Generating Go documentation..."
for service in orchestrator intent-processor agent-manager; do
    if [ -d "services/$service" ]; then
        echo "  - Generating docs for $service..."
        cd "services/$service"
        go doc -all > ../../docs/api/$service-godoc.txt
        cd ../..
    fi
done

# Generate API documentation if swag is installed
if command -v swag &> /dev/null; then
    echo "Generating Swagger API documentation..."
    for service in orchestrator intent-processor agent-manager; do
        if [ -d "services/$service" ]; then
            echo "  - Generating Swagger docs for $service..."
            cd "services/$service"
            swag init -g cmd/server/main.go -o ../../docs/api/$service-swagger || true
            cd ../..
        fi
    done
else
    echo "Swag not installed. Skipping Swagger documentation generation."
    echo "Install with: go install github.com/swaggo/swag/cmd/swag@latest"
fi

# Generate architecture diagrams using mermaid if available
if command -v mmdc &> /dev/null; then
    echo "Generating architecture diagrams..."
    # Create mermaid diagrams here if needed
else
    echo "Mermaid CLI not installed. Skipping diagram generation."
fi

echo "Documentation generation complete!"
echo "Generated documentation can be found in:"
echo "  - docs/api/ - API documentation"
echo "  - docs/ - General documentation"