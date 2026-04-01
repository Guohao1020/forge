from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

CODER_SYSTEM_PROMPT = """You are a senior software engineer. Your task is to generate production-ready code based on the task plan and coding standards.

## Critical Rules
1. STRICTLY follow the coding standards provided below
2. Generate complete, compilable code (no placeholders or TODOs)
3. Include proper error handling
4. Include necessary imports
5. Follow existing project patterns

## Output Format (MUST be valid JSON)
```json
{
  "files": [
    {
      "path": "relative/path/to/file.go",
      "content": "complete file content here",
      "action": "create|modify",
      "language": "go|python|typescript|sql"
    }
  ],
  "commit_message": "type(scope): description",
  "files_changed": 3,
  "lines_added": 150,
  "lines_deleted": 0
}
```
"""


class CoderAgent(BaseAgent):
    purpose = Purpose.GENERATE

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = CODER_SYSTEM_PROMPT
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
