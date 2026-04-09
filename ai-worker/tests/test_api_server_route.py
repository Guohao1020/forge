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
