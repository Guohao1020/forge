"""FastAPI HTTP server — replaces Temporal worker for agent sessions.

Endpoints:
  POST /api/run           — submit a message, run QueryEngine, publish events to Redis
  DELETE /api/sessions/{id} — clear a session
  GET /health             — health check

Dual-storage pattern (Stream 4b): every event is written to Redis
Streams (hot SSE buffer, capped at settings.agent_stream_maxlen) AND
inserted into engine.agent_messages via asyncpg (durable history).
Frontend session recovery hydrates from PG first, then subscribes to
Redis for new events.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import uuid
from typing import Any, Dict, List, Optional

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

logger = logging.getLogger(__name__)

app = FastAPI(title="Forge AI Worker", version="1.0.0")

# In-memory session store: session_id -> QueryEngine
_sessions: Dict[str, Any] = {}

# Lazy-initialized PG pool for dual-storage writes. See _get_pg_pool.
_pg_pool: Any = None


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
    """Run the engine and dual-write events to Redis Streams + PostgreSQL.

    Redis is the hot buffer for SSE (capped at settings.agent_stream_maxlen
    via XADD MAXLEN ~). PostgreSQL is the durable history source that the
    frontend hydrates from on page load. Failures in either path are
    logged but don't abort the other — hot SSE keeps flowing even if the
    PG pool is unavailable, and vice versa.
    """
    redis_client = await _get_redis()
    pg_pool = await _get_pg_pool()
    stream_key = f"agent:stream:{session_id}"
    from src.config import settings

    # Persist the user message before engine runs so history hydration
    # shows the full conversation. Redis gets it too so late-joining SSE
    # clients can see what was asked.
    user_event = {
        "type": "user_message",
        "text": message,
        "role": "user",
        "correlation_id": correlation_id,
    }
    user_redis_id: Optional[str] = None
    if redis_client:
        try:
            user_redis_id = await redis_client.xadd(
                stream_key,
                user_event,
                maxlen=settings.agent_stream_maxlen,
                approximate=True,
            )
        except Exception as e:
            logger.error("Redis XADD (user message) failed: %s", e)
    await _persist_message(pg_pool, session_id, user_redis_id, user_event)

    try:
        async for event in engine.submit_message(message):
            event_data = _serialize_event(event, correlation_id)
            redis_id: Optional[str] = None
            if redis_client:
                try:
                    redis_id = await redis_client.xadd(
                        stream_key,
                        event_data,
                        maxlen=settings.agent_stream_maxlen,
                        approximate=True,
                    )
                except Exception as e:
                    logger.error("Redis XADD failed: %s", e)
            await _persist_message(pg_pool, session_id, redis_id, event_data)
    except Exception as e:
        logger.exception("Agent run failed for session %s", session_id)
        error_data = {
            "type": "error",
            "message": str(e),
            "correlation_id": correlation_id,
        }
        err_redis_id: Optional[str] = None
        if redis_client:
            try:
                err_redis_id = await redis_client.xadd(
                    stream_key,
                    error_data,
                    maxlen=settings.agent_stream_maxlen,
                    approximate=True,
                )
            except Exception:
                pass
        await _persist_message(pg_pool, session_id, err_redis_id, error_data)


async def _persist_message(
    pg_pool: Any,
    session_id: str,
    redis_id: Optional[str],
    event_data: Dict[str, str],
) -> None:
    """Insert one agent_messages row. Silent-fails on any error so the
    hot SSE path keeps flowing even when PG is down or the schema
    hasn't been migrated yet."""
    if pg_pool is None:
        return
    event_type = event_data.get("type", "unknown")
    role = event_data.get("role") or _role_for_event_type(event_type)
    content = event_data.get("text") or event_data.get("message") or None
    tool_name = event_data.get("tool_name")
    correlation_id = event_data.get("correlation_id")
    # Canonical payload: the whole event dict (as JSON). This lets the
    # frontend replay in exactly the same shape Redis delivered.
    data_json = json.dumps(event_data)

    try:
        async with pg_pool.acquire() as conn:
            await conn.execute(
                """
                INSERT INTO engine.agent_messages
                    (session_id, redis_id, event_type, role, content, tool_name, data, correlation_id)
                VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
                ON CONFLICT (session_id, redis_id) WHERE redis_id IS NOT NULL DO NOTHING
                """,
                session_id,
                redis_id,
                event_type,
                role,
                content,
                tool_name,
                data_json,
                correlation_id,
            )
    except Exception as e:
        # Common harmless errors: session not yet committed in PG (race
        # with CreateSession), foreign key failure, PG down. Log and move
        # on — Redis is the source of truth for the live UI.
        logger.debug("agent_messages insert failed: %s", e)


def _role_for_event_type(event_type: str) -> Optional[str]:
    """Derive a message `role` from the stream event_type for
    lookup-by-role queries in the durable store."""
    if event_type in ("text_delta", "turn_complete"):
        return "assistant"
    if event_type == "user_message":
        return "user"
    if event_type in ("tool_started", "tool_completed"):
        return "tool"
    if event_type in ("fix_loop_started", "fix_loop_completed", "session_complete", "error"):
        return "system"
    return None


def _serialize_event(event: Any, correlation_id: str) -> Dict[str, str]:
    """Serialize a StreamEvent to a flat dict for Redis Streams."""
    from src.openharness.engine.stream_events import (
        AssistantTextDelta,
        AssistantTurnComplete,
        ToolExecutionStarted,
        ToolExecutionCompleted,
        ErrorEvent,
        ThinkingStarted,
        ThinkingStopped,
        FixLoopStarted,
        FixLoopCompleted,
        SessionComplete,
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
    elif isinstance(event, ThinkingStarted):
        base["type"] = "thinking_started"
        base["label"] = event.label
    elif isinstance(event, ThinkingStopped):
        base["type"] = "thinking_stopped"
    elif isinstance(event, FixLoopStarted):
        base["type"] = "fix_loop_started"
        base["cycle"] = str(event.cycle)
        base["max_cycles"] = str(event.max_cycles)
        base["errors"] = str(event.errors)
    elif isinstance(event, FixLoopCompleted):
        base["type"] = "fix_loop_completed"
        base["cycle"] = str(event.cycle)
        base["success"] = str(event.success)
    elif isinstance(event, SessionComplete):
        base["type"] = "session_complete"
        base["files_created"] = str(event.files_created)
        base["files_modified"] = str(event.files_modified)
        base["build_status"] = event.build_status
        base["duration_ms"] = str(event.duration_ms)
        base["tokens_total"] = str(event.tokens_total)
        base["cost_usd"] = f"{event.cost_usd:.4f}"
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


async def _get_pg_pool() -> Any:
    """Lazy-initialize the asyncpg connection pool for durable
    agent_messages writes. Returns None when PG is unavailable so the
    caller's try/except in _persist_message can silently skip the
    write — Redis remains the source of truth for the live UI."""
    global _pg_pool
    if _pg_pool is not None:
        return _pg_pool
    try:
        import asyncpg
        from src.config import settings
        _pg_pool = await asyncpg.create_pool(
            host=settings.pg_host,
            port=settings.pg_port,
            user=settings.pg_user,
            password=settings.pg_password,
            database=settings.pg_database,
            min_size=1,
            max_size=5,
            command_timeout=5.0,
        )
        logger.info("agent_messages PG pool initialized")
        return _pg_pool
    except Exception as e:
        logger.warning(
            "PG pool not available — agent history will rely on Redis only: %s", e,
        )
        return None


if __name__ == "__main__":
    import uvicorn
    logging.basicConfig(
        level=logging.INFO,
        format='{"time":"%(asctime)s","level":"%(levelname)s","logger":"%(name)s","msg":"%(message)s"}',
    )
    uvicorn.run(app, host="0.0.0.0", port=8090)
