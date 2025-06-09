"""
Data models for Intent Processor Service
"""

from datetime import datetime
from enum import Enum
from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field, validator


class IntentType(str, Enum):
    """Types of intents that can be classified"""
    FEATURE_REQUEST = "feature_request"
    BUG_FIX = "bug_fix"
    REFACTORING = "refactoring"
    DOCUMENTATION = "documentation"
    TESTING = "testing"
    DEPLOYMENT = "deployment"
    CONFIGURATION = "configuration"
    RESEARCH = "research"
    UNKNOWN = "unknown"


class TaskType(str, Enum):
    """Types of tasks that can be generated"""
    FRONTEND = "frontend"
    BACKEND = "backend"
    DATABASE = "database"
    API = "api"
    INFRASTRUCTURE = "infrastructure"
    TESTING = "testing"
    DOCUMENTATION = "documentation"
    DESIGN = "design"
    DEVOPS = "devops"
    SECURITY = "security"


class TaskPriority(str, Enum):
    """Task priority levels"""
    CRITICAL = "critical"
    HIGH = "high"
    MEDIUM = "medium"
    LOW = "low"


class TaskComplexity(str, Enum):
    """Task complexity levels"""
    SIMPLE = "simple"
    MODERATE = "moderate"
    COMPLEX = "complex"
    VERY_COMPLEX = "very_complex"


class Task(BaseModel):
    """Individual task representation"""
    id: str = Field(..., description="Unique task identifier")
    title: str = Field(..., description="Task title")
    description: str = Field(..., description="Detailed task description")
    type: TaskType = Field(..., description="Type of task")
    priority: TaskPriority = Field(TaskPriority.MEDIUM, description="Task priority")
    complexity: TaskComplexity = Field(TaskComplexity.MODERATE, description="Task complexity")
    estimated_hours: Optional[float] = Field(None, description="Estimated hours to complete")
    dependencies: List[str] = Field(default_factory=list, description="List of task IDs this depends on")
    tags: List[str] = Field(default_factory=list, description="Task tags")
    acceptance_criteria: List[str] = Field(default_factory=list, description="Acceptance criteria")
    technical_requirements: Optional[Dict[str, Any]] = Field(None, description="Technical requirements")
    
    @validator('estimated_hours')
    def validate_estimated_hours(cls, v):
        if v is not None and v <= 0:
            raise ValueError("Estimated hours must be positive")
        return v


class TaskBreakdown(BaseModel):
    """Complete task breakdown from an intent"""
    tasks: List[Task] = Field(..., description="List of tasks")
    total_estimated_hours: Optional[float] = Field(None, description="Total estimated hours")
    suggested_order: List[str] = Field(default_factory=list, description="Suggested task execution order")
    milestones: List[Dict[str, Any]] = Field(default_factory=list, description="Project milestones")
    
    @validator('tasks')
    def validate_tasks(cls, v):
        if not v:
            raise ValueError("At least one task is required")
        return v
    
    def calculate_total_hours(self) -> float:
        """Calculate total estimated hours from all tasks"""
        return sum(task.estimated_hours or 0 for task in self.tasks)


class IntentRequest(BaseModel):
    """Request model for intent processing"""
    text: str = Field(..., description="Natural language requirement text", min_length=10, max_length=5000)
    context: Optional[Dict[str, Any]] = Field(None, description="Additional context for processing")
    project_info: Optional[Dict[str, Any]] = Field(None, description="Project information")
    request_id: str = Field(..., description="Unique request identifier")
    user_id: Optional[str] = Field(None, description="User identifier")
    
    @validator('text')
    def validate_text(cls, v):
        if not v.strip():
            raise ValueError("Text cannot be empty")
        return v.strip()


class IntentAnalysisResult(BaseModel):
    """Result of intent analysis"""
    intent_type: IntentType = Field(..., description="Classified intent type")
    confidence: float = Field(..., description="Confidence score (0-1)")
    summary: str = Field(..., description="Summary of the requirement")
    tasks: List[Task] = Field(..., description="Breakdown of tasks")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Additional metadata")
    
    @validator('confidence')
    def validate_confidence(cls, v):
        if not 0 <= v <= 1:
            raise ValueError("Confidence must be between 0 and 1")
        return v


class IntentResponse(BaseModel):
    """Response model for intent processing"""
    request_id: str = Field(..., description="Request identifier")
    intent_type: IntentType = Field(..., description="Classified intent type")
    confidence: float = Field(..., description="Confidence score")
    summary: str = Field(..., description="Requirement summary")
    tasks: List[Task] = Field(..., description="Task breakdown")
    metadata: Optional[Dict[str, Any]] = Field(None, description="Additional metadata")
    timestamp: datetime = Field(..., description="Processing timestamp")
    processing_time_ms: Optional[int] = Field(None, description="Processing time in milliseconds")


class HealthResponse(BaseModel):
    """Health check response"""
    status: str = Field(..., description="Service health status")
    timestamp: datetime = Field(..., description="Check timestamp")
    service: str = Field(..., description="Service name")
    version: str = Field(..., description="Service version")
    dependencies: Optional[Dict[str, Any]] = Field(None, description="Dependency health status")
    error: Optional[str] = Field(None, description="Error message if unhealthy")


class ErrorResponse(BaseModel):
    """Error response model"""
    error: str = Field(..., description="Error type")
    message: str = Field(..., description="Error message")
    details: Optional[Dict[str, Any]] = Field(None, description="Additional error details")
    request_id: Optional[str] = Field(None, description="Request identifier")
    timestamp: datetime = Field(default_factory=datetime.utcnow, description="Error timestamp")


class ValidationResult(BaseModel):
    """Task validation result"""
    is_valid: bool = Field(..., description="Whether the task breakdown is valid")
    issues: List[str] = Field(default_factory=list, description="List of validation issues")
    suggestions: List[str] = Field(default_factory=list, description="Improvement suggestions")
    
    def add_issue(self, issue: str):
        """Add a validation issue"""
        self.issues.append(issue)
        self.is_valid = False
    
    def add_suggestion(self, suggestion: str):
        """Add an improvement suggestion"""
        self.suggestions.append(suggestion)