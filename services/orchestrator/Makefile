# Variables
APP_NAME := orchestrator
DOCKER_IMAGE := qlp-orchestrator
DOCKER_TAG := latest
GO_VERSION := 1.21
PROTO_DIR := internal/proto
PROTO_OUT := internal/proto

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOLINT := golangci-lint

# Build flags
LDFLAGS := -ldflags "-w -s -X main.Version=$$(git describe --tags --always --dirty) -X main.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Targets
.PHONY: all build clean test coverage lint fmt proto docker help

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## all: Build the application
all: clean lint test build

## build: Build the application binary
build:
	@echo "Building $(APP_NAME)..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME) cmd/server/main.go

## build-linux: Build for Linux
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 cmd/server/main.go

## run: Run the application
run:
	@echo "Running $(APP_NAME)..."
	@$(GOCMD) run cmd/server/main.go

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf bin/
	@rm -rf coverage/

## test: Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v -race -short ./...

## test-all: Run all tests including integration tests
test-all:
	@echo "Running all tests..."
	@$(GOTEST) -v -race ./...

## coverage: Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	@$(GOTEST) -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	@$(GOCMD) tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated at coverage/coverage.html"

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	@$(GOLINT) run --timeout=5m ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@$(GOFMT) -s -w .

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download

## deps-update: Update dependencies
deps-update:
	@echo "Updating dependencies..."
	@$(GOMOD) tidy
	@$(GOMOD) verify

## proto: Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	@protoc --go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/intent/*.proto

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-push: Push Docker image
docker-push:
	@echo "Pushing Docker image..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -d --name $(APP_NAME) -p 8080:8080 -p 9090:9090 $(DOCKER_IMAGE):$(DOCKER_TAG)

## docker-stop: Stop Docker container
docker-stop:
	@echo "Stopping Docker container..."
	@docker stop $(APP_NAME) || true
	@docker rm $(APP_NAME) || true

## compose-up: Start services with docker-compose
compose-up:
	@echo "Starting services..."
	@docker-compose up -d

## compose-down: Stop services with docker-compose
compose-down:
	@echo "Stopping services..."
	@docker-compose down

## compose-logs: View logs
compose-logs:
	@docker-compose logs -f

## compose-ps: List services
compose-ps:
	@docker-compose ps

## migrate-up: Run database migrations up
migrate-up:
	@echo "Running migrations up..."
	@migrate -path migrations -database "$${ORCHESTRATOR_DATABASE_URL}" up

## migrate-down: Run database migrations down
migrate-down:
	@echo "Running migrations down..."
	@migrate -path migrations -database "$${ORCHESTRATOR_DATABASE_URL}" down

## migrate-create: Create a new migration
migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	@$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GOCMD) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## generate: Generate code
generate:
	@echo "Generating code..."
	@$(GOCMD) generate ./...

## check: Run pre-commit checks
check: fmt lint test

## ci: Run CI checks
ci: deps check build

# Development shortcuts
.PHONY: dev dev-deps dev-up dev-down dev-logs

## dev: Start development environment
dev: dev-deps dev-up

## dev-deps: Install development dependencies
dev-deps: install-tools deps

## dev-up: Start development services
dev-up:
	@docker-compose -f docker-compose.yml up -d postgres redis temporal temporal-ui jaeger
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Development services started!"
	@echo "  - PostgreSQL: localhost:5432"
	@echo "  - Redis: localhost:6379"
	@echo "  - Temporal: localhost:7233"
	@echo "  - Temporal UI: http://localhost:8088"
	@echo "  - Jaeger UI: http://localhost:16686"

## dev-down: Stop development services
dev-down:
	@docker-compose -f docker-compose.yml down

## dev-logs: View development services logs
dev-logs:
	@docker-compose -f docker-compose.yml logs -f