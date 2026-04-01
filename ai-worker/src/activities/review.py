from __future__ import annotations
import logging
from dataclasses import dataclass
from temporalio import activity
from src.agents.reviewer import ReviewerAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)

@dataclass
class ReviewInput:
    project_id: int
    task_id: int
    files: list[dict]

@dataclass
class ReviewOutput:
    passed: bool
    score: int
    findings: list[dict]
    summary: str
    fix_instructions: str
    tokens_used: int
    model: str
    provider: str
    latency_ms: int

@activity.defn(name="review_code")
async def review_code_activity(input: ReviewInput) -> ReviewOutput:
    logger.info(f"Reviewing code for task {input.task_id}")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-review")
        code_sections = []
        for f in input.files:
            code_sections.append(f"### {f.get('path', 'unknown')} ({f.get('action', 'create')})\n```{f.get('language', '')}\n{f.get('content', '')}\n```")
        user_prompt = "## Code to Review\n\n" + "\n\n".join(code_sections)
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
