#!/usr/bin/env python3
"""
Basic integration tests to verify core services can communicate.
"""

import requests
import time
import sys
import json

# Service endpoints
INTENT_PROCESSOR_URL = "http://localhost:8081"
AGENT_MANAGER_URL = "http://localhost:8082"
ORCHESTRATOR_URL = "http://localhost:8080"

# Test timeout
TIMEOUT = 5


def wait_for_service(url, service_name, max_retries=30):
    """Wait for a service to become available."""
    print(f"Waiting for {service_name} at {url}...")
    for i in range(max_retries):
        try:
            response = requests.get(f"{url}/health", timeout=TIMEOUT)
            if response.status_code == 200:
                print(f"✓ {service_name} is ready")
                return True
        except requests.exceptions.RequestException:
            pass
        time.sleep(1)
    print(f"✗ {service_name} failed to start")
    return False


def test_intent_processor():
    """Test Intent Processor can receive and process a request."""
    print("\nTesting Intent Processor...")
    
    # Test health endpoint
    try:
        response = requests.get(f"{INTENT_PROCESSOR_URL}/health", timeout=TIMEOUT)
        assert response.status_code == 200, f"Health check failed: {response.status_code}"
        print("  ✓ Health endpoint working")
    except Exception as e:
        print(f"  ✗ Health check failed: {e}")
        return False
    
    # Test intent processing endpoint
    try:
        test_payload = {
            "query": "test query",
            "context": {"test": "data"}
        }
        response = requests.post(
            f"{INTENT_PROCESSOR_URL}/api/v1/process",
            json=test_payload,
            timeout=TIMEOUT
        )
        # We expect either 200 or 501 (not implemented) for now
        assert response.status_code in [200, 501], f"Process endpoint failed: {response.status_code}"
        print("  ✓ Process endpoint accessible")
        return True
    except Exception as e:
        print(f"  ✗ Process endpoint failed: {e}")
        return False


def test_agent_manager():
    """Test Agent Manager can register an agent."""
    print("\nTesting Agent Manager...")
    
    # Test health endpoint
    try:
        response = requests.get(f"{AGENT_MANAGER_URL}/health", timeout=TIMEOUT)
        assert response.status_code == 200, f"Health check failed: {response.status_code}"
        print("  ✓ Health endpoint working")
    except Exception as e:
        print(f"  ✗ Health check failed: {e}")
        return False
    
    # Test agent registration endpoint
    try:
        test_agent = {
            "name": "test-agent",
            "type": "test",
            "capabilities": ["test"],
            "endpoint": "http://test-agent:8000"
        }
        response = requests.post(
            f"{AGENT_MANAGER_URL}/api/v1/agents/register",
            json=test_agent,
            timeout=TIMEOUT
        )
        # We expect either 200, 201 or 501 (not implemented) for now
        assert response.status_code in [200, 201, 501], f"Register endpoint failed: {response.status_code}"
        print("  ✓ Register endpoint accessible")
        return True
    except Exception as e:
        print(f"  ✗ Register endpoint failed: {e}")
        return False


def test_orchestrator():
    """Test Orchestrator can create a workflow."""
    print("\nTesting Orchestrator...")
    
    # Test health endpoint
    try:
        response = requests.get(f"{ORCHESTRATOR_URL}/health", timeout=TIMEOUT)
        assert response.status_code == 200, f"Health check failed: {response.status_code}"
        print("  ✓ Health endpoint working")
    except Exception as e:
        print(f"  ✗ Health check failed: {e}")
        return False
    
    # Test workflow creation endpoint
    try:
        test_workflow = {
            "name": "test-workflow",
            "steps": [
                {
                    "name": "step1",
                    "type": "test",
                    "config": {}
                }
            ]
        }
        response = requests.post(
            f"{ORCHESTRATOR_URL}/api/v1/workflows",
            json=test_workflow,
            timeout=TIMEOUT
        )
        # We expect either 200, 201 or 501 (not implemented) for now
        assert response.status_code in [200, 201, 501], f"Workflow endpoint failed: {response.status_code}"
        print("  ✓ Workflow endpoint accessible")
        return True
    except Exception as e:
        print(f"  ✗ Workflow endpoint failed: {e}")
        return False


def main():
    """Run all tests."""
    print("Starting basic integration tests...\n")
    
    # Wait for all services to be ready
    services = [
        (INTENT_PROCESSOR_URL, "Intent Processor"),
        (AGENT_MANAGER_URL, "Agent Manager"),
        (ORCHESTRATOR_URL, "Orchestrator")
    ]
    
    all_ready = True
    for url, name in services:
        if not wait_for_service(url, name):
            all_ready = False
    
    if not all_ready:
        print("\n✗ Not all services are ready. Exiting...")
        sys.exit(1)
    
    # Run tests
    results = []
    results.append(("Intent Processor", test_intent_processor()))
    results.append(("Agent Manager", test_agent_manager()))
    results.append(("Orchestrator", test_orchestrator()))
    
    # Summary
    print("\n" + "=" * 50)
    print("Test Summary:")
    print("=" * 50)
    
    all_passed = True
    for service, passed in results:
        status = "✓ PASSED" if passed else "✗ FAILED"
        print(f"{service}: {status}")
        if not passed:
            all_passed = False
    
    print("=" * 50)
    
    if all_passed:
        print("\n✓ All tests passed!")
        sys.exit(0)
    else:
        print("\n✗ Some tests failed.")
        sys.exit(1)


if __name__ == "__main__":
    main()
