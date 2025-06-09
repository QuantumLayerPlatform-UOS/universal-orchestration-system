#!/bin/bash

# Meta-Agent Integration Startup Script
# Starts all services and runs integration tests

set -e

echo "🚀 QuantumLayer Meta-Agent Platform Integration Test"
echo "======================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check if service is ready
wait_for_service() {
    local service_name=$1
    local health_url=$2
    local max_attempts=30
    local attempt=1
    
    print_status $YELLOW "⏳ Waiting for $service_name to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$health_url" > /dev/null 2>&1; then
            print_status $GREEN "✅ $service_name is ready"
            return 0
        fi
        
        echo -n "."
        sleep 2
        ((attempt++))
    done
    
    print_status $RED "❌ $service_name failed to start within $(($max_attempts * 2)) seconds"
    return 1
}

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    print_status $RED "❌ Python 3 is required but not installed"
    exit 1
fi

# Install Python dependencies
print_status $BLUE "📦 Installing Python dependencies..."
if ! python3 -m pip install requests websockets asyncio --quiet; then
    print_status $RED "❌ Failed to install Python dependencies"
    exit 1
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    print_status $RED "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Check if services are already running
print_status $BLUE "🔍 Checking for running services..."

SERVICES_RUNNING=true

if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
    print_status $YELLOW "⚠️  Orchestrator not running"
    SERVICES_RUNNING=false
fi

if ! curl -s http://localhost:8082/health > /dev/null 2>&1; then
    print_status $YELLOW "⚠️  Agent Manager not running"
    SERVICES_RUNNING=false
fi

if ! curl -s http://localhost:8083/health > /dev/null 2>&1; then
    print_status $YELLOW "⚠️  Intent Processor not running"
    SERVICES_RUNNING=false
fi

# Start services if not running
if [ "$SERVICES_RUNNING" = false ]; then
    print_status $BLUE "🚀 Starting QuantumLayer Platform services..."
    
    # Start with minimal development setup
    if [ -f "docker-compose.minimal.yml" ]; then
        print_status $BLUE "📦 Starting minimal infrastructure..."
        docker-compose -f docker-compose.minimal.yml up -d
        
        # Wait for infrastructure
        sleep 10
        
        print_status $BLUE "🔧 Starting core services..."
        make dev-start-services || {
            print_status $RED "❌ Failed to start services with make command"
            print_status $YELLOW "ℹ️  Trying alternative startup method..."
            
            # Alternative: Start services individually
            print_status $BLUE "🔄 Starting Agent Manager..."
            cd services/agent-manager && npm start &
            AGENT_MANAGER_PID=$!
            cd ../..
            
            print_status $BLUE "🔄 Starting Meta-Prompt Agent..."
            cd services/agents/meta-prompt-agent && npm start &
            META_AGENT_PID=$!
            cd ../../..
            
            # Give services time to start
            sleep 15
        }
    else
        print_status $YELLOW "⚠️  docker-compose.minimal.yml not found, using development setup"
        if ! make dev-up; then
            print_status $RED "❌ Failed to start services"
            exit 1
        fi
    fi
    
    # Wait for services to be ready
    print_status $BLUE "⏳ Waiting for services to be ready..."
    
    wait_for_service "Agent Manager" "http://localhost:8082/health" || {
        print_status $RED "❌ Agent Manager failed to start"
        exit 1
    }
    
    # Check if other services are available
    if wait_for_service "Orchestrator" "http://localhost:8081/health"; then
        print_status $GREEN "✅ Orchestrator is ready"
    else
        print_status $YELLOW "⚠️  Orchestrator not ready, but continuing with available services"
    fi
    
    if wait_for_service "Intent Processor" "http://localhost:8083/health"; then
        print_status $GREEN "✅ Intent Processor is ready"
    else
        print_status $YELLOW "⚠️  Intent Processor not ready, but continuing with available services"
    fi
    
else
    print_status $GREEN "✅ All services are already running"
fi

# Wait for Meta-Prompt Agent to register
print_status $BLUE "🤖 Waiting for Meta-Prompt Agent registration..."
sleep 10

# Check if Meta-Prompt Agent is registered
META_AGENT_REGISTERED=false
for i in {1..20}; do
    if curl -s http://localhost:8082/api/v1/agents | grep -q "meta-prompt"; then
        META_AGENT_REGISTERED=true
        break
    fi
    echo -n "."
    sleep 3
done

if [ "$META_AGENT_REGISTERED" = true ]; then
    print_status $GREEN "✅ Meta-Prompt Agent registered successfully"
else
    print_status $YELLOW "⚠️  Meta-Prompt Agent not yet registered, but proceeding with tests"
fi

# Run integration tests
print_status $BLUE "🧪 Running Meta-Agent Integration Tests..."
echo ""

if python3 test-meta-agent-integration.py; then
    print_status $GREEN "🎉 Integration tests completed successfully!"
    echo ""
    print_status $GREEN "🚀 META-AGENT PLATFORM VALIDATION COMPLETE"
    print_status $GREEN "📊 Platform ready for breakthrough demonstration"
    print_status $GREEN "💼 Investor demo capabilities confirmed"
    echo ""
    print_status $BLUE "📋 Next Steps:"
    echo "   1. Review the integration test report: meta_agent_integration_test_report.json"
    echo "   2. Access the Agent Manager dashboard: http://localhost:8082"
    echo "   3. Check Grafana monitoring: http://localhost:3000"
    echo "   4. Platform is ready for customer demos and investor presentations"
    echo ""
else
    print_status $YELLOW "⚠️  Integration tests completed with some issues"
    echo ""
    print_status $BLUE "🔧 Troubleshooting Guide:"
    echo "   1. Check service logs: docker-compose logs [service-name]"
    echo "   2. Verify environment variables in .env files"
    echo "   3. Ensure all dependencies are installed"
    echo "   4. Check the integration test report for specific failures"
    echo ""
    print_status $BLUE "📋 For manual testing:"
    echo "   • Agent Manager API: http://localhost:8082/api/v1/agents"
    echo "   • Orchestrator API: http://localhost:8081/api/v1/workflows"
    echo "   • Health Checks: curl http://localhost:8082/health"
fi

echo ""
print_status $BLUE "🏁 Meta-Agent Integration Test Complete"
print_status $BLUE "======================================================"
