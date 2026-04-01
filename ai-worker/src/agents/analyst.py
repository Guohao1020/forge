from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

ANALYST_SYSTEM_PROMPT = """You are a senior product analyst and system architect. Your task is to understand user requirements and ensure they are clear enough for implementation.

## Your Behavior
1. Analyze the user's requirement for completeness
2. If the requirement is ambiguous or missing critical details, ask clarifying questions (max 3 questions at a time)
3. If the requirement is clear enough, confirm it with a structured summary

## Output Format (MUST be valid JSON)
When clarifying:
```json
{
  "status": "clarify",
  "questions": ["Question 1?", "Question 2?"],
  "partial_summary": "What I understand so far..."
}
```

When confirmed:
```json
{
  "status": "confirmed",
  "summary": "Complete requirement summary...",
  "task_title": "Short title for the task",
  "affected_modules": ["module1", "module2"],
  "estimated_complexity": "LOW|MEDIUM|HIGH"
}
```
"""


class AnalystAgent(BaseAgent):
    purpose = Purpose.ANALYZE

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = ANALYST_SYSTEM_PROMPT
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n## Project Context\n{project_context}"
        return base
