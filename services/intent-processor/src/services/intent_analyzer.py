"""
Intent Analyzer Service
Core logic for analyzing user intent and breaking down requirements
"""

import json
import logging
import os
import re
from datetime import datetime
from typing import Dict, Any, List, Optional
from uuid import uuid4

from langchain.schema import HumanMessage, SystemMessage
from langchain_openai import AzureChatOpenAI
from openai import AzureOpenAI

from ..models import (
    IntentType,
    TaskType,
    TaskPriority,
    TaskComplexity,
    Task,
    TaskBreakdown,
    IntentAnalysisResult,
    ValidationResult
)
from .prompt_manager import PromptManager
from .llm_provider_factory import LLMProviderFactory

logger = logging.getLogger(__name__)


class IntentAnalyzer:
    """Service for analyzing natural language requirements and generating tasks"""
    
    def __init__(self, prompt_manager: PromptManager):
        self.prompt_manager = prompt_manager
        self.llm_factory = LLMProviderFactory()
        self.llm = None
        self.client: Optional[AzureOpenAI] = None
        
    async def initialize(self):
        """Initialize the LLM client using the provider factory"""
        try:
            # Get provider from environment or use default
            provider = os.getenv('LLM_PROVIDER')
            model = os.getenv('LLM_MODEL')
            
            logger.info(f"Initializing LLM with provider: {provider or self.llm_factory.default_provider}")
            
            # Create LLM instance using the factory
            self.llm = self.llm_factory.create_llm(
                provider=provider,
                model=model,
                temperature=float(os.getenv('LLM_TEMPERATURE', '0.7')),
                max_tokens=int(os.getenv('LLM_MAX_TOKENS', '2000'))
            )
            
            # Initialize direct Azure OpenAI client for health checks
            self.client = AzureOpenAI(
                azure_endpoint=os.getenv("AZURE_OPENAI_ENDPOINT"),
                api_key=os.getenv("AZURE_OPENAI_API_KEY"),
                api_version=os.getenv("AZURE_OPENAI_API_VERSION", "2024-02-15-preview")
            )
            
            logger.info("Azure OpenAI clients initialized successfully")
        except Exception as e:
            logger.error(f"Failed to initialize Azure OpenAI: {str(e)}")
            raise
    
    async def cleanup(self):
        """Cleanup resources"""
        if self.client:
            await self.client.close()
    
    async def check_openai_health(self) -> bool:
        """Check if Azure OpenAI is accessible"""
        try:
            # Try a simple completion
            response = self.client.chat.completions.create(
                model=os.getenv("AZURE_OPENAI_DEPLOYMENT_NAME"),
                messages=[{"role": "user", "content": "test"}],
                max_tokens=5
            )
            return bool(response.choices)
        except Exception as e:
            logger.error(f"Azure OpenAI health check failed: {str(e)}")
            return False
    
    async def analyze_intent(
        self,
        text: str,
        context: Optional[Dict[str, Any]] = None,
        project_info: Optional[Dict[str, Any]] = None
    ) -> IntentAnalysisResult:
        """
        Analyze user intent from natural language text
        
        Args:
            text: Natural language requirement
            context: Additional context
            project_info: Project information
            
        Returns:
            IntentAnalysisResult with classified intent and tasks
        """
        try:
            # Step 1: Classify the intent
            intent_type, confidence = await self._classify_intent(text)
            
            # Step 2: Extract key information
            extracted_info = await self._extract_information(text, intent_type, context)
            
            # Step 3: Generate task breakdown
            tasks = await self._generate_tasks(
                text, 
                intent_type, 
                extracted_info, 
                project_info
            )
            
            # Step 4: Optimize task dependencies and order
            optimized_tasks = await self._optimize_task_order(tasks)
            
            # Step 5: Generate summary
            summary = await self._generate_summary(text, intent_type, optimized_tasks)
            
            return IntentAnalysisResult(
                intent_type=intent_type,
                confidence=confidence,
                summary=summary,
                tasks=optimized_tasks,
                metadata={
                    "original_text": text,
                    "extracted_info": extracted_info,
                    "processing_timestamp": datetime.utcnow().isoformat()
                }
            )
            
        except Exception as e:
            logger.error(f"Failed to analyze intent: {str(e)}")
            raise
    
    async def _classify_intent(self, text: str) -> tuple[IntentType, float]:
        """Classify the type of intent from the text"""
        try:
            prompt = self.prompt_manager.get_intent_classification_prompt(text)
            
            messages = [
                SystemMessage(content=prompt["system"]),
                HumanMessage(content=prompt["user"])
            ]
            
            response = await self.llm.ainvoke(messages)
            result = self._parse_json_response(response.content)
            
            intent_type = IntentType(result.get("intent_type", "unknown"))
            confidence = float(result.get("confidence", 0.5))
            
            logger.info(f"Classified intent as {intent_type} with confidence {confidence}")
            return intent_type, confidence
            
        except Exception as e:
            logger.error(f"Intent classification failed: {str(e)}")
            return IntentType.UNKNOWN, 0.0
    
    async def _extract_information(
        self, 
        text: str, 
        intent_type: IntentType,
        context: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """Extract key information from the requirement text"""
        try:
            prompt = self.prompt_manager.get_information_extraction_prompt(
                text, intent_type, context
            )
            
            messages = [
                SystemMessage(content=prompt["system"]),
                HumanMessage(content=prompt["user"])
            ]
            
            response = await self.llm.ainvoke(messages)
            extracted = self._parse_json_response(response.content)
            
            return extracted
            
        except Exception as e:
            logger.error(f"Information extraction failed: {str(e)}")
            return {}
    
    async def _generate_tasks(
        self,
        text: str,
        intent_type: IntentType,
        extracted_info: Dict[str, Any],
        project_info: Optional[Dict[str, Any]] = None
    ) -> List[Task]:
        """Generate task breakdown from requirements"""
        try:
            prompt = self.prompt_manager.get_task_generation_prompt(
                text, intent_type, extracted_info, project_info
            )
            
            messages = [
                SystemMessage(content=prompt["system"]),
                HumanMessage(content=prompt["user"])
            ]
            
            response = await self.llm.ainvoke(messages)
            tasks_data = self._parse_json_response(response.content)
            
            # Convert to Task objects
            tasks = []
            for task_data in tasks_data.get("tasks", []):
                task = Task(
                    id=f"task_{uuid4().hex[:8]}",
                    title=task_data["title"],
                    description=task_data["description"],
                    type=TaskType(task_data.get("type", "backend")),
                    priority=TaskPriority(task_data.get("priority", "medium")),
                    complexity=TaskComplexity(task_data.get("complexity", "moderate")),
                    estimated_hours=task_data.get("estimated_hours"),
                    dependencies=task_data.get("dependencies", []),
                    tags=task_data.get("tags", []),
                    acceptance_criteria=task_data.get("acceptance_criteria", []),
                    technical_requirements=task_data.get("technical_requirements", {})
                )
                tasks.append(task)
            
            return tasks
            
        except Exception as e:
            logger.error(f"Task generation failed: {str(e)}")
            raise
    
    async def _optimize_task_order(self, tasks: List[Task]) -> List[Task]:
        """Optimize task order based on dependencies"""
        try:
            # Create dependency graph
            task_map = {task.id: task for task in tasks}
            
            # Topological sort for dependency ordering
            visited = set()
            result = []
            
            def visit(task_id: str):
                if task_id in visited:
                    return
                visited.add(task_id)
                
                task = task_map.get(task_id)
                if task:
                    for dep_id in task.dependencies:
                        if dep_id in task_map:
                            visit(dep_id)
                    result.append(task)
            
            # Visit all tasks
            for task in tasks:
                visit(task.id)
            
            return result
            
        except Exception as e:
            logger.error(f"Task optimization failed: {str(e)}")
            return tasks
    
    async def _generate_summary(
        self, 
        text: str, 
        intent_type: IntentType,
        tasks: List[Task]
    ) -> str:
        """Generate a concise summary of the requirement"""
        try:
            prompt = self.prompt_manager.get_summary_prompt(text, intent_type, tasks)
            
            messages = [
                SystemMessage(content=prompt["system"]),
                HumanMessage(content=prompt["user"])
            ]
            
            response = await self.llm.ainvoke(messages)
            return response.content.strip()
            
        except Exception as e:
            logger.error(f"Summary generation failed: {str(e)}")
            return "Failed to generate summary"
    
    async def validate_tasks(self, task_breakdown: TaskBreakdown) -> ValidationResult:
        """Validate a task breakdown for completeness and consistency"""
        result = ValidationResult(is_valid=True)
        
        try:
            # Check for circular dependencies
            if self._has_circular_dependencies(task_breakdown.tasks):
                result.add_issue("Circular dependencies detected in task breakdown")
            
            # Check for orphaned dependencies
            task_ids = {task.id for task in task_breakdown.tasks}
            for task in task_breakdown.tasks:
                for dep_id in task.dependencies:
                    if dep_id not in task_ids:
                        result.add_issue(f"Task {task.id} depends on non-existent task {dep_id}")
            
            # Check for reasonable estimates
            for task in task_breakdown.tasks:
                if task.estimated_hours and task.estimated_hours > 40:
                    result.add_suggestion(
                        f"Task '{task.title}' has high estimate ({task.estimated_hours}h). "
                        "Consider breaking it down further."
                    )
            
            # Check for missing acceptance criteria
            for task in task_breakdown.tasks:
                if not task.acceptance_criteria:
                    result.add_suggestion(
                        f"Task '{task.title}' lacks acceptance criteria"
                    )
            
            # Validate task types consistency
            if self._has_inconsistent_types(task_breakdown.tasks):
                result.add_suggestion(
                    "Consider grouping related tasks by type for better organization"
                )
            
            return result
            
        except Exception as e:
            logger.error(f"Task validation failed: {str(e)}")
            result.add_issue(f"Validation error: {str(e)}")
            return result
    
    def _parse_json_response(self, content: str) -> Dict[str, Any]:
        """Parse JSON from LLM response"""
        try:
            # Extract JSON from potential markdown code blocks
            json_match = re.search(r'```json\s*(.*?)\s*```', content, re.DOTALL)
            if json_match:
                content = json_match.group(1)
            
            return json.loads(content)
        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse JSON response: {e}")
            logger.debug(f"Response content: {content}")
            raise ValueError("Invalid JSON response from LLM")
    
    def _has_circular_dependencies(self, tasks: List[Task]) -> bool:
        """Check for circular dependencies in tasks"""
        task_map = {task.id: task for task in tasks}
        visited = set()
        rec_stack = set()
        
        def has_cycle(task_id: str) -> bool:
            visited.add(task_id)
            rec_stack.add(task_id)
            
            task = task_map.get(task_id)
            if task:
                for dep_id in task.dependencies:
                    if dep_id not in visited:
                        if has_cycle(dep_id):
                            return True
                    elif dep_id in rec_stack:
                        return True
            
            rec_stack.remove(task_id)
            return False
        
        for task in tasks:
            if task.id not in visited:
                if has_cycle(task.id):
                    return True
        
        return False
    
    def _has_inconsistent_types(self, tasks: List[Task]) -> bool:
        """Check if task types are inconsistently distributed"""
        type_counts = {}
        for task in tasks:
            type_counts[task.type] = type_counts.get(task.type, 0) + 1
        
        # If we have many types with only one task each, suggest grouping
        single_task_types = sum(1 for count in type_counts.values() if count == 1)
        return single_task_types > len(type_counts) * 0.5