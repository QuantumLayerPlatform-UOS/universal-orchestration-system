"""
Chain of Thought Streaming for User Engagement
Provides real-time updates during intent processing
"""

import asyncio
import json
import logging
from typing import AsyncGenerator, Dict, Any, Optional
from datetime import datetime
from enum import Enum

logger = logging.getLogger(__name__)


class ThoughtType(str, Enum):
    """Types of thoughts in the chain"""
    UNDERSTANDING = "understanding"
    ANALYZING = "analyzing"
    CLASSIFYING = "classifying"
    DECOMPOSING = "decomposing"
    PLANNING = "planning"
    VALIDATING = "validating"
    COMPLETE = "complete"
    ERROR = "error"


class ThoughtStream:
    """Manages Chain of Thought streaming to users"""
    
    def __init__(self):
        self.active_streams: Dict[str, asyncio.Queue] = {}
        self.thought_templates = {
            ThoughtType.UNDERSTANDING: [
                "🤔 Reading your request...",
                "📖 Understanding the requirements...",
                "🎯 Analyzing what you need..."
            ],
            ThoughtType.ANALYZING: [
                "🔍 Analyzing technical requirements...",
                "🧠 Processing with AI...",
                "📊 Evaluating complexity..."
            ],
            ThoughtType.CLASSIFYING: [
                "🏷️ Classifying intent type...",
                "📋 Determining project category...",
                "🎨 Identifying domain..."
            ],
            ThoughtType.DECOMPOSING: [
                "🔨 Breaking down into tasks...",
                "📝 Creating action items...",
                "🧩 Organizing dependencies..."
            ],
            ThoughtType.PLANNING: [
                "📅 Estimating effort...",
                "👥 Identifying required skills...",
                "⚡ Setting priorities..."
            ],
            ThoughtType.VALIDATING: [
                "✅ Validating task breakdown...",
                "🔗 Checking dependencies...",
                "📊 Finalizing analysis..."
            ],
            ThoughtType.COMPLETE: [
                "🎉 Analysis complete!",
                "✨ Ready to proceed!",
                "🚀 All set!"
            ],
            ThoughtType.ERROR: [
                "❌ Encountered an issue...",
                "⚠️ Something went wrong...",
                "🔧 Trying alternative approach..."
            ]
        }
        
    async def create_stream(self, request_id: str) -> asyncio.Queue:
        """Create a new thought stream for a request"""
        queue = asyncio.Queue()
        self.active_streams[request_id] = queue
        logger.info(f"Created thought stream for request {request_id}")
        return queue
        
    async def close_stream(self, request_id: str):
        """Close a thought stream"""
        if request_id in self.active_streams:
            queue = self.active_streams[request_id]
            await queue.put(None)  # Signal end of stream
            del self.active_streams[request_id]
            logger.info(f"Closed thought stream for request {request_id}")
            
    async def emit_thought(
        self, 
        request_id: str, 
        thought_type: ThoughtType,
        detail: Optional[str] = None,
        progress: Optional[float] = None,
        metadata: Optional[Dict[str, Any]] = None
    ):
        """Emit a thought to the stream"""
        if request_id not in self.active_streams:
            logger.warning(f"No active stream for request {request_id}")
            return
            
        # Get appropriate message template
        templates = self.thought_templates.get(thought_type, ["Processing..."])
        import random
        message = random.choice(templates)
        
        thought = {
            "timestamp": datetime.utcnow().isoformat(),
            "type": thought_type.value,
            "message": message,
            "detail": detail,
            "progress": progress,
            "metadata": metadata or {}
        }
        
        queue = self.active_streams[request_id]
        await queue.put(thought)
        logger.debug(f"Emitted thought: {thought_type.value} for {request_id}")
        
    async def stream_thoughts(self, request_id: str) -> AsyncGenerator[str, None]:
        """Stream thoughts as Server-Sent Events"""
        queue = self.active_streams.get(request_id)
        if not queue:
            logger.error(f"No stream found for request {request_id}")
            return
            
        try:
            while True:
                thought = await queue.get()
                if thought is None:  # End of stream
                    break
                    
                # Format as SSE
                yield f"data: {json.dumps(thought)}\n\n"
                
        except asyncio.CancelledError:
            logger.info(f"Stream cancelled for request {request_id}")
            raise
        finally:
            await self.close_stream(request_id)
            
    async def emit_detailed_analysis(
        self,
        request_id: str,
        analysis_steps: Dict[str, Any]
    ):
        """Emit a detailed analysis flow"""
        
        # Understanding phase
        await self.emit_thought(
            request_id, 
            ThoughtType.UNDERSTANDING,
            detail=f"Request length: {len(analysis_steps.get('text', ''))} characters",
            progress=0.1
        )
        await asyncio.sleep(0.5)  # Small delay for effect
        
        # Analyzing phase
        if 'domain' in analysis_steps:
            await self.emit_thought(
                request_id,
                ThoughtType.ANALYZING,
                detail=f"Detected domain: {analysis_steps['domain']}",
                progress=0.3
            )
            await asyncio.sleep(0.5)
            
        # Classifying phase
        if 'intent_type' in analysis_steps:
            await self.emit_thought(
                request_id,
                ThoughtType.CLASSIFYING,
                detail=f"Intent type: {analysis_steps['intent_type']}",
                progress=0.5,
                metadata={"confidence": analysis_steps.get('confidence', 0)}
            )
            await asyncio.sleep(0.5)
            
        # Decomposing phase
        if 'task_count' in analysis_steps:
            await self.emit_thought(
                request_id,
                ThoughtType.DECOMPOSING,
                detail=f"Creating {analysis_steps['task_count']} tasks",
                progress=0.7
            )
            await asyncio.sleep(0.5)
            
        # Planning phase
        if 'total_hours' in analysis_steps:
            await self.emit_thought(
                request_id,
                ThoughtType.PLANNING,
                detail=f"Estimated effort: {analysis_steps['total_hours']} hours",
                progress=0.9
            )
            await asyncio.sleep(0.5)
            
        # Complete
        await self.emit_thought(
            request_id,
            ThoughtType.COMPLETE,
            detail="Analysis complete",
            progress=1.0
        )


# Global instance
thought_stream = ThoughtStream()