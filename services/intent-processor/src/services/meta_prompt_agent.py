"""
Meta-Prompt Agent for Dynamic Intent Analysis
This agent uses advanced prompting techniques to understand intent dynamically
"""

import logging
from typing import Dict, Any, List, Optional
from datetime import datetime

logger = logging.getLogger(__name__)


class MetaPromptAgent:
    """Advanced agent that creates dynamic prompts based on context"""
    
    def __init__(self):
        self.context_memory = {}
        self.domain_patterns = {
            "api_development": ["api", "rest", "graphql", "endpoint", "route", "http", "request", "response"],
            "data_processing": ["data", "etl", "pipeline", "transform", "aggregate", "analytics"],
            "ui_development": ["ui", "frontend", "component", "react", "vue", "angular", "interface"],
            "infrastructure": ["deploy", "kubernetes", "docker", "aws", "azure", "terraform", "ci/cd"],
            "machine_learning": ["ml", "model", "train", "predict", "neural", "ai", "dataset"],
            "security": ["security", "auth", "encrypt", "vulnerability", "penetration", "ssl"],
            "database": ["database", "sql", "query", "schema", "migration", "index", "performance"]
        }
        
    def analyze_domain(self, text: str) -> str:
        """Detect the domain of the request"""
        text_lower = text.lower()
        scores = {}
        
        for domain, keywords in self.domain_patterns.items():
            score = sum(1 for keyword in keywords if keyword in text_lower)
            if score > 0:
                scores[domain] = score
                
        if scores:
            return max(scores, key=scores.get)
        return "general"
        
    def extract_entities(self, text: str) -> List[str]:
        """Extract key entities from the text"""
        # Simple entity extraction - in production, use NER
        entities = []
        
        # Extract quoted strings
        import re
        quoted = re.findall(r'"([^"]*)"', text)
        entities.extend(quoted)
        
        # Extract capitalized words (potential proper nouns)
        words = text.split()
        for word in words:
            if word[0].isupper() and len(word) > 2:
                entities.append(word)
                
        return list(set(entities))
        
    def generate_context_aware_prompt(self, text: str, context: Optional[Dict[str, Any]] = None) -> str:
        """Generate a context-aware prompt for intent analysis"""
        
        domain = self.analyze_domain(text)
        entities = self.extract_entities(text)
        
        prompt = f"""You are an expert {domain} architect analyzing a software requirement.

User Request: "{text}"

Domain Context: {domain}
Detected Entities: {entities}
Additional Context: {context or 'None'}

Your task is to:
1. Deeply understand what the user wants to achieve
2. Identify the true intent behind the request
3. Break it down into concrete, actionable tasks
4. Consider best practices for {domain}
5. Identify potential challenges and dependencies

Think step by step:
- What is the core problem the user is trying to solve?
- What are the technical requirements?
- What are the quality attributes (performance, security, scalability)?
- What are the deliverables?

Provide a comprehensive analysis in the specified JSON format.
"""
        return prompt
        
    def learn_from_feedback(self, request_id: str, feedback: Dict[str, Any]):
        """Learn from user feedback to improve future analysis"""
        self.context_memory[request_id] = {
            "feedback": feedback,
            "timestamp": datetime.utcnow(),
            "improvements": feedback.get("improvements", [])
        }
        logger.info(f"Learned from feedback for request {request_id}")
        
    def get_domain_specific_tasks(self, domain: str) -> List[Dict[str, Any]]:
        """Get common task templates for a specific domain"""
        domain_tasks = {
            "api_development": [
                {"type": "design", "title": "API Design & Documentation"},
                {"type": "backend", "title": "Implement API Endpoints"},
                {"type": "testing", "title": "API Testing & Validation"},
                {"type": "documentation", "title": "API Documentation"}
            ],
            "data_processing": [
                {"type": "design", "title": "Data Pipeline Architecture"},
                {"type": "backend", "title": "ETL Implementation"},
                {"type": "database", "title": "Data Storage Design"},
                {"type": "testing", "title": "Data Quality Testing"}
            ],
            "ui_development": [
                {"type": "design", "title": "UI/UX Design"},
                {"type": "frontend", "title": "Component Development"},
                {"type": "testing", "title": "UI Testing"},
                {"type": "documentation", "title": "Component Documentation"}
            ]
        }
        return domain_tasks.get(domain, [])