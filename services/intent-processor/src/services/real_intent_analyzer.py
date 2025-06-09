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

logger = logging.getLogger(__name__)


class RealIntentAnalyzer:
    """Intent analyzer using real LLM providers"""
    
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
            
        # Create prompt for intent analysis
        prompt = self._create_intent_prompt(text, context, project_info)
        
        try:
            # Generate response from LLM
            response = await provider.generate(prompt, temperature=0.3)
            
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
        """Create prompt for LLM"""
        
        prompt = f"""Analyze the following software development request and provide a structured response.

Request: {text}

Context: {json.dumps(context or {}, indent=2)}

Project Info: {json.dumps(project_info or {}, indent=2)}

Provide your analysis in the following JSON format:
{{
    "intent_type": "one of: CREATE_FEATURE, FIX_BUG, IMPROVE_PERFORMANCE, REFACTOR_CODE, ADD_DOCUMENTATION, ADD_TESTS, DEPLOY, UNKNOWN",
    "confidence": 0.0-1.0,
    "summary": "brief summary of what needs to be done",
    "tasks": [
        {{
            "name": "task name",
            "description": "detailed description",
            "type": "one of: DESIGN, IMPLEMENTATION, TESTING, DOCUMENTATION, DEPLOYMENT",
            "priority": "one of: HIGH, MEDIUM, LOW",
            "complexity": "one of: SIMPLE, MEDIUM, COMPLEX",
            "estimated_effort": minutes_as_integer,
            "required_skills": ["skill1", "skill2"],
            "dependencies": []
        }}
    ],
    "metadata": {{
        "key_technologies": ["tech1", "tech2"],
        "estimated_total_effort": total_minutes,
        "risk_factors": ["risk1", "risk2"]
    }}
}}

Important:
- Break down the request into concrete, actionable tasks
- Each task should be specific and measurable
- Consider dependencies between tasks
- Estimate effort realistically
- Identify required skills and technologies

Response (JSON only):"""
        
        return prompt
        
    def _parse_llm_response(self, response: str, original_text: str) -> IntentAnalysisResult:
        """Parse LLM response into structured result"""
        
        try:
            # Extract JSON from response
            json_match = re.search(r'\{.*\}', response, re.DOTALL)
            if json_match:
                data = json.loads(json_match.group())
            else:
                data = json.loads(response)
                
            # Convert to our models
            tasks = []
            for task_data in data.get('tasks', []):
                task = Task(
                    id=str(uuid4()),
                    name=task_data['name'],
                    description=task_data['description'],
                    type=TaskType(task_data['type']),
                    priority=TaskPriority(task_data['priority']),
                    complexity=TaskComplexity(task_data['complexity']),
                    estimated_effort=task_data['estimated_effort'],
                    required_skills=task_data['required_skills'],
                    dependencies=task_data.get('dependencies', []),
                    metadata=task_data.get('metadata', {})
                )
                tasks.append(task)
                
            return IntentAnalysisResult(
                intent_type=IntentType(data['intent_type']),
                confidence=data['confidence'],
                summary=data['summary'],
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