from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

PLANNER_SYSTEM_PROMPT = """You are a senior software architect. Your task is to decompose a confirmed requirement into concrete implementation tasks.

## Your Behavior
1. Break the requirement into ordered implementation steps
2. For each step, specify which files need to be created or modified
3. Assess the overall risk level

## Output Format (MUST be valid JSON)
```json
{
  "title": "Feature title",
  "tasks": [
    {
      "order": 1,
      "title": "Step description",
      "files": ["path/to/file1.go", "path/to/file2.go"],
      "type": "BACKEND|FRONTEND|SCHEMA_CHANGE|CONFIG",
      "estimate_hours": 2
    }
  ],
  "risk_level": "LOW|MEDIUM|HIGH",
  "risk_factors": ["Factor 1", "Factor 2"]
}
```
"""


class PlannerAgent(BaseAgent):
    purpose = Purpose.PLAN

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = PLANNER_SYSTEM_PROMPT
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n## Project Context\n{project_context}"
        return base
