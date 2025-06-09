# Meta-Prompt Agent

The Meta-Prompt Agent is a revolutionary component of the QuantumLayer Platform that enables dynamic agent creation and management through prompts rather than code.

## Overview

Unlike traditional agents that require coding and deployment, meta-prompt agents can be:
- **Created dynamically** using natural language descriptions
- **Self-improving** through performance feedback
- **Instantly deployed** without code changes
- **Easily customized** by non-developers

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                Meta-Prompt Orchestrator              │
│  ┌─────────────────┐      ┌────────────────────┐   │
│  │ MetaPromptEngine │      │   AgentSpawner     │   │
│  │                 │      │                    │   │
│  │ - Design Agents │      │ - Spawn Instances │   │
│  │ - Optimize      │      │ - Manage Lifecycle│   │
│  │ - Decompose     │      │ - Track Performance│  │
│  └─────────────────┘      └────────────────────┘   │
└─────────────────────────────────────────────────────┘
                            │
                            ▼
              ┌──────────────────────────┐
              │    Agent Manager         │
              │  (Dynamic Registration)   │
              └──────────────────────────┘
                            │
                ┌───────────┴───────────┐
                ▼                       ▼
        ┌──────────────┐       ┌──────────────┐
        │Dynamic Agent 1│       │Dynamic Agent 2│
        │ (Code Review) │       │ (Test Gen)    │
        └──────────────┘       └──────────────┘
```

## Features

### 1. Dynamic Agent Creation
Create new agents by describing what you need:

```javascript
{
  "type": "design-agent",
  "payload": {
    "taskDescription": "Create an agent that reviews database schemas for optimization opportunities",
    "requirements": {
      "expertise": ["SQL", "PostgreSQL", "Performance"],
      "outputFormat": "structured recommendations"
    }
  }
}
```

### 2. Prompt Optimization
Improve existing agents based on performance:

```javascript
{
  "type": "optimize-prompt",
  "payload": {
    "agentId": "agent-123",
    "performanceData": {
      "metrics": {
        "successRate": 0.85,
        "avgResponseTime": 2.3
      },
      "feedback": ["Sometimes misses index opportunities"]
    }
  }
}
```

### 3. Agent Spawning
Spawn agents on-demand for specific tasks:

```javascript
{
  "type": "spawn-agent",
  "payload": {
    "designId": "design-456",
    "taskContext": {
      "project": "e-commerce",
      "technology": "Node.js"
    },
    "ttl": 3600000  // 1 hour
  }
}
```

### 4. Task Decomposition
Break complex tasks into agent-executable workflows:

```javascript
{
  "type": "decompose-task",
  "payload": {
    "taskDescription": "Migrate a monolithic application to microservices",
    "constraints": {
      "timeline": "3 months",
      "team_size": 5
    }
  }
}
```

## Getting Started

### Prerequisites
- Node.js 16+
- Azure OpenAI API access
- Running Agent Manager service

### Installation

```bash
cd services/agents/meta-prompt-agent
npm install
```

### Configuration

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
# Edit .env with your configuration
```

### Running Locally

```bash
npm start
```

### Running with Docker

```bash
docker build -t meta-prompt-agent .
docker run -d \
  --name meta-prompt-agent \
  --env-file .env \
  --network qlp-network \
  meta-prompt-agent
```

## Usage Examples

### Creating a Custom Code Analyzer

```javascript
// Request to meta-prompt orchestrator
POST /api/v1/tasks
{
  "type": "design-agent",
  "payload": {
    "taskDescription": "Analyze Python code for async/await best practices",
    "requirements": {
      "language": "Python",
      "framework": "FastAPI",
      "focus": ["performance", "correctness", "error handling"]
    }
  }
}

// Response includes agent design
{
  "designId": "design-789",
  "agentDesign": {
    "name": "Python Async Analyzer",
    "type": "analyzer",
    "systemPrompt": "You are an expert in Python async programming...",
    "capabilities": ["async-analysis", "performance-tips", "error-detection"]
  }
}
```

### Spawning the Agent

```javascript
// Spawn the designed agent
POST /api/v1/tasks
{
  "type": "spawn-agent",
  "payload": {
    "designId": "design-789",
    "ttl": 7200000  // 2 hours
  }
}

// Response
{
  "agentId": "dynamic-abc123",
  "status": "spawned",
  "capabilities": ["async-analysis", "performance-tips", "error-detection"]
}
```

### Using the Spawned Agent

```javascript
// Send task to the dynamic agent
POST /api/v1/tasks
{
  "agentId": "dynamic-abc123",
  "type": "analyze-code",
  "payload": {
    "code": "async def fetch_data():...",
    "context": {
      "file": "api/endpoints.py"
    }
  }
}
```

## Prompt Templates

The system includes pre-built templates for common agent types:

- **Code Reviewer**: Comprehensive code quality analysis
- **Test Generator**: Automated test creation
- **Documentation Writer**: Technical documentation generation
- **Security Auditor**: Vulnerability assessment
- **Performance Optimizer**: Performance improvement suggestions
- **API Designer**: RESTful/GraphQL API design

## Performance Considerations

1. **Agent TTL**: Dynamic agents have a time-to-live to prevent resource waste
2. **Concurrent Limits**: Maximum concurrent agents is configurable
3. **Prompt Caching**: Frequently used prompts are cached
4. **Resource Monitoring**: Track token usage and response times

## Development

### Adding New Templates

1. Edit `templates/agent-prompts.json`
2. Add your template following the structure
3. Test with the design-agent endpoint

### Extending Capabilities

1. Add new meta-templates to `MetaPromptEngine`
2. Implement handlers in the orchestrator
3. Update agent spawner if needed

## Monitoring

The meta-prompt orchestrator exposes metrics:

- Active dynamic agents count
- Agent creation success/failure rates
- Prompt optimization improvements
- Token usage per agent type
- Task completion times

## Troubleshooting

### Agent Creation Fails
- Check Azure OpenAI credentials
- Verify prompt template syntax
- Check agent manager connectivity

### Spawned Agent Not Responding
- Check agent TTL hasn't expired
- Verify task routing in agent manager
- Check socket connection status

### Performance Issues
- Monitor token usage
- Adjust behavior modifiers (temperature, max tokens)
- Consider prompt optimization

## Future Enhancements

1. **Self-Learning**: Agents that improve without manual optimization
2. **Agent Marketplace**: Share and discover agent templates
3. **Visual Agent Designer**: GUI for creating agents
4. **Multi-Model Support**: Use different LLMs for different agents
5. **Agent Collaboration**: Agents that can spawn and coordinate other agents

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

Part of the QuantumLayer Platform - see main repository for license details.