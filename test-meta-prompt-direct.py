#!/usr/bin/env python3
"""Test meta-prompt agent directly"""

import requests
import json
import time

AGENT_MANAGER_URL = "http://localhost:8082"

print("=== Testing Meta-Prompt Agent ===\n")

# First, let's manually register the meta-prompt agent if needed
print("1. Registering meta-prompt agent...")
agent_data = {
    "id": "meta-prompt-orchestrator",
    "name": "Meta-Prompt Orchestrator",
    "type": "meta-prompt",
    "capabilities": [
        {
            "name": "design-agent",
            "description": "Design new agents based on requirements",
            "version": "1.0.0"
        },
        {
            "name": "optimize-prompt",
            "description": "Optimize existing agent prompts",
            "version": "1.0.0"
        }
    ],
    "endpoint": "http://agent-manager:8082/agents/meta-prompt-orchestrator",
    "metadata": {
        "version": "1.0.0",
        "platform": "nodejs",
        "region": "global",
        "tags": ["meta-prompt", "orchestrator", "dynamic-agents"]
    }
}

try:
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/agents", json=agent_data)
    if resp.status_code == 201:
        print("✓ Agent registered successfully")
    elif resp.status_code == 400:
        print(f"✗ Registration failed: {resp.json()}")
    elif resp.status_code == 409:
        print("✓ Agent already registered")
    else:
        print(f"✗ Unexpected response: {resp.status_code} - {resp.text}")
except Exception as e:
    print(f"✗ Error: {e}")

# Check agents
print("\n2. Checking registered agents...")
resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/agents")
agents = resp.json()
print(f"Found {agents['count']} agents:")
for agent in agents['agents']:
    print(f"  - {agent['name']} ({agent['type']}) - Status: {agent.get('status', 'unknown')}")

# Submit a task to design an agent
print("\n3. Submitting agent design task...")
design_task = {
    "type": "meta-prompt",
    "priority": "high",
    "payload": {
        "type": "design-agent",
        "taskDescription": "Create a performance monitoring agent that tracks system metrics",
        "requirements": {
            "language": "nodejs",
            "metrics": ["cpu", "memory", "disk", "network"],
            "reportingInterval": 60
        },
        "context": {
            "purpose": "Monitor microservices health"
        }
    }
}

try:
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=design_task)
    if resp.status_code == 201:
        task_data = resp.json()
        print(f"✓ Task created: {task_data['task']['id']}")
        print(f"  Type: {task_data['task']['type']}")
        print(f"  Status: {task_data['task']['status']}")
    else:
        print(f"✗ Failed to create task: {resp.status_code} - {resp.text}")
except Exception as e:
    print(f"✗ Error: {e}")

print("\n=== Test Complete ===")