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
from pathlib import Path
from typing import Any, Dict, List, Optional

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from src.openharness.engine.stream_events import StreamEvent  # trivial import, safe

try:
    from src.openharness.engine.pair_pipeline import (
        PairPipelineConfig,
        run_pair_pipeline,
    )
    from src.models.router import Purpose
    _PAIR_PIPELINE_AVAILABLE = True
except Exception as e:
    logging.getLogger(__name__).error(
        "pair_pipeline imports failed at startup — pair_pipeline route "
        "will return 503; falling back to QueryEngine for all requests: %s",
        e,
    )
    PairPipelineConfig = None  # type: ignore
    run_pair_pipeline = None  # type: ignore
    Purpose = None  # type: ignore
    _PAIR_PIPELINE_AVAILABLE = False

logger = logging.getLogger(__name__)

app = FastAPI(title="Forge AI Worker", version="1.0.0")

# In-memory session store: session_id -> QueryEngine
_sessions: Dict[str, Any] = {}

# Lazy-initialized PG pool for dual-storage writes. See _get_pg_pool.
_pg_pool: Any = None


class RunRequest(BaseModel):
    session_id: Optional[str] = None
    project_id: int
    workspace_path: Optional[str] = None
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
    """Accept a message and route it through _route_and_stream asynchronously.

    Events are published to Redis Streams for the Go SSE handler to consume.
    Returns 202 Accepted immediately (fire-and-forget pattern).

    Engine creation is lazy: _route_and_stream decides between pair_pipeline
    (when workspace_path is set and resolves to a real directory) and the
    legacy QueryEngine session path, creating/caching engines on demand.
    """
    session_id = req.session_id or str(uuid.uuid4())
    correlation_id = req.correlation_id or str(uuid.uuid4())

    # Fire-and-forget: run in background task. _route_and_stream handles
    # session engine lookup/creation on the legacy path.
    asyncio.create_task(
        _run_and_publish(req, session_id, correlation_id),
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


def _create_engine(req: RunRequest, purpose: "Purpose | None" = None) -> Any:
    """Create a QueryEngine for a new session.

    purpose controls the ModelRouter routing and the default system
    prompt. Purpose.GENERATE (default) gets the coder prompt;
    Purpose.REVIEW gets the reviewer prompt used by pair_pipeline's
    reviewer engine.
    """
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.hooks.executor import HookExecutor
    from src.openharness.permissions.checker import PermissionChecker
    from src.openharness.permissions.modes import PermissionMode
    # NOTE: Purpose is lazy-imported here to tolerate pair_pipeline startup
    # failures — when the module-level guarded import above sets Purpose to
    # None, the legacy QueryEngine path still needs a real Purpose value.
    # See try/except at top of file.
    from src.models.router import Purpose as _Purpose

    if purpose is None:
        purpose = _Purpose.GENERATE

    # Try to load model router adapter
    try:
        from src.models.router import ModelRouter
        from src.openharness.api.providers.router_adapter import ModelRouterAdapter
        router = ModelRouter()
        api_client = ModelRouterAdapter(router, purpose=purpose)
    except Exception as e:
        logger.warning("ModelRouter not available, using mock: %s", e)
        from unittest.mock import AsyncMock
        api_client = AsyncMock()

    # Load registries
    tool_registry = ToolRegistry()
    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    model = req.model or os.getenv("FORGE_DEFAULT_MODEL", "claude-sonnet-4-20250514")

    if req.system_prompt is not None:
        system_prompt = req.system_prompt
    elif purpose == _Purpose.REVIEW:
        system_prompt = (
            "You are a strict code reviewer. You MUST respond with exactly "
            "one of these three forms:\n"
            "- APPROVE (if the code is correct and production-ready)\n"
            "- REVISE <specific changes needed>\n"
            "- REJECT <reason why the approach is fundamentally wrong>\n"
            "Be terse. Do not ramble."
        )
    else:
        system_prompt = "You are a helpful AI coding assistant."

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=hook_executor,
        permission_checker=permission_checker,
    )


async def _route_and_stream(
    req: RunRequest,
    session_id: str,
    correlation_id: str,
):
    """Route a chat message to either pair_pipeline (when workspace_path
    is set and the resolved directory exists) or the legacy QueryEngine
    path. Async generator. Yields only StreamEvent instances.

    Exceptions propagate to the caller (_run_and_publish in Task 2.3c)
    which turns them into ErrorEvent.

    Routing rule (relative-path protocol, see plan amendment):
      - req.workspace_path is empty/None → QueryEngine path
      - req.workspace_path is set → join with FORGE_WORKSPACE_ROOT env,
        then os.path.isdir check; if dir exists → pair_pipeline branch
        (stubbed in Task 2.3a), otherwise WARN + fall back to QueryEngine.
    """
    # Decide routing. workspace_path is a RELATIVE fragment per the
    # protocol amendment; resolve it against the ai-worker side's
    # FORGE_WORKSPACE_ROOT env var so forge-core (host) and ai-worker
    # (container mount) can live in different filesystems.
    use_pair_pipeline = False
    resolved_workspace: Optional[str] = None
    if req.workspace_path:
        ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
        resolved_workspace = os.path.join(ws_root, req.workspace_path)
        if os.path.isdir(resolved_workspace):
            use_pair_pipeline = True
        else:
            logger.warning(
                "workspace_path %r resolved to %r but directory does not exist "
                "— falling back to QueryEngine (check docker volume mount + "
                "FORGE_WORKSPACE_ROOT env)",
                req.workspace_path,
                resolved_workspace,
            )

    if use_pair_pipeline and not _PAIR_PIPELINE_AVAILABLE:
        logger.warning(
            "workspace_path is set but pair_pipeline is not available "
            "(import failed at startup) — falling back to QueryEngine"
        )
        use_pair_pipeline = False

    if not use_pair_pipeline:
        # Legacy path: single-shot QueryEngine. Reuse session engine
        # from _sessions when present (continuity across messages) or
        # create a fresh one on first message.
        engine = _sessions.get(session_id)
        if engine is None:
            engine = _create_engine(req)
            _sessions[session_id] = engine
        async for event in engine.submit_message(req.message):
            yield event
        return

    # pair_pipeline path: two engines, coder + reviewer, differentiated
    # by Purpose. PairPipelineConfig.project_dir is the resolved
    # container-visible absolute path where BuildVerifyHook will run
    # `go build` / `mvn` / etc.
    logger.info(
        "pair_pipeline route: session=%s correlation=%s workspace=%s",
        session_id, correlation_id, resolved_workspace,
    )
    coder = _create_engine(req, purpose=Purpose.GENERATE)
    reviewer = _create_engine(req, purpose=Purpose.REVIEW)
    config = PairPipelineConfig(project_dir=Path(resolved_workspace))

    async for item in run_pair_pipeline(
        config=config,
        coder_engine=coder,
        reviewer_engine=reviewer,
        initial_prompt=req.message,
        code_files=None,  # LLM reads files via Read/Glob/Grep tools; no pre-seeded context
    ):
        if isinstance(item, StreamEvent):
            yield item
        # Non-StreamEvent yields (CycleResult, PairPipelineResult) are
        # informational for direct callers of run_pair_pipeline (like
        # the e2e test). HTTP callers get the event stream only.


async def _run_and_publish(
    req: RunRequest, session_id: str, correlation_id: str,
) -> None:
    """Route the chat message through _route_and_stream and dual-write
    events to Redis Streams + PostgreSQL.

    Redis is the hot buffer for SSE (capped at settings.agent_stream_maxlen
    via XADD MAXLEN ~). PostgreSQL is the durable history source that the
    frontend hydrates from on page load. Failures in either path are
    logged but don't abort the other — hot SSE keeps flowing even if the
    PG pool is unavailable, and vice versa.

    Engine creation/session caching is now lazy inside _route_and_stream,
    so this function no longer takes an `engine` argument. The pair_pipeline
    vs. legacy QueryEngine routing decision is also made there.
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
        "text": req.message,
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
        async for event in _route_and_stream(req, session_id, correlation_id):
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
    if event_type in ("session_complete", "error", "phase_changed", "clarification_requested"):
        return "system"
    return None


def _serialize_event(event: Any, correlation_id: str) -> Dict[str, str]:
    """Serialize a StreamEvent to a flat dict for Redis Streams."""
    from src.openharness.engine.stream_events import (
        AssistantTextDelta,
        AssistantTurnComplete,
        ClarificationRequested,
        ErrorEvent,
        PhaseChanged,
        SessionComplete,
        ThinkingStarted,
        ThinkingStopped,
        ToolExecutionCompleted,
        ToolExecutionStarted,
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
        base["tool_use_id"] = event.tool_use_id
        base["tool_name"] = event.tool_name
        base["tool_input"] = json.dumps(event.tool_input, default=str)
    elif isinstance(event, ToolExecutionCompleted):
        base["type"] = "tool_completed"
        base["tool_use_id"] = event.tool_use_id
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
    elif isinstance(event, PhaseChanged):
        base["type"] = "phase_changed"
        base["phase"] = event.phase
    elif isinstance(event, ClarificationRequested):
        base["type"] = "clarification_requested"
        base["question"] = event.question
        base["tool_use_id"] = event.tool_use_id
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
