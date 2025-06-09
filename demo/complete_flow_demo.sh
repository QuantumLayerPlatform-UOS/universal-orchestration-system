#!/bin/bash

# Complete UOS Demo Flow Script
# Shows the entire system working end-to-end

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Service URLs
INTENT_PROCESSOR="http://localhost:8081"
ORCHESTRATOR="http://localhost:8080"
AGENT_MANAGER="http://localhost:8082"

echo -e "${BLUE}🚀 Universal Orchestration System - Complete Demo${NC}"
echo "================================================"

# Step 1: Check all services are healthy
echo -e "\n${YELLOW}Step 1: Checking service health...${NC}"
services=("intent-processor:8081" "orchestrator:8080" "agent-manager:8082")
for service in "${services[@]}"; do
    IFS=':' read -r name port <<< "$service"
    if curl -s -f "http://localhost:$port/health" > /dev/null; then
        echo -e "✅ $name is healthy"
    else
        echo -e "${RED}❌ $name is not responding${NC}"
        exit 1
    fi
done

# Step 2: Create a project
echo -e "\n${YELLOW}Step 2: Creating a new project...${NC}"
# First check if demo project already exists
EXISTING_PROJECT=$(curl -s "$ORCHESTRATOR/api/v1/projects" | jq -r '.data.projects[] | select(.name == "Demo Chat Application") | .id')

if [[ -n "$EXISTING_PROJECT" ]]; then
    PROJECT_ID=$EXISTING_PROJECT
    echo -e "✅ Using existing project: $PROJECT_ID"
else
    PROJECT_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR/api/v1/projects" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Demo Chat Application",
            "description": "Real-time chat app with authentication",
            "metadata": {
                "demo": true,
                "created_by": "demo_script"
            }
        }')
    
    if echo $PROJECT_RESPONSE | jq -e '.success == true' > /dev/null; then
        PROJECT_ID=$(echo $PROJECT_RESPONSE | jq -r '.data.id')
        echo -e "✅ Created project: $PROJECT_ID"
    else
        echo -e "${RED}❌ Failed to create project${NC}"
        echo $PROJECT_RESPONSE | jq
        exit 1
    fi
fi

# Step 3: Check available agents
echo -e "\n${YELLOW}Step 3: Checking available agents...${NC}"
AGENTS=$(curl -s "$AGENT_MANAGER/api/v1/agents")
echo $AGENTS | jq -r '.agents[] | "- \(.id): \(.name) [\(.status)]"'

# Step 4: Process intent with Chain of Thought
echo -e "\n${YELLOW}Step 4: Processing user intent...${NC}"
REQUEST_ID="demo-$(date +%s)"

# Start streaming endpoint in background
echo -e "${BLUE}Starting Chain of Thought stream...${NC}"
(
    curl -N "$INTENT_PROCESSOR/api/v1/process-intent/$REQUEST_ID/stream" 2>/dev/null | while IFS= read -r line; do
        if [[ $line == data:* ]]; then
            # Extract and parse JSON
            json="${line#data: }"
            if [[ -n "$json" ]]; then
                message=$(echo "$json" | jq -r '.message // empty' 2>/dev/null)
                detail=$(echo "$json" | jq -r '.detail // empty' 2>/dev/null)
                if [[ -n "$message" ]]; then
                    echo -e "${GREEN}  $message${NC}"
                    [[ -n "$detail" ]] && echo -e "    ${detail}"
                fi
            fi
        fi
    done
) &
STREAM_PID=$!

# Small delay to ensure stream starts
sleep 1

# Send the actual intent
echo -e "\n${BLUE}Sending intent: 'Build a real-time chat application with user authentication'${NC}"
INTENT_RESPONSE=$(curl -s -X POST "$INTENT_PROCESSOR/api/v1/process-intent" \
    -H "Content-Type: application/json" \
    -d "{
        \"text\": \"Build a real-time chat application with user authentication, message history, and typing indicators\",
        \"request_id\": \"$REQUEST_ID\",
        \"project_info\": {
            \"project_id\": \"$PROJECT_ID\"
        }
    }")

# Wait for streaming to complete
sleep 2
kill $STREAM_PID 2>/dev/null || true

# Display intent analysis results
echo -e "\n${YELLOW}Intent Analysis Results:${NC}"
echo $INTENT_RESPONSE | jq '{
    intent_type: .intent_type,
    confidence: .confidence,
    summary: .summary,
    task_count: (.tasks | length),
    total_hours: .metadata.total_estimated_hours
}'

# Extract tasks
TASKS=$(echo $INTENT_RESPONSE | jq -c '.tasks')
echo -e "\n${YELLOW}Generated Tasks:${NC}"
echo $TASKS | jq -r '.[] | "- [\(.priority)] \(.title) - \(.estimated_hours)h"'

# Step 5: Create workflow in orchestrator
echo -e "\n${YELLOW}Step 5: Creating workflow in orchestrator...${NC}"
WORKFLOW_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR/api/v1/workflows" \
    -H "Content-Type: application/json" \
    -d "{
        \"project_id\": \"$PROJECT_ID\",
        \"name\": \"Chat App Development Workflow\",
        \"description\": \"Workflow for building chat application\",
        \"type\": \"intent\",
        \"priority\": \"high\",
        \"user_id\": \"demo-user\",
        \"input\": {
            \"intent_result\": $INTENT_RESPONSE,
            \"tasks\": $TASKS
        },
        \"config\": {},
        \"max_retries\": 3,
        \"timeout_seconds\": 3600
    }")

WORKFLOW_ID=$(echo $WORKFLOW_RESPONSE | jq -r '.id // empty')
if [[ -n "$WORKFLOW_ID" ]]; then
    echo -e "✅ Created workflow: $WORKFLOW_ID"
else
    echo -e "${RED}❌ Failed to create workflow${NC}"
    echo $WORKFLOW_RESPONSE
fi

# Step 6: Execute first task with agent
echo -e "\n${YELLOW}Step 6: Executing first task with available agent...${NC}"
FIRST_TASK=$(echo $TASKS | jq -r '.[0]')
TASK_ID=$(echo $FIRST_TASK | jq -r '.id')
TASK_TYPE=$(echo $FIRST_TASK | jq -r '.type')

# Find suitable agent
echo "Looking for agent with capability: $TASK_TYPE"
# For now, we'll use the code-gen agent for backend tasks
if [[ "$TASK_TYPE" == "backend" || "$TASK_TYPE" == "frontend" ]]; then
    SUITABLE_AGENT="code-gen-agent-001"
else
    SUITABLE_AGENT=$(curl -s "$AGENT_MANAGER/api/v1/agents" | jq -r '.agents[] | select(.capabilities[].name | contains("'$TASK_TYPE'")) | .id // empty' | head -1)
fi

if [[ -n "$SUITABLE_AGENT" ]]; then
    echo -e "✅ Found suitable agent: $SUITABLE_AGENT"
    
    # Execute task
    EXECUTION_RESPONSE=$(curl -s -X POST "$AGENT_MANAGER/api/v1/agents/$SUITABLE_AGENT/execute" \
        -H "Content-Type: application/json" \
        -d "{
            \"task_id\": \"$TASK_ID\",
            \"task\": $FIRST_TASK,
            \"context\": {
                \"project_id\": \"$PROJECT_ID\",
                \"workflow_id\": \"$WORKFLOW_ID\"
            }
        }")
    
    EXECUTION_ID=$(echo $EXECUTION_RESPONSE | jq -r '.execution_id // empty')
    if [[ -n "$EXECUTION_ID" ]]; then
        echo -e "✅ Task execution started: $EXECUTION_ID"
        
        # Poll for completion
        echo -e "\n${BLUE}Waiting for task completion...${NC}"
        for i in {1..10}; do
            sleep 2
            STATUS=$(curl -s "$AGENT_MANAGER/api/v1/executions/$EXECUTION_ID" | jq -r '.status // "unknown"')
            echo -e "  Status: $STATUS"
            if [[ "$STATUS" == "completed" ]]; then
                echo -e "${GREEN}✅ Task completed successfully!${NC}"
                break
            elif [[ "$STATUS" == "failed" ]]; then
                echo -e "${RED}❌ Task failed${NC}"
                break
            fi
        done
    fi
else
    echo -e "${YELLOW}⚠️  No suitable agent found for task type: $TASK_TYPE${NC}"
    echo "Available agents:"
    curl -s "$AGENT_MANAGER/api/v1/agents" | jq -r '.[] | "  - \(.id): capabilities=\(.capabilities)"'
fi

# Step 7: Show project status
echo -e "\n${YELLOW}Step 7: Project Status Summary${NC}"
echo "================================"
PROJECT_STATUS=$(curl -s "$ORCHESTRATOR/api/v1/projects/$PROJECT_ID")
echo $PROJECT_STATUS | jq '{
    id: .id,
    name: .name,
    status: .status,
    created_at: .created_at
}'

# Step 8: Show artifacts (if any)
echo -e "\n${YELLOW}Step 8: Generated Artifacts${NC}"
echo "============================"
# This would show actual generated code/artifacts once artifact management is implemented
echo -e "${YELLOW}(Artifact management pending implementation)${NC}"

echo -e "\n${GREEN}🎉 Demo Complete!${NC}"
echo -e "\n${BLUE}What happened:${NC}"
echo "1. ✅ Created a new project"
echo "2. ✅ Processed natural language intent with Chain of Thought"
echo "3. ✅ Generated task breakdown (${#TASKS} tasks)"
echo "4. ✅ Created workflow in orchestrator"
echo "5. 🟡 Attempted task execution (agent integration pending)"
echo "6. 🔴 Artifact collection (pending implementation)"

echo -e "\n${BLUE}Next steps to make this fully functional:${NC}"
echo "- Wire up orchestrator → agent task execution"
echo "- Implement dynamic agent generation"
echo "- Add artifact collection system"
echo "- Create specialized agents for each task type"