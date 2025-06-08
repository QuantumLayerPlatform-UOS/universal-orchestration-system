# QuantumLayer Platform - Local Development Guide

## Overview

This guide provides comprehensive instructions for setting up and running the QuantumLayer Platform Unified Orchestration System (UOS) in a local development environment.

## Prerequisites

Before starting, ensure you have the following installed:

### Required Software

- **Docker Desktop**: Version 24.0 or higher
  - [Download for Mac](https://www.docker.com/products/docker-desktop/)
  - [Download for Windows](https://www.docker.com/products/docker-desktop/)
  - [Download for Linux](https://docs.docker.com/engine/install/)

- **Docker Compose**: Version 2.20 or higher (usually included with Docker Desktop)

- **Go**: Version 1.21 or higher
  ```bash
  # macOS
  brew install go
  
  # Linux
  wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
  sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
  ```

- **Make**: Build automation tool
  ```bash
  # macOS (usually pre-installed)
  xcode-select --install
  
  # Linux
  sudo apt-get install build-essential  # Debian/Ubuntu
  sudo yum install make                  # RHEL/CentOS
  ```

- **Git**: Version control
  ```bash
  # macOS
  brew install git
  
  # Linux
  sudo apt-get install git  # Debian/Ubuntu
  sudo yum install git      # RHEL/CentOS
  ```

### Optional but Recommended

- **jq**: JSON processor for testing scripts
  ```bash
  # macOS
  brew install jq
  
  # Linux
  sudo apt-get install jq  # Debian/Ubuntu
  sudo yum install jq      # RHEL/CentOS
  ```

- **curl**: HTTP client for API testing
- **nc (netcat)**: Network utility for port checking

### System Requirements

- **RAM**: Minimum 8GB, recommended 16GB
- **Storage**: At least 10GB free space
- **CPU**: 4+ cores recommended

## Setup Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/quantumlayer/qlp-uos.git
cd qlp-uos
```

### 2. Configure Environment

Copy the development environment template:

```bash
cp .env.development .env
```

Review and modify `.env` if needed. The default values are configured for local development.

### 3. Initialize Infrastructure

Run the setup script to prepare the infrastructure:

```bash
make setup
```

This command will:
- Check prerequisites
- Create necessary directories
- Initialize infrastructure configurations
- Start all services
- Verify health status

### 4. Start Development Stack

If you need to start the stack manually:

```bash
make dev-up
```

This will start:
- Core services (Orchestrator, Intent Processor, Agent Manager)
- Databases (PostgreSQL, MongoDB)
- Cache (Redis)
- Workflow engine (Temporal)
- Observability stack (Jaeger, Prometheus, Grafana)

### 5. Verify Installation

Check that all services are running:

```bash
make health-check
```

You should see all services marked as healthy.

## Service URLs

Once the stack is running, you can access:

| Service | URL | Credentials |
|---------|-----|-------------|
| Orchestrator API | http://localhost:8080 | - |
| Intent Processor API | http://localhost:8082 | - |
| Agent Manager API | http://localhost:8084 | - |
| Temporal UI | http://localhost:8088 | - |
| Jaeger UI | http://localhost:16686 | - |
| Prometheus | http://localhost:9090 | - |
| Grafana | http://localhost:3000 | admin/admin |

## Common Development Tasks

### Starting and Stopping Services

```bash
# Start all services
make dev-up

# Stop all services
make dev-down

# Restart all services
make dev-restart

# Restart a specific service
make restart-service SERVICE=orchestrator
```

### Viewing Logs

```bash
# View logs for all services
make logs

# View logs for a specific service
make logs-service SERVICE=intent-processor
```

### Running Tests

```bash
# Run all tests (unit + integration)
make test

# Run only unit tests
make test-unit

# Run only integration tests
make test-integration

# Run end-to-end tests
make test-e2e
```

### Database Operations

```bash
# Run database migrations
make db-migrate

# Seed database with test data
make db-seed

# Reset database (drop and recreate)
make db-reset
```

### Code Quality

```bash
# Run linters
make lint

# Format code
make fmt

# Update dependencies
make deps
```

### Monitoring and Debugging

```bash
# Open Prometheus metrics
make metrics

# Open Jaeger traces
make traces

# Open Grafana dashboards
make dashboards
```

## Architecture Overview

The local development environment mirrors the production architecture:

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ Intent Processor│────▶│   Orchestrator   │◀────│  Agent Manager  │
└────────┬────────┘     └────────┬─────────┘     └────────┬────────┘
         │                       │                          │
         └───────────┬───────────┴──────────────────────────┘
                     │
              ┌──────▼──────┐
              │   Temporal  │
              └─────────────┘
                     │
       ┌─────────────┼─────────────┐
       │             │             │
  ┌────▼────┐  ┌─────▼────┐  ┌────▼────┐
  │ PostgreSQL│ │  MongoDB │  │  Redis  │
  └──────────┘  └──────────┘  └─────────┘
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Services Not Starting

**Problem**: Services fail to start or become unhealthy.

**Solutions**:
- Check Docker daemon is running
- Ensure ports are not already in use:
  ```bash
  lsof -i :8080  # Check if port is in use
  ```
- Review service logs:
  ```bash
  make logs-service SERVICE=orchestrator
  ```
- Increase Docker resources in Docker Desktop settings

#### 2. Database Connection Errors

**Problem**: Services can't connect to databases.

**Solutions**:
- Ensure databases are healthy:
  ```bash
  docker ps | grep -E "(postgres|mongodb)"
  ```
- Check database logs:
  ```bash
  docker logs qlp-postgres
  docker logs qlp-mongodb
  ```
- Verify connection strings in `.env`

#### 3. Integration Tests Failing

**Problem**: Integration tests fail during health checks.

**Solutions**:
- Wait for all services to be fully initialized:
  ```bash
  sleep 30 && make test-integration
  ```
- Check individual service health endpoints manually
- Review integration test logs in `integration-test-report.txt`

#### 4. Port Conflicts

**Problem**: Ports already in use by other applications.

**Solutions**:
- Stop conflicting services
- Modify port mappings in `docker-compose.dev.yml`
- Update corresponding environment variables

#### 5. Performance Issues

**Problem**: Services running slowly or timing out.

**Solutions**:
- Allocate more resources to Docker Desktop
- Reduce service replica counts
- Enable development mode optimizations
- Check for resource-intensive operations in logs

### Cleaning Up

If you encounter persistent issues:

```bash
# Stop and remove all containers and volumes
make dev-clean

# Clean Docker system resources
make clean-docker

# Start fresh
make dev-up
```

## Development Workflow

### 1. Making Code Changes

1. Make changes to service code
2. Services will auto-reload if hot reload is enabled
3. For structural changes, rebuild the service:
   ```bash
   make build-service SERVICE=orchestrator
   make restart-service SERVICE=orchestrator
   ```

### 2. Testing Changes

1. Write/update unit tests
2. Run tests locally:
   ```bash
   make test-unit
   ```
3. Run integration tests:
   ```bash
   make test-integration
   ```

### 3. Debugging

1. Enable debug mode in `.env`:
   ```
   DEBUG_MODE=true
   VERBOSE_LOGGING=true
   ```

2. Use service-specific debug endpoints
3. View distributed traces in Jaeger
4. Monitor metrics in Grafana

### 4. API Development

1. Update API definitions
2. Generate documentation:
   ```bash
   make api-docs
   ```
3. Test endpoints using curl or Postman
4. View API docs at service URLs

## Best Practices

1. **Always run tests** before committing changes
2. **Use the Makefile** commands for consistency
3. **Monitor logs** when debugging issues
4. **Keep dependencies updated** with `make deps`
5. **Document significant changes** in code and APIs
6. **Use feature flags** for experimental features
7. **Clean up resources** when done developing

## Getting Help

If you encounter issues not covered in this guide:

1. Check service logs for detailed error messages
2. Review the [technical architecture](technical-architecture.md) documentation
3. Search existing GitHub issues
4. Create a new issue with:
   - Environment details
   - Steps to reproduce
   - Error messages and logs
   - Expected vs actual behavior

## Next Steps

- Read the [Technical Architecture](technical-architecture.md) guide
- Review the [API Documentation](../services/README.md)
- Explore the [Monitoring Guide](monitoring-guide.md)
- Check the [Deployment Guide](deployment-guide.md)