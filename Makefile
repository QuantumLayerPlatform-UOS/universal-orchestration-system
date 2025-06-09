# QuantumLayer Platform Makefile
# Always Demo-Ready Commands

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
NC     := \033[0m # No Color

.PHONY: help
help: ## Display this help message
	@echo "$(GREEN)QuantumLayer Platform - Universal Orchestration System$(NC)"
	@echo "$(YELLOW)Always Demo-Ready Commands:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-30s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Quick Start:$(NC) make up && make demo"

# Quick Commands for Demo
.PHONY: quick
quick: up demo ## Quick demo (start services and run demo)

# Development Stack Commands
.PHONY: dev-up
dev-up: ## Start the entire development stack
	@echo "Starting QuantumLayer Platform development environment..."
	docker-compose -f docker-compose.dev.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 10
	@make health-check
	@echo "Development stack is ready!"
	@echo "Services available at:"
	@echo "  - Orchestrator: http://localhost:8080"
	@echo "  - Intent Processor: http://localhost:8082"
	@echo "  - Agent Manager: http://localhost:8084"
	@echo "  - Temporal UI: http://localhost:8088"
	@echo "  - Jaeger UI: http://localhost:16686"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Grafana: http://localhost:3000 (admin/admin)"

.PHONY: dev-down
dev-down: ## Stop and remove all development containers
	@echo "Stopping development environment..."
	docker-compose -f docker-compose.dev.yml down
	@echo "Development environment stopped."

.PHONY: dev-clean
dev-clean: ## Stop containers and remove volumes
	@echo "Cleaning up development environment..."
	docker-compose -f docker-compose.dev.yml down -v
	@echo "Development environment cleaned."

.PHONY: dev-restart
dev-restart: dev-down dev-up ## Restart the development stack

# Service Management
.PHONY: restart-service
restart-service: ## Restart a specific service (usage: make restart-service SERVICE=orchestrator)
	@if [ -z "$(SERVICE)" ]; then \
		echo "Error: SERVICE not specified. Usage: make restart-service SERVICE=orchestrator"; \
		exit 1; \
	fi
	docker-compose -f docker-compose.dev.yml restart $(SERVICE)

# Logging Commands
.PHONY: logs
logs: ## Show logs for all services
	docker-compose -f docker-compose.dev.yml logs -f

.PHONY: logs-service
logs-service: ## Show logs for a specific service (usage: make logs-service SERVICE=orchestrator)
	@if [ -z "$(SERVICE)" ]; then \
		echo "Error: SERVICE not specified. Usage: make logs-service SERVICE=orchestrator"; \
		exit 1; \
	fi
	docker-compose -f docker-compose.dev.yml logs -f $(SERVICE)

# Health Check Commands
.PHONY: health-check
health-check: ## Check health of all services
	@echo "Checking service health..."
	@./scripts/health-check.sh

.PHONY: service-status
service-status: ## Show status of all services
	docker-compose -f docker-compose.dev.yml ps

# Testing Commands
.PHONY: test
test: ## Run all tests
	@echo "Running unit tests..."
	@make test-unit
	@echo "Running integration tests..."
	@make test-integration
	@echo "All tests completed!"

.PHONY: test-unit
test-unit: ## Run unit tests for all services
	@echo "Running orchestrator unit tests..."
	cd services/orchestrator && go test ./... -v -cover
	@echo "Running intent-processor unit tests..."
	cd services/intent-processor && go test ./... -v -cover
	@echo "Running agent-manager unit tests..."
	cd services/agent-manager && go test ./... -v -cover

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@./scripts/test-integration.sh

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "Running end-to-end tests..."
	@./scripts/test-e2e.sh

# Build Commands
.PHONY: build
build: ## Build all service images
	@echo "Building all service images..."
	docker-compose -f docker-compose.dev.yml build

.PHONY: build-service
build-service: ## Build a specific service (usage: make build-service SERVICE=orchestrator)
	@if [ -z "$(SERVICE)" ]; then \
		echo "Error: SERVICE not specified. Usage: make build-service SERVICE=orchestrator"; \
		exit 1; \
	fi
	docker-compose -f docker-compose.dev.yml build $(SERVICE)

# Database Commands
.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "Running database migrations..."
	cd services/orchestrator && go run cmd/migrate/main.go up

.PHONY: db-seed
db-seed: ## Seed database with test data
	@echo "Seeding database..."
	cd services/orchestrator && go run cmd/seed/main.go

.PHONY: db-reset
db-reset: ## Reset database (drop and recreate)
	@echo "Resetting database..."
	docker-compose -f docker-compose.dev.yml exec postgres psql -U postgres -c "DROP DATABASE IF EXISTS orchestrator_db;"
	docker-compose -f docker-compose.dev.yml exec postgres psql -U postgres -c "CREATE DATABASE orchestrator_db;"
	@make db-migrate
	@make db-seed

# Monitoring Commands
.PHONY: metrics
metrics: ## Open Prometheus metrics dashboard
	@echo "Opening Prometheus dashboard..."
	@open http://localhost:9090 || xdg-open http://localhost:9090

.PHONY: traces
traces: ## Open Jaeger tracing UI
	@echo "Opening Jaeger UI..."
	@open http://localhost:16686 || xdg-open http://localhost:16686

.PHONY: dashboards
dashboards: ## Open Grafana dashboards
	@echo "Opening Grafana dashboards..."
	@open http://localhost:3000 || xdg-open http://localhost:3000

# Development Tools
.PHONY: lint
lint: ## Run linters on all services
	@echo "Running linters..."
	cd services/orchestrator && golangci-lint run
	cd services/intent-processor && golangci-lint run
	cd services/agent-manager && golangci-lint run

.PHONY: fmt
fmt: ## Format all Go code
	@echo "Formatting Go code..."
	cd services/orchestrator && go fmt ./...
	cd services/intent-processor && go fmt ./...
	cd services/agent-manager && go fmt ./...

.PHONY: deps
deps: ## Update all dependencies
	@echo "Updating dependencies..."
	cd services/orchestrator && go mod tidy
	cd services/intent-processor && go mod tidy
	cd services/agent-manager && go mod tidy

# Infrastructure Commands
.PHONY: infra-init
infra-init: ## Initialize infrastructure configuration
	@echo "Initializing infrastructure..."
	@mkdir -p infrastructure/prometheus
	@mkdir -p infrastructure/grafana/provisioning/dashboards
	@mkdir -p infrastructure/grafana/provisioning/datasources
	@./scripts/init-infra.sh

# Documentation
.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	@./scripts/generate-docs.sh

.PHONY: api-docs
api-docs: ## Generate API documentation
	@echo "Generating API documentation..."
	cd services/orchestrator && swag init -g cmd/server/main.go
	cd services/intent-processor && swag init -g cmd/server/main.go
	cd services/agent-manager && swag init -g cmd/server/main.go

# Utility Commands
.PHONY: clean-docker
clean-docker: ## Clean up Docker resources
	@echo "Cleaning up Docker resources..."
	docker system prune -f
	docker volume prune -f

.PHONY: setup
setup: ## Initial setup for development
	@echo "Setting up development environment..."
	@./scripts/setup-dev.sh
	@make infra-init
	@make dev-up
	@echo "Setup complete!"

# Demo Commands
.PHONY: demo
demo: ## Run interactive demo (always ready!)
	@echo "$(GREEN)🎭 Starting Interactive Demo$(NC)"
	@echo ""
	@echo "Choose a demo scenario:"
	@echo "  1) Create REST API from description"
	@echo "  2) Fix a bug with AI assistance"
	@echo "  3) Generate test suite"
	@echo "  4) Full workflow demonstration"
	@echo ""
	@read -p "Enter choice (1-4): " choice; \
	case $$choice in \
		1) make demo-api ;; \
		2) make demo-bugfix ;; \
		3) make demo-tests ;; \
		4) make demo-full ;; \
		*) echo "Invalid choice" ;; \
	esac

.PHONY: demo-api
demo-api: ## Demo: Create REST API
	@echo "$(YELLOW)Demo: Creating REST API from natural language$(NC)"
	@curl -X POST http://localhost:8081/api/v1/process-intent \
		-H "Content-Type: application/json" \
		-d '{"text": "Create a REST API for user management with CRUD operations", "request_id": "demo-001"}' \
		| jq '.'

.PHONY: up
up: ## Start minimal services for demo
	@echo "$(YELLOW)Starting services for demo...$(NC)"
	@docker-compose -f docker-compose.minimal.yml up -d
	@echo "$(GREEN)✅ Services started! Waiting for health checks...$(NC)"
	@sleep 15
	@make health

.PHONY: down
down: ## Stop all services
	@echo "$(YELLOW)Stopping all services...$(NC)"
	@docker-compose -f docker-compose.minimal.yml down
	@echo "$(GREEN)✅ Services stopped!$(NC)"

.PHONY: health
health: ## Check health of all services (pretty output)
	@echo "$(YELLOW)Checking service health...$(NC)"
	@echo ""
	@echo "🏥 $(GREEN)Health Status:$(NC)"
	@echo "├── Orchestrator:      $$(curl -s http://localhost:8080/health | jq -r '.success' | sed 's/true/✅/;s/false/❌/' 2>/dev/null || echo '❌')"
	@echo "├── Agent Manager:     $$(curl -s http://localhost:8082/health | jq -r '.status' | sed 's/healthy/✅/;s/unhealthy/❌/' 2>/dev/null || echo '❌')"
	@echo "├── Intent Processor:  $$(curl -s http://localhost:8081/health | jq -r '.status' | sed 's/healthy/✅/;s/unhealthy/❌/' 2>/dev/null || echo '❌')"
	@echo "└── Registered Agents: $$(curl -s http://localhost:8082/api/v1/agents | jq '.count' 2>/dev/null || echo '0')"
	@echo ""

.PHONY: update-status
update-status: ## Update README with current status
	@python scripts/update-readme-status.py