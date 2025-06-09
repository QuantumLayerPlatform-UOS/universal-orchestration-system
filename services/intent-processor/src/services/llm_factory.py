"""
LLM Factory - Manages connections to various LLM providers
Supports: Ollama, OpenAI, Groq, Anthropic
"""

import os
import logging
from typing import Optional, Dict, Any, List
from abc import ABC, abstractmethod
import aiohttp
import json

logger = logging.getLogger(__name__)


class LLMProvider(ABC):
    """Base class for LLM providers"""
    
    @abstractmethod
    async def generate(self, prompt: str, **kwargs) -> str:
        """Generate text from prompt"""
        pass
    
    @abstractmethod
    async def is_available(self) -> bool:
        """Check if provider is available"""
        pass


class OllamaProvider(LLMProvider):
    """Ollama provider for local/remote models"""
    
    def __init__(self, base_url: str, model: str = "mistral"):
        self.base_url = base_url.rstrip('/')
        self.model = model
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            self.session = aiohttp.ClientSession()
            
    async def generate(self, prompt: str, **kwargs) -> str:
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
                timeout=aiohttp.ClientTimeout(total=60)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    return data.get("response", "")
                else:
                    error = await response.text()
                    raise Exception(f"Ollama error: {error}")
        except Exception as e:
            logger.error(f"Ollama generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        await self._ensure_session()
        try:
            async with self.session.get(
                f"{self.base_url}/api/tags",
                timeout=aiohttp.ClientTimeout(total=5)
            ) as response:
                return response.status == 200
        except:
            return False


class GroqProvider(LLMProvider):
    """Groq provider for fast inference"""
    
    def __init__(self, api_key: str, model: str = "mixtral-8x7b-32768"):
        self.api_key = api_key
        self.model = model
        self.base_url = "https://api.groq.com/openai/v1"
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            self.session = aiohttp.ClientSession(
                headers={"Authorization": f"Bearer {self.api_key}"}
            )
            
    async def generate(self, prompt: str, **kwargs) -> str:
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
                timeout=aiohttp.ClientTimeout(total=60)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    return data["choices"][0]["message"]["content"]
                else:
                    error = await response.text()
                    raise Exception(f"Groq error: {error}")
        except Exception as e:
            logger.error(f"Groq generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        return bool(self.api_key and self.api_key != "")


class OpenAIProvider(LLMProvider):
    """OpenAI provider"""
    
    def __init__(self, api_key: str, model: str = "gpt-4"):
        self.api_key = api_key
        self.model = model
        self.base_url = "https://api.openai.com/v1"
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            self.session = aiohttp.ClientSession(
                headers={"Authorization": f"Bearer {self.api_key}"}
            )
            
    async def generate(self, prompt: str, **kwargs) -> str:
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
                timeout=aiohttp.ClientTimeout(total=60)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    return data["choices"][0]["message"]["content"]
                else:
                    error = await response.text()
                    raise Exception(f"OpenAI error: {error}")
        except Exception as e:
            logger.error(f"OpenAI generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        return bool(self.api_key and self.api_key != "")


class AnthropicProvider(LLMProvider):
    """Anthropic Claude provider"""
    
    def __init__(self, api_key: str, model: str = "claude-3-opus-20240229"):
        self.api_key = api_key
        self.model = model
        self.base_url = "https://api.anthropic.com/v1"
        self.session = None
        
    async def _ensure_session(self):
        if not self.session:
            self.session = aiohttp.ClientSession(
                headers={
                    "x-api-key": self.api_key,
                    "anthropic-version": "2023-06-01"
                }
            )
            
    async def generate(self, prompt: str, **kwargs) -> str:
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
                timeout=aiohttp.ClientTimeout(total=60)
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    return data["content"][0]["text"]
                else:
                    error = await response.text()
                    raise Exception(f"Anthropic error: {error}")
        except Exception as e:
            logger.error(f"Anthropic generation failed: {str(e)}")
            raise
            
    async def is_available(self) -> bool:
        return bool(self.api_key and self.api_key != "")


class LLMFactory:
    """Factory for creating LLM providers"""
    
    def __init__(self):
        self.providers: Dict[str, LLMProvider] = {}
        self._initialize_providers()
        
    def _initialize_providers(self):
        """Initialize available providers based on environment variables"""
        
        # Ollama
        ollama_url = os.getenv("OLLAMA_BASE_URL", "")
        if ollama_url:
            self.providers["ollama"] = OllamaProvider(
                base_url=ollama_url,
                model=os.getenv("OLLAMA_MODEL", "mistral")
            )
            logger.info(f"Initialized Ollama provider: {ollama_url}")
            
        # Groq
        groq_key = os.getenv("GROQ_API_KEY", "")
        if groq_key:
            self.providers["groq"] = GroqProvider(
                api_key=groq_key,
                model=os.getenv("GROQ_MODEL", "mixtral-8x7b-32768")
            )
            logger.info("Initialized Groq provider")
            
        # OpenAI
        openai_key = os.getenv("OPENAI_API_KEY", "")
        if openai_key:
            self.providers["openai"] = OpenAIProvider(
                api_key=openai_key,
                model=os.getenv("OPENAI_MODEL", "gpt-4")
            )
            logger.info("Initialized OpenAI provider")
            
        # Anthropic
        anthropic_key = os.getenv("ANTHROPIC_API_KEY", "")
        if anthropic_key:
            self.providers["anthropic"] = AnthropicProvider(
                api_key=anthropic_key,
                model=os.getenv("ANTHROPIC_MODEL", "claude-3-opus-20240229")
            )
            logger.info("Initialized Anthropic provider")
            
    async def get_available_providers(self) -> List[str]:
        """Get list of available providers"""
        available = []
        for name, provider in self.providers.items():
            if await provider.is_available():
                available.append(name)
        return available
        
    async def get_provider(self, preferred: Optional[str] = None) -> Optional[LLMProvider]:
        """Get an available provider, preferring the specified one"""
        
        # Try preferred provider first
        if preferred and preferred in self.providers:
            provider = self.providers[preferred]
            if await provider.is_available():
                return provider
                
        # Fall back to any available provider
        for name, provider in self.providers.items():
            if await provider.is_available():
                logger.info(f"Using {name} provider")
                return provider
                
        return None
        
    async def cleanup(self):
        """Cleanup resources"""
        for provider in self.providers.values():
            if hasattr(provider, 'session') and provider.session:
                await provider.session.close()


# Global factory instance
llm_factory = LLMFactory()