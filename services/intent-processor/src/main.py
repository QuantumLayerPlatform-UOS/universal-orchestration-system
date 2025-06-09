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
from .services.mock_intent_analyzer import MockIntentAnalyzer
from .services.real_intent_analyzer import RealIntentAnalyzer
from .services.prompt_manager import PromptManager
from .utils.resilience import (
    retry_with_backoff,
    CircuitBreaker,
    rate_limit,
    timeout_wrapper,
    HealthChecker
)

# Configure structured logging
logHandler = logging.StreamHandler()
formatter = jsonlogger.JsonFormatter()
logHandler.setFormatter(formatter)
logger = logging.getLogger(__name__)
logger.addHandler(logHandler)
logger.setLevel(logging.DEBUG if os.getenv("LOG_LEVEL", "info").lower() == "debug" else logging.INFO)

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

# Initialize services - Use app state instead of globals
# intent_analyzer: IntentAnalyzer = None
# prompt_manager: PromptManager = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle"""
    logger.info("Starting Intent Processor Service")
    
    # Initialize health checker
    app.state.health_checker = HealthChecker()
    
    # Initialize services with retry
    @retry_with_backoff(retries=3, backoff_in_seconds=2)
    async def initialize_services():
        # Check if Azure OpenAI credentials are available
        azure_key = os.getenv("AZURE_OPENAI_API_KEY", "")
        azure_endpoint = os.getenv("AZURE_OPENAI_ENDPOINT", "")
        
        # Check for real LLM providers first
        use_real_llm = any([
            os.getenv("OLLAMA_BASE_URL"),
            os.getenv("GROQ_API_KEY"),
            os.getenv("OPENAI_API_KEY"),
            os.getenv("ANTHROPIC_API_KEY")
        ])
        
        if use_real_llm:
            # Use real LLM analyzer
            app.state.prompt_manager = PromptManager()
            app.state.intent_analyzer = RealIntentAnalyzer()
            await app.state.intent_analyzer.initialize()
            logger.info("Services initialized with real LLM provider")
        elif azure_key and azure_endpoint and azure_key != "dummy-key":
            # Use Azure OpenAI analyzer
            app.state.prompt_manager = PromptManager()
            app.state.intent_analyzer = IntentAnalyzer(app.state.prompt_manager)
            await app.state.intent_analyzer.initialize()
            logger.info("Services initialized with Azure OpenAI")
        else:
            # Use mock analyzer for testing
            app.state.prompt_manager = PromptManager()  # Initialize prompt manager even for mock
            app.state.intent_analyzer = MockIntentAnalyzer()
            await app.state.intent_analyzer.initialize()
            logger.warning("Using mock intent analyzer (no LLM credentials)")
        
        # Register health checks
        app.state.health_checker.register_check(
            "intent_analyzer",
            lambda: app.state.intent_analyzer.check_openai_health()
        )
        
        logger.info("Services initialized successfully")
    
    try:
        await initialize_services()
    except Exception as e:
        logger.error(f"Failed to initialize services after retries: {str(e)}")
        raise
    
    yield
    
    # Cleanup
    logger.info("Shutting down Intent Processor Service")
    if hasattr(app.state, 'intent_analyzer') and app.state.intent_analyzer:
        await app.state.intent_analyzer.cleanup()


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
async def health_check(request: Request) -> HealthResponse:
    """Health check endpoint with detailed status"""
    try:
        # Check if services are initialized
        has_analyzer = hasattr(request.app.state, 'intent_analyzer') and request.app.state.intent_analyzer is not None
        has_prompt_manager = hasattr(request.app.state, 'prompt_manager') and request.app.state.prompt_manager is not None
        has_health_checker = hasattr(request.app.state, 'health_checker') and request.app.state.health_checker is not None
        
        services_healthy = has_analyzer and has_prompt_manager
        
        logger.debug(f"Health check - intent_analyzer: {has_analyzer}, prompt_manager: {has_prompt_manager}")
        
        # Run detailed health checks if available
        detailed_checks = {}
        if has_health_checker:
            detailed_checks = await request.app.state.health_checker.run_checks()
        
        # Check Azure OpenAI connectivity
        openai_healthy = False
        if has_analyzer:
            try:
                openai_healthy = await timeout_wrapper(
                    request.app.state.intent_analyzer.check_openai_health(),
                    timeout=3.0
                )
            except Exception as e:
                logger.warning(f"OpenAI health check failed: {str(e)}")
                openai_healthy = False
        
        is_healthy = services_healthy and openai_healthy
        
        logger.debug(f"Health check - services_healthy: {services_healthy}, openai_healthy: {openai_healthy}, is_healthy: {is_healthy}")
        
        dependencies = {
            "services_initialized": services_healthy,
            "azure_openai": openai_healthy
        }
        
        # Add detailed checks to dependencies
        if detailed_checks:
            dependencies["detailed_checks"] = detailed_checks
        
        return HealthResponse(
            status="healthy" if is_healthy else "unhealthy",
            timestamp=datetime.utcnow(),
            service="intent-processor",
            version="1.0.0",
            dependencies=dependencies
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


# Apply circuit breaker to the intent processing
intent_processor_circuit = CircuitBreaker(failure_threshold=5, recovery_timeout=30)

@app.post("/api/v1/process-intent", response_model=IntentResponse)
@rate_limit(rate=100, per=60.0)  # 100 requests per minute
async def process_intent(request: IntentRequest, req: Request) -> IntentResponse:
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
        
        # Process the intent with timeout
        @intent_processor_circuit
        @retry_with_backoff(retries=2, backoff_in_seconds=1)
        async def analyze_with_resilience():
            return await timeout_wrapper(
                req.app.state.intent_analyzer.analyze_intent(
                    text=request.text,
                    context=request.context,
                    project_info=request.project_info
                ),
                timeout=10.0  # 10 second timeout
            )
        
        result = await analyze_with_resilience()
        
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
async def validate_tasks(tasks: TaskBreakdown, req: Request) -> Dict[str, Any]:
    """
    Validate a task breakdown for completeness and consistency
    
    Args:
        tasks: Task breakdown to validate
        
    Returns:
        Validation results with any issues found
    """
    try:
        validation_result = await req.app.state.intent_analyzer.validate_tasks(tasks)
        
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
async def get_prompt_templates(req: Request) -> Dict[str, Any]:
    """Get available prompt templates"""
    try:
        templates = req.app.state.prompt_manager.get_available_templates()
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