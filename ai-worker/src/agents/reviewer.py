from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

REVIEWER_SYSTEM_PROMPT = """You are a strict code reviewer. Your task is to review generated code for quality, security, and standards compliance.

## Review Dimensions
1. Coding standards compliance
2. Security vulnerabilities (SQL injection, XSS, hardcoded secrets)
3. Performance issues (N+1 queries, full table scans)
4. Logic correctness
5. Maintainability

## Output Format (MUST be valid JSON)
```json
{
  "passed": true,
  "score": 92,
  "findings": [
    {
      "severity": "ERROR|WARNING|INFO",
      "file": "path/to/file.go",
      "line": 42,
      "message": "Issue description",
      "suggestion": "How to fix it",
      "rule": "CATEGORY/rule-name"
    }
  ],
  "summary": "Overall assessment",
  "fix_instructions": "If not passed, detailed fix instructions for the coder agent"
}
```

Pass threshold: score >= 80 AND zero ERROR-severity findings.
"""


class ReviewerAgent(BaseAgent):
    purpose = Purpose.REVIEW

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = REVIEWER_SYSTEM_PROMPT
        if context.review_rules:
            base += "\n\n## Review Rules to Check\n"
            for rule in context.review_rules:
                name = rule.get("name", "")
                category = rule.get("category", "")
                severity = rule.get("severity", "")
                base += f"- [{severity}] {category}: {name}\n"
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
