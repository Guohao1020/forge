import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.planner import PlannerAgent
from src.context.cache import ContextCache
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)


@dataclass
class PlanInput:
    task_id: int
    tenant_id: int
    project_id: int
    requirement_summary: Optional[str] = None  # May come from Go or be fetched
    confirmed_requirements: Optional[Dict[str, Any]] = None  # Structured requirements from analysis


@dataclass
class PlanOutput:
    title: str
    tasks: List[Dict[str, Any]]  # includes depends_on, requirement_ref, description
    risk_level: str
    risk_factors: List[str]
    total_estimate_hours: float = 0
    parallel_tracks: int = 1
    touched_files: Dict[str, List[str]] = field(default_factory=dict)  # {"create": [...], "modify": [...]}
    recommendations: Optional[Dict[str, Any]] = None  # AI recommendations when multiple approaches exist
    tokens_used: int = 0
    model: str = ""
    provider: str = ""
    latency_ms: int = 0


@activity.defn(name="plan_task")
async def plan_task_activity(input: PlanInput) -> PlanOutput:
    logger.info(f"Planning task {input.task_id}")
    cache = ContextCache()
    try:
        ctx = await cache.get_or_build(input.project_id, purpose="code-generation")

        # Use provided summary or a default prompt
        summary = input.requirement_summary or "Please analyze the task and create an implementation plan."

        from src.context.tools import CONTEXT_TOOLS, ContextToolExecutor
        # PlannerAgent uses 3 tools: api_catalog, module_graph, read_project_file
        planner_tools = [t for t in CONTEXT_TOOLS if t["name"] in (
            "query_api_catalog", "query_module_graph", "read_project_file"
        )]
        router = ModelRouter()
        agent = PlannerAgent(router, confirmed_requirements=input.confirmed_requirements)
        tool_executor = ContextToolExecutor(ctx, input.project_id)
        result = await agent.run(summary, ctx, tools=planner_tools, tool_executor=tool_executor)

        # Extract touched_files from plan output
        touched_files = result.structured.get("touched_files", {})
        if not touched_files:
            # Fallback: aggregate files from all tasks
            all_files = set()
            for task in result.structured.get("tasks", []):
                for f in task.get("files", []):
                    all_files.add(f)
            touched_files = {"create": [], "modify": list(all_files)}

        return PlanOutput(
            title=result.structured.get("title", ""),
            tasks=result.structured.get("tasks", []),
            risk_level=result.structured.get("risk_level", "MEDIUM"),
            risk_factors=result.structured.get("risk_factors", []),
            total_estimate_hours=result.structured.get("total_estimate_hours", 0),
            parallel_tracks=result.structured.get("parallel_tracks", 1),
            touched_files=touched_files,
            recommendations=result.structured.get("recommendations"),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await cache.close()
