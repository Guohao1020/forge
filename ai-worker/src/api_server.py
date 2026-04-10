"""FastAPI HTTP server — replaces Temporal worker for agent sessions.

Endpoints:
  POST /api/run           — submit a message, run QueryEngine, publish events to Redis
  DELETE /api/sessions/{id} — clear a session
  GET /health             — health check
  POST /api/workspace/prep — language-specific dependency pre-install

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
from typing import Any, AsyncIterator, Dict, List, Optional

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from src.openharness.engine.stream_events import StreamEvent

# LRU-bounded session cache (spec §5.8, implemented in Phase 5 Task 5.4)
from src.openharness.engine.session_cache import LRUSessionCache

logger = logging.getLogger(__name__)

app = FastAPI(title="Forge AI Worker", version="1.0.0")

# In-memory session store: session_id -> QueryEngine (LRU bounded)
_sessions = LRUSessionCache(max_size=100)

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
    """
    session_id = req.session_id or str(uuid.uuid4())
    correlation_id = req.correlation_id or str(uuid.uuid4())

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
    engine = _sessions.pop(session_id)
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


# ---------------------------------------------------------------------------
# Workspace prep — dependency pre-install for the A2 architecture.
# Called by forge-core's workspace.Manager.EnsureReady (Phase 1) after
# a fresh clone. We run go mod download / mvn dependency:go-offline /
# npm ci / etc. in the ai-worker container (which has network +
# language toolchains, unlike the bwrap sandbox). Spec §3.9.
# ---------------------------------------------------------------------------


class PrepRequest(BaseModel):
    tenant_id: int
    project_id: int
    workspace_path: str  # relative to FORGE_WORKSPACE_ROOT


class PrepResponse(BaseModel):
    status: str  # "ok" | "skipped" | "error"
    language: Optional[str] = None
    command: Optional[str] = None
    error: Optional[str] = None
    reason: Optional[str] = None


# Fallback prep commands by language name. Used when the
# LanguageProfile doesn't expose a prep_command attribute.
_FALLBACK_PREP_COMMANDS = {
    "go": "go mod download",
    "python": "pip install -r requirements.txt",
    "java": "mvn dependency:go-offline -B",
    "javascript": "npm ci",
    "typescript": "npm ci",
    "rust": "cargo fetch",
}

PREP_TIMEOUT_SECONDS = 600  # 10 minutes


@app.post("/api/workspace/prep", response_model=PrepResponse)
async def workspace_prep(req: PrepRequest) -> PrepResponse:
    """Run language-specific dependency pre-install for a workspace.

    Called by forge-core after a fresh clone (Phase 1 Task 1.5).
    Runs OUTSIDE the bash sandbox because we need network access
    and the language toolchains here — the ai-worker container
    has all of them, the bwrap sandbox does not.
    """
    from src.openharness.skills.project_language import (
        detect_language,
        load_all_language_profiles,
    )

    ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
    workspace_dir = Path(os.path.join(ws_root, req.workspace_path))

    if not workspace_dir.is_dir():
        return PrepResponse(
            status="error",
            error=f"workspace directory does not exist: {workspace_dir}",
        )

    # Detect language
    try:
        profiles = load_all_language_profiles("skills/languages")
        profile = detect_language(workspace_dir, profiles)
    except Exception as e:
        logger.warning("workspace prep: language detection failed: %s", e)
        return PrepResponse(
            status="skipped",
            reason=f"language detection error: {e}",
        )

    if profile is None:
        return PrepResponse(
            status="skipped",
            reason="no language detected; agent will see dependency errors if any",
        )

    # Resolve prep command
    prep_cmd = getattr(profile, "prep_command", None)
    if not prep_cmd:
        prep_cmd = _FALLBACK_PREP_COMMANDS.get(profile.name.lower())
    if not prep_cmd:
        return PrepResponse(
            status="skipped",
            language=profile.name,
            reason=(
                f"language '{profile.name}' detected but no prep command known; "
                "agent will see dependency errors if any"
            ),
        )

    # Run the prep command — NOT in bwrap
    logger.info(
        "workspace prep: running %r in %s (language=%s)",
        prep_cmd,
        workspace_dir,
        profile.name,
    )
    try:
        proc = await asyncio.create_subprocess_shell(
            prep_cmd,
            cwd=str(workspace_dir),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.STDOUT,
        )
        stdout, _ = await asyncio.wait_for(
            proc.communicate(), timeout=PREP_TIMEOUT_SECONDS
        )
    except asyncio.TimeoutError:
        try:
            proc.kill()
        except ProcessLookupError:
            pass
        return PrepResponse(
            status="error",
            language=profile.name,
            command=prep_cmd,
            error=f"prep command timed out after {PREP_TIMEOUT_SECONDS} seconds",
        )
    except Exception as e:
        return PrepResponse(
            status="error",
            language=profile.name,
            command=prep_cmd,
            error=f"prep command failed to start: {e}",
        )

    if proc.returncode != 0:
        tail = stdout.decode("utf-8", errors="replace")[-1000:]
        return PrepResponse(
            status="error",
            language=profile.name,
            command=prep_cmd,
            error=f"prep command exited {proc.returncode}: ...{tail}",
        )

    logger.info("workspace prep: %s ok (language=%s)", prep_cmd, profile.name)
    return PrepResponse(
        status="ok",
        language=profile.name,
        command=prep_cmd,
    )


# ---------------------------------------------------------------------------
# Engine construction + routing
# ---------------------------------------------------------------------------


def _create_engine(req: RunRequest, workspace_dir: Path) -> Any:
    """Create a QueryEngine wired with the full T2 tool set.

    Called lazily by _route_and_stream when a new session_id is seen.
    Hard-fails if the model router is unavailable — no AsyncMock
    fallback.
    """
    from src.openharness.engine.agent_hooks import AgentHookContext, AgentHookRegistry
    from src.openharness.engine.prompts import build_system_prompt
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.hooks.executor import HookExecutor
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.permissions.checker import PermissionChecker
    from src.openharness.permissions.modes import PermissionMode
    from src.openharness.tools import (
        register_exec_tools,
        register_file_tools,
    )
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.tools.context_tools import register_context_tools
    from src.openharness.tools.interaction_tools import register_interaction_tools

    # ModelRouter — required. No AsyncMock fallback.
    try:
        from src.models.router import ModelRouter, Purpose
        from src.openharness.api.providers.router_adapter import ModelRouterAdapter

        router = ModelRouter()
        api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
    except Exception as e:
        raise RuntimeError(
            f"ModelRouter unavailable — agent cannot start. "
            f"Check provider credentials and network. Underlying error: {e}"
        ) from e

    # Fail-fast: verify reviewer model is configured (§2.9.3.f)
    try:
        router.require_model_for(Purpose.REVIEW)
    except Exception as e:
        logger.warning("Reviewer model not configured (non-fatal): %s", e)

    # Tool registry — all tools
    tool_registry = ToolRegistry()

    # Context tools (6): profile queries + read_project_file HTTP
    register_context_tools(
        tool_registry,
        profiles={},
        project_id=req.project_id,
    )

    # File tools (6): read/write/edit/glob/grep/list_directory
    register_file_tools(tool_registry, workspace_dir)

    # Exec tools (2): bash + set_phase
    register_exec_tools(tool_registry, workspace_dir)

    # Interaction meta-tools (2): request_clarification + request_review
    register_interaction_tools(
        registry=tool_registry,
        model_router=router,
        workspace_dir=workspace_dir,
    )

    # Agent hooks (Round 2) — empty registry for now
    agent_hook_registry = AgentHookRegistry()
    session_id = req.session_id or str(uuid.uuid4())
    agent_hook_context = AgentHookContext(
        project_id=req.project_id,
        session_id=session_id,
        workspace_dir=workspace_dir,
        system_prompt_buffer=[],
    )

    # Hooks and permissions
    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    # Model + system prompt
    model = req.model or os.getenv("FORGE_DEFAULT_MODEL", "claude-sonnet-4-20250514")

    if req.system_prompt is not None:
        system_prompt = req.system_prompt
    else:
        from src.openharness.skills.project_language import (
            detect_language,
            load_all_language_profiles,
        )

        language_name: Optional[str] = None
        try:
            profiles = load_all_language_profiles("skills/languages")
            profile = detect_language(workspace_dir, profiles)
            if profile is not None:
                language_name = profile.name
        except Exception as e:
            logger.warning(
                "language detection failed: %s (proceeding without)", e,
            )

        # build_system_prompt is async, but _create_engine is sync.
        # Use asyncio to run it synchronously since we're called
        # from an async context that will await us.
        import asyncio

        loop = asyncio.get_event_loop()
        if loop.is_running():
            # We're inside an async context — create a new event loop
            # for the sync call. This is safe because build_system_prompt
            # only does string manipulation + optional async slot fillers.
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor() as pool:
                system_prompt = pool.submit(
                    asyncio.run,
                    build_system_prompt(
                        language=language_name,
                        workspace_path=str(workspace_dir),
                        slots=agent_hook_registry.system_prompt_slots or None,
                        hook_context=agent_hook_context,
                    ),
                ).result()
        else:
            system_prompt = loop.run_until_complete(
                build_system_prompt(
                    language=language_name,
                    workspace_path=str(workspace_dir),
                    slots=agent_hook_registry.system_prompt_slots or None,
                    hook_context=agent_hook_context,
                )
            )

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=hook_executor,
        permission_checker=permission_checker,
        cwd=workspace_dir,
        agent_hook_registry=agent_hook_registry,
        agent_hook_context=agent_hook_context,
    )


async def _route_and_stream(
    req: RunRequest,
    session_id: str,
    correlation_id: str,
) -> AsyncIterator[Any]:
    """Route a chat message to the agent loop and yield stream events.

    The pair_pipeline fork is deleted in A2 — all requests go through
    a single QueryEngine path. workspace_path is required; missing
    workspace is a 400.
    """
    ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")

    if not req.workspace_path:
        # Dev fallback: no workspace_path means forge-core legacy path
        # (tenantID=0) or missing EnsureReady. Create a temp dir so the
        # agent can still run for smoke-testing. Production always has
        # workspace_path set by forge-core after EnsureReady.
        import tempfile
        resolved_workspace = Path(tempfile.mkdtemp(prefix="forge-dev-ws-"))
        logger.warning(
            "workspace_path is empty — using temp directory %s (dev fallback)",
            resolved_workspace,
        )
    else:
        resolved_workspace = Path(os.path.join(ws_root, req.workspace_path))

        if not resolved_workspace.is_dir():
            logger.error(
                "workspace_path %r resolved to %r but directory does not exist "
                "— forge-core should have called EnsureReady first.",
                req.workspace_path,
                str(resolved_workspace),
            )
            raise HTTPException(
                status_code=500,
                detail=f"workspace not ready: {resolved_workspace}",
            )

    # Get or create the engine
    engine = _sessions.get(session_id)
    if engine is None:
        try:
            engine = _create_engine(req, workspace_dir=resolved_workspace)
        except RuntimeError as e:
            from src.openharness.engine.stream_events import ErrorEvent
            yield ErrorEvent(message=str(e), recoverable=False)
            return
        _sessions.put(session_id, engine)

    async for event in engine.submit_message(req.message):
        yield event


async def _run_and_publish(
    req: RunRequest, session_id: str, correlation_id: str,
) -> None:
    """Route the chat message through _route_and_stream and dual-write
    events to Redis Streams + PostgreSQL.
    """
    redis_client = await _get_redis()
    pg_pool = await _get_pg_pool()
    stream_key = f"agent:stream:{session_id}"
    from src.config import settings

    # Persist the user message before engine runs
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
    """Insert one agent_messages row. Silent-fails on any error."""
    if pg_pool is None:
        return
    event_type = event_data.get("type", "unknown")
    role = event_data.get("role") or _role_for_event_type(event_type)
    content = event_data.get("text") or event_data.get("message") or None
    tool_name = event_data.get("tool_name")
    correlation_id = event_data.get("correlation_id")
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
        logger.debug("agent_messages insert failed: %s", e)


def _role_for_event_type(event_type: str) -> Optional[str]:
    """Derive a message `role` from the stream event_type."""
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
    elif isinstance(event, PhaseChanged):
        base["type"] = "phase_changed"
        base["phase"] = event.phase
    elif isinstance(event, ErrorEvent):
        base["type"] = "error"
        base["message"] = event.message
        base["recoverable"] = str(event.recoverable)
    elif isinstance(event, ThinkingStarted):
        base["type"] = "thinking_started"
        base["label"] = event.label
    elif isinstance(event, ThinkingStopped):
        base["type"] = "thinking_stopped"
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
    """Lazy-initialize the asyncpg connection pool."""
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
