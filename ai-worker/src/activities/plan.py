import logging
from dataclasses import dataclass
from typing import Any, Dict, List
from temporalio import activity
from src.agents.planner import PlannerAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)

@dataclass
class PlanInput:
    project_id: int
    task_id: int
    requirement_summary: str

@dataclass
class PlanOutput:
    title: str
    tasks: List[Dict[str, Any]]
    risk_level: str
    risk_factors: List[str]
    tokens_used: int
    model: str
    provider: str
    latency_ms: int

@activity.defn(name="plan_task")
async def plan_task_activity(input: PlanInput) -> PlanOutput:
    logger.info(f"Planning task {input.task_id}")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="requirement-analysis")
        router = ModelRouter()
        agent = PlannerAgent(router)
        result = await agent.run(input.requirement_summary, ctx)
        return PlanOutput(
            title=result.structured.get("title", ""),
            tasks=result.structured.get("tasks", []),
            risk_level=result.structured.get("risk_level", "MEDIUM"),
            risk_factors=result.structured.get("risk_factors", []),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await builder.close()
