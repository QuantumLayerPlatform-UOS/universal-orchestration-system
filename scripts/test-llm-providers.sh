#!/bin/bash

# Test script for LLM providers
echo "🧪 Testing LLM Provider Connectivity"
echo "===================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test Ollama
echo -e "\n${YELLOW}Testing Ollama at model.gonella.co.uk...${NC}"
OLLAMA_RESPONSE=$(curl -s -X GET "https://model.gonella.co.uk/api/tags" -w "\nHTTP_STATUS:%{http_code}")
HTTP_STATUS=$(echo "$OLLAMA_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)

if [ "$HTTP_STATUS" = "200" ]; then
    echo -e "${GREEN}✓ Ollama is accessible${NC}"
    echo "Available models:"
    echo "$OLLAMA_RESPONSE" | grep -v "HTTP_STATUS:" | jq -r '.models[].name' 2>/dev/null || echo "Unable to parse models"
else
    echo -e "${RED}✗ Ollama is not accessible (HTTP $HTTP_STATUS)${NC}"
fi

# Test Groq API
echo -e "\n${YELLOW}Testing Groq API...${NC}"
if [ -n "$GROQ_API_KEY" ]; then
    GROQ_TEST=$(curl -s -X POST "https://api.groq.com/openai/v1/chat/completions" \
        -H "Authorization: Bearer $GROQ_API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "mixtral-8x7b-32768",
            "messages": [{"role": "user", "content": "Say hello"}],
            "max_tokens": 10
        }' -w "\nHTTP_STATUS:%{http_code}")
    
    HTTP_STATUS=$(echo "$GROQ_TEST" | grep "HTTP_STATUS:" | cut -d: -f2)
    if [ "$HTTP_STATUS" = "200" ]; then
        echo -e "${GREEN}✓ Groq API is working${NC}"
    else
        echo -e "${RED}✗ Groq API error (HTTP $HTTP_STATUS)${NC}"
    fi
else
    echo -e "${YELLOW}⚠ GROQ_API_KEY not set${NC}"
fi

# Test OpenAI API
echo -e "\n${YELLOW}Testing OpenAI API...${NC}"
if [ -n "$OPENAI_API_KEY" ]; then
    echo -e "${GREEN}✓ OpenAI API key is set${NC}"
else
    echo -e "${YELLOW}⚠ OPENAI_API_KEY not set${NC}"
fi

# Test Anthropic API
echo -e "\n${YELLOW}Testing Anthropic API...${NC}"
if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo -e "${GREEN}✓ Anthropic API key is set${NC}"
else
    echo -e "${YELLOW}⚠ ANTHROPIC_API_KEY not set${NC}"
fi

# Create test .env file
echo -e "\n${YELLOW}Creating test .env file...${NC}"
cat > .env.test << EOF
# LLM Provider Configuration for Testing
LLM_PROVIDER=ollama
LLM_MODEL=mistral
LLM_TEMPERATURE=0.7
LLM_MAX_TOKENS=2000

# Ollama Configuration
OLLAMA_BASE_URL=https://model.gonella.co.uk
USE_OLLAMA=true

# API Keys (if available)
GROQ_API_KEY=${GROQ_API_KEY}
OPENAI_API_KEY=${OPENAI_API_KEY}
ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}

# Service Configuration
NODE_ENV=development
LOG_LEVEL=info
EOF

echo -e "${GREEN}✓ Created .env.test file${NC}"

echo -e "\n${YELLOW}Summary:${NC}"
echo "- Ollama: Primary provider for development"
echo "- Groq: Available for fast inference"
echo "- OpenAI: Available if needed"
echo "- Anthropic: Available if needed"

echo -e "\n${YELLOW}To start the services with Ollama:${NC}"
echo "cp .env.test .env"
echo "docker-compose -f docker-compose.minimal.yml up"

echo -e "\n${YELLOW}To test meta-prompt agent creation:${NC}"
echo "curl -X POST http://localhost:8082/api/v1/tasks \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{"
echo "    \"type\": \"meta-prompt\","
echo "    \"priority\": \"high\","
echo "    \"payload\": {"
echo "      \"type\": \"design-agent\","
echo "      \"taskDescription\": \"Create an agent that reviews Python code for best practices\","
echo "      \"requirements\": {"
echo "        \"language\": \"Python\","
echo "        \"focus\": [\"PEP8\", \"type hints\", \"docstrings\"]"
echo "      }"
echo "    }"
echo "  }'"