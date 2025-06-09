const { ChatOpenAI } = require('@langchain/openai');
const { ChatAnthropic } = require('@langchain/anthropic');
const { ChatGroq } = require('@langchain/groq');
const { ChatOllama } = require('@langchain/community/chat_models/ollama');
const winston = require('winston');

/**
 * Factory for creating LLM instances based on provider configuration
 */
class LLMProviderFactory {
  constructor() {
    this.logger = winston.createLogger({
      level: 'info',
      format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
      ),
      transports: [new winston.transports.Console()]
    });

    // Provider configurations
    this.providers = {
      ollama: {
        name: 'Ollama (Local)',
        models: ['llama2', 'mistral', 'codellama', 'neural-chat'],
        default: 'mistral',
        config: {
          baseUrl: process.env.OLLAMA_BASE_URL || 'https://model.gonella.co.uk',
          temperature: 0.7,
          maxRetries: 3
        }
      },
      groq: {
        name: 'Groq',
        models: ['mixtral-8x7b-32768', 'llama2-70b-4096', 'gemma-7b-it'],
        default: 'mixtral-8x7b-32768',
        config: {
          apiKey: process.env.GROQ_API_KEY,
          temperature: 0.7,
          maxTokens: 2000
        }
      },
      openai: {
        name: 'OpenAI',
        models: ['gpt-4-turbo', 'gpt-4', 'gpt-3.5-turbo'],
        default: 'gpt-3.5-turbo',
        config: {
          apiKey: process.env.OPENAI_API_KEY,
          temperature: 0.7,
          maxTokens: 2000
        }
      },
      anthropic: {
        name: 'Anthropic',
        models: ['claude-3-opus-20240229', 'claude-3-sonnet-20240229', 'claude-3-haiku-20240307'],
        default: 'claude-3-sonnet-20240229',
        config: {
          apiKey: process.env.ANTHROPIC_API_KEY,
          temperature: 0.7,
          maxTokens: 2000
        }
      },
      azure: {
        name: 'Azure OpenAI',
        models: ['gpt-35-turbo', 'gpt-4'],
        default: 'gpt-35-turbo',
        config: {
          azureOpenAIApiKey: process.env.AZURE_OPENAI_API_KEY,
          azureOpenAIApiInstanceName: process.env.AZURE_OPENAI_INSTANCE_NAME,
          azureOpenAIApiDeploymentName: process.env.AZURE_OPENAI_DEPLOYMENT_NAME,
          azureOpenAIApiVersion: process.env.AZURE_OPENAI_API_VERSION || '2023-05-15',
          temperature: 0.7,
          maxTokens: 2000
        }
      }
    };

    // Determine default provider based on available credentials
    this.defaultProvider = this.detectDefaultProvider();
  }

  /**
   * Detect the best available provider based on environment variables
   */
  detectDefaultProvider() {
    // Priority order for development
    if (process.env.OLLAMA_BASE_URL || process.env.USE_OLLAMA === 'true') {
      this.logger.info('Using Ollama as default provider');
      return 'ollama';
    }
    if (process.env.GROQ_API_KEY) {
      this.logger.info('Using Groq as default provider');
      return 'groq';
    }
    if (process.env.OPENAI_API_KEY) {
      this.logger.info('Using OpenAI as default provider');
      return 'openai';
    }
    if (process.env.ANTHROPIC_API_KEY) {
      this.logger.info('Using Anthropic as default provider');
      return 'anthropic';
    }
    if (process.env.AZURE_OPENAI_API_KEY) {
      this.logger.info('Using Azure OpenAI as default provider');
      return 'azure';
    }
    
    this.logger.warn('No LLM provider credentials found, defaulting to Ollama');
    return 'ollama';
  }

  /**
   * Create an LLM instance
   * @param {Object} options - Configuration options
   * @param {string} options.provider - Provider name (ollama, groq, openai, anthropic, azure)
   * @param {string} options.model - Model name (optional, uses default if not specified)
   * @param {Object} options.config - Additional configuration to override defaults
   */
  createLLM(options = {}) {
    const provider = options.provider || this.defaultProvider;
    const providerConfig = this.providers[provider];
    
    if (!providerConfig) {
      throw new Error(`Unknown LLM provider: ${provider}`);
    }

    const model = options.model || providerConfig.default;
    const config = { ...providerConfig.config, ...options.config };

    this.logger.info(`Creating LLM instance`, { provider, model });

    switch (provider) {
      case 'ollama':
        return new ChatOllama({
          baseUrl: config.baseUrl,
          model,
          temperature: config.temperature,
          maxRetries: config.maxRetries,
          format: config.format,
          numPredict: config.maxTokens
        });

      case 'groq':
        return new ChatGroq({
          apiKey: config.apiKey,
          model,
          temperature: config.temperature,
          maxTokens: config.maxTokens,
          maxRetries: config.maxRetries || 3
        });

      case 'openai':
        return new ChatOpenAI({
          openAIApiKey: config.apiKey,
          modelName: model,
          temperature: config.temperature,
          maxTokens: config.maxTokens,
          maxRetries: config.maxRetries || 3
        });

      case 'anthropic':
        return new ChatAnthropic({
          anthropicApiKey: config.apiKey,
          modelName: model,
          temperature: config.temperature,
          maxTokens: config.maxTokens,
          maxRetries: config.maxRetries || 3
        });

      case 'azure':
        return new ChatOpenAI({
          azureOpenAIApiKey: config.azureOpenAIApiKey,
          azureOpenAIApiInstanceName: config.azureOpenAIApiInstanceName,
          azureOpenAIApiDeploymentName: config.azureOpenAIApiDeploymentName,
          azureOpenAIApiVersion: config.azureOpenAIApiVersion,
          temperature: config.temperature,
          maxTokens: config.maxTokens,
          maxRetries: config.maxRetries || 3
        });

      default:
        throw new Error(`Unsupported LLM provider: ${provider}`);
    }
  }

  /**
   * Get available providers and their status
   */
  getAvailableProviders() {
    const available = [];
    
    for (const [key, provider] of Object.entries(this.providers)) {
      const status = this.checkProviderStatus(key);
      available.push({
        id: key,
        name: provider.name,
        models: provider.models,
        default: provider.default,
        status,
        isDefault: key === this.defaultProvider
      });
    }
    
    return available;
  }

  /**
   * Check if a provider is properly configured
   */
  checkProviderStatus(provider) {
    switch (provider) {
      case 'ollama':
        return { available: true, reason: 'Always available for local development' };
      
      case 'groq':
        return {
          available: !!process.env.GROQ_API_KEY,
          reason: process.env.GROQ_API_KEY ? 'API key configured' : 'Missing GROQ_API_KEY'
        };
      
      case 'openai':
        return {
          available: !!process.env.OPENAI_API_KEY,
          reason: process.env.OPENAI_API_KEY ? 'API key configured' : 'Missing OPENAI_API_KEY'
        };
      
      case 'anthropic':
        return {
          available: !!process.env.ANTHROPIC_API_KEY,
          reason: process.env.ANTHROPIC_API_KEY ? 'API key configured' : 'Missing ANTHROPIC_API_KEY'
        };
      
      case 'azure':
        const hasAzure = process.env.AZURE_OPENAI_API_KEY && 
                        process.env.AZURE_OPENAI_INSTANCE_NAME && 
                        process.env.AZURE_OPENAI_DEPLOYMENT_NAME;
        return {
          available: hasAzure,
          reason: hasAzure ? 'Azure OpenAI configured' : 'Missing Azure OpenAI configuration'
        };
      
      default:
        return { available: false, reason: 'Unknown provider' };
    }
  }

  /**
   * Create multiple LLM instances for ensemble/fallback strategies
   */
  createLLMChain(providers = ['ollama', 'groq']) {
    const llms = [];
    
    for (const provider of providers) {
      try {
        const llm = this.createLLM({ provider });
        llms.push({ provider, llm });
      } catch (error) {
        this.logger.warn(`Failed to create LLM for provider ${provider}:`, error.message);
      }
    }
    
    if (llms.length === 0) {
      throw new Error('No LLM providers could be initialized');
    }
    
    return llms;
  }

  /**
   * Execute with fallback - try multiple providers until one succeeds
   */
  async executeWithFallback(messages, providers = null) {
    const chain = providers ? 
      this.createLLMChain(providers) : 
      this.createLLMChain(['ollama', 'groq', 'openai']);
    
    for (const { provider, llm } of chain) {
      try {
        this.logger.info(`Attempting execution with ${provider}`);
        const response = await llm.invoke(messages);
        this.logger.info(`Successfully executed with ${provider}`);
        return { provider, response };
      } catch (error) {
        this.logger.warn(`Failed with ${provider}:`, error.message);
        if (provider === chain[chain.length - 1].provider) {
          throw error; // Re-throw if this was the last provider
        }
      }
    }
  }

  /**
   * Get provider-specific prompt adjustments
   */
  getPromptAdjustments(provider, basePrompt) {
    // Some providers work better with certain prompt formats
    const adjustments = {
      ollama: {
        // Ollama works well with clear, structured prompts
        prefix: 'Please provide a clear and structured response.\n\n',
        suffix: '\n\nRespond in a well-formatted manner.'
      },
      groq: {
        // Groq is fast but benefits from concise prompts
        prefix: 'Provide a concise and accurate response.\n\n',
        suffix: ''
      },
      anthropic: {
        // Claude likes conversational, detailed prompts
        prefix: '',
        suffix: '\n\nPlease think through this step-by-step and provide a comprehensive response.'
      },
      openai: {
        // GPT models are flexible
        prefix: '',
        suffix: ''
      },
      azure: {
        // Same as OpenAI
        prefix: '',
        suffix: ''
      }
    };

    const adj = adjustments[provider] || { prefix: '', suffix: '' };
    return adj.prefix + basePrompt + adj.suffix;
  }
}

module.exports = LLMProviderFactory;