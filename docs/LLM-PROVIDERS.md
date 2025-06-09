# Multi-LLM Provider Support

QuantumLayer Platform supports multiple LLM providers for maximum flexibility in development and production environments.

## Supported Providers

### 1. **Ollama** (Default for Development)
- **Best for**: Local development, privacy-conscious deployments
- **Models**: llama2, mistral, codellama, neural-chat, mixtral
- **Configuration**:
  ```bash
  LLM_PROVIDER=ollama
  LLM_MODEL=mistral
  OLLAMA_BASE_URL=https://model.gonella.co.uk  # Or http://localhost:11434
  ```

### 2. **Groq** (Fast Inference)
- **Best for**: Rapid prototyping, low-latency requirements
- **Models**: mixtral-8x7b-32768, llama2-70b-4096, gemma-7b-it
- **Configuration**:
  ```bash
  LLM_PROVIDER=groq
  LLM_MODEL=mixtral-8x7b-32768
  GROQ_API_KEY=your-groq-api-key
  ```

### 3. **OpenAI**
- **Best for**: General-purpose AI, well-tested models
- **Models**: gpt-4-turbo, gpt-4, gpt-3.5-turbo
- **Configuration**:
  ```bash
  LLM_PROVIDER=openai
  LLM_MODEL=gpt-3.5-turbo
  OPENAI_API_KEY=your-openai-api-key
  ```

### 4. **Anthropic**
- **Best for**: Complex reasoning, safety-focused applications
- **Models**: claude-3-opus, claude-3-sonnet, claude-3-haiku
- **Configuration**:
  ```bash
  LLM_PROVIDER=anthropic
  LLM_MODEL=claude-3-sonnet-20240229
  ANTHROPIC_API_KEY=your-anthropic-api-key
  ```

### 5. **Azure OpenAI**
- **Best for**: Enterprise deployments, compliance requirements
- **Models**: gpt-35-turbo, gpt-4
- **Configuration**:
  ```bash
  LLM_PROVIDER=azure
  AZURE_OPENAI_API_KEY=your-api-key
  AZURE_OPENAI_ENDPOINT=https://your-instance.openai.azure.com
  AZURE_OPENAI_DEPLOYMENT_NAME=gpt-35-turbo
  ```

## Auto-Detection

If no `LLM_PROVIDER` is specified, the system automatically detects the best available provider based on environment variables:

1. Ollama (if `OLLAMA_BASE_URL` or `USE_OLLAMA=true`)
2. Groq (if `GROQ_API_KEY` exists)
3. OpenAI (if `OPENAI_API_KEY` exists)
4. Anthropic (if `ANTHROPIC_API_KEY` exists)
5. Azure OpenAI (if Azure credentials exist)

## Usage Examples

### Development with Ollama
```bash
# Using remote Ollama instance
export LLM_PROVIDER=ollama
export OLLAMA_BASE_URL=https://model.gonella.co.uk
export LLM_MODEL=mistral

# Start services
docker-compose -f docker-compose.minimal.yml up
```

### Production with Multiple Providers
```bash
# Set multiple API keys for fallback
export GROQ_API_KEY=gsk_...
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...

# Let system auto-detect (will use Groq first)
docker-compose -f docker-compose.minimal.yml up
```

### Testing Different Models
```bash
# Test with Groq's fast mixtral
export LLM_PROVIDER=groq
export LLM_MODEL=mixtral-8x7b-32768

# Test with Anthropic's Claude
export LLM_PROVIDER=anthropic
export LLM_MODEL=claude-3-opus-20240229

# Test with local Ollama
export LLM_PROVIDER=ollama
export LLM_MODEL=codellama
```

## Provider-Specific Features

### Ollama
- Supports custom models via `ollama pull`
- Can run completely offline
- Ideal for development and testing

### Groq
- Extremely fast inference (100x faster than traditional)
- Limited context window
- Best for quick iterations

### OpenAI
- Most mature ecosystem
- Best documentation and examples
- Reliable performance

### Anthropic
- Larger context windows
- Better at following complex instructions
- Strong safety features

### Azure OpenAI
- Enterprise SLAs
- Data residency compliance
- Integration with Azure services

## Fallback Strategy

The system includes automatic fallback:

```python
# Services will try providers in order until one succeeds
providers = ['ollama', 'groq', 'openai']
result = await llm_factory.execute_with_fallback(messages, providers)
```

## Performance Considerations

| Provider | Latency | Cost | Privacy | Reliability |
|----------|---------|------|---------|-------------|
| Ollama   | Medium  | Free | High    | High (local) |
| Groq     | Low     | Low  | Medium  | High |
| OpenAI   | Medium  | Med  | Low     | Very High |
| Anthropic| Medium  | High | Medium  | High |
| Azure    | Medium  | Med  | High    | Very High |

## Troubleshooting

### Ollama Connection Issues
```bash
# Check if Ollama is accessible
curl https://model.gonella.co.uk/api/tags

# Test with local Ollama
export OLLAMA_BASE_URL=http://localhost:11434
```

### API Key Issues
```bash
# Verify API keys are set
env | grep -E "(GROQ|OPENAI|ANTHROPIC)_API_KEY"

# Test specific provider
export LLM_PROVIDER=groq
export LOG_LEVEL=debug
```

### Model Not Found
```bash
# List available models for Ollama
curl https://model.gonella.co.uk/api/tags

# Use a known good model
export LLM_MODEL=mistral
```

## Best Practices

1. **Development**: Use Ollama for cost-free iteration
2. **Testing**: Use Groq for fast feedback loops
3. **Production**: Use Azure OpenAI for enterprise features
4. **Fallback**: Configure multiple providers for resilience

## Adding New Providers

To add a new LLM provider:

1. Update `LLMProviderFactory` in both Python and JavaScript
2. Add provider configuration to `.env.example`
3. Update docker-compose environment variables
4. Add provider-specific prompt adjustments if needed

## Cost Management

- **Free**: Ollama (self-hosted)
- **Low Cost**: Groq, OpenAI GPT-3.5
- **Medium Cost**: OpenAI GPT-4, Azure OpenAI
- **Higher Cost**: Anthropic Claude Opus

Always set `LLM_MAX_TOKENS` to control costs:
```bash
export LLM_MAX_TOKENS=1000  # Limit response length
```