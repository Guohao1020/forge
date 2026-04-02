import json
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.coder import CoderAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter, Purpose

logger = logging.getLogger(__name__)


@dataclass
class GenerateInput:
    task_id: int
    tenant_id: int
    project_id: int
    plan: Optional[Dict[str, Any]] = None          # Plan result from previous step
    requirement_summary: Optional[str] = None
    task_plan: Optional[List[Dict[str, Any]]] = None
    test_cases: Optional[Dict[str, Any]] = None  # From TEST_WRITING step
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

        # Inject test cases as constraints
        if input.test_cases:
            test_files = input.test_cases.get("test_files", [])
            if test_files:
                user_prompt += "\n## Test Cases (MUST PASS)\nThe following test cases have been written. Your code MUST make these tests pass.\n"
                for tf in test_files:
                    user_prompt += f"\n### {tf.get('path', 'test')}\n```{tf.get('language', '')}\n{tf.get('content', '')}\n```\n"
                user_prompt += "\nGenerate implementation code that satisfies all the test cases above.\n\n"

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

        # Try streaming mode first — publishes tokens to Redis for live UI
        try:
            from src.agents.coder import CoderAgent, CODER_SYSTEM_PROMPT, _build_language_constraints
            system_prompt = CODER_SYSTEM_PROMPT
            # Inject language constraints (same logic as CoderAgent._build_system_prompt)
            if ctx.tech_stack:
                lang_constraints = _build_language_constraints(ctx.tech_stack)
                if lang_constraints:
                    system_prompt += f"\n\n{lang_constraints}"
            project_context = ctx.to_system_prompt()
            if project_context:
                system_prompt += f"\n\n{project_context}"
            messages = [{"role": "user", "content": user_prompt}]
            for msg in ctx.conversation_history:
                messages.insert(-1, {"role": msg.get("role", "user"), "content": msg.get("content", "")})

            llm_result = await router.chat_stream(
                system=system_prompt,
                messages=messages,
                purpose=Purpose.GENERATE,
                task_id=input.task_id,
            )

            # Parse JSON from the streamed response (same logic as BaseAgent)
            from src.agents.base import BaseAgent
            parser = BaseAgent(router)
            structured = parser._parse_json(llm_result.content)

            return GenerateOutput(
                files=structured.get("files", []),
                commit_message=structured.get("commit_message", ""),
                files_changed=structured.get("files_changed", 0),
                lines_added=structured.get("lines_added", 0),
                lines_deleted=structured.get("lines_deleted", 0),
                tokens_used=llm_result.input_tokens + llm_result.output_tokens,
                model=llm_result.model,
                provider=llm_result.provider,
                latency_ms=llm_result.latency_ms,
            )
        except Exception as e:
            logger.warning(f"Streaming generation failed, falling back to agent: {e}")

        # Fallback: non-streaming via CoderAgent
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
