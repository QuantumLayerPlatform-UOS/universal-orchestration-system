# Agent Manager Service

The Agent Manager service is a critical component of the QuantumLayer Platform UOS that manages the lifecycle of AI agents and coordinates their activities. It provides a scalable, fault-tolerant system for orchestrating multiple AI agents to work together on complex tasks.

## Features

- **Agent Lifecycle Management**: Register, monitor, and manage AI agents
- **Task Orchestration**: Distribute tasks to appropriate agents based on capabilities
- **Real-time Communication**: WebSocket-based communication with agents
- **Queue Management**: Priority-based task queuing with retry logic
- **Health Monitoring**: Comprehensive health checks and metrics
- **Scalability**: Horizontal scaling support with Redis-backed queues
- **Fault Tolerance**: Automatic agent failover and task retry mechanisms

## Architecture

### Core Components

1. **Agent Registry**: Maintains a registry of all available agents with their capabilities and status
2. **Agent Orchestrator**: Coordinates task assignment and multi-agent workflows
3. **Agent Communicator**: Handles real-time bidirectional communication with agents
4. **Task Queue**: Manages task prioritization and distribution
5. **Metrics Service**: Collects and exposes performance metrics

### Agent Types

- `code-gen`: Code generation agents
- `test-gen`: Test generation agents
- `deploy`: Deployment automation agents
- `monitor`: Monitoring and observability agents
- `security`: Security scanning agents
- `documentation`: Documentation generation agents
- `review`: Code review agents
- `optimization`: Performance optimization agents

## Prerequisites

- Node.js 18+ and npm 9+
- MongoDB 6.0+
- Redis 7.0+
- Azure account with configured credentials

## Installation

1. Clone the repository and navigate to the service directory:
```bash
cd services/agent-manager
```

2. Install dependencies:
```bash
npm install
```

3. Copy the environment template and configure:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Build the TypeScript code:
```bash
npm run build
```

## Configuration

Key environment variables:

```bash
# Server
PORT=3002
NODE_ENV=production

# Database
MONGODB_URI=mongodb://localhost:27017/agent-manager
REDIS_URL=redis://localhost:6379

# Azure
AZURE_TENANT_ID=your-tenant-id
AZURE_CLIENT_ID=your-client-id
AZURE_CLIENT_SECRET=your-client-secret

# CORS
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

## Running the Service

### Development Mode

```bash
npm run dev
```

### Production Mode

```bash
npm run build
npm start
```

### Docker

```bash
# Build the image
docker build -t qlp-agent-manager .

# Run the container
docker run -p 3002:3002 --env-file .env qlp-agent-manager
```

### Docker Compose

```yaml
agent-manager:
  build: ./services/agent-manager
  ports:
    - "3002:3002"
  environment:
    - NODE_ENV=production
    - MONGODB_URI=mongodb://mongo:27017/agent-manager
    - REDIS_URL=redis://redis:6379
  depends_on:
    - mongodb
    - redis
```

## API Documentation

### REST Endpoints

#### Agent Management

**Register Agent**
```http
POST /api/v1/agents
Content-Type: application/json

{
  "name": "CodeGen Agent 1",
  "type": "code-gen",
  "capabilities": [
    { "name": "javascript", "version": "1.0" },
    { "name": "typescript", "version": "1.0" }
  ],
  "metadata": {
    "version": "1.0.0",
    "platform": "linux",
    "region": "us-east-1",
    "tags": ["production", "high-performance"]
  }
}
```

**Get All Agents**
```http
GET /api/v1/agents?type=code-gen&status=available&region=us-east-1
```

**Get Agent by ID**
```http
GET /api/v1/agents/{agentId}
```

**Update Agent Status**
```http
PATCH /api/v1/agents/{agentId}/status
Content-Type: application/json

{
  "status": "busy"
}
```

**Get Agent Health**
```http
GET /api/v1/agents/{agentId}/health
```

#### Task Management

**Submit Task**
```http
POST /api/v1/tasks
Content-Type: application/json

{
  "type": "code-gen",
  "priority": 1,
  "payload": {
    "language": "javascript",
    "requirements": "Create a REST API endpoint"
  },
  "requiredCapabilities": ["javascript", "express"],
  "metadata": {
    "source": "api",
    "userId": "user123",
    "projectId": "project456"
  },
  "timeout": 300000,
  "maxAttempts": 3
}
```

**Get Task Status**
```http
GET /api/v1/tasks/{taskId}
```

**Cancel Task**
```http
DELETE /api/v1/tasks/{taskId}
```

**Get Queue Statistics**
```http
GET /api/v1/tasks/queue/stats
```

#### Health & Metrics

**Health Check**
```http
GET /health
GET /health/detailed
GET /health/live
GET /health/ready
```

**Metrics**
```http
GET /api/v1/metrics
```

### WebSocket Communication

The service provides WebSocket endpoints for real-time communication:

#### Agent Namespace (`/agents`)

Agents connect to this namespace to receive tasks and send results:

```javascript
const socket = io('http://localhost:3002/agents', {
  auth: {
    token: 'agent-auth-token',
    agentId: 'agent-123'
  }
});

// Listen for tasks
socket.on('task:execute', (task) => {
  console.log('Received task:', task);
  // Process task...
  
  // Send result
  socket.emit('task:result', {
    taskId: task.id,
    agentId: 'agent-123',
    status: 'success',
    result: { /* task results */ }
  });
});

// Send heartbeat
setInterval(() => {
  socket.emit('heartbeat');
}, 15000);
```

#### Monitor Namespace (`/monitor`)

Clients can connect to monitor agent and task status in real-time:

```javascript
const socket = io('http://localhost:3002/monitor');

socket.on('agents:update', (agents) => {
  console.log('Agents updated:', agents);
});

socket.on('task:completed', (task) => {
  console.log('Task completed:', task);
});
```

## Task Orchestration Strategies

### Single Agent Strategy

Default strategy that assigns a task to the best available agent:

```json
{
  "type": "code-gen",
  "payload": {
    "requirements": "Generate unit tests"
  }
}
```

### Pipeline Strategy

Executes tasks in sequence across multiple agents:

```json
{
  "type": "code-gen",
  "metadata": {
    "tags": ["pipeline"]
  },
  "payload": {
    "pipeline": [
      { "type": "code-gen", "capabilities": ["javascript"] },
      { "type": "test-gen", "capabilities": ["jest"] },
      { "type": "review", "capabilities": ["eslint"] }
    ],
    "initialData": { /* initial input */ }
  }
}
```

### Parallel Strategy

Executes multiple tasks in parallel:

```json
{
  "type": "code-gen",
  "metadata": {
    "tags": ["parallel"]
  },
  "payload": {
    "subtasks": [
      {
        "type": "code-gen",
        "payload": { "component": "frontend" }
      },
      {
        "type": "code-gen",
        "payload": { "component": "backend" }
      }
    ]
  }
}
```

## Testing

Run the test suite:

```bash
# Unit tests
npm test

# With coverage
npm run test:coverage

# Watch mode
npm run test:watch
```

## Monitoring and Observability

### Logs

Logs are written to console in development and to files in production:
- `logs/error.log`: Error logs
- `logs/combined.log`: All logs

### Metrics

The service exposes metrics including:
- Agent availability and performance
- Task completion rates and processing times
- Queue statistics
- System resource usage

### Health Checks

- `/health`: Basic health check
- `/health/detailed`: Comprehensive health status
- `/health/live`: Kubernetes liveness probe
- `/health/ready`: Kubernetes readiness probe

## Scaling

### Horizontal Scaling

The service supports horizontal scaling through:
- Redis-backed task queues
- MongoDB for shared state
- Stateless request handling

### Deployment Recommendations

1. **Development**: Single instance with local MongoDB and Redis
2. **Staging**: 2-3 instances with managed MongoDB and Redis
3. **Production**: 
   - 3+ instances behind a load balancer
   - MongoDB replica set
   - Redis cluster or Azure Cache for Redis
   - Configure pod autoscaling based on queue depth

## Security

- JWT-based authentication for API endpoints
- WebSocket authentication for agent connections
- Environment-based configuration for secrets
- Request validation and sanitization
- Rate limiting on API endpoints

## Troubleshooting

### Common Issues

1. **Agent not connecting**
   - Check agent authentication token
   - Verify WebSocket connectivity
   - Check CORS configuration

2. **Tasks stuck in queue**
   - Verify Redis connectivity
   - Check agent availability
   - Review task requirements vs agent capabilities

3. **High memory usage**
   - Monitor task queue size
   - Check for memory leaks in long-running agents
   - Review metrics retention settings

### Debug Mode

Enable debug logging:
```bash
LOG_LEVEL=debug npm run dev
```

## Contributing

1. Follow TypeScript best practices
2. Write tests for new features
3. Update documentation
4. Run linter before committing:
   ```bash
   npm run lint:fix
   ```

## License

Copyright (c) 2024 QuantumLayer Platform. All rights reserved.