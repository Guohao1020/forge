import json
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.coder import CoderAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)


@dataclass
class GenerateInput:
    task_id: int
    tenant_id: int
    project_id: int
    plan: Optional[Dict[str, Any]] = None          # Plan result from previous step
    requirement_summary: Optional[str] = None
    task_plan: Optional[List[Dict[str, Any]]] = None
    fix_instructions: Optional[str] = None
    code: Optional[Dict[str, Any]] = None           # Previous code for fix pass
    review: Optional[Dict[str, Any]] = None          # Review result for fix pass


@dataclass
class GenerateOutput:
    files: List[Dict[str, Any]]
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

        # Build user prompt from available data
        user_prompt = ""

        if input.requirement_summary:
            user_prompt += f"## Requirement\n{input.requirement_summary}\n\n"

        # Use task_plan or extract tasks from plan result
        tasks = input.task_plan
        if not tasks and input.plan:
            tasks = input.plan.get("tasks", [])
        if tasks:
            user_prompt += f"## Implementation Tasks\n{json.dumps(tasks, indent=2)}\n\n"

        # Fix mode: include review feedback
        if input.review:
            fix_instr = input.review.get("fix_instructions", "")
            findings = input.review.get("findings", [])
            if fix_instr:
                user_prompt += f"## Fix Instructions (from review)\n{fix_instr}\n\n"
            if findings:
                user_prompt += f"## Review Findings\n{json.dumps(findings, indent=2)}\n\n"
            user_prompt += "Please fix the issues identified above and regenerate the code.\n"

        if input.fix_instructions:
            user_prompt += f"\n## Fix Instructions\n{input.fix_instructions}\n"

        if not user_prompt.strip():
            user_prompt = "Generate code based on the project context and coding standards."

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
