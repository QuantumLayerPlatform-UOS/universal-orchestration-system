"""
Mock Intent Analyzer Service
Provides mock responses for testing without Azure OpenAI
"""

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

logger = logging.getLogger(__name__)


class MockIntentAnalyzer:
    """Mock service for analyzing natural language requirements"""
    
    async def initialize(self):
        """Initialize the mock analyzer"""
        logger.info("Mock Intent Analyzer initialized")
    
    async def cleanup(self):
        """Cleanup resources"""
        pass
    
    async def check_openai_health(self) -> bool:
        """Mock health check - always returns True"""
        return True
    
    async def analyze_intent(
        self,
        text: str,
        context: Optional[Dict[str, Any]] = None,
        project_info: Optional[Dict[str, Any]] = None
    ) -> IntentAnalysisResult:
        """
        Analyze user intent from natural language text
        Returns mock data based on keywords in the text
        """
        # Determine intent type based on keywords
        intent_type = self._determine_intent_type(text)
        
        # Generate mock tasks based on intent
        tasks = self._generate_mock_tasks(text, intent_type)
        
        # Create summary
        summary = f"Mock analysis: Detected {intent_type.value} request with {len(tasks)} tasks"
        
        return IntentAnalysisResult(
            intent_type=intent_type,
            confidence=0.95,
            summary=summary,
            tasks=tasks,
            metadata={
                "original_text": text,
                "mock_analyzer": True,
                "processing_timestamp": datetime.utcnow().isoformat()
            }
        )
    
    def _determine_intent_type(self, text: str) -> IntentType:
        """Determine intent type based on keywords"""
        text_lower = text.lower()
        
        if any(word in text_lower for word in ["create", "build", "develop", "generate"]):
            return IntentType.CREATE_FEATURE
        elif any(word in text_lower for word in ["fix", "bug", "error", "issue"]):
            return IntentType.FIX_BUG
        elif any(word in text_lower for word in ["improve", "optimize", "enhance"]):
            return IntentType.IMPROVE_PERFORMANCE
        elif any(word in text_lower for word in ["refactor", "restructure", "reorganize"]):
            return IntentType.REFACTOR_CODE
        elif any(word in text_lower for word in ["document", "docs", "readme"]):
            return IntentType.ADD_DOCUMENTATION
        elif any(word in text_lower for word in ["test", "testing", "unit test"]):
            return IntentType.ADD_TESTS
        elif any(word in text_lower for word in ["deploy", "deployment", "release"]):
            return IntentType.DEPLOY
        else:
            return IntentType.UNKNOWN
    
    def _generate_mock_tasks(self, text: str, intent_type: IntentType) -> List[Task]:
        """Generate mock tasks based on intent type"""
        tasks = []
        
        if intent_type == IntentType.CREATE_FEATURE:
            # For API creation requests
            if "api" in text.lower() or "rest" in text.lower():
                tasks = [
                    Task(
                        id=str(uuid4()),
                        name="Design API endpoints",
                        description="Design RESTful API endpoints based on requirements",
                        type=TaskType.DESIGN,
                        priority=TaskPriority.HIGH,
                        complexity=TaskComplexity.MEDIUM,
                        estimated_effort=120,
                        required_skills=["API Design", "REST"],
                        dependencies=[],
                        metadata={"agent_type": "design"}
                    ),
                    Task(
                        id=str(uuid4()),
                        name="Implement API routes",
                        description="Implement the API routes and controllers",
                        type=TaskType.IMPLEMENTATION,
                        priority=TaskPriority.HIGH,
                        complexity=TaskComplexity.HIGH,
                        estimated_effort=240,
                        required_skills=["Node.js", "Express"],
                        dependencies=[],
                        metadata={"agent_type": "code-gen"}
                    ),
                    Task(
                        id=str(uuid4()),
                        name="Add validation middleware",
                        description="Implement request validation middleware",
                        type=TaskType.IMPLEMENTATION,
                        priority=TaskPriority.MEDIUM,
                        complexity=TaskComplexity.MEDIUM,
                        estimated_effort=90,
                        required_skills=["Validation", "Middleware"],
                        dependencies=[],
                        metadata={"agent_type": "code-gen"}
                    ),
                    Task(
                        id=str(uuid4()),
                        name="Write API tests",
                        description="Write unit and integration tests for the API",
                        type=TaskType.TESTING,
                        priority=TaskPriority.MEDIUM,
                        complexity=TaskComplexity.MEDIUM,
                        estimated_effort=180,
                        required_skills=["Testing", "Jest"],
                        dependencies=[],
                        metadata={"agent_type": "test-gen"}
                    )
                ]
            else:
                # Generic feature creation
                tasks = [
                    Task(
                        id=str(uuid4()),
                        name="Analyze requirements",
                        description="Analyze and break down the feature requirements",
                        type=TaskType.ANALYSIS,
                        priority=TaskPriority.HIGH,
                        complexity=TaskComplexity.LOW,
                        estimated_effort=60,
                        required_skills=["Analysis"],
                        dependencies=[],
                        metadata={"agent_type": "analysis"}
                    ),
                    Task(
                        id=str(uuid4()),
                        name="Implement feature",
                        description="Implement the requested feature",
                        type=TaskType.IMPLEMENTATION,
                        priority=TaskPriority.HIGH,
                        complexity=TaskComplexity.MEDIUM,
                        estimated_effort=180,
                        required_skills=["Programming"],
                        dependencies=[],
                        metadata={"agent_type": "code-gen"}
                    )
                ]
        
        # Set dependencies (second task depends on first, etc.)
        for i in range(1, len(tasks)):
            tasks[i].dependencies = [tasks[i-1].id]
        
        return tasks