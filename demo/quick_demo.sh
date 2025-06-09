#!/bin/bash

# Quick UOS Demo - Shows current working state

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}🚀 Universal Orchestration System - Quick Demo${NC}"
echo "=============================================="

# Step 1: Service Health
echo -e "\n${YELLOW}1. Service Health Check${NC}"
services=("intent-processor:8081" "orchestrator:8080" "agent-manager:8082")
for service in "${services[@]}"; do
    IFS=':' read -r name port <<< "$service"
    if curl -s -f "http://localhost:$port/health" > /dev/null; then
        echo -e "✅ $name is healthy"
    else
        echo -e "${RED}❌ $name is not responding${NC}"
    fi
done

# Step 2: Show Registered Agents
echo -e "\n${YELLOW}2. Available AI Agents${NC}"
curl -s "http://localhost:8082/api/v1/agents" | jq -r '.agents[] | "✅ \(.name) [\(.type)] - Status: \(.status)"'

# Step 3: Test Intent Processing
echo -e "\n${YELLOW}3. Intent Processing Demo${NC}"
echo -e "${BLUE}Sending request: 'Create a REST API for user management'${NC}"

# Create a simpler request
INTENT_RESPONSE=$(curl -s -X POST "http://localhost:8081/api/v1/process-intent" \
    -H "Content-Type: application/json" \
    -d '{
        "text": "Create a REST API for user management",
        "request_id": "demo-simple"
    }' --max-time 30)

if [[ -n "$INTENT_RESPONSE" ]]; then
    echo -e "\n${GREEN}Intent Analysis Results:${NC}"
    echo "$INTENT_RESPONSE" | jq '{
        intent_type: .intent_type,
        confidence: .confidence,
        task_count: (.tasks | length),
        tasks: [.tasks[] | {title: .title, type: .type, hours: .estimated_hours}]
    }'
else
    echo -e "${RED}Intent processing timed out or failed${NC}"
fi

# Step 4: Show What Would Happen Next
echo -e "\n${YELLOW}4. Next Steps (Currently Manual)${NC}"
echo "The system has identified the tasks needed. In a complete flow:"
echo "1. ✅ Intent analyzed and tasks generated"
echo "2. 🟡 Orchestrator would create a workflow (integration pending)"
echo "3. 🟡 Meta-Prompt Agent would generate specialized agents (pending)"
echo "4. 🟡 Agents would execute tasks in parallel (pending)"
echo "5. 🔴 Artifacts would be collected and delivered (not implemented)"

echo -e "\n${GREEN}✨ Demo Complete!${NC}"
echo -e "\n${BLUE}Current Capabilities:${NC}"
echo "- Natural language understanding with real LLMs"
echo "- Multi-strategy analysis with fallbacks"
echo "- Chain of Thought streaming for transparency"
echo "- Distributed agent registry"
echo "- Task breakdown and estimation"

echo -e "\n${YELLOW}Coming Soon:${NC}"
echo "- Dynamic agent generation"
echo "- Automatic task execution"
echo "- Artifact management"
echo "- Cost tracking and optimization"