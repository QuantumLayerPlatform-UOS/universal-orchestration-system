#!/bin/bash

# Integrated UOS Demo - Shows complete flow from intent to task execution
# This demonstrates the new integration between Orchestrator and Agent Manager

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Service URLs
INTENT_PROCESSOR="http://localhost:8081"
ORCHESTRATOR="http://localhost:8080"
AGENT_MANAGER="http://localhost:8082"

echo -e "${BLUE}🚀 Universal Orchestration System - Integrated Flow Demo${NC}"
echo "======================================================"
echo -e "${CYAN}This demo shows the complete flow from natural language to task execution${NC}"
echo

# Step 1: Check all services are healthy
echo -e "\n${YELLOW}Step 1: Checking service health...${NC}"
services=(
    "intent-processor:8081:Intent Processor"
    "orchestrator:8080:Orchestrator"
    "agent-manager:8082:Agent Manager"
)

all_healthy=true
for service in "${services[@]}"; do
    IFS=':' read -r name port display <<< "$service"
    if curl -s -f "http://localhost:$port/health" > /dev/null; then
        echo -e "✅ $display is healthy"
    else
        echo -e "${RED}❌ $display is not responding${NC}"
        all_healthy=false
    fi
done

if [ "$all_healthy" = false ]; then
    echo -e "\n${RED}Some services are not running. Please ensure all services are up.${NC}"
    exit 1
fi

# Step 2: Create or use existing project
echo -e "\n${YELLOW}Step 2: Setting up project...${NC}"
PROJECT_ID=$(curl -s "$ORCHESTRATOR/api/v1/projects" | jq -r '.data.projects[0].id // empty')

if [[ -z "$PROJECT_ID" ]]; then
    echo "Creating new project..."
    PROJECT_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR/api/v1/projects" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Integrated Demo Project",
            "description": "Demo project for integrated flow",
            "type": "standard",
            "owner_id": "demo-user"
        }')
    
    PROJECT_ID=$(echo $PROJECT_RESPONSE | jq -r '.data.id // empty')
    if [[ -z "$PROJECT_ID" ]]; then
        echo -e "${RED}Failed to create project${NC}"
        echo $PROJECT_RESPONSE | jq
        exit 1
    fi
    echo -e "✅ Created project: $PROJECT_ID"
else
    echo -e "✅ Using existing project: $PROJECT_ID"
fi

# Step 3: Check available agents
echo -e "\n${YELLOW}Step 3: Checking available agents...${NC}"
AGENTS=$(curl -s "$AGENT_MANAGER/api/v1/agents")
echo "Available agents:"
echo $AGENTS | jq -r '.agents[] | "  - \(.name) [\(.type)] - \(.status)"'

# Step 4: Process intent with real LLM
echo -e "\n${YELLOW}Step 4: Processing user intent with real LLM...${NC}"
REQUEST_ID="integrated-demo-$(date +%s)"

echo -e "${BLUE}Intent: 'Build a task management API with user authentication and real-time updates'${NC}\n"

# Send the intent for processing
INTENT_RESPONSE=$(curl -s -X POST "$INTENT_PROCESSOR/api/v1/process-intent" \
    -H "Content-Type: application/json" \
    -d "{
        \"text\": \"Build a task management API with user authentication and real-time updates\",
        \"request_id\": \"$REQUEST_ID\",
        \"project_info\": {
            \"project_id\": \"$PROJECT_ID\"
        }
    }")

# Display intent analysis results
echo -e "\n${GREEN}Intent Analysis Results:${NC}"
echo $INTENT_RESPONSE | jq '{
    intent_type: .intent_type,
    confidence: .confidence,
    summary: .summary,
    task_count: (.tasks | length),
    total_hours: .metadata.total_estimated_hours
}'

# Show generated tasks
echo -e "\n${GREEN}Generated Tasks:${NC}"
echo $INTENT_RESPONSE | jq -r '.tasks[] | "  [\(.priority)] \(.title) - \(.type) (\(.estimated_hours)h)"'

# Step 5: Execute tasks through the new integrated workflow
echo -e "\n${YELLOW}Step 5: Executing tasks through integrated workflow...${NC}"

# Create the task execution workflow
WORKFLOW_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR/api/v1/demo/intent-to-execution" \
    -H "Content-Type: application/json" \
    -d "{
        \"project_id\": \"$PROJECT_ID\",
        \"intent_result\": $INTENT_RESPONSE
    }")

WORKFLOW_ID=$(echo $WORKFLOW_RESPONSE | jq -r '.workflow.workflow_id // empty')

if [[ -n "$WORKFLOW_ID" ]]; then
    echo -e "✅ Task execution workflow started: $WORKFLOW_ID"
    
    echo -e "\n${GREEN}Workflow Summary:${NC}"
    echo $WORKFLOW_RESPONSE | jq '.summary'
    
    # Monitor workflow progress
    echo -e "\n${YELLOW}Step 6: Monitoring workflow execution...${NC}"
    echo "Checking workflow status..."
    
    # Poll for workflow completion (with timeout)
    max_attempts=10
    attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        sleep 3
        
        WORKFLOW_STATUS=$(curl -s "$ORCHESTRATOR/api/v1/workflows/$WORKFLOW_ID")
        STATUS=$(echo $WORKFLOW_STATUS | jq -r '.data.status // empty')
        
        if [[ -z "$STATUS" ]]; then
            echo -e "${RED}Failed to get workflow status${NC}"
            break
        fi
        
        echo -e "  Status: ${CYAN}$STATUS${NC}"
        
        # Check if workflow is in terminal state
        if [[ "$STATUS" == "completed" || "$STATUS" == "failed" || "$STATUS" == "cancelled" ]]; then
            echo -e "\n${GREEN}Workflow finished with status: $STATUS${NC}"
            
            # Get final results if available
            OUTPUT=$(echo $WORKFLOW_STATUS | jq -r '.data.output // empty')
            if [[ -n "$OUTPUT" && "$OUTPUT" != "null" ]]; then
                echo -e "\n${GREEN}Workflow Results:${NC}"
                echo $OUTPUT | jq '.'
            fi
            break
        fi
        
        attempt=$((attempt + 1))
    done
    
    if [ $attempt -gt $max_attempts ]; then
        echo -e "\n${YELLOW}Workflow still running after $((max_attempts * 3)) seconds${NC}"
        echo "You can check the status at: GET $ORCHESTRATOR/api/v1/workflows/$WORKFLOW_ID"
    fi
else
    echo -e "${RED}❌ Failed to start task execution workflow${NC}"
    echo $WORKFLOW_RESPONSE | jq
fi

# Step 7: Summary
echo -e "\n${BLUE}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}🎉 Integrated Flow Demo Complete!${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"

echo -e "\n${CYAN}What happened:${NC}"
echo "1. ✅ Processed natural language intent with real LLM (fast!)"
echo "2. ✅ Generated structured task breakdown"
echo "3. ✅ Started task execution workflow in Orchestrator"
echo "4. ✅ Orchestrator communicates with Agent Manager"
echo "5. 🔄 Agents execute tasks (or get created if needed)"
echo "6. 📦 Results and artifacts are collected"

echo -e "\n${CYAN}Key Integration Points:${NC}"
echo "• Intent Processor → Fast LLM analysis with fallbacks"
echo "• Orchestrator → Temporal workflow management"
echo "• Agent Manager → Dynamic agent selection/creation"
echo "• Task Execution → Parallel processing where possible"

echo -e "\n${YELLOW}Next Steps:${NC}"
echo "• Implement artifact storage and delivery"
echo "• Add human-in-the-loop (HITL) capabilities"
echo "• Create specialized agent templates"
echo "• Add cost tracking and optimization"

echo -e "\n${GREEN}The system is now capable of end-to-end task execution!${NC}"