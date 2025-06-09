"""
Real Intent Analyzer using actual LLM providers
"""

import json
import logging
from datetime import datetime
from typing import Dict, Any, List, Optional
from uuid import uuid4

from ..models import (
    IntentType,
    TaskType,
    TaskPriority,
    TaskComplexity,
    Task,
    IntentAnalysisResult
)
from .llm_factory import llm_factory
from .meta_prompt_agent import MetaPromptAgent

logger = logging.getLogger(__name__)


class RealIntentAnalyzer:
    """Intent analyzer using real LLM providers with meta-prompt capabilities"""
    
    def __init__(self):
        self.meta_agent = MetaPromptAgent()
    
    async def initialize(self):
        """Initialize the analyzer"""
        available = await llm_factory.get_available_providers()
        logger.info(f"Available LLM providers: {available}")
        
    async def cleanup(self):
        """Cleanup resources"""
        await llm_factory.cleanup()
        
    async def check_openai_health(self) -> bool:
        """Check if LLM provider is available"""
        provider = await llm_factory.get_provider()
        return provider is not None
        
    async def analyze_intent(
        self,
        text: str,
        context: Optional[Dict[str, Any]] = None,
        project_info: Optional[Dict[str, Any]] = None
    ) -> IntentAnalysisResult:
        """Analyze user intent using real LLM"""
        
        # Get available provider
        provider = await llm_factory.get_provider()
        if not provider:
            logger.warning("No LLM provider available, falling back to basic analysis")
            return self._basic_analysis(text)
            
        # Use meta-prompt agent for intelligent analysis
        prompt = self.meta_agent.generate_context_aware_prompt(text, context)
        
        try:
            # Generate response from LLM
            logger.info(f"Sending prompt to LLM provider: {provider.__class__.__name__}")
            logger.debug(f"Prompt: {prompt}")
            
            response = await provider.generate(prompt, temperature=0.3, max_tokens=800)
            
            logger.info(f"Received response from LLM, length: {len(response)} characters")
            logger.debug(f"LLM Response: {response}")
            
            # Parse LLM response
            result = self._parse_llm_response(response, text)
            return result
            
        except Exception as e:
            logger.error(f"LLM analysis failed: {str(e)}")
            return self._basic_analysis(text)
            
    def _create_intent_prompt(
        self, 
        text: str, 
        context: Optional[Dict[str, Any]], 
        project_info: Optional[Dict[str, Any]]
    ) -> str:
        """Create optimized prompt for LLM"""
        
        # Dynamic NLP-based prompt
        prompt = f"""You are an expert software architect. Analyze this natural language request and understand what the user wants to build:

"{text}"

Based on your understanding, categorize the intent and break it down into actionable tasks.

Valid intent types: feature_request, bug_fix, refactoring, documentation, testing, deployment, configuration, research, unknown

Valid task types: frontend, backend, database, api, infrastructure, testing, documentation, design, devops, security

Valid priorities: critical, high, medium, low

Valid complexities: simple, moderate, complex, very_complex

Analyze the request deeply and respond with JSON only:
{{
  "intent_type": "<detected intent>",
  "confidence": <0.0-1.0>,
  "summary": "<what user wants to achieve>",
  "tasks": [
    {{
      "id": "<unique_id>",
      "title": "<task title>",
      "description": "<detailed description>",
      "type": "<task type>",
      "priority": "<priority>",
      "complexity": "<complexity>",
      "estimated_hours": <number>,
      "dependencies": [],
      "tags": ["<relevant tags>"]
    }}
  ],
  "metadata": {{
    "key_entities": ["<detected entities>"],
    "technologies": ["<detected technologies>"],
    "domain": "<detected domain>"
  }}
}}"""
        
        return prompt
        
    def _parse_llm_response(self, response: str, original_text: str) -> IntentAnalysisResult:
        """Parse LLM response into structured result"""
        
        try:
            # Clean and extract JSON from response
            response = response.strip()
            
            # Try to find JSON in the response
            json_start = response.find('{')
            json_end = response.rfind('}') + 1
            
            if json_start >= 0 and json_end > json_start:
                json_str = response[json_start:json_end]
                data = json.loads(json_str)
            else:
                # Try direct parsing
                data = json.loads(response)
                
            # Convert to our models with validation
            tasks = []
            for task_data in data.get('tasks', []):
                try:
                    task = Task(
                        id=str(uuid4()),
                        name=task_data.get('name', 'Unnamed task'),
                        description=task_data.get('description', 'No description'),
                        type=TaskType(task_data.get('type', 'IMPLEMENTATION')),
                        priority=TaskPriority(task_data.get('priority', 'MEDIUM')),
                        complexity=TaskComplexity(task_data.get('complexity', 'MEDIUM')),
                        estimated_effort=task_data.get('estimated_effort', 60),
                        required_skills=task_data.get('required_skills', []),
                        dependencies=task_data.get('dependencies', []),
                        metadata=task_data.get('metadata', {})
                    )
                    tasks.append(task)
                except Exception as e:
                    logger.warning(f"Failed to parse task: {str(e)}")
                    
            # Ensure we have at least one task
            if not tasks:
                tasks = [Task(
                    id=str(uuid4()),
                    name="Implement request",
                    description=f"Implementation of: {original_text}",
                    type=TaskType.IMPLEMENTATION,
                    priority=TaskPriority.MEDIUM,
                    complexity=TaskComplexity.MEDIUM,
                    estimated_effort=120,
                    required_skills=["general"],
                    dependencies=[],
                    metadata={}
                )]
                
            return IntentAnalysisResult(
                intent_type=IntentType(data.get('intent_type', 'UNKNOWN')),
                confidence=float(data.get('confidence', 0.5)),
                summary=data.get('summary', 'Analysis complete'),
                tasks=tasks,
                metadata=data.get('metadata', {})
            )
            
        except Exception as e:
            logger.error(f"Failed to parse LLM response: {str(e)}")
            return self._basic_analysis(original_text)
            
    def _basic_analysis(self, text: str) -> IntentAnalysisResult:
        """Basic analysis fallback"""
        
        # Determine intent type based on keywords
        text_lower = text.lower()
        
        if any(word in text_lower for word in ["create", "build", "develop", "add", "implement"]):
            intent_type = IntentType.CREATE_FEATURE
        elif any(word in text_lower for word in ["fix", "bug", "error", "issue", "problem"]):
            intent_type = IntentType.FIX_BUG
        elif any(word in text_lower for word in ["improve", "optimize", "enhance", "speed up"]):
            intent_type = IntentType.IMPROVE_PERFORMANCE
        elif any(word in text_lower for word in ["refactor", "restructure", "clean up"]):
            intent_type = IntentType.REFACTOR_CODE
        elif any(word in text_lower for word in ["document", "docs", "readme"]):
            intent_type = IntentType.ADD_DOCUMENTATION
        elif any(word in text_lower for word in ["test", "testing", "coverage"]):
            intent_type = IntentType.ADD_TESTS
        elif any(word in text_lower for word in ["deploy", "release", "ship"]):
            intent_type = IntentType.DEPLOY
        else:
            intent_type = IntentType.UNKNOWN
            
        # Create basic task
        task = Task(
            id=str(uuid4()),
            name="Analyze and implement request",
            description=f"Implementation of: {text}",
            type=TaskType.IMPLEMENTATION,
            priority=TaskPriority.MEDIUM,
            complexity=TaskComplexity.MEDIUM,
            estimated_effort=240,
            required_skills=["general"],
            dependencies=[],
            metadata={}
        )
        
        return IntentAnalysisResult(
            intent_type=intent_type,
            confidence=0.5,
            summary=f"Basic analysis: {intent_type.value} request",
            tasks=[task],
            metadata={"fallback": True}
        )
        
    async def validate_tasks(self, task_breakdown: Any) -> Any:
        """Validate task breakdown"""
        # For now, return a simple validation
        return type('ValidationResult', (), {
            'is_valid': True,
            'issues': [],
            'suggestions': []
        })


# For missing import
import re