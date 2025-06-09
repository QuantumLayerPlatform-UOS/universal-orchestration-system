# Code Generation Agent

A simple but functional code generation agent for the QLP system that can generate boilerplate code based on requirements.

## Features

- Connects to Agent Manager via Socket.IO
- Registers as a code generation agent
- Handles task assignments for code generation
- Generates various types of code:
  - REST API endpoints (Express)
  - React components (Functional and Class-based)
  - Express servers
  - Database schemas (Mongoose and Sequelize)
  - Custom code from prompts

## Prerequisites

- Node.js 14.x or higher
- Agent Manager service running
- Network connectivity to Agent Manager

## Installation

1. Install dependencies:
```bash
npm install
```

2. Create a `.env` file (optional):
```env
AGENT_ID=code-gen-agent-001
AGENT_MANAGER_URL=http://localhost:8001
```

## Running the Agent

### Development Mode
```bash
npm run dev
```

### Production Mode
```bash
npm start
```

### Using Docker
```bash
# Build the image
docker build -t code-gen-agent .

# Run the container
docker run -d \
  --name code-gen-agent \
  -e AGENT_MANAGER_URL=http://agent-manager:8001 \
  --network qlp-network \
  code-gen-agent
```

## Usage

The agent will automatically:
1. Connect to the Agent Manager
2. Register itself with its capabilities
3. Wait for task assignments
4. Process code generation tasks
5. Return generated code to the Agent Manager

### Task Payload Examples

#### Generate REST API
```json
{
  "type": "rest-api",
  "payload": {
    "type": "rest-api",
    "requirements": {
      "framework": "express",
      "endpoint": "/api/users",
      "methods": ["GET", "POST", "PUT", "DELETE"],
      "modelName": "User",
      "fields": [
        { "name": "name", "type": "string", "required": true },
        { "name": "email", "type": "string", "required": true, "unique": true },
        { "name": "age", "type": "number", "required": false }
      ]
    }
  }
}
```

#### Generate React Component
```json
{
  "type": "react-component",
  "payload": {
    "type": "react-component",
    "requirements": {
      "name": "UserProfile",
      "type": "functional",
      "props": ["user", "onUpdate"],
      "useState": true,
      "useEffect": true
    }
  }
}
```

#### Generate Express Server
```json
{
  "type": "express-server",
  "payload": {
    "type": "express-server",
    "requirements": {
      "port": 5000,
      "cors": true,
      "helmet": true,
      "morgan": true,
      "routes": [
        { "path": "/api/users", "file": "users" },
        { "path": "/api/auth", "file": "auth" }
      ]
    }
  }
}
```

#### Generate Database Schema
```json
{
  "type": "database-schema",
  "payload": {
    "type": "database-schema",
    "requirements": {
      "orm": "mongoose",
      "modelName": "Product",
      "fields": [
        { "name": "name", "type": "string", "required": true },
        { "name": "price", "type": "number", "required": true },
        { "name": "inStock", "type": "boolean", "default": true }
      ]
    }
  }
}
```

#### Custom Code Generation
```json
{
  "type": "custom",
  "payload": {
    "type": "custom",
    "requirements": {
      "prompt": "Create a function to validate email addresses",
      "language": "javascript"
    }
  }
}
```

## Environment Variables

- `AGENT_ID`: Unique identifier for this agent instance (default: 'code-gen-agent-001')
- `AGENT_MANAGER_URL`: URL of the Agent Manager service (default: 'http://localhost:8001')

## Logging

The agent uses Winston for logging. Logs include:
- Connection status
- Registration events
- Task assignments and completions
- Errors and warnings

## Architecture

```
code-gen-agent/
├── src/
│   ├── index.js          # Main agent logic and Socket.IO client
│   └── codeGenerator.js  # Code generation templates and logic
├── package.json          # Dependencies and scripts
├── Dockerfile           # Container definition
└── README.md           # This file
```

## Extending the Agent

To add new code generation capabilities:

1. Add new template methods in `codeGenerator.js`
2. Add case handler in `generateCode()` method in `index.js`
3. Update capabilities array in `AGENT_CONFIG`

## Error Handling

The agent handles errors gracefully:
- Connection failures trigger reconnection attempts
- Task processing errors are reported back to Agent Manager
- Agent maintains idle/busy status appropriately

## Contributing

1. Add new templates for additional frameworks/languages
2. Improve code generation logic
3. Add validation for generated code
4. Implement more sophisticated prompt parsing

## License

Part of the QLP-UOS project