"""
LLM Factory - Manages connections to various LLM providers
Supports: Ollama, OpenAI, Groq, Anthropic
Prioritizes faster providers and implements retry logic with exponential backoff
"""

import os
import logging
import asyncio
from typing import Optional, Dict, Any, List, Tuple
from abc import ABC, abstractmethod
import aiohttp
import json
import time
from functools import wraps

logger = logging.getLogger(__name__)


def retry_with_backoff(max_retries: int = 3, base_delay: float = 1.0):
    """Decorator for retry logic with exponential backoff"""
    def decorator(func):
        @wraps(func)
        async def wrapper(*args, **kwargs):
            last_exception = None
            for attempt in range(max_retries):
                try:
                    return await func(*args, **kwargs)
                except Exception as e:
                    last_exception = e
                    if attempt < max_retries - 1:
                        delay = base_delay * (2 ** attempt)
                        logger.warning(f"Attempt {attempt + 1} failed: {str(e)}. Retrying in {delay}s...")
                        await asyncio.sleep(delay)
                    else:
                        logger.error(f"All {max_retries} attempts failed: {str(e)}")
            raise last_exception
        return wrapper
    return decorator


class LLMProvider(ABC):
    """Base class for LLM providers"""
    
    def __init__(self, priority: int = 100):
        """Initialize provider with priority (lower = higher priority)"""
        self.priority = priority
        self._last_response_time = None
    
    @abstractmethod
    async def generate(self, prompt: str, **kwargs) -> str:
        """Generate text from prompt"""
        pass
    
    @abstractmethod
    async def is_available(self) -> bool:
        """Check if provider is available"""
        pass
    
    @property
    def average_response_time(self) -> Optional[float]:
        """Get average response time for this provider"""
        return self._last_response_time


class OllamaProvider(LLMProvider):
    """Ollama provider for local/remote models"""
    
    def __init__(self, base_url: str, model: str = "mistral"):
        super().__init__(priority=100)  # Lowest priority due to slowness
        self.base_url = base_url.rstrip('/')
        self.model = model
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            timeout = aiohttp.ClientTimeout(total=30, connect=5)
            self.session = aiohttp.ClientSession(timeout=timeout)
    
    @retry_with_backoff(max_retries=2, base_delay=0.5)        
    async def generate(self, prompt: str, **kwargs) -> str:
        start_time = time.time()
        await self._ensure_session()
        
        payload = {
            "model": self.model,
            "prompt": prompt,
            "stream": False,
            "temperature": kwargs.get("temperature", 0.7),
            "max_tokens": kwargs.get("max_tokens", 2000)
        }
        
        try:
            async with self.session.post(
                f"{self.base_url}/api/generate",
                json=payload,
                timeout=aiohttp.ClientTimeout(total=20)  # Reduced timeout
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    self._last_response_time = time.time() - start_time
                    return data.get("response", "")
                else:
                    error = await response.text()
                    raise Exception(f"Ollama error: {error}")
        except asyncio.TimeoutError:
            raise Exception("Ollama request timed out after 20 seconds")
        except Exception as e:
            logger.error(f"Ollama generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        await self._ensure_session()
        try:
            async with self.session.get(
                f"{self.base_url}/api/tags",
                timeout=aiohttp.ClientTimeout(total=3)
            ) as response:
                return response.status == 200
        except:
            return False
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()


class GroqProvider(LLMProvider):
    """Groq provider for fast inference"""
    
    def __init__(self, api_key: str, model: str = "llama-3.3-70b-versatile"):
        super().__init__(priority=10)  # High priority - very fast
        self.api_key = api_key
        self.model = model
        self.base_url = "https://api.groq.com/openai/v1"
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            timeout = aiohttp.ClientTimeout(total=15, connect=3)
            self.session = aiohttp.ClientSession(
                headers={"Authorization": f"Bearer {self.api_key}"},
                timeout=timeout
            )
    
    @retry_with_backoff(max_retries=3, base_delay=0.5)        
    async def generate(self, prompt: str, **kwargs) -> str:
        start_time = time.time()
        await self._ensure_session()
        
        payload = {
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "temperature": kwargs.get("temperature", 0.7),
            "max_tokens": kwargs.get("max_tokens", 2000)
        }
        
        try:
            async with self.session.post(
                f"{self.base_url}/chat/completions",
                json=payload,
                timeout=aiohttp.ClientTimeout(total=10)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    self._last_response_time = time.time() - start_time
                    return data["choices"][0]["message"]["content"]
                else:
                    error = await response.text()
                    raise Exception(f"Groq error: {error}")
        except Exception as e:
            logger.error(f"Groq generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        return bool(self.api_key and self.api_key != "" and self.api_key != "dummy-key")
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()


class OpenAIProvider(LLMProvider):
    """OpenAI provider"""
    
    def __init__(self, api_key: str, model: str = "gpt-4"):
        super().__init__(priority=20)  # Good priority - fast and reliable
        self.api_key = api_key
        self.model = model
        self.base_url = "https://api.openai.com/v1"
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            timeout = aiohttp.ClientTimeout(total=20, connect=3)
            self.session = aiohttp.ClientSession(
                headers={"Authorization": f"Bearer {self.api_key}"},
                timeout=timeout
            )
    
    @retry_with_backoff(max_retries=3, base_delay=1.0)        
    async def generate(self, prompt: str, **kwargs) -> str:
        start_time = time.time()
        await self._ensure_session()
        
        payload = {
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "temperature": kwargs.get("temperature", 0.7),
            "max_tokens": kwargs.get("max_tokens", 2000)
        }
        
        try:
            async with self.session.post(
                f"{self.base_url}/chat/completions",
                json=payload,
                timeout=aiohttp.ClientTimeout(total=15)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    self._last_response_time = time.time() - start_time
                    return data["choices"][0]["message"]["content"]
                else:
                    error = await response.text()
                    raise Exception(f"OpenAI error: {error}")
        except Exception as e:
            logger.error(f"OpenAI generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        return bool(self.api_key and self.api_key != "" and self.api_key != "dummy-key")
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()


class AnthropicProvider(LLMProvider):
    """Anthropic Claude provider"""
    
    def __init__(self, api_key: str, model: str = "claude-3-opus-20240229"):
        super().__init__(priority=15)  # High priority - fast and capable
        self.api_key = api_key
        self.model = model
        self.base_url = "https://api.anthropic.com/v1"
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            timeout = aiohttp.ClientTimeout(total=20, connect=3)
            self.session = aiohttp.ClientSession(
                headers={
                    "x-api-key": self.api_key,
                    "anthropic-version": "2023-06-01"
                },
                timeout=timeout
            )
    
    @retry_with_backoff(max_retries=3, base_delay=1.0)        
    async def generate(self, prompt: str, **kwargs) -> str:
        start_time = time.time()
        await self._ensure_session()
        
        payload = {
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": kwargs.get("max_tokens", 2000),
            "temperature": kwargs.get("temperature", 0.7)
        }
        
        try:
            async with self.session.post(
                f"{self.base_url}/messages",
                json=payload,
                timeout=aiohttp.ClientTimeout(total=15)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    self._last_response_time = time.time() - start_time
                    return data["content"][0]["text"]
                else:
                    error = await response.text()
                    raise Exception(f"Anthropic error: {error}")
        except Exception as e:
            logger.error(f"Anthropic generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        return bool(self.api_key and self.api_key != "" and self.api_key != "dummy-key")
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()


class LLMFactory:
    """Factory for creating LLM providers with smart prioritization"""
    
    def __init__(self):
        self.providers: Dict[str, LLMProvider] = {}
        self._initialize_providers()
        
    def _initialize_providers(self):
        """Initialize available providers based on environment variables"""
        
        # Groq - Fastest provider
        groq_key = os.getenv("GROQ_API_KEY", "")
        if groq_key and groq_key != "dummy-key":
            self.providers["groq"] = GroqProvider(
                api_key=groq_key,
                model=os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")
            )
            logger.info("Initialized Groq provider (priority: 10)")
            
        # Anthropic - Fast and capable
        anthropic_key = os.getenv("ANTHROPIC_API_KEY", "")
        if anthropic_key and anthropic_key != "dummy-key":
            self.providers["anthropic"] = AnthropicProvider(
                api_key=anthropic_key,
                model=os.getenv("ANTHROPIC_MODEL", "claude-3-opus-20240229")
            )
            logger.info("Initialized Anthropic provider (priority: 15)")
            
        # OpenAI - Reliable and fast
        openai_key = os.getenv("OPENAI_API_KEY", "")
        if openai_key and openai_key != "dummy-key":
            self.providers["openai"] = OpenAIProvider(
                api_key=openai_key,
                model=os.getenv("OPENAI_MODEL", "gpt-4")
            )
            logger.info("Initialized OpenAI provider (priority: 20)")
            
        # Ollama - Slowest, use as last resort
        ollama_url = os.getenv("OLLAMA_BASE_URL", "")
        if ollama_url:
            self.providers["ollama"] = OllamaProvider(
                base_url=ollama_url,
                model=os.getenv("OLLAMA_MODEL", "mistral")
            )
            logger.info(f"Initialized Ollama provider (priority: 100): {ollama_url}")
            
    async def get_available_providers(self) -> List[Tuple[str, LLMProvider]]:
        """Get list of available providers sorted by priority"""
        available = []
        
        # Check availability in parallel
        async def check_provider(name: str, provider: LLMProvider):
            try:
                if await provider.is_available():
                    return (name, provider)
            except Exception as e:
                logger.warning(f"Provider {name} availability check failed: {e}")
            return None
        
        tasks = [check_provider(name, provider) for name, provider in self.providers.items()]
        results = await asyncio.gather(*tasks)
        
        # Filter and sort by priority
        available = [r for r in results if r is not None]
        available.sort(key=lambda x: x[1].priority)
        
        return available
        
    async def get_provider(self, preferred: Optional[str] = None) -> Optional[LLMProvider]:
        """Get an available provider, preferring the specified one or fastest available"""
        
        # Try preferred provider first if it's fast enough
        if preferred and preferred in self.providers:
            provider = self.providers[preferred]
            # Only use preferred if it's reasonably fast (priority < 50)
            if provider.priority < 50 and await provider.is_available():
                logger.info(f"Using preferred provider: {preferred}")
                return provider
                
        # Get all available providers sorted by priority
        available = await self.get_available_providers()
        
        if available:
            name, provider = available[0]
            logger.info(f"Using {name} provider (priority: {provider.priority})")
            return provider
                
        logger.error("No LLM providers available")
        return None
        
    async def get_fast_provider(self, max_priority: int = 30) -> Optional[LLMProvider]:
        """Get a fast provider (priority <= max_priority)"""
        available = await self.get_available_providers()
        
        for name, provider in available:
            if provider.priority <= max_priority:
                logger.info(f"Using fast provider: {name} (priority: {provider.priority})")
                return provider
                
        return None
        
    async def execute_with_fastest(self, prompt: str, **kwargs) -> Optional[str]:
        """Execute prompt with the fastest available provider"""
        provider = await self.get_provider()
        if provider:
            return await provider.generate(prompt, **kwargs)
        return None
        
    async def execute_concurrent(self, prompt: str, providers: Optional[List[str]] = None, **kwargs) -> Optional[str]:
        """Execute prompt concurrently with multiple providers, return first successful result"""
        if providers:
            selected_providers = [(name, self.providers[name]) for name in providers if name in self.providers]
        else:
            selected_providers = await self.get_available_providers()
            # Limit to top 3 fastest providers
            selected_providers = selected_providers[:3]
        
        if not selected_providers:
            return None
            
        async def try_provider(name: str, provider: LLMProvider):
            try:
                logger.debug(f"Trying provider: {name}")
                return await provider.generate(prompt, **kwargs)
            except Exception as e:
                logger.warning(f"Provider {name} failed: {e}")
                return None
        
        # Create tasks for concurrent execution
        tasks = [try_provider(name, provider) for name, provider in selected_providers]
        
        # Use as_completed to return first successful result
        for completed in asyncio.as_completed(tasks):
            result = await completed
            if result:
                # Cancel remaining tasks
                for task in tasks:
                    if not task.done():
                        task.cancel()
                return result
                
        return None
        
    async def cleanup(self):
        """Cleanup resources"""
        for provider in self.providers.values():
            if hasattr(provider, '__aexit__'):
                await provider.__aexit__(None, None, None)
            elif hasattr(provider, 'session') and provider.session:
                await provider.session.close()


# Global factory instance
llm_factory = LLMFactory()