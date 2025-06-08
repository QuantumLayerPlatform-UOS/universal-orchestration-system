#!/bin/bash

# Initialize infrastructure configuration for QuantumLayer Platform

set -e

echo "Initializing infrastructure configuration..."

# Create Prometheus configuration
mkdir -p infrastructure/prometheus

cat > infrastructure/prometheus/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'qlp-dev'
    environment: 'development'

scrape_configs:
  - job_name: 'orchestrator'
    static_configs:
      - targets: ['orchestrator:8081']
        labels:
          service: 'orchestrator'

  - job_name: 'intent-processor'
    static_configs:
      - targets: ['intent-processor:8083']
        labels:
          service: 'intent-processor'

  - job_name: 'agent-manager'
    static_configs:
      - targets: ['agent-manager:8085']
        labels:
          service: 'agent-manager'

  - job_name: 'temporal'
    static_configs:
      - targets: ['temporal:7233']
        labels:
          service: 'temporal'

  - job_name: 'redis'
    static_configs:
      - targets: ['redis:6379']
        labels:
          service: 'redis'

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres:5432']
        labels:
          service: 'postgres'

  - job_name: 'mongodb'
    static_configs:
      - targets: ['mongodb:27017']
        labels:
          service: 'mongodb'

alerting:
  alertmanagers:
    - static_configs:
        - targets: []

rule_files:
  - /etc/prometheus/rules/*.yml
EOF

# Create Grafana provisioning directories
mkdir -p infrastructure/grafana/provisioning/dashboards
mkdir -p infrastructure/grafana/provisioning/datasources

# Create Grafana datasource configuration
cat > infrastructure/grafana/provisioning/datasources/prometheus.yml << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: true

  - name: Jaeger
    type: jaeger
    access: proxy
    url: http://jaeger:16686
    editable: true
EOF

# Create Grafana dashboard provisioning configuration
cat > infrastructure/grafana/provisioning/dashboards/dashboard.yml << 'EOF'
apiVersion: 1

providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
EOF

# Create a sample Grafana dashboard
cat > infrastructure/grafana/provisioning/dashboards/qlp-overview.json << 'EOF'
{
  "dashboard": {
    "id": null,
    "uid": "qlp-overview",
    "title": "QuantumLayer Platform Overview",
    "tags": ["qlp", "overview"],
    "timezone": "browser",
    "schemaVersion": 16,
    "version": 0,
    "refresh": "10s",
    "panels": [
      {
        "id": 1,
        "gridPos": {"x": 0, "y": 0, "w": 12, "h": 8},
        "type": "graph",
        "title": "Service Health",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "up{job=~\"orchestrator|intent-processor|agent-manager\"}",
            "refId": "A",
            "legendFormat": "{{job}}"
          }
        ]
      },
      {
        "id": 2,
        "gridPos": {"x": 12, "y": 0, "w": 12, "h": 8},
        "type": "graph",
        "title": "Request Rate",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "refId": "A",
            "legendFormat": "{{service}} - {{method}} {{path}}"
          }
        ]
      },
      {
        "id": 3,
        "gridPos": {"x": 0, "y": 8, "w": 12, "h": 8},
        "type": "graph",
        "title": "Response Time",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "refId": "A",
            "legendFormat": "p95 - {{service}}"
          }
        ]
      },
      {
        "id": 4,
        "gridPos": {"x": 12, "y": 8, "w": 12, "h": 8},
        "type": "graph",
        "title": "Error Rate",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "rate(http_requests_total{status=~\"5..\"}[5m])",
            "refId": "A",
            "legendFormat": "{{service}} - {{status}}"
          }
        ]
      }
    ]
  },
  "overwrite": true
}
EOF

# Create directories for service Dockerfiles (if they don't exist)
for service in orchestrator intent-processor agent-manager; do
    if [ ! -f "services/$service/Dockerfile" ]; then
        echo "Creating Dockerfile for $service..."
        mkdir -p "services/$service"
        
        cat > "services/$service/Dockerfile" << 'EOF'
# Build stage
FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/server .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./server"]
EOF
    fi
done

# Create go.sum files if they don't exist
for service in orchestrator intent-processor agent-manager; do
    if [ ! -f "services/$service/go.sum" ]; then
        touch "services/$service/go.sum"
    fi
done

echo "Infrastructure initialization complete!"