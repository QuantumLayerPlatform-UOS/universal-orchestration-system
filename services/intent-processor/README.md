# Intent Processor Service

The Intent Processor is a Python service that processes natural language requirements and breaks them down into actionable tasks for the QuantumLayer Platform UOS.

## Features

- **Natural Language Processing**: Analyzes user requirements written in natural language
- **Intent Classification**: Categorizes requirements into types (feature request, bug fix, refactoring, etc.)
- **Task Generation**: Automatically breaks down requirements into specific, actionable tasks
- **Dependency Management**: Identifies and manages task dependencies
- **Validation**: Validates task breakdowns for completeness and consistency
- **Azure OpenAI Integration**: Leverages Azure OpenAI for advanced NLP capabilities

## Architecture

The service is built with:
- **FastAPI**: Modern, fast web framework for building APIs
- **LangChain**: Framework for developing applications powered by language models
- **Azure OpenAI**: For natural language processing and understanding
- **Pydantic**: Data validation using Python type annotations

## Prerequisites

- Python 3.11+
- Azure OpenAI account with API access
- Docker (for containerized deployment)

## Configuration

Set the following environment variables:

```bash
# Azure OpenAI Configuration
AZURE_OPENAI_ENDPOINT=https://your-instance.openai.azure.com
AZURE_OPENAI_API_KEY=your-api-key
AZURE_OPENAI_DEPLOYMENT_NAME=your-deployment-name
AZURE_OPENAI_API_VERSION=2024-02-15-preview

# Service Configuration
CORS_ORIGINS=http://localhost:3000,http://localhost:8080
ENV=development  # or production

# Logging
LOG_LEVEL=INFO
```

## Installation

### Local Development

1. Create a virtual environment:
```bash
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
```

2. Install dependencies:
```bash
pip install -r requirements.txt
```

3. Run the service:
```bash
uvicorn src.main:app --reload --port 8001
```

### Docker Deployment

1. Build the Docker image:
```bash
docker build -t intent-processor:latest .
```

2. Run the container:
```bash
docker run -d \
  -p 8001:8001 \
  -e AZURE_OPENAI_ENDPOINT=$AZURE_OPENAI_ENDPOINT \
  -e AZURE_OPENAI_API_KEY=$AZURE_OPENAI_API_KEY \
  -e AZURE_OPENAI_DEPLOYMENT_NAME=$AZURE_OPENAI_DEPLOYMENT_NAME \
  --name intent-processor \
  intent-processor:latest
```

## API Endpoints

### Health Check
```
GET /health
```
Returns the service health status and dependency checks.

### Process Intent
```
POST /api/v1/process-intent
```
Processes natural language requirements and returns task breakdown.

**Request Body:**
```json
{
  "text": "Create a user authentication system with JWT tokens",
  "context": {
    "project": "web-app",
    "tech_stack": ["Python", "FastAPI", "PostgreSQL"]
  },
  "project_info": {
    "name": "E-commerce Platform",
    "phase": "MVP"
  },
  "request_id": "req_123456"
}
```

**Response:**
```json
{
  "request_id": "req_123456",
  "intent_type": "feature_request",
  "confidence": 0.95,
  "summary": "Implement JWT-based authentication system for user login and session management",
  "tasks": [
    {
      "id": "task_a1b2c3d4",
      "title": "Create User Model",
      "description": "Design and implement user database model with authentication fields",
      "type": "backend",
      "priority": "high",
      "complexity": "moderate",
      "estimated_hours": 4,
      "dependencies": [],
      "tags": ["database", "auth"],
      "acceptance_criteria": [
        "User model includes email, password hash, and status fields",
        "Model supports user profile information"
      ],
      "technical_requirements": {
        "technologies": ["SQLAlchemy", "PostgreSQL"],
        "apis": [],
        "data_models": ["User"]
      }
    }
  ],
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Validate Tasks
```
POST /api/v1/validate-tasks
```
Validates a task breakdown for completeness and consistency.

### Get Prompt Templates
```
GET /api/v1/prompt-templates
```
Returns available prompt templates used by the service.

### Metrics
```
GET /metrics
```
Prometheus-compatible metrics endpoint.

## Task Types

The service categorizes tasks into the following types:
- **frontend**: UI/UX related tasks
- **backend**: Server-side logic and APIs
- **database**: Database design and migrations
- **api**: API design and integration
- **infrastructure**: DevOps and infrastructure tasks
- **testing**: Test creation and quality assurance
- **documentation**: Documentation updates
- **design**: System design and architecture
- **devops**: CI/CD and deployment tasks
- **security**: Security-related tasks

## Intent Types

Supported intent classifications:
- **feature_request**: New functionality or feature
- **bug_fix**: Fixing an existing issue
- **refactoring**: Code improvement without changing functionality
- **documentation**: Documentation updates
- **testing**: Test creation or improvement
- **deployment**: Deployment or infrastructure changes
- **configuration**: Configuration changes
- **research**: Research or investigation tasks

## Development

### Running Tests
```bash
pytest tests/ -v --cov=src
```

### Code Formatting
```bash
black src/ tests/
flake8 src/ tests/
mypy src/
```

### Pre-commit Hooks
```bash
pre-commit install
pre-commit run --all-files
```

## Monitoring

The service exposes Prometheus metrics at `/metrics` including:
- Request counts and durations
- Intent processing success/failure rates
- Task generation metrics

## Error Handling

The service implements comprehensive error handling:
- Input validation with detailed error messages
- Graceful handling of Azure OpenAI API failures
- Structured error responses with request tracking
- Automatic retry logic for transient failures

## Security

- Non-root user in Docker container
- Environment-based configuration (no hardcoded secrets)
- CORS configuration for API access control
- Input validation and sanitization
- Rate limiting ready (implement with reverse proxy)

## Performance

- Asynchronous request handling
- Connection pooling for Azure OpenAI
- Efficient prompt management
- Response caching capabilities
- Horizontal scaling support

## Troubleshooting

### Common Issues

1. **Azure OpenAI Connection Failed**
   - Verify environment variables are set correctly
   - Check API key validity
   - Ensure deployment name matches Azure configuration

2. **Task Generation Timeout**
   - Check Azure OpenAI quota limits
   - Verify network connectivity
   - Review input complexity

3. **Invalid JSON Response**
   - Check Azure OpenAI model deployment
   - Review prompt templates
   - Enable debug logging

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

Part of the QuantumLayer Platform UOS project.