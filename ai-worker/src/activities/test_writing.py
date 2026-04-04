import json
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from temporalio import activity

from src.agents.test_writer import TestWriterAgent
from src.context.cache import ContextCache
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)


@dataclass
class TestWritingInput:
    task_id: int
    tenant_id: int
    project_id: int
    plan: Optional[Dict[str, Any]] = None
    requirement_summary: Optional[str] = None


@dataclass
class TestWritingOutput:
    test_files: List[Dict[str, Any]] = field(default_factory=list)
    test_count: int = 0
    framework: str = ""
    coverage_targets: List[str] = field(default_factory=list)
    tokens_used: int = 0
    model: str = ""
    provider: str = ""
    latency_ms: int = 0


@activity.defn(name="generate_test_cases")
async def generate_test_cases_activity(input: TestWritingInput) -> TestWritingOutput:
    logger.info(f"Generating test cases for task {input.task_id}")
    cache = ContextCache()
    try:
        ctx = await cache.get_or_build(input.project_id, purpose="code-generation")

        # Build user prompt from plan
        user_prompt = ""
        if input.requirement_summary:
            user_prompt += f"## Requirement\n{input.requirement_summary}\n\n"

        if input.plan:
            tasks = input.plan.get("tasks", [])
            if tasks:
                user_prompt += f"## Implementation Plan\n{json.dumps(tasks, indent=2, ensure_ascii=False)}\n\n"

        if not user_prompt.strip():
            user_prompt = "Generate test cases based on the project context."

        user_prompt += "\nGenerate test cases for the implementation tasks above. Write tests that will validate the code BEFORE it is written."

        from src.context.tools import CONTEXT_TOOLS, ContextToolExecutor
        # TestWriterAgent uses 3 tools: db_schema, api_catalog, read_project_file
        test_tools = [t for t in CONTEXT_TOOLS if t["name"] in (
            "query_db_schema", "query_api_catalog", "read_project_file"
        )]
        router = ModelRouter()
        agent = TestWriterAgent(router)
        tool_executor = ContextToolExecutor(ctx, input.project_id)
        result = await agent.run(user_prompt, ctx, tools=test_tools, tool_executor=tool_executor)

        return TestWritingOutput(
            test_files=result.structured.get("test_files", []),
            test_count=result.structured.get("test_count", 0),
            framework=result.structured.get("framework", ""),
            coverage_targets=result.structured.get("coverage_targets", []),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await cache.close()
