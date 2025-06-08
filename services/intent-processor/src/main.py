"""
Intent Processor Service - Main Application
Processes natural language requirements and breaks them down into actionable tasks
"""

import logging
import os
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Dict, Any

from fastapi import FastAPI, HTTPException, Request, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from prometheus_client import Counter, Histogram, generate_latest
from prometheus_client.core import CollectorRegistry
from pythonjsonlogger import jsonlogger

from .models import (
    IntentRequest,
    IntentResponse,
    HealthResponse,
    ErrorResponse,
    TaskBreakdown
)
from .services.intent_analyzer import IntentAnalyzer
from .services.prompt_manager import PromptManager

# Configure structured logging
logHandler = logging.StreamHandler()
formatter = jsonlogger.JsonFormatter()
logHandler.setFormatter(formatter)
logger = logging.getLogger(__name__)
logger.addHandler(logHandler)
logger.setLevel(logging.INFO)

# Prometheus metrics
registry = CollectorRegistry()
request_counter = Counter(
    'intent_processor_requests_total',
    'Total number of requests',
    ['method', 'endpoint', 'status'],
    registry=registry
)
request_duration = Histogram(
    'intent_processor_request_duration_seconds',
    'Request duration in seconds',
    ['method', 'endpoint'],
    registry=registry
)
intent_processing_counter = Counter(
    'intent_processing_total',
    'Total number of intents processed',
    ['status'],
    registry=registry
)

# Initialize services
intent_analyzer: IntentAnalyzer = None
prompt_manager: PromptManager = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle"""
    global intent_analyzer, prompt_manager
    
    logger.info("Starting Intent Processor Service")
    
    # Initialize services
    try:
        prompt_manager = PromptManager()
        intent_analyzer = IntentAnalyzer(prompt_manager)
        await intent_analyzer.initialize()
        logger.info("Services initialized successfully")
    except Exception as e:
        logger.error(f"Failed to initialize services: {str(e)}")
        raise
    
    yield
    
    # Cleanup
    logger.info("Shutting down Intent Processor Service")
    if intent_analyzer:
        await intent_analyzer.cleanup()


# Create FastAPI application
app = FastAPI(
    title="Intent Processor Service",
    description="Processes natural language requirements for QuantumLayer Platform",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=os.getenv("CORS_ORIGINS", "*").split(","),
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.middleware("http")
async def add_metrics(request: Request, call_next):
    """Add prometheus metrics to all requests"""
    start_time = datetime.now()
    
    # Process request
    response = await call_next(request)
    
    # Record metrics
    duration = (datetime.now() - start_time).total_seconds()
    request_counter.labels(
        method=request.method,
        endpoint=request.url.path,
        status=response.status_code
    ).inc()
    request_duration.labels(
        method=request.method,
        endpoint=request.url.path
    ).observe(duration)
    
    return response


@app.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    """Global exception handler"""
    logger.error(
        "Unhandled exception",
        extra={
            "path": request.url.path,
            "method": request.method,
            "error": str(exc),
            "type": type(exc).__name__
        }
    )
    return JSONResponse(
        status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
        content={
            "error": "Internal server error",
            "message": "An unexpected error occurred",
            "request_id": request.headers.get("X-Request-ID", "unknown")
        }
    )


@app.get("/health", response_model=HealthResponse)
async def health_check() -> HealthResponse:
    """Health check endpoint"""
    try:
        # Check if services are initialized
        services_healthy = intent_analyzer is not None and prompt_manager is not None
        
        # Check Azure OpenAI connectivity
        openai_healthy = False
        if intent_analyzer:
            openai_healthy = await intent_analyzer.check_openai_health()
        
        is_healthy = services_healthy and openai_healthy
        
        return HealthResponse(
            status="healthy" if is_healthy else "unhealthy",
            timestamp=datetime.utcnow(),
            service="intent-processor",
            version="1.0.0",
            dependencies={
                "services_initialized": services_healthy,
                "azure_openai": openai_healthy
            }
        )
    except Exception as e:
        logger.error(f"Health check failed: {str(e)}")
        return HealthResponse(
            status="unhealthy",
            timestamp=datetime.utcnow(),
            service="intent-processor",
            version="1.0.0",
            error=str(e)
        )


@app.get("/metrics")
async def metrics():
    """Prometheus metrics endpoint"""
    return generate_latest(registry)


@app.post("/api/v1/process-intent", response_model=IntentResponse)
async def process_intent(request: IntentRequest) -> IntentResponse:
    """
    Process natural language requirements and break them down into tasks
    
    Args:
        request: Intent request containing natural language input
        
    Returns:
        IntentResponse with classified intent and task breakdown
    """
    try:
        logger.info(
            "Processing intent request",
            extra={
                "request_id": request.request_id,
                "context": request.context
            }
        )
        
        # Process the intent
        result = await intent_analyzer.analyze_intent(
            text=request.text,
            context=request.context,
            project_info=request.project_info
        )
        
        # Record success metric
        intent_processing_counter.labels(status="success").inc()
        
        logger.info(
            "Intent processed successfully",
            extra={
                "request_id": request.request_id,
                "intent_type": result.intent_type,
                "confidence": result.confidence,
                "task_count": len(result.tasks)
            }
        )
        
        return IntentResponse(
            request_id=request.request_id,
            intent_type=result.intent_type,
            confidence=result.confidence,
            summary=result.summary,
            tasks=result.tasks,
            metadata=result.metadata,
            timestamp=datetime.utcnow()
        )
        
    except ValueError as e:
        intent_processing_counter.labels(status="validation_error").inc()
        logger.warning(f"Validation error: {str(e)}")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(e)
        )
    except Exception as e:
        intent_processing_counter.labels(status="error").inc()
        logger.error(
            "Failed to process intent",
            extra={
                "request_id": request.request_id,
                "error": str(e),
                "type": type(e).__name__
            }
        )
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to process intent"
        )


@app.post("/api/v1/validate-tasks", response_model=Dict[str, Any])
async def validate_tasks(tasks: TaskBreakdown) -> Dict[str, Any]:
    """
    Validate a task breakdown for completeness and consistency
    
    Args:
        tasks: Task breakdown to validate
        
    Returns:
        Validation results with any issues found
    """
    try:
        validation_result = await intent_analyzer.validate_tasks(tasks)
        
        return {
            "valid": validation_result.is_valid,
            "issues": validation_result.issues,
            "suggestions": validation_result.suggestions,
            "timestamp": datetime.utcnow()
        }
        
    except Exception as e:
        logger.error(f"Failed to validate tasks: {str(e)}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to validate tasks"
        )


@app.get("/api/v1/prompt-templates")
async def get_prompt_templates() -> Dict[str, Any]:
    """Get available prompt templates"""
    try:
        templates = prompt_manager.get_available_templates()
        return {
            "templates": templates,
            "timestamp": datetime.utcnow()
        }
    except Exception as e:
        logger.error(f"Failed to get prompt templates: {str(e)}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to retrieve prompt templates"
        )


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "src.main:app",
        host="0.0.0.0",
        port=8001,
        reload=os.getenv("ENV", "production") == "development"
    )