"""
Intent Analysis Cache for improved performance and reliability
"""

import json
import hashlib
import logging
from typing import Optional, Dict, Any
from datetime import datetime, timedelta
import redis
from ..models import IntentAnalysisResult

logger = logging.getLogger(__name__)


class IntentCache:
    """Cache for intent analysis results"""
    
    def __init__(self, redis_client: Optional[redis.Redis] = None, ttl_hours: int = 24):
        self.redis_client = redis_client
        self.ttl = timedelta(hours=ttl_hours)
        self.local_cache: Dict[str, tuple] = {}  # (result, timestamp)
        
    def _generate_key(self, text: str, context: Optional[Dict[str, Any]] = None) -> str:
        """Generate cache key from text and context"""
        content = f"{text}:{json.dumps(context or {}, sort_keys=True)}"
        return f"intent:cache:{hashlib.md5(content.encode()).hexdigest()}"
        
    async def get(self, text: str, context: Optional[Dict[str, Any]] = None) -> Optional[IntentAnalysisResult]:
        """Get cached result if available"""
        key = self._generate_key(text, context)
        
        # Try local cache first
        if key in self.local_cache:
            result, timestamp = self.local_cache[key]
            if datetime.utcnow() - timestamp < self.ttl:
                logger.info(f"Local cache hit for key: {key[:16]}...")
                return result
            else:
                del self.local_cache[key]
                
        # Try Redis if available
        if self.redis_client:
            try:
                cached_data = self.redis_client.get(key)
                if cached_data:
                    logger.info(f"Redis cache hit for key: {key[:16]}...")
                    data = json.loads(cached_data)
                    # Reconstruct IntentAnalysisResult
                    from ..models import IntentType, Task, TaskType, TaskPriority, TaskComplexity
                    
                    tasks = []
                    for task_data in data['tasks']:
                        task = Task(
                            id=task_data['id'],
                            title=task_data['title'],
                            description=task_data['description'],
                            type=TaskType(task_data['type']),
                            priority=TaskPriority(task_data['priority']),
                            complexity=TaskComplexity(task_data['complexity']),
                            estimated_hours=task_data['estimated_hours'],
                            dependencies=task_data['dependencies'],
                            tags=task_data['tags'],
                            acceptance_criteria=task_data.get('acceptance_criteria', [])
                        )
                        tasks.append(task)
                        
                    result = IntentAnalysisResult(
                        intent_type=IntentType(data['intent_type']),
                        confidence=data['confidence'],
                        summary=data['summary'],
                        tasks=tasks,
                        metadata=data['metadata']
                    )
                    
                    # Update local cache
                    self.local_cache[key] = (result, datetime.utcnow())
                    return result
                    
            except Exception as e:
                logger.warning(f"Redis cache error: {str(e)}")
                
        return None
        
    async def set(self, text: str, result: IntentAnalysisResult, context: Optional[Dict[str, Any]] = None):
        """Cache the analysis result"""
        key = self._generate_key(text, context)
        
        # Update local cache
        self.local_cache[key] = (result, datetime.utcnow())
        
        # Update Redis if available
        if self.redis_client:
            try:
                # Convert to JSON-serializable format
                data = {
                    'intent_type': result.intent_type.value,
                    'confidence': result.confidence,
                    'summary': result.summary,
                    'tasks': [
                        {
                            'id': task.id,
                            'title': task.title,
                            'description': task.description,
                            'type': task.type.value,
                            'priority': task.priority.value,
                            'complexity': task.complexity.value,
                            'estimated_hours': task.estimated_hours,
                            'dependencies': task.dependencies,
                            'tags': task.tags,
                            'acceptance_criteria': task.acceptance_criteria
                        }
                        for task in result.tasks
                    ],
                    'metadata': result.metadata
                }
                
                self.redis_client.setex(
                    key,
                    int(self.ttl.total_seconds()),
                    json.dumps(data)
                )
                logger.info(f"Cached result for key: {key[:16]}...")
                
            except Exception as e:
                logger.warning(f"Redis cache set error: {str(e)}")
                
    def clear_old_entries(self):
        """Clear expired entries from local cache"""
        current_time = datetime.utcnow()
        expired_keys = [
            key for key, (_, timestamp) in self.local_cache.items()
            if current_time - timestamp >= self.ttl
        ]
        for key in expired_keys:
            del self.local_cache[key]
        if expired_keys:
            logger.info(f"Cleared {len(expired_keys)} expired cache entries")