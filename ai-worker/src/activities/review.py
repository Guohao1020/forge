import json
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.reviewer import ReviewerAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)


@dataclass
class ReviewInput:
    task_id: int
    tenant_id: int
    project_id: int
    code: Optional[Dict[str, Any]] = None     # Generated code result from previous step
    files: Optional[List[Dict[str, Any]]] = None
    attempt: Optional[int] = 1


@dataclass
class ReviewOutput:
    passed: bool
    score: int
    findings: List[Dict[str, Any]]
    summary: str
    fix_instructions: str
    tokens_used: int
    model: str
    provider: str
    latency_ms: int


@activity.defn(name="review_code")
async def review_code_activity(input: ReviewInput) -> ReviewOutput:
    logger.info(f"Reviewing code for task {input.task_id} (attempt {input.attempt})")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-review")

        # Extract files from direct files param or from code result
        files = input.files
        if not files and input.code:
            files = input.code.get("files", [])
        if not files:
            files = []

        # Build code review prompt
        code_sections = []
        for f in files:
            path = f.get("path", "unknown")
            action = f.get("action", "create")
            language = f.get("language", "")
            content = f.get("content", "")
            code_sections.append(f"### {path} ({action})\n```{language}\n{content}\n```")

        user_prompt = "## Code to Review\n\n" + "\n\n".join(code_sections) if code_sections else "No code files provided for review."

        router = ModelRouter()
        agent = ReviewerAgent(router)
        result = await agent.run(user_prompt, ctx)
        return ReviewOutput(
            passed=result.structured.get("passed", False),
            score=result.structured.get("score", 0),
            findings=result.structured.get("findings", []),
            summary=result.structured.get("summary", ""),
            fix_instructions=result.structured.get("fix_instructions", ""),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await builder.close()
