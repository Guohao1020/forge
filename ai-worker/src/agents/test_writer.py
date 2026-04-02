from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

TEST_WRITER_SYSTEM_PROMPT = """You are a senior test engineer. Your task is to write test cases BEFORE the implementation code exists. These tests define the expected behavior.

## Rules
1. Select test framework based on the project's tech stack:
   - Go → use "testing" package (go test)
   - Java → use JUnit 5
   - Python → use pytest
   - JavaScript/TypeScript → use Jest
   - If unknown → use pytest as default
2. For each implementation task in the plan, write corresponding test cases
3. Cover: happy path, edge cases, error cases (at least 2 cases per function)
4. Tests should be compilable/runnable even before implementation exists (use interfaces/mocks)
5. Use descriptive test names that explain the expected behavior

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown.

{"test_files": [{"path": "tests/user_test.go", "content": "package tests\\n\\nimport ...", "language": "go", "framework": "go_test", "covers_task": 1}], "test_count": 6, "framework": "go_test", "coverage_targets": ["UserService.Create", "UserService.Delete"]}
"""


class TestWriterAgent(BaseAgent):
    purpose = Purpose.TEST_WRITING

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = TEST_WRITER_SYSTEM_PROMPT

        # Inject tech stack for framework selection
        tech = context.tech_stack
        if tech:
            frameworks = tech.get("frameworks", [])
            languages = tech.get("languages", {})
            if frameworks or languages:
                base += f"\n\n## Detected Tech Stack\nLanguages: {languages}\nFrameworks: {frameworks}"

        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
