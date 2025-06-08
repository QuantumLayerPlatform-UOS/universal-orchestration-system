#!/bin/bash

# QuantumLayer Platform End-to-End Test Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Base URL configuration
ORCHESTRATOR_URL="http://localhost:8080"
INTENT_PROCESSOR_URL="http://localhost:8082"
AGENT_MANAGER_URL="http://localhost:8084"

# Test data
PROJECT_NAME="E2E Test Project $(date +%s)"
PROJECT_DESCRIPTION="End-to-end test project"

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    
    case $status in
        "success")
            echo -e "${GREEN}✓${NC} $message"
            ;;
        "error")
            echo -e "${RED}✗${NC} $message"
            ;;
        "info")
            echo -e "${YELLOW}→${NC} $message"
            ;;
        "test")
            echo -e "${BLUE}▶${NC} $message"
            ;;
    esac
}

# Function to make API call and check response
api_call() {
    local method=$1
    local url=$2
    local data=$3
    local expected_status=$4
    
    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X $method $url)
    else
        response=$(curl -s -w "\n%{http_code}" -X $method -H "Content-Type: application/json" -d "$data" $url)
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "$expected_status" ]; then
        echo "$body"
        return 0
    else
        print_status "error" "API call failed: $method $url returned $http_code (expected $expected_status)"
        echo "Response body: $body"
        return 1
    fi
}

# Test 1: Create a project
test_create_project() {
    print_status "test" "Test 1: Create a project"
    
    project_data='{
        "name": "'"$PROJECT_NAME"'",
        "description": "'"$PROJECT_DESCRIPTION"'",
        "type": "infrastructure",
        "metadata": {
            "environment": "test",
            "team": "e2e-testing"
        }
    }'
    
    result=$(api_call POST "$ORCHESTRATOR_URL/api/v1/projects" "$project_data" 201)
    if [ $? -eq 0 ]; then
        PROJECT_ID=$(echo $result | jq -r '.id' 2>/dev/null)
        if [ -n "$PROJECT_ID" ] && [ "$PROJECT_ID" != "null" ]; then
            print_status "success" "Project created with ID: $PROJECT_ID"
            return 0
        else
            print_status "error" "Failed to extract project ID from response"
            return 1
        fi
    else
        return 1
    fi
}

# Test 2: Get project details
test_get_project() {
    print_status "test" "Test 2: Get project details"
    
    result=$(api_call GET "$ORCHESTRATOR_URL/api/v1/projects/$PROJECT_ID" "" 200)
    if [ $? -eq 0 ]; then
        name=$(echo $result | jq -r '.name' 2>/dev/null)
        if [ "$name" = "$PROJECT_NAME" ]; then
            print_status "success" "Project retrieved successfully"
            return 0
        else
            print_status "error" "Project name mismatch: expected '$PROJECT_NAME', got '$name'"
            return 1
        fi
    else
        return 1
    fi
}

# Test 3: Start a workflow
test_start_workflow() {
    print_status "test" "Test 3: Start a deployment workflow"
    
    workflow_data='{
        "project_id": "'"$PROJECT_ID"'",
        "type": "deploy_infrastructure",
        "parameters": {
            "environment": "test",
            "components": ["vpc", "compute", "storage"],
            "region": "us-east-1"
        }
    }'
    
    result=$(api_call POST "$ORCHESTRATOR_URL/api/v1/workflows" "$workflow_data" 201)
    if [ $? -eq 0 ]; then
        WORKFLOW_ID=$(echo $result | jq -r '.id' 2>/dev/null)
        if [ -n "$WORKFLOW_ID" ] && [ "$WORKFLOW_ID" != "null" ]; then
            print_status "success" "Workflow started with ID: $WORKFLOW_ID"
            return 0
        else
            print_status "error" "Failed to extract workflow ID from response"
            return 1
        fi
    else
        return 1
    fi
}

# Test 4: Check workflow status
test_workflow_status() {
    print_status "test" "Test 4: Check workflow status"
    
    # Wait a bit for workflow to process
    sleep 3
    
    result=$(api_call GET "$ORCHESTRATOR_URL/api/v1/workflows/$WORKFLOW_ID" "" 200)
    if [ $? -eq 0 ]; then
        status=$(echo $result | jq -r '.status' 2>/dev/null)
        print_status "success" "Workflow status: $status"
        return 0
    else
        return 1
    fi
}

# Test 5: Process an intent
test_process_intent() {
    print_status "test" "Test 5: Process an intent"
    
    intent_data='{
        "type": "scale_service",
        "target": "web-service",
        "parameters": {
            "replicas": 5,
            "cpu": "2",
            "memory": "4Gi"
        }
    }'
    
    result=$(api_call POST "$INTENT_PROCESSOR_URL/api/v1/process" "$intent_data" 200)
    if [ $? -eq 0 ]; then
        INTENT_ID=$(echo $result | jq -r '.id' 2>/dev/null)
        if [ -n "$INTENT_ID" ] && [ "$INTENT_ID" != "null" ]; then
            print_status "success" "Intent processed with ID: $INTENT_ID"
            return 0
        else
            print_status "error" "Failed to extract intent ID from response"
            return 1
        fi
    else
        return 1
    fi
}

# Test 6: List agents
test_list_agents() {
    print_status "test" "Test 6: List available agents"
    
    result=$(api_call GET "$AGENT_MANAGER_URL/api/v1/agents" "" 200)
    if [ $? -eq 0 ]; then
        agent_count=$(echo $result | jq -r '.agents | length' 2>/dev/null)
        if [ "$agent_count" -gt 0 ]; then
            print_status "success" "Found $agent_count agents"
            # Get first agent ID for next test
            AGENT_ID=$(echo $result | jq -r '.agents[0].id' 2>/dev/null)
            return 0
        else
            print_status "error" "No agents found"
            return 1
        fi
    else
        return 1
    fi
}

# Test 7: Execute agent task
test_agent_task() {
    print_status "test" "Test 7: Execute agent task"
    
    task_data='{
        "task": "health_check",
        "parameters": {
            "verbose": true
        }
    }'
    
    result=$(api_call POST "$AGENT_MANAGER_URL/api/v1/agents/$AGENT_ID/execute" "$task_data" 202)
    if [ $? -eq 0 ]; then
        task_id=$(echo $result | jq -r '.task_id' 2>/dev/null)
        if [ -n "$task_id" ] && [ "$task_id" != "null" ]; then
            print_status "success" "Agent task queued with ID: $task_id"
            return 0
        else
            print_status "error" "Failed to extract task ID from response"
            return 1
        fi
    else
        return 1
    fi
}

# Test 8: Get workflow metrics
test_workflow_metrics() {
    print_status "test" "Test 8: Get workflow metrics"
    
    result=$(api_call GET "$ORCHESTRATOR_URL/api/v1/workflows/$WORKFLOW_ID/metrics" "" 200)
    if [ $? -eq 0 ]; then
        print_status "success" "Workflow metrics retrieved successfully"
        return 0
    else
        return 1
    fi
}

# Test 9: List projects
test_list_projects() {
    print_status "test" "Test 9: List all projects"
    
    result=$(api_call GET "$ORCHESTRATOR_URL/api/v1/projects" "" 200)
    if [ $? -eq 0 ]; then
        project_count=$(echo $result | jq -r '.projects | length' 2>/dev/null)
        if [ "$project_count" -gt 0 ]; then
            print_status "success" "Found $project_count projects"
            return 0
        else
            print_status "error" "No projects found"
            return 1
        fi
    else
        return 1
    fi
}

# Test 10: Delete project
test_delete_project() {
    print_status "test" "Test 10: Delete project"
    
    result=$(api_call DELETE "$ORCHESTRATOR_URL/api/v1/projects/$PROJECT_ID" "" 204)
    if [ $? -eq 0 ]; then
        print_status "success" "Project deleted successfully"
        return 0
    else
        # Try 200 as well (some APIs return 200 instead of 204)
        result=$(api_call DELETE "$ORCHESTRATOR_URL/api/v1/projects/$PROJECT_ID" "" 200)
        if [ $? -eq 0 ]; then
            print_status "success" "Project deleted successfully"
            return 0
        else
            return 1
        fi
    fi
}

# Main execution
main() {
    echo "========================================="
    echo "QuantumLayer Platform E2E Tests"
    echo "========================================="
    echo ""
    
    # Check if services are running
    print_status "info" "Checking service availability..."
    
    # Simple health checks
    if ! curl -s -f "$ORCHESTRATOR_URL/health" > /dev/null; then
        print_status "error" "Orchestrator is not responding"
        exit 1
    fi
    
    if ! curl -s -f "$INTENT_PROCESSOR_URL:8083/health" > /dev/null; then
        print_status "error" "Intent Processor is not responding"
        exit 1
    fi
    
    if ! curl -s -f "$AGENT_MANAGER_URL:8085/health" > /dev/null; then
        print_status "error" "Agent Manager is not responding"
        exit 1
    fi
    
    print_status "success" "All services are available"
    echo ""
    
    # Run tests
    tests_passed=0
    tests_failed=0
    
    # Array of test functions
    tests=(
        "test_create_project"
        "test_get_project"
        "test_start_workflow"
        "test_workflow_status"
        "test_process_intent"
        "test_list_agents"
        "test_agent_task"
        "test_workflow_metrics"
        "test_list_projects"
        "test_delete_project"
    )
    
    # Execute each test
    for test in "${tests[@]}"; do
        if $test; then
            ((tests_passed++))
        else
            ((tests_failed++))
        fi
        echo ""
    done
    
    # Summary
    echo "========================================="
    echo "E2E Test Summary"
    echo "========================================="
    echo "Total tests: $((tests_passed + tests_failed))"
    echo -e "${GREEN}Passed: $tests_passed${NC}"
    echo -e "${RED}Failed: $tests_failed${NC}"
    echo ""
    
    # Save report
    report_file="e2e-test-report-$(date +%Y%m%d-%H%M%S).txt"
    echo "E2E Test Report - $(date)" > $report_file
    echo "=========================================" >> $report_file
    echo "Total tests: $((tests_passed + tests_failed))" >> $report_file
    echo "Passed: $tests_passed" >> $report_file
    echo "Failed: $tests_failed" >> $report_file
    echo "" >> $report_file
    echo "Test details saved to: $report_file"
    
    # Exit with appropriate code
    if [ "$tests_failed" -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}

# Run main function
main