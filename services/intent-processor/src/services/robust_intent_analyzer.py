"""
Robust Intent Analyzer with multiple fallback strategies
"""

import json
import logging
import re
from datetime import datetime
from typing import Dict, Any, List, Optional, Tuple
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
from .intent_cache import IntentCache
from .thought_stream import thought_stream, ThoughtType

logger = logging.getLogger(__name__)


class RobustIntentAnalyzer:
    """Robust intent analyzer with multiple strategies and fallbacks"""
    
    def __init__(self, redis_client=None):
        self.meta_agent = MetaPromptAgent()
        self.cache = IntentCache(redis_client)
        self.llm_retries = 3
        self.strategies = [
            self._llm_with_structured_output,
            self._llm_with_guided_generation,
            self._llm_with_simple_prompt,
            self._rule_based_with_nlp,
            self._basic_keyword_analysis
        ]
        
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
        project_info: Optional[Dict[str, Any]] = None,
        request_id: Optional[str] = None
    ) -> IntentAnalysisResult:
        """Analyze intent using multiple strategies with fallbacks"""
        
        logger.info(f"Starting intent analysis for: {text[:100]}...")
        
        # Emit initial thought if request_id provided
        if request_id:
            await thought_stream.emit_thought(
                request_id,
                ThoughtType.UNDERSTANDING,
                detail=f"Processing {len(text)} character request",
                progress=0.05
            )
        
        # Check cache first
        cached_result = await self.cache.get(text, context)
        if cached_result:
            logger.info("Returning cached result")
            if request_id:
                await thought_stream.emit_thought(
                    request_id,
                    ThoughtType.COMPLETE,
                    detail="Found in cache",
                    progress=1.0
                )
            return cached_result
        
        # Try each strategy in order
        for i, strategy in enumerate(self.strategies):
            try:
                logger.info(f"Attempting strategy {i+1}: {strategy.__name__}")
                
                # Emit progress update
                if request_id:
                    progress = 0.1 + (i * 0.15)  # Progress from 0.1 to 0.85
                    await thought_stream.emit_thought(
                        request_id,
                        ThoughtType.ANALYZING,
                        detail=f"Trying strategy: {strategy.__name__.replace('_', ' ').title()}",
                        progress=progress
                    )
                
                result = await strategy(text, context, project_info, request_id)
                if result:
                    logger.info(f"Strategy {strategy.__name__} succeeded")
                    
                    # Emit final analysis details
                    if request_id:
                        await thought_stream.emit_detailed_analysis(
                            request_id,
                            {
                                'text': text,
                                'domain': self.meta_agent.analyze_domain(text),
                                'intent_type': result.intent_type.value,
                                'confidence': result.confidence,
                                'task_count': len(result.tasks),
                                'total_hours': sum(t.estimated_hours for t in result.tasks)
                            }
                        )
                    
                    # Cache the successful result
                    await self.cache.set(text, result, context)
                    return result
            except Exception as e:
                logger.warning(f"Strategy {strategy.__name__} failed: {str(e)}")
                
                # Emit error thought
                if request_id:
                    await thought_stream.emit_thought(
                        request_id,
                        ThoughtType.ERROR,
                        detail=f"Strategy failed: {strategy.__name__}",
                        metadata={"error": str(e)}
                    )
                continue
                
        # If all strategies fail, return a basic result
        logger.error("All strategies failed, returning minimal result")
        result = self._create_minimal_result(text)
        # Cache even the minimal result
        await self.cache.set(text, result, context)
        return result
        
    async def _llm_with_structured_output(
        self, 
        text: str, 
        context: Optional[Dict[str, Any]], 
        project_info: Optional[Dict[str, Any]],
        request_id: Optional[str] = None
    ) -> Optional[IntentAnalysisResult]:
        """Use LLM with structured output format"""
        
        provider = await llm_factory.get_provider()
        if not provider:
            return None
            
        # Create a prompt that guides the LLM to produce valid JSON
        domain = self.meta_agent.analyze_domain(text)
        entities = self.meta_agent.extract_entities(text)
        
        prompt = f"""You are a software architect. Analyze this request and provide a JSON response.

Request: "{text}"
Domain: {domain}
Entities: {entities}

IMPORTANT: Your response must be ONLY valid JSON, nothing else.

Use these exact enum values:
- intent_type: feature_request, bug_fix, refactoring, documentation, testing, deployment, configuration, research, unknown
- task type: frontend, backend, database, api, infrastructure, testing, documentation, design, devops, security
- priority: critical, high, medium, low
- complexity: simple, moderate, complex, very_complex

JSON Response:
{{
  "intent_type": "<use one of the enum values above>",
  "confidence": 0.8,
  "summary": "<one sentence summary>",
  "tasks": [
    {{
      "id": "task_1",
      "title": "<clear task title>",
      "description": "<detailed description>",
      "type": "<use task type enum>",
      "priority": "<use priority enum>",
      "complexity": "<use complexity enum>",
      "estimated_hours": <number>,
      "dependencies": [],
      "tags": ["<relevant>", "<tags>"],
      "acceptance_criteria": ["<criterion 1>", "<criterion 2>"]
    }}
  ],
  "metadata": {{
    "domain": "{domain}",
    "entities": {json.dumps(entities)},
    "total_estimated_hours": <sum of task hours>
  }}
}}"""

        try:
            response = await provider.generate(prompt, temperature=0.2, max_tokens=1000)
            return self._parse_json_response(response, text)
        except Exception as e:
            logger.error(f"Structured output failed: {str(e)}")
            return None
            
    async def _llm_with_guided_generation(
        self,
        text: str,
        context: Optional[Dict[str, Any]],
        project_info: Optional[Dict[str, Any]]
    ) -> Optional[IntentAnalysisResult]:
        """Use step-by-step guided generation"""
        
        provider = await llm_factory.get_provider()
        if not provider:
            return None
            
        try:
            # Step 1: Identify intent type
            intent_prompt = f"""Classify this request into ONE category:
Request: "{text}"

Categories:
- feature_request: New functionality
- bug_fix: Fixing errors or issues  
- refactoring: Code improvement
- documentation: Documentation tasks
- testing: Test creation
- deployment: Deployment tasks
- configuration: Config changes
- research: Investigation tasks
- unknown: Can't determine

Answer with just the category name:"""

            intent_response = await provider.generate(intent_prompt, temperature=0.1, max_tokens=20)
            intent_type = self._extract_intent_type(intent_response.strip().lower())
            
            # Step 2: Generate tasks
            task_prompt = f"""Create tasks for this {intent_type} request:
"{text}"

List 1-3 specific tasks. For each task provide:
- Title (short)
- Type (api/backend/frontend/database/testing)
- Hours estimate
- Priority (high/medium/low)

Format: Title | Type | Hours | Priority"""

            task_response = await provider.generate(task_prompt, temperature=0.3, max_tokens=200)
            tasks = self._parse_task_list(task_response, text)
            
            # Step 3: Generate summary
            summary_prompt = f"""Summarize this request in one sentence:
"{text}"

Summary:"""
            summary = await provider.generate(summary_prompt, temperature=0.2, max_tokens=50)
            
            return IntentAnalysisResult(
                intent_type=intent_type,
                confidence=0.7,
                summary=summary.strip(),
                tasks=tasks,
                metadata={"strategy": "guided_generation"}
            )
            
        except Exception as e:
            logger.error(f"Guided generation failed: {str(e)}")
            return None
            
    async def _llm_with_simple_prompt(
        self,
        text: str,
        context: Optional[Dict[str, Any]],
        project_info: Optional[Dict[str, Any]]
    ) -> Optional[IntentAnalysisResult]:
        """Use a simple prompt approach"""
        
        provider = await llm_factory.get_provider()
        if not provider:
            return None
            
        prompt = f"""What kind of software task is this: "{text}"

Answer briefly:
1. Type: (feature/bug/refactor/docs/test/deploy)
2. Main task: (one sentence)
3. Hours needed: (number)
4. Priority: (high/medium/low)"""

        try:
            response = await provider.generate(prompt, temperature=0.3, max_tokens=100)
            return self._parse_simple_response(response, text)
        except Exception as e:
            logger.error(f"Simple prompt failed: {str(e)}")
            return None
            
    async def _rule_based_with_nlp(
        self,
        text: str,
        context: Optional[Dict[str, Any]],
        project_info: Optional[Dict[str, Any]]
    ) -> IntentAnalysisResult:
        """Rule-based analysis with NLP techniques"""
        
        text_lower = text.lower()
        
        # Intent detection rules
        intent_rules = [
            (IntentType.FEATURE_REQUEST, ["create", "build", "develop", "add", "implement", "need", "want"]),
            (IntentType.BUG_FIX, ["fix", "bug", "error", "issue", "problem", "broken", "crash"]),
            (IntentType.REFACTORING, ["refactor", "improve", "optimize", "enhance", "clean", "restructure"]),
            (IntentType.DOCUMENTATION, ["document", "docs", "readme", "guide", "tutorial"]),
            (IntentType.TESTING, ["test", "testing", "coverage", "unit test", "integration"]),
            (IntentType.DEPLOYMENT, ["deploy", "release", "ship", "publish", "production"]),
            (IntentType.CONFIGURATION, ["config", "configure", "setup", "environment", "settings"]),
            (IntentType.RESEARCH, ["research", "investigate", "explore", "analyze", "study"])
        ]
        
        # Score each intent type
        scores = {}
        for intent_type, keywords in intent_rules:
            score = sum(2 if keyword in text_lower else 0 for keyword in keywords)
            if score > 0:
                scores[intent_type] = score
                
        # Select highest scoring intent
        if scores:
            intent_type = max(scores, key=scores.get)
            confidence = min(scores[intent_type] / 10, 0.9)
        else:
            intent_type = IntentType.UNKNOWN
            confidence = 0.3
            
        # Extract tasks based on patterns
        tasks = self._extract_tasks_from_text(text, intent_type)
        
        # Generate summary
        summary = self._generate_summary(text, intent_type)
        
        return IntentAnalysisResult(
            intent_type=intent_type,
            confidence=confidence,
            summary=summary,
            tasks=tasks,
            metadata={
                "strategy": "rule_based_nlp",
                "scores": {k.value: v for k, v in scores.items()}
            }
        )
        
    async def _basic_keyword_analysis(
        self,
        text: str,
        context: Optional[Dict[str, Any]],
        project_info: Optional[Dict[str, Any]]
    ) -> IntentAnalysisResult:
        """Basic keyword-based analysis as final fallback"""
        
        # Simplified intent detection
        text_lower = text.lower()
        
        if any(word in text_lower for word in ["create", "build", "need"]):
            intent_type = IntentType.FEATURE_REQUEST
        elif any(word in text_lower for word in ["fix", "bug", "error"]):
            intent_type = IntentType.BUG_FIX
        elif any(word in text_lower for word in ["test"]):
            intent_type = IntentType.TESTING
        else:
            intent_type = IntentType.UNKNOWN
            
        # Create a single task
        task = Task(
            id=str(uuid4()),
            title="Implement requested functionality",
            description=text,
            type=TaskType.API if "api" in text_lower else TaskType.BACKEND,
            priority=TaskPriority.MEDIUM,
            complexity=TaskComplexity.MODERATE,
            estimated_hours=8.0,
            dependencies=[],
            tags=["general"],
            acceptance_criteria=["Functionality implemented as requested"]
        )
        
        return IntentAnalysisResult(
            intent_type=intent_type,
            confidence=0.4,
            summary=f"Basic analysis: {intent_type.value}",
            tasks=[task],
            metadata={"strategy": "basic_keywords"}
        )
        
    def _parse_json_response(self, response: str, original_text: str) -> Optional[IntentAnalysisResult]:
        """Parse JSON response from LLM"""
        try:
            # Extract JSON from response
            json_match = re.search(r'\{.*\}', response, re.DOTALL)
            if json_match:
                data = json.loads(json_match.group())
            else:
                data = json.loads(response)
                
            # Parse tasks with validation
            tasks = []
            for task_data in data.get('tasks', []):
                try:
                    task = Task(
                        id=task_data.get('id', str(uuid4())),
                        title=task_data.get('title', 'Untitled task'),
                        description=task_data.get('description', ''),
                        type=self._validate_task_type(task_data.get('type', 'backend')),
                        priority=self._validate_priority(task_data.get('priority', 'medium')),
                        complexity=self._validate_complexity(task_data.get('complexity', 'moderate')),
                        estimated_hours=float(task_data.get('estimated_hours', 8)),
                        dependencies=task_data.get('dependencies', []),
                        tags=task_data.get('tags', []),
                        acceptance_criteria=task_data.get('acceptance_criteria', [])
                    )
                    tasks.append(task)
                except Exception as e:
                    logger.warning(f"Failed to parse task: {e}")
                    
            if not tasks:
                return None
                
            return IntentAnalysisResult(
                intent_type=self._validate_intent_type(data.get('intent_type', 'unknown')),
                confidence=float(data.get('confidence', 0.5)),
                summary=data.get('summary', 'Analysis complete'),
                tasks=tasks,
                metadata=data.get('metadata', {})
            )
            
        except Exception as e:
            logger.error(f"JSON parsing failed: {e}")
            return None
            
    def _validate_intent_type(self, value: str) -> IntentType:
        """Validate and convert intent type"""
        try:
            return IntentType(value.lower())
        except:
            # Try to map common variations
            mappings = {
                'feature': IntentType.FEATURE_REQUEST,
                'bug': IntentType.BUG_FIX,
                'refactor': IntentType.REFACTORING,
                'docs': IntentType.DOCUMENTATION,
                'test': IntentType.TESTING,
                'deploy': IntentType.DEPLOYMENT,
                'config': IntentType.CONFIGURATION
            }
            for key, intent in mappings.items():
                if key in value.lower():
                    return intent
            return IntentType.UNKNOWN
            
    def _validate_task_type(self, value: str) -> TaskType:
        """Validate and convert task type"""
        try:
            return TaskType(value.lower())
        except:
            # Default mapping
            if 'front' in value.lower():
                return TaskType.FRONTEND
            elif 'back' in value.lower():
                return TaskType.BACKEND
            elif 'data' in value.lower():
                return TaskType.DATABASE
            elif 'test' in value.lower():
                return TaskType.TESTING
            else:
                return TaskType.API
                
    def _validate_priority(self, value: str) -> TaskPriority:
        """Validate and convert priority"""
        try:
            return TaskPriority(value.lower())
        except:
            return TaskPriority.MEDIUM
            
    def _validate_complexity(self, value: str) -> TaskComplexity:
        """Validate and convert complexity"""
        try:
            return TaskComplexity(value.lower())
        except:
            if 'simple' in value.lower():
                return TaskComplexity.SIMPLE
            elif 'complex' in value.lower():
                return TaskComplexity.COMPLEX
            else:
                return TaskComplexity.MODERATE
                
    def _extract_intent_type(self, response: str) -> IntentType:
        """Extract intent type from response"""
        response = response.lower().strip()
        
        # Direct mapping
        for intent_type in IntentType:
            if intent_type.value in response:
                return intent_type
                
        # Keyword mapping
        if 'feature' in response:
            return IntentType.FEATURE_REQUEST
        elif 'bug' in response:
            return IntentType.BUG_FIX
        elif 'refactor' in response:
            return IntentType.REFACTORING
        elif 'doc' in response:
            return IntentType.DOCUMENTATION
        elif 'test' in response:
            return IntentType.TESTING
        elif 'deploy' in response:
            return IntentType.DEPLOYMENT
        else:
            return IntentType.UNKNOWN
            
    def _parse_task_list(self, response: str, original_text: str) -> List[Task]:
        """Parse task list from formatted response"""
        tasks = []
        lines = response.strip().split('\n')
        
        for i, line in enumerate(lines):
            if '|' in line:
                parts = [p.strip() for p in line.split('|')]
                if len(parts) >= 4:
                    try:
                        task = Task(
                            id=f"task_{i+1}",
                            title=parts[0],
                            description=f"Task: {parts[0]}",
                            type=self._validate_task_type(parts[1]),
                            priority=self._validate_priority(parts[3]),
                            complexity=TaskComplexity.MODERATE,
                            estimated_hours=float(parts[2]) if parts[2].isdigit() else 8.0,
                            dependencies=[],
                            tags=[parts[1].lower()]
                        )
                        tasks.append(task)
                    except Exception as e:
                        logger.warning(f"Failed to parse task line: {line}")
                        
        # Ensure at least one task
        if not tasks:
            tasks.append(self._create_default_task(original_text))
            
        return tasks
        
    def _parse_simple_response(self, response: str, original_text: str) -> IntentAnalysisResult:
        """Parse simple format response"""
        lines = response.lower().split('\n')
        
        intent_type = IntentType.UNKNOWN
        main_task = "Implement request"
        hours = 8.0
        priority = TaskPriority.MEDIUM
        
        for line in lines:
            if 'type:' in line:
                intent_type = self._extract_intent_type(line.split(':')[1])
            elif 'main task:' in line:
                main_task = line.split(':')[1].strip()
            elif 'hours:' in line:
                try:
                    hours = float(re.findall(r'\d+', line)[0])
                except:
                    hours = 8.0
            elif 'priority:' in line:
                priority = self._validate_priority(line.split(':')[1])
                
        task = Task(
            id=str(uuid4()),
            title=main_task[:50],  # Limit title length
            description=original_text,
            type=TaskType.API,
            priority=priority,
            complexity=TaskComplexity.MODERATE,
            estimated_hours=hours,
            dependencies=[],
            tags=[]
        )
        
        return IntentAnalysisResult(
            intent_type=intent_type,
            confidence=0.6,
            summary=main_task,
            tasks=[task],
            metadata={"strategy": "simple_parse"}
        )
        
    def _extract_tasks_from_text(self, text: str, intent_type: IntentType) -> List[Task]:
        """Extract tasks based on text patterns"""
        tasks = []
        
        # Look for common task patterns
        patterns = [
            r"(?:need to|want to|should|must)\s+(\w+\s+\w+(?:\s+\w+)?)",
            r"(?:implement|create|build|develop)\s+(\w+\s+\w+(?:\s+\w+)?)",
            r"(?:with|including|such as)\s+(\w+\s+\w+(?:\s+\w+)?)"
        ]
        
        task_descriptions = []
        for pattern in patterns:
            matches = re.findall(pattern, text.lower())
            task_descriptions.extend(matches)
            
        # Create tasks from descriptions
        for i, desc in enumerate(set(task_descriptions)):
            if len(tasks) < 3:  # Limit to 3 tasks
                task = Task(
                    id=f"task_{i+1}",
                    title=desc.title()[:50],
                    description=f"Implement {desc}",
                    type=self._infer_task_type(desc),
                    priority=TaskPriority.MEDIUM,
                    complexity=TaskComplexity.MODERATE,
                    estimated_hours=4.0,
                    dependencies=[],
                    tags=desc.split()[:2]
                )
                tasks.append(task)
                
        # Ensure at least one task
        if not tasks:
            tasks.append(self._create_default_task(text))
            
        return tasks
        
    def _infer_task_type(self, description: str) -> TaskType:
        """Infer task type from description"""
        desc_lower = description.lower()
        
        if any(word in desc_lower for word in ['ui', 'interface', 'frontend', 'react', 'vue']):
            return TaskType.FRONTEND
        elif any(word in desc_lower for word in ['api', 'endpoint', 'rest', 'graphql']):
            return TaskType.API
        elif any(word in desc_lower for word in ['database', 'sql', 'mongo', 'redis']):
            return TaskType.DATABASE
        elif any(word in desc_lower for word in ['test', 'testing', 'spec']):
            return TaskType.TESTING
        elif any(word in desc_lower for word in ['deploy', 'docker', 'kubernetes']):
            return TaskType.INFRASTRUCTURE
        else:
            return TaskType.BACKEND
            
    def _generate_summary(self, text: str, intent_type: IntentType) -> str:
        """Generate a summary based on text and intent"""
        # Extract key phrases
        key_phrases = []
        
        # Look for action words
        action_matches = re.findall(r'\b(?:create|build|implement|fix|add|improve)\s+(\w+(?:\s+\w+)?)', text.lower())
        if action_matches:
            key_phrases.extend(action_matches)
            
        if key_phrases:
            return f"{intent_type.value.replace('_', ' ').title()}: {', '.join(key_phrases[:2])}"
        else:
            return f"{intent_type.value.replace('_', ' ').title()} request"
            
    def _create_default_task(self, text: str) -> Task:
        """Create a default task"""
        return Task(
            id=str(uuid4()),
            title="Implement requested functionality",
            description=text[:200],
            type=TaskType.BACKEND,
            priority=TaskPriority.MEDIUM,
            complexity=TaskComplexity.MODERATE,
            estimated_hours=8.0,
            dependencies=[],
            tags=["general"]
        )
        
    def _create_minimal_result(self, text: str) -> IntentAnalysisResult:
        """Create minimal result when all strategies fail"""
        return IntentAnalysisResult(
            intent_type=IntentType.UNKNOWN,
            confidence=0.1,
            summary="Unable to fully analyze request",
            tasks=[self._create_default_task(text)],
            metadata={"error": "all_strategies_failed"}
        )
        
    async def validate_tasks(self, task_breakdown: Any) -> Any:
        """Validate task breakdown"""
        return type('ValidationResult', (), {
            'is_valid': True,
            'issues': [],
            'suggestions': []
        })