#!/bin/bash

echo "🚀 Starting QuantumLayer Platform with Ollama"
echo "============================================"

# Check if .env exists
if [ ! -f .env ]; then
    echo "Creating .env file from template..."
    cp .env.test .env || cp .env.example .env
    
    # Update .env with Ollama settings
    cat > .env << EOF
# LLM Provider Configuration
LLM_PROVIDER=ollama
LLM_MODEL=mixtral:8x7b
LLM_TEMPERATURE=0.7
LLM_MAX_TOKENS=2000

# Ollama Configuration
OLLAMA_BASE_URL=https://model.gonella.co.uk
USE_OLLAMA=true

# Service Configuration
NODE_ENV=development
LOG_LEVEL=info
EOF
    echo "✓ Created .env file with Ollama configuration"
fi

# Export environment variables
export LLM_PROVIDER=ollama
export LLM_MODEL=mixtral:8x7b
export OLLAMA_BASE_URL=https://model.gonella.co.uk
export USE_OLLAMA=true

echo ""
echo "Configuration:"
echo "- LLM Provider: Ollama"
echo "- Model: mixtral:8x7b"
echo "- Ollama URL: https://model.gonella.co.uk"
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

if ! docker info &> /dev/null; then
    echo "❌ Docker is not running. Please start Docker."
    exit 1
fi

echo "✓ Docker is running"

# Build services
echo ""
echo "Building services..."
docker-compose -f docker-compose.minimal.yml build

# Start services
echo ""
echo "Starting services..."
docker-compose -f docker-compose.minimal.yml up -d

# Wait for services to be ready
echo ""
echo "Waiting for services to be ready..."
sleep 10

# Check service health
echo ""
echo "Checking service health:"

# Check orchestrator
ORCHESTRATOR_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
if [ "$ORCHESTRATOR_HEALTH" = "200" ]; then
    echo "✓ Orchestrator: Running on http://localhost:8080"
else
    echo "✗ Orchestrator: Not ready (HTTP $ORCHESTRATOR_HEALTH)"
fi

# Check intent processor
INTENT_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/health)
if [ "$INTENT_HEALTH" = "200" ]; then
    echo "✓ Intent Processor: Running on http://localhost:8081"
else
    echo "✗ Intent Processor: Not ready (HTTP $INTENT_HEALTH)"
fi

# Check agent manager
AGENT_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/health)
if [ "$AGENT_HEALTH" = "200" ]; then
    echo "✓ Agent Manager: Running on http://localhost:8082"
else
    echo "✗ Agent Manager: Not ready (HTTP $AGENT_HEALTH)"
fi

# List agents
echo ""
echo "Registered agents:"
curl -s http://localhost:8082/api/v1/agents | jq -r '.agents[] | "- \(.name) (\(.type)) - Status: \(.status)"' 2>/dev/null || echo "Unable to list agents"

echo ""
echo "============================================"
echo "✅ QuantumLayer Platform is running!"
echo ""
echo "Test meta-prompt agent:"
echo "python tests/integration/test_meta_prompt_agent.py"
echo ""
echo "View logs:"
echo "docker-compose -f docker-compose.minimal.yml logs -f"
echo ""
echo "Stop services:"
echo "docker-compose -f docker-compose.minimal.yml down"
echo "============================================"