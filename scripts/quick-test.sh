#!/bin/bash

# Quick test script for minimal QLP-UOS stack

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.."; pwd)"
cd "$PROJECT_ROOT"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up..."
    docker-compose -f docker-compose.minimal.yml down -v
    rm -f .env
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Main execution
log_info "Starting QLP-UOS minimal test setup..."

# Check for required tools
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    log_error "Docker Compose is not installed"
    exit 1
fi

if ! command -v python3 &> /dev/null; then
    log_error "Python 3 is not installed"
    exit 1
fi

# Copy minimal env file
log_info "Setting up environment..."
cp .env.minimal .env

# Check if we need to create test directories
mkdir -p tests/integration

# Install Python dependencies for testing
log_info "Installing test dependencies..."
if [ ! -d "venv" ]; then
    python3 -m venv venv
fi
source venv/bin/activate
pip install -q requests

# Stop any existing containers
log_info "Stopping any existing containers..."
docker-compose -f docker-compose.minimal.yml down -v || true

# Build and start services
log_info "Building services..."
docker-compose -f docker-compose.minimal.yml build

log_info "Starting services..."
docker-compose -f docker-compose.minimal.yml up -d

# Wait a bit for services to initialize
log_info "Waiting for services to initialize (30 seconds)..."
sleep 30

# Show service status
log_info "Service status:"
docker-compose -f docker-compose.minimal.yml ps

# Run basic connectivity tests
log_info "Running basic connectivity tests..."

# Test database connections
log_info "Testing PostgreSQL connection..."
if docker-compose -f docker-compose.minimal.yml exec -T postgres pg_isready -U postgres; then
    log_info "PostgreSQL is ready"
else
    log_error "PostgreSQL connection failed"
fi

log_info "Testing MongoDB connection..."
if docker-compose -f docker-compose.minimal.yml exec -T mongodb mongosh --eval "db.adminCommand('ping')" --quiet; then
    log_info "MongoDB is ready"
else
    log_error "MongoDB connection failed"
fi

log_info "Testing Redis connection..."
if docker-compose -f docker-compose.minimal.yml exec -T redis redis-cli -a redis123 ping | grep -q PONG; then
    log_info "Redis is ready"
else
    log_error "Redis connection failed"
fi

# Run integration tests
log_info "Running integration tests..."
if python3 tests/integration/test_basic_flow.py; then
    log_info "Integration tests passed!"
else
    log_error "Integration tests failed"
    
    # Show logs for debugging
    log_info "Showing service logs for debugging..."
    echo -e "\n${YELLOW}=== Orchestrator Logs ===${NC}"
    docker-compose -f docker-compose.minimal.yml logs --tail=50 orchestrator
    
    echo -e "\n${YELLOW}=== Intent Processor Logs ===${NC}"
    docker-compose -f docker-compose.minimal.yml logs --tail=50 intent-processor
    
    echo -e "\n${YELLOW}=== Agent Manager Logs ===${NC}"
    docker-compose -f docker-compose.minimal.yml logs --tail=50 agent-manager
    
    exit 1
fi

# Test complete
log_info "\nMinimal test setup completed successfully!"
log_info "Services are running. You can:"
log_info "  - View logs: docker-compose -f docker-compose.minimal.yml logs -f"
log_info "  - Stop services: docker-compose -f docker-compose.minimal.yml down"
log_info "  - Access services:"
log_info "    - Orchestrator: http://localhost:8080"
log_info "    - Intent Processor: http://localhost:8081"
log_info "    - Agent Manager: http://localhost:8082"

# Ask if user wants to keep services running
read -p "\nKeep services running? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Services will continue running. To stop them, run:"
    log_info "  docker-compose -f docker-compose.minimal.yml down -v"
    trap - EXIT  # Remove the cleanup trap
else
    log_info "Stopping services..."
fi
