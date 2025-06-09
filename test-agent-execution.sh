#!/bin/bash

echo "🧪 Testing Agent Execution Endpoint"
echo "=================================="

# Test the agent execution endpoint directly
echo -e "\n1️⃣ Testing direct agent execution..."
AGENT_ID="meta-prompt-orchestrator"

EXEC_RESPONSE=$(curl -s -X POST "http://localhost:8082/api/v1/agents/${AGENT_ID}/execute" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "test-execution",
    "input": {
      "task": "Test the agent execution endpoint",
      "description": "This is a test to verify the endpoint works"
    },
    "config": {
      "test_mode": true
    },
    "priority": "high",
    "timeout": 30
  }')

echo "Response from agent execution:"
echo "$EXEC_RESPONSE" | jq '.'

# Check if we got a task ID
TASK_ID=$(echo "$EXEC_RESPONSE" | jq -r '.id // empty')
if [ -n "$TASK_ID" ]; then
  echo -e "\n✅ Agent execution endpoint is working!"
  echo "Task ID: $TASK_ID"
  echo "Status: $(echo "$EXEC_RESPONSE" | jq -r '.status')"
else
  echo -e "\n❌ Agent execution endpoint returned unexpected response"
fi

echo -e "\n2️⃣ Testing integrated flow with new endpoint..."
# Run a full integrated test
./demo/integrated_flow_demo.sh 2>&1 | grep -A5 -B5 "task execution"

echo -e "\n3️⃣ Checking orchestrator logs for execution details..."
docker logs qlp-uos-orchestrator-1 --tail 20 | grep -i "execute"

echo -e "\n✅ Test complete!"