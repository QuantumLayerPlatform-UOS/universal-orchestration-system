"""
Tests for Intent Processor Service
"""

import pytest
from datetime import datetime
from unittest.mock import Mock, AsyncMock, patch

from src.models import (
    IntentType,
    TaskType,
    TaskPriority,
    TaskComplexity,
    Task,
    TaskBreakdown,
    IntentRequest,
    IntentResponse,
    ValidationResult
)
from src.services.intent_analyzer import IntentAnalyzer
from src.services.prompt_manager import PromptManager


class TestPromptManager:
    """Test cases for PromptManager"""
    
    def test_initialization(self):
        """Test prompt manager initialization"""
        manager = PromptManager()
        assert len(manager.templates) > 0
        assert "intent_classification" in manager.templates
        assert "task_generation" in manager.templates
    
    def test_get_intent_classification_prompt(self):
        """Test intent classification prompt generation"""
        manager = PromptManager()
        prompt = manager.get_intent_classification_prompt("Create a user login feature")
        
        assert "system" in prompt
        assert "user" in prompt
        assert "Create a user login feature" in prompt["user"]
        assert "intent_type" in prompt["user"]
    
    def test_get_information_extraction_prompt(self):
        """Test information extraction prompt"""
        manager = PromptManager()
        prompt = manager.get_information_extraction_prompt(
            "Add dark mode",
            IntentType.FEATURE_REQUEST,
            {"project": "web-app"}
        )
        
        assert "system" in prompt
        assert "user" in prompt
        assert "Add dark mode" in prompt["user"]
        assert "web-app" in prompt["user"]
    
    def test_add_custom_template(self):
        """Test adding custom template"""
        manager = PromptManager()
        manager.add_template("custom", "This is a {variable} template")
        
        assert "custom" in manager.get_available_templates()
        assert manager.get_template("custom") == "This is a {variable} template"
    
    def test_few_shot_prompt(self):
        """Test few-shot prompt creation"""
        manager = PromptManager()
        examples = [
            {"input": "Login feature", "output": "feature_request"},
            {"input": "Fix crash", "output": "bug_fix"}
        ]
        
        prompt = manager.create_few_shot_prompt(
            "intent_classification",
            examples,
            "Add payment system"
        )
        
        assert "Example 1:" in prompt["user"]
        assert "Login feature" in prompt["user"]
        assert "Add payment system" in prompt["user"]


@pytest.mark.asyncio
class TestIntentAnalyzer:
    """Test cases for IntentAnalyzer"""
    
    async def test_initialization(self):
        """Test intent analyzer initialization"""
        prompt_manager = PromptManager()
        analyzer = IntentAnalyzer(prompt_manager)
        
        assert analyzer.prompt_manager == prompt_manager
        assert analyzer.llm is None
        assert analyzer.client is None
    
    @patch("src.services.intent_analyzer.AzureChatOpenAI")
    @patch("src.services.intent_analyzer.AzureOpenAI")
    async def test_initialize_with_azure(self, mock_azure, mock_langchain):
        """Test initialization with Azure OpenAI"""
        prompt_manager = PromptManager()
        analyzer = IntentAnalyzer(prompt_manager)
        
        with patch.dict("os.environ", {
            "AZURE_OPENAI_ENDPOINT": "https://test.openai.azure.com",
            "AZURE_OPENAI_API_KEY": "test-key",
            "AZURE_OPENAI_DEPLOYMENT_NAME": "test-deployment"
        }):
            await analyzer.initialize()
            
            assert mock_langchain.called
            assert mock_azure.called
    
    async def test_parse_json_response(self):
        """Test JSON response parsing"""
        prompt_manager = PromptManager()
        analyzer = IntentAnalyzer(prompt_manager)
        
        # Test with markdown code block
        response = '''```json
        {"intent_type": "feature_request", "confidence": 0.9}
        ```'''
        result = analyzer._parse_json_response(response)
        assert result["intent_type"] == "feature_request"
        assert result["confidence"] == 0.9
        
        # Test with plain JSON
        response = '{"test": "value"}'
        result = analyzer._parse_json_response(response)
        assert result["test"] == "value"
    
    async def test_has_circular_dependencies(self):
        """Test circular dependency detection"""
        prompt_manager = PromptManager()
        analyzer = IntentAnalyzer(prompt_manager)
        
        # Create tasks with circular dependency
        task1 = Task(
            id="task1",
            title="Task 1",
            description="Description",
            type=TaskType.BACKEND,
            dependencies=["task2"]
        )
        task2 = Task(
            id="task2",
            title="Task 2",
            description="Description",
            type=TaskType.BACKEND,
            dependencies=["task1"]
        )
        
        assert analyzer._has_circular_dependencies([task1, task2]) is True
        
        # Test without circular dependency
        task3 = Task(
            id="task3",
            title="Task 3",
            description="Description",
            type=TaskType.BACKEND,
            dependencies=[]
        )
        
        assert analyzer._has_circular_dependencies([task1, task3]) is False
    
    async def test_validate_tasks(self):
        """Test task validation"""
        prompt_manager = PromptManager()
        analyzer = IntentAnalyzer(prompt_manager)
        
        # Create valid task breakdown
        task1 = Task(
            id="task1",
            title="Create API",
            description="Create REST API",
            type=TaskType.BACKEND,
            estimated_hours=8,
            acceptance_criteria=["API responds to requests"]
        )
        
        task2 = Task(
            id="task2",
            title="Create UI",
            description="Create user interface",
            type=TaskType.FRONTEND,
            dependencies=["task1"],
            estimated_hours=50,  # High estimate
            acceptance_criteria=[]  # Missing criteria
        )
        
        breakdown = TaskBreakdown(tasks=[task1, task2])
        result = await analyzer.validate_tasks(breakdown)
        
        # Should have suggestions for high estimate and missing criteria
        assert len(result.suggestions) >= 2
        assert any("high estimate" in s for s in result.suggestions)
        assert any("acceptance criteria" in s for s in result.suggestions)


class TestModels:
    """Test cases for data models"""
    
    def test_task_creation(self):
        """Test Task model creation"""
        task = Task(
            id="test-id",
            title="Test Task",
            description="Test Description",
            type=TaskType.BACKEND,
            priority=TaskPriority.HIGH,
            complexity=TaskComplexity.COMPLEX,
            estimated_hours=10
        )
        
        assert task.id == "test-id"
        assert task.title == "Test Task"
        assert task.type == TaskType.BACKEND
        assert task.priority == TaskPriority.HIGH
        assert task.complexity == TaskComplexity.COMPLEX
        assert task.estimated_hours == 10
    
    def test_task_validation(self):
        """Test Task model validation"""
        # Test negative hours validation
        with pytest.raises(ValueError):
            Task(
                id="test",
                title="Test",
                description="Test",
                type=TaskType.BACKEND,
                estimated_hours=-5
            )
    
    def test_intent_request_validation(self):
        """Test IntentRequest validation"""
        # Test empty text
        with pytest.raises(ValueError):
            IntentRequest(
                text="   ",
                request_id="test-id"
            )
        
        # Test text too short
        with pytest.raises(ValueError):
            IntentRequest(
                text="short",
                request_id="test-id"
            )
        
        # Valid request
        request = IntentRequest(
            text="Create a user authentication system with JWT tokens",
            request_id="test-id"
        )
        assert request.text == "Create a user authentication system with JWT tokens"
    
    def test_task_breakdown_total_hours(self):
        """Test TaskBreakdown total hours calculation"""
        tasks = [
            Task(
                id=f"task{i}",
                title=f"Task {i}",
                description="Description",
                type=TaskType.BACKEND,
                estimated_hours=i * 5
            )
            for i in range(1, 4)
        ]
        
        breakdown = TaskBreakdown(tasks=tasks)
        assert breakdown.calculate_total_hours() == 30  # 5 + 10 + 15
    
    def test_validation_result(self):
        """Test ValidationResult model"""
        result = ValidationResult(is_valid=True)
        
        assert result.is_valid is True
        assert len(result.issues) == 0
        
        # Add issue
        result.add_issue("Test issue")
        assert result.is_valid is False
        assert len(result.issues) == 1
        assert "Test issue" in result.issues
        
        # Add suggestion
        result.add_suggestion("Test suggestion")
        assert len(result.suggestions) == 1
        assert "Test suggestion" in result.suggestions


@pytest.mark.asyncio
class TestIntegration:
    """Integration tests"""
    
    @patch("src.services.intent_analyzer.AzureChatOpenAI")
    async def test_analyze_intent_mock(self, mock_llm):
        """Test full intent analysis flow with mocked LLM"""
        # Setup mock responses
        mock_response = Mock()
        mock_response.content = '''```json
        {
            "intent_type": "feature_request",
            "confidence": 0.95,
            "reasoning": "User wants to add new functionality"
        }
        ```'''
        
        mock_llm_instance = AsyncMock()
        mock_llm_instance.ainvoke = AsyncMock(return_value=mock_response)
        mock_llm.return_value = mock_llm_instance
        
        # Create analyzer
        prompt_manager = PromptManager()
        analyzer = IntentAnalyzer(prompt_manager)
        analyzer.llm = mock_llm_instance
        
        # Mock other methods
        analyzer._extract_information = AsyncMock(return_value={
            "main_objective": "Add login feature",
            "technologies": ["JWT", "OAuth"]
        })
        
        analyzer._generate_tasks = AsyncMock(return_value=[
            Task(
                id="task1",
                title="Create auth API",
                description="Create authentication API",
                type=TaskType.BACKEND
            )
        ])
        
        analyzer._optimize_task_order = AsyncMock(side_effect=lambda x: x)
        analyzer._generate_summary = AsyncMock(
            return_value="Add user authentication with JWT"
        )
        
        # Analyze intent
        result = await analyzer.analyze_intent(
            "Create a user login system with JWT tokens",
            {"project": "web-app"}
        )
        
        assert result.intent_type == IntentType.FEATURE_REQUEST
        assert result.confidence == 0.95
        assert len(result.tasks) == 1
        assert result.summary == "Add user authentication with JWT"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])