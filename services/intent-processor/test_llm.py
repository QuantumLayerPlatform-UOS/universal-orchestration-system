#!/usr/bin/env python3
"""Test LLM connectivity and response"""

import asyncio
import os
import sys

# Add the src directory to the path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from src.services.llm_factory import llm_factory


async def test_llm():
    """Test LLM provider"""
    print("Testing LLM connectivity...")
    
    # Get available providers
    available = await llm_factory.get_available_providers()
    print(f"Available providers: {available}")
    
    # Get a provider
    provider = await llm_factory.get_provider()
    if not provider:
        print("No LLM provider available!")
        return
        
    print(f"Using provider: {provider.__class__.__name__}")
    
    # Test simple generation
    print("\nTest 1: Simple generation")
    try:
        response = await provider.generate("Say hello", temperature=0.7, max_tokens=20)
        print(f"Response: {response}")
    except Exception as e:
        print(f"Error: {e}")
        
    # Test JSON generation
    print("\nTest 2: JSON generation")
    prompt = """Generate JSON:
{"greeting": "hello", "number": 42}

JSON:"""
    try:
        response = await provider.generate(prompt, temperature=0.1, max_tokens=50)
        print(f"Response: {response}")
    except Exception as e:
        print(f"Error: {e}")
        
    # Test intent analysis prompt
    print("\nTest 3: Intent analysis")
    prompt = """Analyze this request and categorize it:
"Create a simple REST API"

Respond with simple JSON:
{
  "intent_type": "CREATE_FEATURE",
  "confidence": 0.8,
  "summary": "Create REST API",
  "tasks": [{
    "name": "Build API",
    "description": "Implement REST endpoints",
    "type": "IMPLEMENTATION",
    "priority": "HIGH",
    "complexity": "MEDIUM",
    "estimated_effort": 120,
    "required_skills": ["API"],
    "dependencies": []
  }],
  "metadata": {
    "key_technologies": ["REST"],
    "estimated_total_effort": 120
  }
}

JSON:"""
    try:
        response = await provider.generate(prompt, temperature=0.3, max_tokens=500)
        print(f"Response: {response}")
    except Exception as e:
        print(f"Error: {e}")
        
    # Cleanup
    await llm_factory.cleanup()


if __name__ == "__main__":
    asyncio.run(test_llm())