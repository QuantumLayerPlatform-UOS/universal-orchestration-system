#!/usr/bin/env python3
"""
Integration test for Meta-Prompt Agent functionality
Tests dynamic agent creation using Ollama
"""

import requests
import json
import time
import sys

# Service URLs
import os
AGENT_MANAGER_URL = os.getenv("AGENT_MANAGER_URL", "http://localhost:8082")

def wait_for_agent_manager(timeout=30):
    """Wait for agent manager to be ready"""
    print("Waiting for Agent Manager...")
    start_time = time.time()
    
    while time.time() - start_time < timeout:
        try:
            resp = requests.get(f"{AGENT_MANAGER_URL}/health", timeout=2)
            if resp.status_code == 200:
                print("✓ Agent Manager is ready")
                return True
        except:
            pass
        time.sleep(2)
    
    print("✗ Agent Manager not ready")
    return False

def test_list_agents():
    """List all registered agents"""
    print("\n1. Listing registered agents...")
    
    resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/agents")
    if resp.status_code != 200:
        print(f"✗ Failed to list agents: {resp.text}")
        return False
    
    agents = resp.json()['agents']
    print(f"✓ Found {len(agents)} agents:")
    
    meta_agent_found = False
    for agent in agents:
        print(f"  - {agent['name']} ({agent['type']}) - Status: {agent['status']}")
        if agent['type'] == 'meta-prompt':
            meta_agent_found = True
    
    if not meta_agent_found:
        print("✗ Meta-prompt agent not found!")
        return False
    
    return True

def test_design_agent():
    """Test designing a new agent using meta-prompt"""
    print("\n2. Testing agent design with meta-prompt...")
    
    design_request = {
        "type": "meta-prompt",
        "priority": "high",
        "payload": {
            "type": "design-agent",
            "taskDescription": "Create an agent that reviews Python code for async/await best practices and suggests improvements",
            "requirements": {
                "language": "Python",
                "framework": "FastAPI",
                "focus": ["async patterns", "performance", "error handling"],
                "outputFormat": "detailed analysis with code examples"
            },
            "context": {
                "projectType": "REST API",
                "teamSize": 5,
                "experienceLevel": "intermediate"
            }
        },
        "metadata": {
            "source": "integration-test",
            "correlationId": "test-001"
        }
    }
    
    print("  Sending design request...")
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=design_request)
    
    if resp.status_code not in [200, 201, 202]:
        print(f"✗ Failed to create design task: {resp.text}")
        return None
    
    task = resp.json()
    task_id = task.get('taskId') or task.get('id')
    print(f"✓ Design task created: {task_id}")
    
    # Poll for task completion
    print("  Waiting for agent design...")
    for i in range(30):  # Wait up to 30 seconds
        time.sleep(1)
        
        # Check task status
        resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/tasks/{task_id}")
        if resp.status_code == 200:
            task_data = resp.json().get('task', {})
            status = task_data.get('status', 'unknown')
            
            if status == 'completed':
                print("✓ Agent design completed!")
                result = task_data.get('result', {})
                
                # Display the designed agent
                if 'agentDesign' in result:
                    design = result['agentDesign']
                    print("\n  Designed Agent:")
                    print(f"  Name: {design.get('name', 'Unknown')}")
                    print(f"  Type: {design.get('type', 'Unknown')}")
                    print(f"  Purpose: {design.get('purpose', 'No purpose defined')}")
                    print("  Capabilities:")
                    for cap in design.get('capabilities', []):
                        print(f"    - {cap}")
                    
                    return result.get('designId')
                
            elif status in ['failed', 'error']:
                print(f"✗ Design task failed: {task_data.get('error', 'Unknown error')}")
                return None
        
        # Show progress
        if i % 5 == 0:
            print(f"  Still waiting... ({i}s)")
    
    print("✗ Design task timed out")
    return None

def test_spawn_agent(design_id):
    """Test spawning a dynamic agent from design"""
    print(f"\n3. Testing agent spawning from design {design_id}...")
    
    spawn_request = {
        "type": "meta-prompt",
        "priority": "high",
        "payload": {
            "type": "spawn-agent",
            "designId": design_id,
            "taskContext": {
                "environment": "development",
                "testMode": True
            },
            "ttl": 60000  # 1 minute TTL for testing
        },
        "metadata": {
            "source": "integration-test",
            "correlationId": "test-002"
        }
    }
    
    print("  Sending spawn request...")
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=spawn_request)
    
    if resp.status_code not in [200, 201, 202]:
        print(f"✗ Failed to create spawn task: {resp.text}")
        return None
    
    task = resp.json()
    task_id = task.get('taskId') or task.get('id')
    print(f"✓ Spawn task created: {task_id}")
    
    # Poll for task completion
    print("  Waiting for agent spawn...")
    for i in range(20):  # Wait up to 20 seconds
        time.sleep(1)
        
        resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/tasks/{task_id}")
        if resp.status_code == 200:
            task_data = resp.json().get('task', {})
            status = task_data.get('status', 'unknown')
            
            if status == 'completed':
                print("✓ Agent spawned successfully!")
                result = task_data.get('result', {})
                
                agent_id = result.get('agentId')
                print(f"  Dynamic Agent ID: {agent_id}")
                print(f"  TTL: {result.get('ttl', 0) / 1000}s")
                print(f"  Status: {result.get('status', 'Unknown')}")
                
                return agent_id
                
            elif status in ['failed', 'error']:
                print(f"✗ Spawn task failed: {task_data.get('error', 'Unknown error')}")
                return None
    
    print("✗ Spawn task timed out")
    return None

def test_use_dynamic_agent(agent_id):
    """Test using the spawned dynamic agent"""
    print(f"\n4. Testing dynamic agent {agent_id}...")
    
    # Create a task for the dynamic agent
    test_code = '''
async def fetch_user_data(user_id: int):
    # Fetch user from database
    user = await db.get_user(user_id)
    if not user:
        return None
    
    # Fetch related data
    posts = await db.get_user_posts(user_id)
    comments = await db.get_user_comments(user_id)
    
    return {
        "user": user,
        "posts": posts,
        "comments": comments
    }
'''
    
    analysis_request = {
        "type": "dynamic",  # Dynamic agent type
        "priority": "high",
        "payload": {
            "code": test_code,
            "filename": "user_service.py",
            "analysisType": "async-review"
        },
        "requiredCapabilities": ["async-analysis"],
        "metadata": {
            "source": "integration-test",
            "targetAgent": agent_id
        }
    }
    
    print("  Sending analysis request to dynamic agent...")
    resp = requests.post(f"{AGENT_MANAGER_URL}/api/v1/tasks", json=analysis_request)
    
    if resp.status_code not in [200, 201, 202]:
        print(f"✗ Failed to create analysis task: {resp.text}")
        return False
    
    task = resp.json()
    task_id = task.get('taskId') or task.get('id')
    print(f"✓ Analysis task created: {task_id}")
    
    # Poll for completion
    print("  Waiting for analysis...")
    for i in range(15):
        time.sleep(1)
        
        resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/tasks/{task_id}")
        if resp.status_code == 200:
            task_data = resp.json().get('task', {})
            status = task_data.get('status', 'unknown')
            
            if status == 'completed':
                print("✓ Analysis completed!")
                result = task_data.get('result', {})
                
                # Display analysis (truncated)
                if 'output' in result:
                    output = str(result['output'])[:500] + "..." if len(str(result['output'])) > 500 else str(result['output'])
                    print(f"\n  Analysis Output:\n{output}")
                
                return True
                
            elif status in ['failed', 'error']:
                print(f"✗ Analysis failed: {task_data.get('error', 'Unknown error')}")
                return False
    
    print("✗ Analysis timed out")
    return False

def main():
    """Run all meta-prompt agent tests"""
    print("=" * 60)
    print("Meta-Prompt Agent Integration Test")
    print("Using Ollama at model.gonella.co.uk")
    print("=" * 60)
    
    # Check if running in Docker
    in_docker = os.path.exists('/.dockerenv')
    if in_docker:
        print("Running inside Docker container")
        global AGENT_MANAGER_URL
        AGENT_MANAGER_URL = "http://agent-manager:8082"
    
    # Wait for services
    if not wait_for_agent_manager():
        print("\n✗ Agent Manager not ready. Exiting.")
        sys.exit(1)
    
    # Run tests
    tests_passed = 0
    tests_total = 0
    
    # Test 1: List agents
    tests_total += 1
    if test_list_agents():
        tests_passed += 1
    
    # Test 2: Design agent
    tests_total += 1
    design_id = test_design_agent()
    if design_id:
        tests_passed += 1
        
        # Test 3: Spawn agent (only if design succeeded)
        tests_total += 1
        agent_id = test_spawn_agent(design_id)
        if agent_id:
            tests_passed += 1
            
            # Test 4: Use dynamic agent (only if spawn succeeded)
            tests_total += 1
            if test_use_dynamic_agent(agent_id):
                tests_passed += 1
    
    # Summary
    print("\n" + "=" * 60)
    print(f"Test Summary: {tests_passed}/{tests_total} tests passed")
    print("=" * 60)
    
    if tests_passed == tests_total:
        print("\n✓ All tests passed! Meta-prompt agent is working correctly.")
        sys.exit(0)
    else:
        print(f"\n✗ {tests_total - tests_passed} tests failed.")
        sys.exit(1)

if __name__ == "__main__":
    main()