# Orchestrator Service

The Orchestrator service is the core workflow coordination component of the QuantumLayer Platform UOS. It manages the execution of complex workflows, coordinates between different services, and ensures reliable task execution using Temporal.

## Features

- **Workflow Management**: Create, execute, and monitor complex workflows
- **Intent Processing**: Process natural language intents through integrated services
- **Code Execution**: Coordinate code execution across distributed agents
- **Project Management**: Manage projects, environments, and resources
- **Distributed Tracing**: Full observability with OpenTelemetry and Jaeger
- **Metrics & Monitoring**: Prometheus metrics and Grafana dashboards
- **High Availability**: Built on Temporal for reliable workflow execution
- **REST API**: Comprehensive HTTP API for all operations
- **WebSocket Support**: Real-time updates for workflow status

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   REST API      │     │  Intent Service │     │  Agent Manager  │
│   (Gin)         │────▶│    (gRPC)       │────▶│  (HTTP/WS)      │
└────────┬────────┘     └─────────────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Workflow Engine │────▶│    Temporal     │────▶│   PostgreSQL    │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│     Redis       │     │     Jaeger      │
│    (Cache)      │     │   (Tracing)     │
└─────────────────┘     └─────────────────┘
```

## Prerequisites

- Go 1.21+
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+
- Temporal Server
- Protocol Buffers compiler (protoc)

## Quick Start

### Using Docker Compose

```bash
# Start all services
make compose-up

# View logs
make compose-logs

# Stop services
make compose-down
```

### Local Development

```bash
# Install dependencies
make deps

# Start development services (PostgreSQL, Redis, Temporal)
make dev-up

# Run the service
make run

# Run tests
make test

# Run with coverage
make coverage
```

## Configuration

The service can be configured via environment variables or a config file. Key configuration options:

```yaml
# config.yaml
server:
  port: 8080
  host: 0.0.0.0
  enable_metrics: true
  metrics_port: 9090

database:
  url: postgres://user:pass@localhost:5432/orchestrator?sslmode=disable
  max_open_conns: 25
  max_idle_conns: 5

redis:
  addr: localhost:6379
  db: 0

temporal:
  host_port: localhost:7233
  namespace: default
  task_queue: orchestrator-task-queue

telemetry:
  enabled: true
  service_name: orchestrator
  environment: development
  jaeger:
    collector_endpoint: http://localhost:14268/api/traces
```

## API Documentation

### Projects API

```bash
# Create a project
POST /api/v1/projects
{
  "name": "My Project",
  "description": "Project description",
  "type": "standard"
}

# Get project
GET /api/v1/projects/{id}

# List projects
GET /api/v1/projects?status=active&limit=10

# Update project
PUT /api/v1/projects/{id}

# Delete project
DELETE /api/v1/projects/{id}
```

### Workflows API

```bash
# Start a workflow
POST /api/v1/workflows
{
  "name": "Process Intent",
  "type": "intent_processing",
  "project_id": "project-uuid",
  "input": {
    "intent": "create a REST API"
  }
}

# Get workflow status
GET /api/v1/workflows/{id}

# List workflows
GET /api/v1/workflows?project_id=xxx&status=running

# Cancel workflow
POST /api/v1/workflows/{id}/cancel

# Get workflow metrics
GET /api/v1/workflows/{id}/metrics
```

### Agents API

```bash
# List agents
GET /api/v1/agents

# Get agent details
GET /api/v1/agents/{id}

# Restart agent
POST /api/v1/agents/{id}/restart
```

## Workflow Types

### 1. Intent Processing Workflow
Processes natural language intents through the Intent Processor service.

```go
type IntentProcessingWorkflow struct {
    Steps: []string{
        "AnalyzeIntent",
        "CreateExecutionPlan", 
        "ExecuteSteps",
        "AggregateResults",
    }
}
```

### 2. Code Execution Workflow
Executes code on distributed agents.

```go
type CodeExecutionWorkflow struct {
    Steps: []string{
        "SelectAgent",
        "PrepareEnvironment",
        "ExecuteCode",
        "ProcessResults",
        "CleanupEnvironment",
    }
}
```

### 3. Code Analysis Workflow
Performs comprehensive code analysis.

```go
type CodeAnalysisWorkflow struct {
    Steps: []string{
        "FetchCode",
        "RunStaticAnalysis",
        "RunSecurityAnalysis",
        "RunPerformanceAnalysis",
        "GenerateReport",
    }
}
```

### 4. Code Review Workflow
Automated code review with AI assistance.

```go
type CodeReviewWorkflow struct {
    Steps: []string{
        "FetchChanges",
        "RunAutomatedChecks",
        "RunAIReview",
        "GenerateSummary",
        "PostComments",
    }
}
```

### 5. Deployment Workflow
Manages application deployments.

```go
type DeploymentWorkflow struct {
    Steps: []string{
        "ValidateDeployment",
        "BuildArtifacts",
        "RunTests",
        "DeployToStaging",
        "RunSmokeTests",
        "DeployToProduction",
        "HealthCheck",
    }
}
```

## Development

### Project Structure

```
orchestrator/
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── api/
│   │   └── handlers.go      # HTTP handlers
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── database/
│   │   └── database.go      # Database connection
│   ├── middleware/
│   │   └── middleware.go    # HTTP middleware
│   ├── models/
│   │   ├── workflow.go      # Workflow models
│   │   ├── project.go       # Project models
│   │   └── execution.go     # Execution models
│   ├── proto/
│   │   └── intent/          # Protobuf definitions
│   ├── services/
│   │   ├── workflow_engine.go
│   │   ├── intent_client.go
│   │   ├── agent_client.go
│   │   └── project_service.go
│   └── temporal/
│       ├── workflows.go     # Workflow implementations
│       ├── activities.go    # Activity implementations
│       └── worker.go        # Temporal worker
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
└── README.md
```

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Build for Linux
make build-linux
```

### Testing

```bash
# Run unit tests
make test

# Run all tests (including integration)
make test-all

# Run benchmarks
make benchmark

# Generate coverage report
make coverage
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run all checks
make check
```

## Monitoring

### Metrics

The service exposes Prometheus metrics on port 9090:

- `orchestrator_workflows_total` - Total number of workflows
- `orchestrator_workflows_duration_seconds` - Workflow execution duration
- `orchestrator_workflows_active` - Currently active workflows
- `orchestrator_api_requests_total` - API request count
- `orchestrator_api_request_duration_seconds` - API request duration

### Tracing

Distributed tracing is available via Jaeger UI at http://localhost:16686

### Dashboards

Grafana dashboards are available at http://localhost:3000 (admin/admin)

## Deployment

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orchestrator
spec:
  replicas: 3
  selector:
    matchLabels:
      app: orchestrator
  template:
    metadata:
      labels:
        app: orchestrator
    spec:
      containers:
      - name: orchestrator
        image: qlp-orchestrator:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: ORCHESTRATOR_DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: orchestrator-secrets
              key: database-url
```

### Environment Variables

```bash
# Server
ORCHESTRATOR_SERVER_PORT=8080
ORCHESTRATOR_SERVER_HOST=0.0.0.0

# Database
ORCHESTRATOR_DATABASE_URL=postgres://user:pass@host:5432/db

# Redis
ORCHESTRATOR_REDIS_ADDR=redis:6379

# Temporal
ORCHESTRATOR_TEMPORAL_HOST_PORT=temporal:7233
ORCHESTRATOR_TEMPORAL_NAMESPACE=default

# Services
ORCHESTRATOR_INTENT_API_ADDRESS=intent-processor:50051
ORCHESTRATOR_AGENT_MANAGER_BASE_URL=http://agent-manager:8081

# Telemetry
ORCHESTRATOR_TELEMETRY_ENABLED=true
ORCHESTRATOR_TELEMETRY_JAEGER_COLLECTOR_ENDPOINT=http://jaeger:14268/api/traces
```

## Troubleshooting

### Common Issues

1. **Temporal connection failed**
   - Ensure Temporal is running: `docker-compose ps temporal`
   - Check Temporal UI: http://localhost:8088

2. **Database migration failed**
   - Check database connection: `psql $ORCHESTRATOR_DATABASE_URL`
   - Run migrations manually: `make migrate-up`

3. **Redis connection failed**
   - Check Redis is running: `redis-cli ping`
   - Verify Redis configuration

### Debug Mode

Enable debug logging:

```bash
ORCHESTRATOR_TELEMETRY_LOG_LEVEL=debug make run
```

### Health Checks

```bash
# Service health
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready

# Metrics
curl http://localhost:9090/metrics
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is part of the QuantumLayer Platform and is proprietary software.