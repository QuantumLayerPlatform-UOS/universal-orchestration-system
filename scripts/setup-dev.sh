#!/bin/bash

# QuantumLayer Platform - Development Environment Setup
# This script sets up the complete development environment for UOS

set -e

echo "🚀 QuantumLayer Platform - Development Setup"
echo "============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing_deps=()
    
    if ! command_exists docker; then
        missing_deps+=("docker")
    fi
    
    if ! command_exists docker-compose; then
        missing_deps+=("docker-compose")
    fi
    
    if ! command_exists go; then
        missing_deps+=("go (version 1.21+)")
    fi
    
    if ! command_exists node; then
        missing_deps+=("node.js (version 20+)")
    fi
    
    if ! command_exists python3; then
        missing_deps+=("python3 (version 3.11+)")
    fi
    
    if ! command_exists git; then
        missing_deps+=("git")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing dependencies:"
        for dep in "${missing_deps[@]}"; do
            echo "  - $dep"
        done
        echo ""
        echo "Please install the missing dependencies and run this script again."
        exit 1
    fi
    
    log_success "All prerequisites are installed"
}

# Setup environment variables
setup_environment() {
    log_info "Setting up environment variables..."
    
    if [ ! -f .env ]; then
        cat > .env << EOF
# QuantumLayer Platform Environment Configuration

# Database Configuration
POSTGRES_URL=postgresql://dev:dev_password_123@localhost:5432/quantumlayer
REDIS_URL=redis://localhost:6379
NEO4J_URL=bolt://neo4j:dev_password_123@localhost:7687

# Temporal Configuration
TEMPORAL_HOST_PORT=localhost:7233
TEMPORAL_NAMESPACE=default

# AI/ML Configuration
OPENAI_API_KEY=your_openai_api_key_here
ANTHROPIC_API_KEY=your_anthropic_api_key_here

# Service Configuration
ORCHESTRATOR_PORT=8001
INTENT_PROCESSOR_PORT=8002
AGENT_MANAGER_PORT=8003
DEPLOYMENT_ENGINE_PORT=8004

# Monitoring
PROMETHEUS_URL=http://localhost:9090
GRAFANA_URL=http://localhost:3001
JAEGER_URL=http://localhost:16686

# Security
JWT_SECRET=your_jwt_secret_here_change_in_production
ENCRYPTION_KEY=your_encryption_key_here_change_in_production

# Development
LOG_LEVEL=debug
ENVIRONMENT=development
EOF
        log_success "Created .env file with default configuration"
        log_warning "Please update API keys in .env file before running services"
    else
        log_info ".env file already exists"
    fi
}

# Setup Git hooks
setup_git_hooks() {
    log_info "Setting up Git hooks..."
    
    if [ ! -d .git ]; then
        log_warning "Not a Git repository. Skipping Git hooks setup."
        return
    fi
    
    # Pre-commit hook
    cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash

echo "Running pre-commit checks..."

# Check Go formatting
if command -v gofmt >/dev/null 2>&1; then
    unformatted=$(find . -name "*.go" -not -path "./vendor/*" | xargs gofmt -l)
    if [ -n "$unformatted" ]; then
        echo "Go files need formatting:"
        echo "$unformatted"
        echo "Run: gofmt -w ."
        exit 1
    fi
fi

# Run Go tests
if command -v go >/dev/null 2>&1; then
    if [ -f go.mod ]; then
        go test ./... || exit 1
    fi
fi

echo "Pre-commit checks passed!"
EOF
    
    chmod +x .git/hooks/pre-commit
    log_success "Git hooks configured"
}

# Create necessary directories
create_directories() {
    log_info "Creating project directories..."
    
    local directories=(
        "services/intent-processor"
        "services/agent-manager"
        "services/deployment-engine"
        "web/dashboard"
        "web/api-docs"
        "infrastructure/terraform"
        "infrastructure/kubernetes"
        "infrastructure/monitoring"
        "tools/cli"
        "tools/testing"
        "docs/api"
        "docs/deployment"
        "logs"
        "tmp"
    )
    
    for dir in "${directories[@]}"; do
        mkdir -p "$dir"
    done
    
    log_success "Project directories created"
}

# Setup monitoring configuration
setup_monitoring() {
    log_info "Setting up monitoring configuration..."
    
    mkdir -p infrastructure/monitoring
    
    # Prometheus configuration
    cat > infrastructure/monitoring/prometheus.yml << EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'orchestrator'
    static_configs:
      - targets: ['localhost:8001']
    metrics_path: '/metrics'

  - job_name: 'intent-processor'
    static_configs:
      - targets: ['localhost:8002']
    metrics_path: '/metrics'

  - job_name: 'agent-manager'
    static_configs:
      - targets: ['localhost:8003']
    metrics_path: '/metrics'

  - job_name: 'deployment-engine'
    static_configs:
      - targets: ['localhost:8004']
    metrics_path: '/metrics'
EOF

    log_success "Monitoring configuration created"
}

# Start development services
start_services() {
    log_info "Starting development services..."
    
    # Pull latest images
    docker-compose pull
    
    # Start services
    docker-compose up -d
    
    # Wait for services to be ready
    log_info "Waiting for services to be ready..."
    sleep 30
    
    # Check service health
    local services=(
        "postgres:5432"
        "redis:6379"
        "neo4j:7474"
        "temporal:7233"
        "kafka:9092"
    )
    
    for service in "${services[@]}"; do
        IFS=':' read -r host port <<< "$service"
        if timeout 10 bash -c "</dev/tcp/$host/$port"; then
            log_success "$host is ready on port $port"
        else
            log_error "$host is not ready on port $port"
        fi
    done
}

# Setup development databases
setup_databases() {
    log_info "Setting up development databases..."
    
    # Wait for PostgreSQL to be ready
    timeout 60 bash -c 'until pg_isready -h localhost -p 5432 -U dev; do sleep 1; done'
    
    # Create database schema (placeholder)
    log_info "Database schema will be created by migration scripts"
    
    log_success "Database setup completed"
}

# Display service information
show_service_info() {
    echo ""
    echo "🎉 Development environment is ready!"
    echo "====================================="
    echo ""
    echo "📊 Service URLs:"
    echo "  • Temporal UI:    http://localhost:8080"
    echo "  • Prometheus:     http://localhost:9090"
    echo "  • Grafana:        http://localhost:3001 (admin/admin123)"
    echo "  • Jaeger:         http://localhost:16686"
    echo "  • Neo4j Browser:  http://localhost:7474 (neo4j/dev_password_123)"
    echo "  • MinIO Console:  http://localhost:9001 (minioadmin/minioadmin123)"
    echo ""
    echo "🗄️  Database Connections:"
    echo "  • PostgreSQL:     localhost:5432 (dev/dev_password_123)"
    echo "  • Redis:          localhost:6379"
    echo "  • Neo4j:          bolt://localhost:7687"
    echo "  • Kafka:          localhost:9092"
    echo ""
    echo "🚀 Next Steps:"
    echo "  1. Update API keys in .env file"
    echo "  2. Run 'make dev' to start all services"
    echo "  3. Visit http://localhost:3000 for the dashboard"
    echo ""
    echo "📚 Documentation:"
    echo "  • Architecture:   docs/technical-architecture.md"
    echo "  • Development:    docs/development-guide.md"
    echo "  • API Reference:  docs/api/README.md"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    setup_environment
    create_directories
    setup_git_hooks
    setup_monitoring
    start_services
    setup_databases
    show_service_info
}

# Parse command line arguments
case "${1:-}" in
    --skip-services)
        log_info "Skipping service startup"
        check_prerequisites
        setup_environment
        create_directories
        setup_git_hooks
        setup_monitoring
        show_service_info
        ;;
    --services-only)
        log_info "Starting services only"
        start_services
        ;;
    *)
        main
        ;;
esac
