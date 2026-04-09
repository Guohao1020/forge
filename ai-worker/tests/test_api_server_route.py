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

from src.api_server import RunRequest
from src.api_server import _create_engine
from src.models.router import Purpose


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
