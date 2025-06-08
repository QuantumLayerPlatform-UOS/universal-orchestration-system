"""
Prompt Manager Service
Manages prompt templates for different types of analysis
"""

import json
from typing import Dict, Any, List, Optional

from ..models import IntentType, Task


class PromptManager:
    """Manages prompt templates for LLM interactions"""
    
    def __init__(self):
        self.templates = self._initialize_templates()
    
    def _initialize_templates(self) -> Dict[str, str]:
        """Initialize prompt templates"""
        return {
            "intent_classification": """You are an expert software project analyst. Classify the following requirement into one of these intent types:
- feature_request: New functionality or feature
- bug_fix: Fixing an existing issue
- refactoring: Code improvement without changing functionality
- documentation: Documentation updates
- testing: Test creation or improvement
- deployment: Deployment or infrastructure changes
- configuration: Configuration changes
- research: Research or investigation tasks

Analyze the requirement and provide your classification with confidence score.""",

            "information_extraction": """You are an expert at extracting key information from software requirements. Extract and structure the following information:
- Main objective
- Technical components involved
- User impact
- Constraints or limitations
- Success criteria
- Any specific technologies mentioned""",

            "task_generation": """You are an expert software architect and project planner. Break down the given requirement into specific, actionable tasks. For each task provide:
- Clear title
- Detailed description
- Task type (frontend, backend, database, api, infrastructure, testing, documentation, design, devops, security)
- Priority (critical, high, medium, low)
- Complexity (simple, moderate, complex, very_complex)
- Estimated hours
- Dependencies (task IDs if any)
- Tags
- Acceptance criteria
- Technical requirements

Ensure tasks are:
1. Specific and actionable
2. Appropriately sized (not too large or too small)
3. Have clear acceptance criteria
4. Include all necessary work (implementation, testing, documentation)""",

            "summary_generation": """You are an expert at creating concise, informative summaries. Create a brief summary (2-3 sentences) that captures:
- What is being requested
- Why it's important
- Expected outcome""",

            "chain_of_thought": """Think step by step about this requirement:
1. What is the core need?
2. What are the technical implications?
3. What are potential challenges?
4. What dependencies exist?
5. How can this be broken down logically?"""
        }
    
    def get_intent_classification_prompt(self, text: str) -> Dict[str, str]:
        """Get prompt for intent classification"""
        return {
            "system": self.templates["intent_classification"],
            "user": f"""Classify this requirement:

{text}

Respond in JSON format:
{{
    "intent_type": "<type>",
    "confidence": <0-1>,
    "reasoning": "<brief explanation>"
}}"""
        }
    
    def get_information_extraction_prompt(
        self, 
        text: str, 
        intent_type: IntentType,
        context: Optional[Dict[str, Any]] = None
    ) -> Dict[str, str]:
        """Get prompt for information extraction"""
        context_str = ""
        if context:
            context_str = f"\n\nAdditional context:\n{json.dumps(context, indent=2)}"
        
        return {
            "system": self.templates["information_extraction"],
            "user": f"""Extract key information from this {intent_type.value} requirement:

{text}{context_str}

Respond in JSON format:
{{
    "main_objective": "<objective>",
    "technical_components": ["<component1>", "<component2>"],
    "user_impact": "<impact description>",
    "constraints": ["<constraint1>", "<constraint2>"],
    "success_criteria": ["<criterion1>", "<criterion2>"],
    "technologies": ["<tech1>", "<tech2>"],
    "additional_notes": "<any other important information>"
}}"""
        }
    
    def get_task_generation_prompt(
        self,
        text: str,
        intent_type: IntentType,
        extracted_info: Dict[str, Any],
        project_info: Optional[Dict[str, Any]] = None
    ) -> Dict[str, str]:
        """Get prompt for task generation"""
        project_context = ""
        if project_info:
            project_context = f"\n\nProject Information:\n{json.dumps(project_info, indent=2)}"
        
        # Add chain of thought for complex requirements
        cot_prompt = ""
        if intent_type in [IntentType.FEATURE_REQUEST, IntentType.REFACTORING]:
            cot_prompt = f"\n\n{self.templates['chain_of_thought']}"
        
        return {
            "system": self.templates["task_generation"] + cot_prompt,
            "user": f"""Generate tasks for this {intent_type.value}:

Requirement: {text}

Extracted Information:
{json.dumps(extracted_info, indent=2)}{project_context}

Create a comprehensive task breakdown. Respond in JSON format:
{{
    "tasks": [
        {{
            "title": "<task title>",
            "description": "<detailed description>",
            "type": "<task type>",
            "priority": "<priority level>",
            "complexity": "<complexity level>",
            "estimated_hours": <number>,
            "dependencies": ["<task_id>"],
            "tags": ["<tag1>", "<tag2>"],
            "acceptance_criteria": ["<criterion1>", "<criterion2>"],
            "technical_requirements": {{
                "technologies": ["<tech1>"],
                "apis": ["<api1>"],
                "data_models": ["<model1>"]
            }}
        }}
    ]
}}"""
        }
    
    def get_summary_prompt(
        self, 
        text: str, 
        intent_type: IntentType,
        tasks: List[Task]
    ) -> Dict[str, str]:
        """Get prompt for summary generation"""
        task_summary = f"Generated {len(tasks)} tasks covering: " + \
                      ", ".join(set(task.type.value for task in tasks))
        
        return {
            "system": self.templates["summary_generation"],
            "user": f"""Summarize this {intent_type.value}:

Original requirement: {text}

{task_summary}

Provide a concise 2-3 sentence summary."""
        }
    
    def get_custom_prompt(self, template_name: str, **kwargs) -> Dict[str, str]:
        """Get a custom prompt with variable substitution"""
        if template_name not in self.templates:
            raise ValueError(f"Template '{template_name}' not found")
        
        template = self.templates[template_name]
        
        # Simple variable substitution
        for key, value in kwargs.items():
            template = template.replace(f"{{{key}}}", str(value))
        
        return {
            "system": template,
            "user": kwargs.get("user_prompt", "")
        }
    
    def add_template(self, name: str, template: str):
        """Add a new prompt template"""
        self.templates[name] = template
    
    def get_available_templates(self) -> List[str]:
        """Get list of available template names"""
        return list(self.templates.keys())
    
    def get_template(self, name: str) -> str:
        """Get a specific template"""
        return self.templates.get(name, "")
    
    def create_few_shot_prompt(
        self, 
        template_name: str, 
        examples: List[Dict[str, str]],
        query: str
    ) -> Dict[str, str]:
        """Create a few-shot learning prompt with examples"""
        if template_name not in self.templates:
            raise ValueError(f"Template '{template_name}' not found")
        
        examples_text = "\n\n".join([
            f"Example {i+1}:\nInput: {ex['input']}\nOutput: {ex['output']}"
            for i, ex in enumerate(examples)
        ])
        
        return {
            "system": self.templates[template_name],
            "user": f"""Here are some examples:

{examples_text}

Now process this:
Input: {query}
Output:"""
        }