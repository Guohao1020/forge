from __future__ import annotations
import json
import logging
from dataclasses import dataclass
from temporalio import activity
from src.agents.coder import CoderAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)

@dataclass
class GenerateInput:
    project_id: int
    task_id: int
    requirement_summary: str
    task_plan: list[dict]
    fix_instructions: str | None = None

@dataclass
class GenerateOutput:
    files: list[dict]
    commit_message: str
    files_changed: int
    lines_added: int
    lines_deleted: int
    tokens_used: int
    model: str
    provider: str
    latency_ms: int

@activity.defn(name="generate_code")
async def generate_code_activity(input: GenerateInput) -> GenerateOutput:
    logger.info(f"Generating code for task {input.task_id}")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-generation")
        user_prompt = f"## Requirement\n{input.requirement_summary}\n\n"
        user_prompt += f"## Implementation Tasks\n{json.dumps(input.task_plan, indent=2)}\n"
        if input.fix_instructions:
            user_prompt += f"\n## Fix Instructions (from previous review)\n{input.fix_instructions}\n"
            user_prompt += "\nPlease fix the issues identified above and regenerate the code.\n"
        router = ModelRouter()
        agent = CoderAgent(router)
        result = await agent.run(user_prompt, ctx)
        return GenerateOutput(
            files=result.structured.get("files", []),
            commit_message=result.structured.get("commit_message", ""),
            files_changed=result.structured.get("files_changed", 0),
            lines_added=result.structured.get("lines_added", 0),
            lines_deleted=result.structured.get("lines_deleted", 0),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await builder.close()
