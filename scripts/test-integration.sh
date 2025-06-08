#!/bin/bash

# QuantumLayer Platform Integration Test Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
MAX_WAIT_TIME=300  # 5 minutes
SLEEP_INTERVAL=5

# Services to check
SERVICES=(
    "orchestrator:8080"
    "intent-processor:8082"
    "agent-manager:8084"
    "postgres:5432"
    "mongodb:27017"
    "redis:6379"
    "temporal:7233"
    "jaeger:16686"
    "prometheus:9090"
    "grafana:3000"
)

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
    esac
}

# Function to check if a service is healthy
check_service() {
    local service=$1
    local host=$(echo $service | cut -d: -f1)
    local port=$(echo $service | cut -d: -f2)
    
    if nc -z localhost $port 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Function to wait for a service to be ready
wait_for_service() {
    local service=$1
    local elapsed=0
    
    print_info "Waiting for $service to be ready..."
    
    while ! check_service $service; do
        if [ $elapsed -ge $MAX_WAIT_TIME ]; then
            print_status "error" "$service failed to start within $MAX_WAIT_TIME seconds"
            return 1
        fi
        
        sleep $SLEEP_INTERVAL
        elapsed=$((elapsed + SLEEP_INTERVAL))
        echo -n "."
    done
    
    echo ""
    print_status "success" "$service is ready"
    return 0
}

# Function to check HTTP endpoint
check_http_endpoint() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}
    
    response=$(curl -s -o /dev/null -w "%{http_code}" $url)
    
    if [ "$response" = "$expected_status" ]; then
        print_status "success" "$name endpoint is responding correctly (HTTP $response)"
        return 0
    else
        print_status "error" "$name endpoint returned HTTP $response (expected $expected_status)"
        return 1
    fi
}

# Function to test service connectivity
test_service_connectivity() {
    print_status "info" "Testing service connectivity..."
    
    # Check health endpoints
    check_http_endpoint "Orchestrator health" "http://localhost:8081/health"
    check_http_endpoint "Intent Processor health" "http://localhost:8083/health"
    check_http_endpoint "Agent Manager health" "http://localhost:8085/health"
    
    # Check metrics endpoints
    check_http_endpoint "Orchestrator metrics" "http://localhost:8081/metrics"
    check_http_endpoint "Intent Processor metrics" "http://localhost:8083/metrics"
    check_http_endpoint "Agent Manager metrics" "http://localhost:8085/metrics"
    
    # Check UI services
    check_http_endpoint "Temporal UI" "http://localhost:8088"
    check_http_endpoint "Jaeger UI" "http://localhost:16686"
    check_http_endpoint "Prometheus" "http://localhost:9090"
    check_http_endpoint "Grafana" "http://localhost:3000" 302
}

# Function to run a simple end-to-end test
run_e2e_test() {
    print_status "info" "Running end-to-end test flow..."
    
    # Create a test intent
    print_status "info" "Creating test intent..."
    intent_response=$(curl -s -X POST http://localhost:8080/api/v1/intents \
        -H "Content-Type: application/json" \
        -d '{
            "type": "deploy_service",
            "parameters": {
                "service": "test-service",
                "environment": "development",
                "version": "1.0.0"
            }
        }')
    
    intent_id=$(echo $intent_response | jq -r '.id' 2>/dev/null)
    
    if [ -z "$intent_id" ] || [ "$intent_id" = "null" ]; then
        print_status "error" "Failed to create intent: $intent_response"
        return 1
    fi
    
    print_status "success" "Intent created with ID: $intent_id"
    
    # Check intent status
    print_status "info" "Checking intent status..."
    sleep 2
    
    status_response=$(curl -s http://localhost:8080/api/v1/intents/$intent_id)
    status=$(echo $status_response | jq -r '.status' 2>/dev/null)
    
    if [ -z "$status" ] || [ "$status" = "null" ]; then
        print_status "error" "Failed to get intent status: $status_response"
        return 1
    fi
    
    print_status "success" "Intent status: $status"
    
    # Check if intent was processed
    print_status "info" "Verifying intent processing..."
    
    # Query Temporal workflows
    temporal_workflows=$(docker exec qlp-temporal temporal workflow list --namespace default 2>/dev/null | grep -c "$intent_id" || true)
    
    if [ "$temporal_workflows" -gt 0 ]; then
        print_status "success" "Intent workflow found in Temporal"
    else
        print_status "error" "Intent workflow not found in Temporal"
    fi
    
    # Check tracing
    print_status "info" "Verifying distributed tracing..."
    trace_count=$(curl -s "http://localhost:16686/api/traces?service=orchestrator&limit=10" | jq '. | length' 2>/dev/null || echo "0")
    
    if [ "$trace_count" -gt 0 ]; then
        print_status "success" "Traces found in Jaeger"
    else
        print_status "error" "No traces found in Jaeger"
    fi
    
    # Check metrics
    print_status "info" "Verifying metrics collection..."
    metrics_response=$(curl -s "http://localhost:9090/api/v1/query?query=up" | jq -r '.status' 2>/dev/null)
    
    if [ "$metrics_response" = "success" ]; then
        print_status "success" "Metrics are being collected in Prometheus"
    else
        print_status "error" "Metrics collection issue in Prometheus"
    fi
}

# Function to generate test report
generate_report() {
    local test_name=$1
    local status=$2
    local details=$3
    
    echo "Test: $test_name" >> integration-test-report.txt
    echo "Status: $status" >> integration-test-report.txt
    echo "Details: $details" >> integration-test-report.txt
    echo "---" >> integration-test-report.txt
}

# Main execution
main() {
    echo "========================================="
    echo "QuantumLayer Platform Integration Tests"
    echo "========================================="
    echo ""
    
    # Initialize report
    echo "Integration Test Report - $(date)" > integration-test-report.txt
    echo "=========================================" >> integration-test-report.txt
    echo "" >> integration-test-report.txt
    
    # Step 1: Wait for all services to be healthy
    print_status "info" "Step 1: Checking service availability"
    
    all_services_ready=true
    for service in "${SERVICES[@]}"; do
        if ! wait_for_service $service; then
            all_services_ready=false
            generate_report "Service Check: $service" "FAILED" "Service did not start"
        else
            generate_report "Service Check: $service" "PASSED" "Service is healthy"
        fi
    done
    
    if [ "$all_services_ready" = false ]; then
        print_status "error" "Some services failed to start. Aborting tests."
        exit 1
    fi
    
    echo ""
    
    # Step 2: Test service connectivity
    print_status "info" "Step 2: Testing service connectivity"
    if test_service_connectivity; then
        generate_report "Service Connectivity" "PASSED" "All endpoints responding"
    else
        generate_report "Service Connectivity" "FAILED" "Some endpoints not responding"
    fi
    
    echo ""
    
    # Step 3: Run end-to-end test
    print_status "info" "Step 3: Running end-to-end test"
    if run_e2e_test; then
        generate_report "End-to-End Test" "PASSED" "Intent created and processed successfully"
        print_status "success" "End-to-end test completed successfully"
    else
        generate_report "End-to-End Test" "FAILED" "Intent processing failed"
        print_status "error" "End-to-end test failed"
    fi
    
    echo ""
    echo "========================================="
    echo "Integration Test Summary"
    echo "========================================="
    
    # Display summary
    passed=$(grep -c "PASSED" integration-test-report.txt)
    failed=$(grep -c "FAILED" integration-test-report.txt)
    
    echo "Total tests: $((passed + failed))"
    echo -e "${GREEN}Passed: $passed${NC}"
    echo -e "${RED}Failed: $failed${NC}"
    
    echo ""
    echo "Full report saved to: integration-test-report.txt"
    
    # Exit with appropriate code
    if [ "$failed" -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}

# Run main function
main