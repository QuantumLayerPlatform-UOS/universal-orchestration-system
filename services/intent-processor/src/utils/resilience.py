"""
Resilience utilities for production-ready service
"""

import asyncio
import functools
import logging
from datetime import datetime, timedelta
from typing import Callable, Any, Optional, TypeVar, Union
from collections import defaultdict

logger = logging.getLogger(__name__)

T = TypeVar('T')


class CircuitBreaker:
    """Circuit breaker pattern implementation"""
    
    def __init__(
        self,
        failure_threshold: int = 5,
        recovery_timeout: int = 30,
        expected_exception: type = Exception
    ):
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout
        self.expected_exception = expected_exception
        self.failure_count = 0
        self.last_failure_time = None
        self.state = 'closed'  # closed, open, half-open
        
    def __call__(self, func: Callable) -> Callable:
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            if self.state == 'open':
                if self._should_attempt_reset():
                    self.state = 'half-open'
                else:
                    raise Exception(f"Circuit breaker is open for {func.__name__}")
            
            try:
                result = await func(*args, **kwargs)
                self._on_success()
                return result
            except self.expected_exception as e:
                self._on_failure()
                raise e
                
        return wrapper
    
    def _should_attempt_reset(self) -> bool:
        return (
            self.last_failure_time and
            datetime.now() - self.last_failure_time > timedelta(seconds=self.recovery_timeout)
        )
    
    def _on_success(self):
        self.failure_count = 0
        self.state = 'closed'
        
    def _on_failure(self):
        self.failure_count += 1
        self.last_failure_time = datetime.now()
        if self.failure_count >= self.failure_threshold:
            self.state = 'open'
            logger.warning(f"Circuit breaker opened after {self.failure_count} failures")


def retry_with_backoff(
    retries: int = 3,
    backoff_in_seconds: float = 1,
    exponential: bool = True,
    exceptions: tuple = (Exception,)
):
    """Retry decorator with exponential backoff"""
    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            retry_count = 0
            delay = backoff_in_seconds
            
            while retry_count < retries:
                try:
                    return await func(*args, **kwargs)
                except exceptions as e:
                    retry_count += 1
                    if retry_count >= retries:
                        logger.error(f"Failed after {retries} retries: {str(e)}")
                        raise
                    
                    logger.warning(
                        f"Retry {retry_count}/{retries} for {func.__name__} "
                        f"after {delay}s delay. Error: {str(e)}"
                    )
                    await asyncio.sleep(delay)
                    
                    if exponential:
                        delay *= 2
                        
        return wrapper
    return decorator


class RateLimiter:
    """Token bucket rate limiter"""
    
    def __init__(self, rate: int, per: float):
        self.rate = rate
        self.per = per
        self.allowance = rate
        self.last_check = asyncio.get_event_loop().time()
        
    async def acquire(self) -> bool:
        current = asyncio.get_event_loop().time()
        time_passed = current - self.last_check
        self.last_check = current
        self.allowance += time_passed * (self.rate / self.per)
        
        if self.allowance > self.rate:
            self.allowance = self.rate
            
        if self.allowance < 1.0:
            return False
            
        self.allowance -= 1.0
        return True


def rate_limit(rate: int, per: float = 1.0):
    """Rate limiting decorator"""
    limiter = RateLimiter(rate, per)
    
    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            if not await limiter.acquire():
                raise Exception(f"Rate limit exceeded for {func.__name__}")
            return await func(*args, **kwargs)
        return wrapper
    return decorator


class TimeoutManager:
    """Context manager for handling timeouts"""
    
    def __init__(self, timeout: float):
        self.timeout = timeout
        self._task = None
        
    async def __aenter__(self):
        self._task = asyncio.current_task()
        return self
        
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self._task and not self._task.done():
            self._task.cancel()


async def timeout_wrapper(coro, timeout: float):
    """Wrap a coroutine with a timeout"""
    try:
        return await asyncio.wait_for(coro, timeout=timeout)
    except asyncio.TimeoutError:
        logger.error(f"Operation timed out after {timeout} seconds")
        raise


class HealthChecker:
    """Health check manager with detailed status tracking"""
    
    def __init__(self):
        self.checks = {}
        self.last_check_time = {}
        self.failure_counts = defaultdict(int)
        
    def register_check(self, name: str, check_func: Callable):
        """Register a health check function"""
        self.checks[name] = check_func
        
    async def run_checks(self) -> dict:
        """Run all registered health checks"""
        results = {}
        
        for name, check_func in self.checks.items():
            try:
                start_time = datetime.now()
                result = await check_func()
                elapsed = (datetime.now() - start_time).total_seconds()
                
                results[name] = {
                    'status': 'healthy' if result else 'unhealthy',
                    'response_time': elapsed,
                    'last_check': datetime.now().isoformat(),
                    'consecutive_failures': 0 if result else self.failure_counts[name] + 1
                }
                
                if result:
                    self.failure_counts[name] = 0
                else:
                    self.failure_counts[name] += 1
                    
            except Exception as e:
                self.failure_counts[name] += 1
                results[name] = {
                    'status': 'unhealthy',
                    'error': str(e),
                    'last_check': datetime.now().isoformat(),
                    'consecutive_failures': self.failure_counts[name]
                }
                
        return results


def graceful_shutdown(cleanup_func: Callable):
    """Decorator for graceful shutdown handling"""
    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            try:
                return await func(*args, **kwargs)
            finally:
                logger.info("Initiating graceful shutdown...")
                await cleanup_func()
                logger.info("Graceful shutdown completed")
        return wrapper
    return decorator