#!/usr/bin/env python3
"""
Example: Create a Dynamic Agent using Meta-Prompt
This demonstrates how to create agents without writing code
"""

import requests
import json
import time

AGENT_MANAGER_URL = "http://localhost:8082"

def create_code_review_agent():
    """Create a Python code review agent using natural language"""
    
    print("🤖 Creating a Python Code Review Agent using Meta-Prompt")
    print("=" * 50)
    
    # Step 1: Design the agent
    design_request = {
        "type": "meta-prompt",
        "priority": "high",
        "payload": {
            "type": "design-agent",
            "taskDescription": """
                Create an expert Python code reviewer that:
                1. Checks for PEP 8 compliance and code style
                2. Identifies potential bugs and edge cases
                3. Suggests performance improvements
                4. Reviews type hints and docstrings
                5. Checks for security vulnerabilities
                6. Provides actionable feedback with examples
            """,
            "requirements": {
                "language": "Python",
                "expertise": ["Django", "FastAPI", "SQLAlchemy"],
                "outputFormat": "structured JSON with severity levels",
                "style": "constructive and educational"
            },
            "context": {
                "teamSize": "10-50 developers",
                "projectType": "Enterprise REST API",
                "codebaseAge": "2+ years"
            }
        }
    }
    
    print("📝 Sending agent design request...")
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=design_request)
    
    if resp.status_code not in [200, 201]:
        print(f"❌ Failed to create design task: {resp.text}")
        return
    
    task_id = resp.json().get('taskId')
    print(f"✅ Design task created: {task_id}")
    
    # Wait for design completion
    print("⏳ Designing agent (this may take 10-30 seconds)...")
    design_id = None
    
    for i in range(30):
        time.sleep(2)
        resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/tasks/{task_id}")
        
        if resp.status_code == 200:
            task_data = resp.json().get('task', {})
            if task_data.get('status') == 'completed':
                result = task_data.get('result', {})
                design_id = result.get('designId')
                agent_design = result.get('agentDesign', {})
                
                print("\n✅ Agent Designed Successfully!")
                print(f"\n📋 Agent Details:")
                print(f"Name: {agent_design.get('name')}")
                print(f"Type: {agent_design.get('type')}")
                print(f"Purpose: {agent_design.get('purpose')}")
                print(f"\nCapabilities:")
                for cap in agent_design.get('capabilities', []):
                    print(f"  - {cap}")
                break
    
    if not design_id:
        print("❌ Agent design failed or timed out")
        return
    
    # Step 2: Spawn the agent
    print(f"\n🚀 Spawning agent from design {design_id}...")
    
    spawn_request = {
        "type": "meta-prompt",
        "priority": "high",
        "payload": {
            "type": "spawn-agent",
            "designId": design_id,
            "taskContext": {
                "environment": "development",
                "maxConcurrentTasks": 5
            },
            "ttl": 3600000  # 1 hour
        }
    }
    
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=spawn_request)
    task_id = resp.json().get('taskId')
    
    print("⏳ Spawning agent...")
    agent_id = None
    
    for i in range(20):
        time.sleep(1)
        resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/tasks/{task_id}")
        
        if resp.status_code == 200:
            task_data = resp.json().get('task', {})
            if task_data.get('status') == 'completed':
                result = task_data.get('result', {})
                agent_id = result.get('agentId')
                print(f"\n✅ Agent Spawned: {agent_id}")
                print(f"TTL: {result.get('ttl', 0) / 1000 / 60} minutes")
                break
    
    if not agent_id:
        print("❌ Agent spawn failed")
        return
    
    # Step 3: Use the agent
    print("\n🔍 Testing the agent with sample code...")
    
    sample_code = '''
def calculate_total_price(items, tax_rate):
    """Calculate total price including tax"""
    total = 0
    for item in items:
        total += item['price'] * item['quantity']
    
    tax = total * tax_rate
    return total + tax

# Usage
items = [
    {'name': 'Widget', 'price': 10.0, 'quantity': 2},
    {'name': 'Gadget', 'price': 25.0, 'quantity': 1}
]
total = calculate_total_price(items, 0.08)
print(f"Total: ${total}")
'''
    
    review_request = {
        "type": "dynamic",
        "priority": "high",
        "payload": {
            "code": sample_code,
            "filename": "pricing.py",
            "context": "E-commerce pricing module"
        },
        "metadata": {
            "targetAgent": agent_id
        }
    }
    
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=review_request)
    task_id = resp.json().get('taskId')
    
    print("⏳ Agent is reviewing code...")
    
    for i in range(20):
        time.sleep(2)
        resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/tasks/{task_id}")
        
        if resp.status_code == 200:
            task_data = resp.json().get('task', {})
            if task_data.get('status') == 'completed':
                result = task_data.get('result', {})
                print("\n✅ Code Review Complete!")
                print("\n" + "=" * 50)
                print("REVIEW RESULTS:")
                print("=" * 50)
                
                # Pretty print the result
                if isinstance(result, dict) and 'output' in result:
                    print(json.dumps(result['output'], indent=2))
                else:
                    print(str(result)[:1000] + "..." if len(str(result)) > 1000 else str(result))
                break
    
    print("\n" + "=" * 50)
    print("🎉 Dynamic agent successfully created and used!")
    print(f"Agent ID: {agent_id}")
    print("This agent will remain active for 1 hour")
    print("=" * 50)

if __name__ == "__main__":
    # Check if services are running
    try:
        resp = requests.get(f"{AGENT_MANAGER_URL}/health", timeout=2)
        if resp.status_code != 200:
            print("❌ Agent Manager is not running!")
            print("Please run: ./scripts/start-with-ollama.sh")
            exit(1)
    except:
        print("❌ Cannot connect to Agent Manager!")
        print("Please run: ./scripts/start-with-ollama.sh")
        exit(1)
    
    create_code_review_agent()