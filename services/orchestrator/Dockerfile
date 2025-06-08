# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always --dirty) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o orchestrator cmd/server/main.go

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S orchestrator && \
    adduser -u 1000 -S orchestrator -G orchestrator

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/orchestrator .

# Copy any additional files (configs, migrations, etc.)
# COPY --from=builder /app/migrations ./migrations

# Change ownership
RUN chown -R orchestrator:orchestrator /app

# Switch to non-root user
USER orchestrator

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables
ENV GIN_MODE=release \
    ORCHESTRATOR_SERVER_PORT=8080 \
    ORCHESTRATOR_SERVER_HOST=0.0.0.0

# Run the application
ENTRYPOINT ["./orchestrator"]