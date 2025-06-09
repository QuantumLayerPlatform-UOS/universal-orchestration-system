"""
LLM Provider Factory for flexible model selection
Supports: Ollama, Groq, OpenAI, Anthropic, Azure OpenAI
"""
import os
import logging
from typing import Optional, Dict, Any, List
from langchain_openai import ChatOpenAI, AzureChatOpenAI
from langchain_anthropic import ChatAnthropic
from langchain_groq import ChatGroq
from langchain_community.chat_models import ChatOllama

logger = logging.getLogger(__name__)


class LLMProviderFactory:
    """Factory for creating LLM instances based on provider configuration"""
    
    def __init__(self):
        self.providers = {
            'ollama': {
                'name': 'Ollama (Local)',
                'models': ['llama2', 'mistral', 'codellama', 'neural-chat', 'mixtral'],
                'default': 'mistral',
                'config': {
                    'base_url': os.getenv('OLLAMA_BASE_URL', 'https://model.gonella.co.uk'),
                    'temperature': 0.7,
                    'timeout': 120
                }
            },
            'groq': {
                'name': 'Groq',
                'models': ['mixtral-8x7b-32768', 'llama2-70b-4096', 'gemma-7b-it'],
                'default': 'mixtral-8x7b-32768',
                'config': {
                    'api_key': os.getenv('GROQ_API_KEY'),
                    'temperature': 0.7,
                    'max_tokens': 2000
                }
            },
            'openai': {
                'name': 'OpenAI',
                'models': ['gpt-4-turbo-preview', 'gpt-4', 'gpt-3.5-turbo'],
                'default': 'gpt-3.5-turbo',
                'config': {
                    'api_key': os.getenv('OPENAI_API_KEY'),
                    'temperature': 0.7,
                    'max_tokens': 2000
                }
            },
            'anthropic': {
                'name': 'Anthropic',
                'models': ['claude-3-opus-20240229', 'claude-3-sonnet-20240229', 'claude-3-haiku-20240307'],
                'default': 'claude-3-sonnet-20240229',
                'config': {
                    'api_key': os.getenv('ANTHROPIC_API_KEY'),
                    'temperature': 0.7,
                    'max_tokens': 2000
                }
            },
            'azure': {
                'name': 'Azure OpenAI',
                'models': ['gpt-35-turbo', 'gpt-4'],
                'default': 'gpt-35-turbo',
                'config': {
                    'api_key': os.getenv('AZURE_OPENAI_API_KEY'),
                    'azure_endpoint': os.getenv('AZURE_OPENAI_ENDPOINT'),
                    'deployment_name': os.getenv('AZURE_OPENAI_DEPLOYMENT_NAME'),
                    'api_version': os.getenv('AZURE_OPENAI_API_VERSION', '2023-05-15'),
                    'temperature': 0.7,
                    'max_tokens': 2000
                }
            }
        }
        
        self.default_provider = self._detect_default_provider()
    
    def _detect_default_provider(self) -> str:
        """Detect the best available provider based on environment variables"""
        # Priority order for development
        if os.getenv('OLLAMA_BASE_URL') or os.getenv('USE_OLLAMA') == 'true':
            logger.info('Using Ollama as default provider')
            return 'ollama'
        if os.getenv('GROQ_API_KEY'):
            logger.info('Using Groq as default provider')
            return 'groq'
        if os.getenv('OPENAI_API_KEY'):
            logger.info('Using OpenAI as default provider')
            return 'openai'
        if os.getenv('ANTHROPIC_API_KEY'):
            logger.info('Using Anthropic as default provider')
            return 'anthropic'
        if os.getenv('AZURE_OPENAI_API_KEY'):
            logger.info('Using Azure OpenAI as default provider')
            return 'azure'
        
        logger.warning('No LLM provider credentials found, defaulting to Ollama')
        return 'ollama'
    
    def create_llm(self, provider: Optional[str] = None, model: Optional[str] = None, **kwargs):
        """
        Create an LLM instance
        
        Args:
            provider: Provider name (ollama, groq, openai, anthropic, azure)
            model: Model name (optional, uses default if not specified)
            **kwargs: Additional configuration to override defaults
        """
        provider = provider or self.default_provider
        provider_config = self.providers.get(provider)
        
        if not provider_config:
            raise ValueError(f"Unknown LLM provider: {provider}")
        
        model = model or provider_config['default']
        config = {**provider_config['config'], **kwargs}
        
        logger.info(f"Creating LLM instance: provider={provider}, model={model}")
        
        if provider == 'ollama':
            return ChatOllama(
                base_url=config['base_url'],
                model=model,
                temperature=config.get('temperature', 0.7),
                timeout=config.get('timeout', 120),
                num_predict=config.get('max_tokens', 2000)
            )
        
        elif provider == 'groq':
            return ChatGroq(
                groq_api_key=config['api_key'],
                model_name=model,
                temperature=config.get('temperature', 0.7),
                max_tokens=config.get('max_tokens', 2000)
            )
        
        elif provider == 'openai':
            return ChatOpenAI(
                openai_api_key=config['api_key'],
                model_name=model,
                temperature=config.get('temperature', 0.7),
                max_tokens=config.get('max_tokens', 2000)
            )
        
        elif provider == 'anthropic':
            return ChatAnthropic(
                anthropic_api_key=config['api_key'],
                model_name=model,
                temperature=config.get('temperature', 0.7),
                max_tokens=config.get('max_tokens', 2000)
            )
        
        elif provider == 'azure':
            return AzureChatOpenAI(
                azure_endpoint=config['azure_endpoint'],
                openai_api_key=config['api_key'],
                deployment_name=config['deployment_name'],
                openai_api_version=config['api_version'],
                temperature=config.get('temperature', 0.7),
                max_tokens=config.get('max_tokens', 2000)
            )
        
        else:
            raise ValueError(f"Unsupported LLM provider: {provider}")
    
    def get_available_providers(self) -> List[Dict[str, Any]]:
        """Get available providers and their status"""
        available = []
        
        for key, provider in self.providers.items():
            status = self._check_provider_status(key)
            available.append({
                'id': key,
                'name': provider['name'],
                'models': provider['models'],
                'default': provider['default'],
                'status': status,
                'is_default': key == self.default_provider
            })
        
        return available
    
    def _check_provider_status(self, provider: str) -> Dict[str, Any]:
        """Check if a provider is properly configured"""
        if provider == 'ollama':
            return {'available': True, 'reason': 'Always available for local development'}
        
        elif provider == 'groq':
            return {
                'available': bool(os.getenv('GROQ_API_KEY')),
                'reason': 'API key configured' if os.getenv('GROQ_API_KEY') else 'Missing GROQ_API_KEY'
            }
        
        elif provider == 'openai':
            return {
                'available': bool(os.getenv('OPENAI_API_KEY')),
                'reason': 'API key configured' if os.getenv('OPENAI_API_KEY') else 'Missing OPENAI_API_KEY'
            }
        
        elif provider == 'anthropic':
            return {
                'available': bool(os.getenv('ANTHROPIC_API_KEY')),
                'reason': 'API key configured' if os.getenv('ANTHROPIC_API_KEY') else 'Missing ANTHROPIC_API_KEY'
            }
        
        elif provider == 'azure':
            has_azure = bool(
                os.getenv('AZURE_OPENAI_API_KEY') and
                os.getenv('AZURE_OPENAI_ENDPOINT') and
                os.getenv('AZURE_OPENAI_DEPLOYMENT_NAME')
            )
            return {
                'available': has_azure,
                'reason': 'Azure OpenAI configured' if has_azure else 'Missing Azure OpenAI configuration'
            }
        
        return {'available': False, 'reason': 'Unknown provider'}
    
    async def execute_with_fallback(self, messages, providers: Optional[List[str]] = None):
        """Execute with fallback - try multiple providers until one succeeds"""
        providers = providers or ['ollama', 'groq', 'openai']
        
        for provider in providers:
            try:
                if not self._check_provider_status(provider)['available']:
                    continue
                
                logger.info(f"Attempting execution with {provider}")
                llm = self.create_llm(provider=provider)
                response = await llm.ainvoke(messages)
                logger.info(f"Successfully executed with {provider}")
                return {'provider': provider, 'response': response}
                
            except Exception as e:
                logger.warning(f"Failed with {provider}: {str(e)}")
                if provider == providers[-1]:
                    raise
        
        raise Exception("All providers failed")
    
    def get_prompt_adjustments(self, provider: str, base_prompt: str) -> str:
        """Get provider-specific prompt adjustments"""
        adjustments = {
            'ollama': {
                'prefix': 'Please provide a clear and structured response.\n\n',
                'suffix': '\n\nRespond in a well-formatted manner.'
            },
            'groq': {
                'prefix': 'Provide a concise and accurate response.\n\n',
                'suffix': ''
            },
            'anthropic': {
                'prefix': '',
                'suffix': '\n\nPlease think through this step-by-step and provide a comprehensive response.'
            },
            'openai': {
                'prefix': '',
                'suffix': ''
            },
            'azure': {
                'prefix': '',
                'suffix': ''
            }
        }
        
        adj = adjustments.get(provider, {'prefix': '', 'suffix': ''})
        return adj['prefix'] + base_prompt + adj['suffix']