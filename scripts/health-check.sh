#!/bin/bash

# QuantumLayer Platform Health Check Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check HTTP endpoint health
check_http_health() {
    local name=$1
    local url=$2
    
    response=$(curl -s -o /dev/null -w "%{http_code}" $url 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✓${NC} $name is healthy"
        return 0
    else
        echo -e "${RED}✗${NC} $name is unhealthy (HTTP $response)"
        return 1
    fi
}

# Function to check TCP port
check_port() {
    local name=$1
    local port=$2
    
    if nc -z localhost $port 2>/dev/null; then
        echo -e "${GREEN}✓${NC} $name is accessible on port $port"
        return 0
    else
        echo -e "${RED}✗${NC} $name is not accessible on port $port"
        return 1
    fi
}

echo "======================================"
echo "QuantumLayer Platform Health Check"
echo "======================================"
echo ""

# Check core services
echo "Core Services:"
check_http_health "Orchestrator" "http://localhost:8081/health"
check_http_health "Intent Processor" "http://localhost:8083/health"
check_http_health "Agent Manager" "http://localhost:8085/health"
echo ""

# Check infrastructure services
echo "Infrastructure Services:"
check_port "PostgreSQL" 5432
check_port "MongoDB" 27017
check_port "Redis" 6379
check_port "Temporal" 7233
echo ""

# Check observability stack
echo "Observability Stack:"
check_port "Jaeger" 16686
check_port "Prometheus" 9090
check_port "Grafana" 3000
echo ""

# Check UI services
echo "UI Services:"
check_http_health "Temporal UI" "http://localhost:8088"
echo ""

echo "======================================"
echo "Health check complete"
echo "======================================