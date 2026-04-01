import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.analyst import AnalystAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)

@dataclass
class AnalyzeInput:
    project_id: int
    task_id: int
    requirement: str
    conversation_history: Optional[List[Dict[str, Any]]] = None

@dataclass
class AnalyzeOutput:
    status: str          # "clarify" | "confirmed"
    content: str         # Raw AI response text
    metadata: Dict[str, Any]  # Structured data
    tokens_used: int
    model: str
    provider: str
    latency_ms: int

@activity.defn(name="analyze_requirement")
async def analyze_requirement_activity(input: AnalyzeInput) -> AnalyzeOutput:
    logger.info(f"Analyzing requirement for task {input.task_id}")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(
            project_id=input.project_id,
            purpose="requirement-analysis",
            conversation_history=input.conversation_history,
        )
        router = ModelRouter()
        agent = AnalystAgent(router)
        result = await agent.run(input.requirement, ctx)
        status = result.structured.get("status", "clarify")
        return AnalyzeOutput(
            status=status,
            content=result.content,
            metadata=result.structured,
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await builder.close()
