#!/usr/bin/env python3
"""Simple test to check meta-prompt agent functionality"""

import requests
import json
import time

AGENT_MANAGER_URL = "http://localhost:8082"

# First check agents
print("Checking registered agents...")
resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/agents")
agents = resp.json()
print(f"Found {agents['count']} agents:")
for agent in agents['agents']:
    print(f"  - {agent['name']} ({agent['type']}) - Status: {agent['status']}")

# Submit a simple test task
print("\nSubmitting test task...")
test_task = {
    "type": "meta-prompt",
    "priority": "high",
    "payload": {
        "action": "test",
        "message": "Hello from meta-prompt test"
    }
}

resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=test_task)
if resp.status_code == 201:
    task_data = resp.json()
    print(f"Task created: {json.dumps(task_data, indent=2)}")
else:
    print(f"Failed to create task: {resp.status_code} - {resp.text}")