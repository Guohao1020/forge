"""FastAPI HTTP server — replaces Temporal worker for agent sessions.

Endpoints:
  POST /api/run           — submit a message, run QueryEngine, publish events to Redis
  DELETE /api/sessions/{id} — clear a session
  GET /health             — health check
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import uuid
from typing import Any, Dict, Optional

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

logger = logging.getLogger(__name__)

app = FastAPI(title="Forge AI Worker", version="1.0.0")

# In-memory session store: session_id -> QueryEngine
_sessions: Dict[str, Any] = {}


class RunRequest(BaseModel):
    session_id: Optional[str] = None
    project_id: int
    message: str
    model: Optional[str] = None
    system_prompt: Optional[str] = None
    correlation_id: Optional[str] = None


class RunResponse(BaseModel):
    session_id: str
    status: str
    correlation_id: Optional[str] = None


@app.post("/api/run", response_model=RunResponse)
async def run_agent(req: RunRequest) -> RunResponse:
    """Accept a message and run the QueryEngine asynchronously.

    Events are published to Redis Streams for the Go SSE handler to consume.
    Returns 202 Accepted immediately (fire-and-forget pattern).
    """
    session_id = req.session_id or str(uuid.uuid4())
    correlation_id = req.correlation_id or str(uuid.uuid4())

    # Get or create session engine
    engine = _sessions.get(session_id)
    if engine is None:
        engine = _create_engine(req)
        _sessions[session_id] = engine

    # Fire-and-forget: run in background task
    asyncio.create_task(
        _run_and_publish(engine, session_id, req.message, correlation_id),
    )

    return RunResponse(
        session_id=session_id,
        status="accepted",
        correlation_id=correlation_id,
    )


@app.delete("/api/sessions/{session_id}")
async def delete_session(session_id: str) -> JSONResponse:
    engine = _sessions.pop(session_id, None)
    if engine is None:
        raise HTTPException(status_code=404, detail="Session not found")
    engine.clear()
    return JSONResponse({"status": "deleted"})


@app.get("/health")
async def health() -> Dict[str, Any]:
    return {
        "status": "ok",
        "sessions": len(_sessions),
        "version": "1.0.0",
    }


def _create_engine(req: RunRequest) -> Any:
    """Create a QueryEngine for a new session."""
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.hooks.executor import HookExecutor
    from src.openharness.permissions.checker import PermissionChecker
    from src.openharness.permissions.modes import PermissionMode
    from src.openharness.skills.loader import load_skill_registry

    # Try to load model router adapter
    try:
        from src.models.router import ModelRouter, Purpose
        from src.openharness.api.providers.router_adapter import ModelRouterAdapter
        router = ModelRouter()
        api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
    except Exception as e:
        logger.warning("ModelRouter not available, using mock: %s", e)
        from unittest.mock import AsyncMock
        api_client = AsyncMock()

    # Load registries
    tool_registry = ToolRegistry()
    # Context tools require project-specific data; skip for now
    # They'll be registered when project context is loaded

    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    model = req.model or os.getenv("FORGE_DEFAULT_MODEL", "claude-sonnet-4-20250514")
    system_prompt = req.system_prompt or "You are a helpful AI coding assistant."

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=hook_executor,
        permission_checker=permission_checker,
    )


async def _run_and_publish(
    engine: Any, session_id: str, message: str, correlation_id: str,
) -> None:
    """Run the engine and publish events to Redis Streams + PostgreSQL."""
    redis_client = await _get_redis()
    stream_key = f"agent:stream:{session_id}"

    try:
        async for event in engine.submit_message(message):
            event_data = _serialize_event(event, correlation_id)
            if redis_client:
                try:
                    await redis_client.xadd(stream_key, event_data)
                except Exception as e:
                    logger.error("Redis XADD failed: %s", e)
            # TODO: Dual-write to PostgreSQL event log table
    except Exception as e:
        logger.exception("Agent run failed for session %s", session_id)
        error_data = {
            "type": "error",
            "message": str(e),
            "correlation_id": correlation_id,
        }
        if redis_client:
            try:
                await redis_client.xadd(stream_key, error_data)
            except Exception:
                pass


def _serialize_event(event: Any, correlation_id: str) -> Dict[str, str]:
    """Serialize a StreamEvent to a flat dict for Redis Streams."""
    from src.openharness.engine.stream_events import (
        AssistantTextDelta,
        AssistantTurnComplete,
        ToolExecutionStarted,
        ToolExecutionCompleted,
        ErrorEvent,
    )

    base = {"correlation_id": correlation_id}

    if isinstance(event, AssistantTextDelta):
        base["type"] = "text_delta"
        base["text"] = event.text
    elif isinstance(event, AssistantTurnComplete):
        base["type"] = "turn_complete"
        base["text"] = event.message.text
        base["input_tokens"] = str(event.usage.input_tokens)
        base["output_tokens"] = str(event.usage.output_tokens)
    elif isinstance(event, ToolExecutionStarted):
        base["type"] = "tool_started"
        base["tool_name"] = event.tool_name
        base["tool_input"] = json.dumps(event.tool_input, default=str)
    elif isinstance(event, ToolExecutionCompleted):
        base["type"] = "tool_completed"
        base["tool_name"] = event.tool_name
        base["output"] = event.output[:4000]  # Truncate for Redis
        base["is_error"] = str(event.is_error)
    elif isinstance(event, ErrorEvent):
        base["type"] = "error"
        base["message"] = event.message
        base["recoverable"] = str(event.recoverable)
    else:
        base["type"] = "unknown"
        base["data"] = str(event)

    return base


_redis: Any = None


async def _get_redis() -> Any:
    global _redis
    if _redis is not None:
        return _redis
    try:
        import redis.asyncio as aioredis
        from src.config import settings
        _redis = aioredis.from_url(
            f"redis://:{settings.redis_password}@{settings.redis_host}:{settings.redis_port}/0",
        )
        return _redis
    except Exception as e:
        logger.warning("Redis not available: %s", e)
        return None


if __name__ == "__main__":
    import uvicorn
    logging.basicConfig(
        level=logging.INFO,
        format='{"time":"%(asctime)s","level":"%(levelname)s","logger":"%(name)s","msg":"%(message)s"}',
    )
    uvicorn.run(app, host="0.0.0.0", port=8090)
