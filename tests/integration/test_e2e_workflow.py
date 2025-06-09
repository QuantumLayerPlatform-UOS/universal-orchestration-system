#!/usr/bin/env python3
"""
End-to-End Integration Test for QuantumLayer Platform
Tests the complete workflow from intent to code generation
"""

import requests
import json
import time
import sys

# Service URLs
import os
ORCHESTRATOR_URL = os.getenv("ORCHESTRATOR_URL", "http://localhost:8080")
INTENT_PROCESSOR_URL = os.getenv("INTENT_PROCESSOR_URL", "http://localhost:8081")
AGENT_MANAGER_URL = os.getenv("AGENT_MANAGER_URL", "http://localhost:8082")

def wait_for_services(timeout=30):
    """Wait for all services to be healthy"""
    print("Waiting for services to be ready...")
    services = {
        "Orchestrator": f"{ORCHESTRATOR_URL}/health",
        "Agent Manager": f"{AGENT_MANAGER_URL}/health",
        "Intent Processor": f"{INTENT_PROCESSOR_URL}/health"
    }
    
    start_time = time.time()
    while time.time() - start_time < timeout:
        all_healthy = True
        for name, url in services.items():
            try:
                resp = requests.get(url, timeout=2)
                if resp.status_code != 200:
                    all_healthy = False
                    print(f"  {name}: Not ready (status {resp.status_code})")
                else:
                    print(f"  {name}: ✓ Ready")
            except Exception as e:
                all_healthy = False
                print(f"  {name}: Not ready ({str(e)})")
        
        if all_healthy:
            print("\nAll services are ready!")
            return True
        
        time.sleep(2)
    
    print("\nTimeout waiting for services")
    return False

def test_project_creation():
    """Test creating a new project via orchestrator"""
    print("\n1. Testing Project Creation...")
    
    import uuid
    project_data = {
        "id": str(uuid.uuid4()),
        "name": "Test E2E Project",
        "description": "Testing end-to-end workflow",
        "owner_id": "test-user-001"
    }
    
    resp = requests.post(f"{ORCHESTRATOR_URL}/api/v1/projects", json=project_data)
    if resp.status_code != 200:
        print(f"  ✗ Failed to create project: {resp.text}")
        return None
    
    project = resp.json()
    print(f"  ✓ Project created: {project['data']['id']}")
    return project['data']['id']

def test_intent_processing():
    """Test processing natural language intent"""
    print("\n2. Testing Intent Processing...")
    
    import uuid
    intent_data = {
        "request_id": str(uuid.uuid4()),
        "text": "Create a REST API for user management with CRUD operations",
        "context": {
            "project_id": "test-project",
            "user_id": "test-user"
        }
    }
    
    resp = requests.post(f"{INTENT_PROCESSOR_URL}/api/v1/process-intent", json=intent_data)
    if resp.status_code != 200:
        print(f"  ✗ Failed to process intent: {resp.text}")
        return None
    
    result = resp.json()
    print(f"  ✓ Intent processed successfully")
    print(f"    - Intent: {result.get('intent', 'N/A')}")
    print(f"    - Tasks: {len(result.get('tasks', []))}")
    return result

def test_agent_registration():
    """Test checking if agents are registered"""
    print("\n3. Testing Agent Registration...")
    
    resp = requests.get(f"{AGENT_MANAGER_URL}/api/v1/agents")
    if resp.status_code != 200:
        print(f"  ✗ Failed to get agents: {resp.text}")
        return False
    
    result = resp.json()
    agents = result.get('agents', [])
    print(f"  ✓ Found {len(agents)} registered agents")
    for agent in agents:
        print(f"    - {agent['name']} ({agent['type']}) - Status: {agent['status']}")
    
    return len(agents) > 0

def test_workflow_execution():
    """Test executing a complete workflow"""
    print("\n4. Testing Workflow Execution...")
    
    workflow_data = {
        "project_id": "test-project-001",
        "type": "code_generation",
        "input": {
            "requirements": "Create a REST API for user management",
            "technology": "Node.js with Express",
            "features": ["CRUD operations", "Authentication", "Validation"]
        }
    }
    
    resp = requests.post(f"{ORCHESTRATOR_URL}/api/v1/workflows", json=workflow_data)
    if resp.status_code != 200:
        print(f"  ✗ Failed to start workflow: {resp.text}")
        return None
    
    workflow = resp.json()
    workflow_id = workflow['data']['id']
    print(f"  ✓ Workflow started: {workflow_id}")
    
    # Poll for workflow completion
    print("  Waiting for workflow to complete...")
    for i in range(30):  # Wait up to 30 seconds
        resp = requests.get(f"{ORCHESTRATOR_URL}/api/v1/workflows/{workflow_id}")
        if resp.status_code == 200:
            status = resp.json()['data']['status']
            print(f"    Status: {status}")
            if status in ['completed', 'failed']:
                return status == 'completed'
        time.sleep(1)
    
    print("  ✗ Workflow timed out")
    return False

def main():
    """Run all integration tests"""
    print("=" * 60)
    print("QuantumLayer Platform - End-to-End Integration Test")
    print("=" * 60)
    
    # Wait for services
    if not wait_for_services():
        print("\n✗ Services not ready. Exiting.")
        sys.exit(1)
    
    # Run tests
    tests_passed = 0
    tests_total = 0
    
    # Test 1: Project Creation
    tests_total += 1
    project_id = test_project_creation()
    if project_id:
        tests_passed += 1
    
    # Test 2: Intent Processing
    tests_total += 1
    intent_result = test_intent_processing()
    if intent_result:
        tests_passed += 1
    
    # Test 3: Agent Registration
    tests_total += 1
    if test_agent_registration():
        tests_passed += 1
    
    # Test 4: Workflow Execution
    tests_total += 1
    if test_workflow_execution():
        tests_passed += 1
    
    # Summary
    print("\n" + "=" * 60)
    print(f"Test Summary: {tests_passed}/{tests_total} tests passed")
    print("=" * 60)
    
    if tests_passed == tests_total:
        print("\n✓ All tests passed! The system is working end-to-end.")
        sys.exit(0)
    else:
        print(f"\n✗ {tests_total - tests_passed} tests failed.")
        sys.exit(1)

if __name__ == "__main__":
    main()