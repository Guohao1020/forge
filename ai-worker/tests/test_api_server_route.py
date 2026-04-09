"""Routing contract for api_server._route_and_stream.

_route_and_stream decides between the pair_pipeline path and the legacy
QueryEngine path based on whether RunRequest.workspace_path is set and
the directory exists on disk. This test file pins that contract
incrementally across Tasks 2.1, 2.2, 2.3a, 2.3b, 2.3c.

The workspace_path is a RELATIVE fragment like "tenant-1/project-1/repo"
that both forge-core and ai-worker join to their own FORGE_WORKSPACE_ROOT
to produce the physical path. See the protocol amendment in
docs/plans/2026-04-09-pair-pipeline-production-wire.md.
"""

from __future__ import annotations

import pytest
from unittest.mock import MagicMock

from src.api_server import RunRequest
from src.api_server import _create_engine
from src.api_server import _route_and_stream
from src.models.router import Purpose
from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    SessionComplete,
    StreamEvent,
)


def test_run_request_accepts_workspace_path():
    """RunRequest must accept an optional workspace_path field as a
    relative fragment (not an absolute path)."""
    req = RunRequest(
        project_id=1,
        message="hello",
        workspace_path="tenant-1/project-1/repo",
    )
    assert req.workspace_path == "tenant-1/project-1/repo"


def test_run_request_workspace_path_is_optional():
    """Legacy callers that don't set workspace_path must still work."""
    req = RunRequest(project_id=1, message="hello")
    assert req.workspace_path is None


def test_create_engine_defaults_to_generate_purpose():
    """Backwards compat: _create_engine called without purpose should
    still work, defaulting to Purpose.GENERATE."""
    req = RunRequest(project_id=1, message="hello")
    engine = _create_engine(req)
    assert engine is not None
    # QueryEngine stores the resolved system_prompt in a private attribute
    # `_system_prompt` (verified at query_engine.py:40). Reach through it
    # for the assertion — keeping the coupling explicit.
    assert "helpful" in engine._system_prompt.lower()


def test_create_engine_accepts_review_purpose():
    """New capability: _create_engine called with Purpose.REVIEW should
    produce an engine configured with the reviewer system prompt."""
    req = RunRequest(project_id=1, message="hello")
    engine = _create_engine(req, purpose=Purpose.REVIEW)
    assert engine is not None
    # The reviewer prompt must include "APPROVE" and "REVISE" as literal
    # tokens because pair_pipeline's _parse_review_decision matches on
    # those exact prefixes. This is a load-bearing contract.
    assert "APPROVE" in engine._system_prompt
    assert "REVISE" in engine._system_prompt


def test_create_engine_user_prompt_overrides_purpose_default():
    """If the caller provides system_prompt explicitly, it wins over the
    purpose-derived default. This is how the legacy single-shot path
    keeps its own prompt when it gets wired through the new router."""
    req = RunRequest(
        project_id=1,
        message="hello",
        system_prompt="Custom prompt for this caller only.",
    )
    engine = _create_engine(req, purpose=Purpose.REVIEW)
    assert engine._system_prompt == "Custom prompt for this caller only."


# Fake events emitted by mock iterators — shared with 2.3b/c tests
def _fake_query_engine_iter():
    """Mimic engine.submit_message(): yield a single AssistantTextDelta."""
    async def _gen():
        yield AssistantTextDelta(text="hello from QueryEngine")
    return _gen()


@pytest.mark.asyncio
async def test_route_empty_workspace_uses_queryengine(monkeypatch):
    """workspace_path is None → QueryEngine path yields its events through."""
    req = RunRequest(project_id=1, message="hello")

    qe_called = {"v": 0}

    def fake_qe(r, purpose=None):
        qe_called["v"] += 1
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    # Isolate _sessions between tests: the legacy path caches engines
    # per session_id, so without this monkeypatch a prior test that
    # used "sid-1" would shadow the _create_engine call here.
    monkeypatch.setattr("src.api_server._sessions", {})
    monkeypatch.setattr("src.api_server._create_engine", fake_qe)

    events = []
    async for ev in _route_and_stream(req, "sid-1", "corr-1"):
        events.append(ev)

    assert qe_called["v"] == 1, "QueryEngine must be created when workspace is empty"
    assert len(events) == 1
    assert isinstance(events[0], AssistantTextDelta)


@pytest.mark.asyncio
async def test_route_nonexistent_workspace_falls_back(monkeypatch, caplog):
    """workspace_path is set but the resolved directory does not exist →
    fallback to QueryEngine. Misconfigured volume mount must not fail the
    chat; fall back and log a WARN."""
    # RELATIVE fragment (amendment). The ai-worker side joins it with
    # FORGE_WORKSPACE_ROOT before os.path.isdir. We deliberately do NOT
    # set FORGE_WORKSPACE_ROOT in the test, so it falls back to the
    # default "/data/forge/workspaces", which probably doesn't exist
    # on the Windows dev host where tests run.
    req = RunRequest(
        project_id=1,
        message="hello",
        workspace_path="bogus/tenant-999/project-999/repo",
    )
    # Make sure the test is self-contained: monkeypatch an env var that
    # deterministically does not exist on disk, so the test passes on
    # any machine without relying on /data/forge/workspaces absence.
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", "/nonexistent/ws-root-for-test")

    qe_called = {"v": 0}

    def fake_qe(r, purpose=None):
        qe_called["v"] += 1
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    # Isolate _sessions between tests (see test_route_empty_workspace_uses_queryengine).
    monkeypatch.setattr("src.api_server._sessions", {})
    monkeypatch.setattr("src.api_server._create_engine", fake_qe)

    import logging
    with caplog.at_level(logging.WARNING, logger="src.api_server"):
        events = []
        async for ev in _route_and_stream(req, "sid-1", "corr-1"):
            events.append(ev)

    assert qe_called["v"] == 1
    # WARN log must mention workspace_path
    assert any("workspace_path" in r.message.lower() or "workspace_path" in r.message for r in caplog.records), \
        f"expected WARN log mentioning workspace_path, got: {[r.message for r in caplog.records]}"


def _fake_pair_pipeline_iter_with_non_event():
    """Mimic run_pair_pipeline: yield a StreamEvent, a non-StreamEvent
    (simulating CycleResult), and a final SessionComplete."""
    from dataclasses import dataclass

    @dataclass
    class FakeCycleResult:
        cycle: int = 1

    async def _gen():
        yield AssistantTextDelta(text="hello from pair_pipeline")
        yield FakeCycleResult()
        yield SessionComplete(
            files_created=1,
            files_modified=0,
            build_status="passed",
            duration_ms=100,
            tokens_total=50,
            cost_usd=0.001,
        )
    return _gen()


@pytest.mark.asyncio
async def test_route_valid_workspace_uses_pair_pipeline(tmp_path, monkeypatch):
    """workspace_path resolves to a real directory → pair_pipeline path,
    2 engines (coder + reviewer) are created via _create_engine, the
    PairPipelineConfig.project_dir is the resolved path, and
    non-StreamEvent yields (FakeCycleResult) are filtered out."""
    # Isolate _sessions across tests (see 2.3a note).
    monkeypatch.setattr("src.api_server._sessions", {})

    # Create a real directory to resolve to
    ws_root = tmp_path / "workspaces"
    (ws_root / "repo").mkdir(parents=True)
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(ws_root))

    req = RunRequest(
        project_id=1,
        message="add a hello function",
        workspace_path="repo",
    )

    qe_calls = []  # list of Purpose values each call received

    def fake_qe(r, purpose=None):
        qe_calls.append(purpose)
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    captured = {}

    def fake_pipeline(config, coder_engine, reviewer_engine, initial_prompt, code_files=None):
        captured["config"] = config
        captured["code_files"] = code_files
        return _fake_pair_pipeline_iter_with_non_event()

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)
    monkeypatch.setattr("src.api_server.run_pair_pipeline", fake_pipeline)

    events = []
    async for ev in _route_and_stream(req, "sid-1", "corr-1"):
        events.append(ev)

    assert "config" in captured, "pair_pipeline must be called"
    # The config's project_dir must be the RESOLVED absolute container
    # path, not the relative fragment sent in RunRequest.
    assert str(captured["config"].project_dir) == str(ws_root / "repo"), \
        f"PairPipelineConfig.project_dir must be resolved absolute, got {captured['config'].project_dir}"
    # Bonus pin for the code_files=None contract — the LLM reads files via
    # Read/Glob/Grep tools, so no pre-seeded context should be passed.
    assert captured["code_files"] is None
    assert len(qe_calls) == 2, f"Two engines expected (coder + reviewer), got calls: {qe_calls}"
    # The two engines must be created with different Purposes
    assert Purpose.GENERATE in qe_calls, "GENERATE engine missing"
    assert Purpose.REVIEW in qe_calls, "REVIEW engine missing"
    # The non-StreamEvent (FakeCycleResult) must have been filtered out
    assert len(events) == 2, f"Expected 2 filtered events, got {len(events)}: {events}"
    assert all(isinstance(e, StreamEvent) for e in events)


@pytest.mark.asyncio
async def test_route_filters_non_stream_events(tmp_path, monkeypatch):
    """CycleResult / PairPipelineResult yields must not reach the caller.

    Pins the isinstance(event, StreamEvent) filter: a future refactor
    that drops the filter would cause non-events to leak into the HTTP
    SSE stream as type:unknown entries."""
    monkeypatch.setattr("src.api_server._sessions", {})

    ws_root = tmp_path / "workspaces"
    (ws_root / "repo").mkdir(parents=True)
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(ws_root))

    req = RunRequest(
        project_id=1,
        message="add code",
        workspace_path="repo",
    )

    def fake_qe(r, purpose=None):
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    def fake_pipeline(*args, **kwargs):
        return _fake_pair_pipeline_iter_with_non_event()

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)
    monkeypatch.setattr("src.api_server.run_pair_pipeline", fake_pipeline)

    events = []
    async for ev in _route_and_stream(req, "sid-1", "corr-1"):
        events.append(ev)

    # fake pipeline yields 3 items: 1 StreamEvent, 1 non-event, 1 StreamEvent
    assert len(events) == 2, f"Expected 2 filtered events, got {len(events)}"
    assert all(isinstance(e, StreamEvent) for e in events)


def _fake_pair_pipeline_that_raises():
    async def _gen():
        yield AssistantTextDelta(text="before crash")
        raise RuntimeError("pipeline exploded")
    return _gen()


@pytest.mark.asyncio
async def test_route_pair_pipeline_exception_propagates(tmp_path, monkeypatch):
    """Exceptions in pair_pipeline propagate from _route_and_stream so
    _run_and_publish can catch them and emit an ErrorEvent to Redis/PG.

    Contract pin: events yielded BEFORE the exception must have been
    observable by the caller — that's how _run_and_publish can write
    partial history to Redis + PG before the crash."""
    monkeypatch.setattr("src.api_server._sessions", {})
    ws_root = tmp_path / "workspaces"
    (ws_root / "repo").mkdir(parents=True)
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(ws_root))

    req = RunRequest(
        project_id=1,
        message="add code",
        workspace_path="repo",
    )

    def fake_qe(r, purpose=None):
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    def fake_pipeline(*args, **kwargs):
        return _fake_pair_pipeline_that_raises()

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)
    monkeypatch.setattr("src.api_server.run_pair_pipeline", fake_pipeline)

    events = []
    with pytest.raises(RuntimeError, match="pipeline exploded"):
        async for ev in _route_and_stream(req, "sid-1", "corr-1"):
            events.append(ev)

    # Events yielded BEFORE the exception must have been observable
    assert len(events) == 1
    assert events[0].text == "before crash"
