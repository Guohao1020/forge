"""Tests for RequestReviewTool — the reviewer meta-tool.

Spec: §2.9.3.a-g.
"""

from __future__ import annotations

import asyncio
import subprocess
from pathlib import Path
from unittest.mock import AsyncMock

import pytest

from src.openharness.engine.prompts import REVIEWER_DIFF_MAX_BYTES
from src.openharness.tools.base import ToolExecutionContext
from src.openharness.tools.interaction_tools import (
    RequestReviewInput,
    RequestReviewTool,
)


# ---------------------------------------------------------------------------
# Test fixtures
# ---------------------------------------------------------------------------


class _FakeResponse:
    """Minimal mock of LLMResponse with a content attribute."""
    def __init__(self, content: str):
        self.content = content


def _make_mock_router(response):
    """Build a mock ModelRouter whose chat() returns `response`
    or raises if `response` is an Exception."""
    router = AsyncMock()
    if isinstance(response, Exception):
        router.chat.side_effect = response
    else:
        router.chat.return_value = _FakeResponse(response)
    return router


def _make_context(workspace: Path) -> ToolExecutionContext:
    return ToolExecutionContext(
        cwd=workspace,
        tool_use_id="toolu_review_test",
        original_user_request="add a health endpoint",
    )


@pytest.fixture
def real_git_workspace(tmp_path: Path) -> Path:
    """Create a real git workspace with one committed file and one
    staged change."""
    subprocess.run(["git", "init", "-q"], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "config", "user.email", "test@test.com"],
        cwd=tmp_path, check=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "test"],
        cwd=tmp_path, check=True,
    )
    (tmp_path / "main.go").write_text("package main\n\nfunc main() {}\n")
    subprocess.run(["git", "add", "main.go"], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "commit", "-q", "-m", "initial"],
        cwd=tmp_path, check=True,
    )
    # Modify the file so git diff HEAD has output
    (tmp_path / "main.go").write_text(
        'package main\n\nfunc main() { println("hi") }\n'
    )
    return tmp_path


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_happy_path_mocked_router(real_git_workspace: Path):
    """The tool collects git diff, builds the reviewer prompt, calls
    the router, and returns the router's response as ToolResult.output."""
    router = _make_mock_router(response="APPROVE\n")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )

    arguments = RequestReviewInput(
        summary="Added println call to main."
    )
    ctx = _make_context(real_git_workspace)

    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is False
    assert result.output == "APPROVE\n"
    router.chat.assert_called_once()
    call_kwargs = router.chat.call_args.kwargs
    from src.openharness.engine.prompts import REVIEWER_SYSTEM_PROMPT
    assert call_kwargs["system"] == REVIEWER_SYSTEM_PROMPT
    assert call_kwargs["max_tokens"] == 1024


@pytest.mark.asyncio
async def test_revise_response_preserved(real_git_workspace: Path):
    """A REVISE response is returned verbatim."""
    router = _make_mock_router(
        response="REVISE add null check on line 42\nDetails follow."
    )
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    arguments = RequestReviewInput(summary="x")
    ctx = _make_context(real_git_workspace)

    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is False
    assert "REVISE add null check on line 42" in result.output


# ---------------------------------------------------------------------------
# Router exception path
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_router_exception_returns_error(real_git_workspace: Path):
    """When ModelRouter.generate raises, the tool returns is_error=True."""
    from src.openharness.tools.interaction_tools import ModelRouterError

    router = _make_mock_router(
        response=ModelRouterError("no providers configured")
    )
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    arguments = RequestReviewInput(summary="x")
    ctx = _make_context(real_git_workspace)

    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is True
    assert "reviewer unavailable" in result.output
    assert "no providers configured" in result.output


# ---------------------------------------------------------------------------
# Git diff collection
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_collect_git_diff_real_workspace(real_git_workspace: Path):
    """In a real git workspace with an unstaged modification,
    _collect_git_diff returns the diff text."""
    router = _make_mock_router(response="APPROVE")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    diff = await tool._collect_git_diff()
    assert "main.go" in diff
    assert "println" in diff


@pytest.mark.asyncio
async def test_collect_git_diff_truncation(real_git_workspace: Path):
    """If the diff exceeds REVIEWER_DIFF_MAX_BYTES, the result is
    truncated."""
    big_content = "\n".join(
        f"line {i} " + "x" * 80 for i in range(2000)
    )
    (real_git_workspace / "big.txt").write_text(big_content)
    subprocess.run(
        ["git", "add", "big.txt"],
        cwd=real_git_workspace, check=True,
    )

    router = _make_mock_router(response="APPROVE")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    diff = await tool._collect_git_diff()
    assert len(diff.encode("utf-8")) <= REVIEWER_DIFF_MAX_BYTES + 100
    assert "<diff truncated" in diff


@pytest.mark.asyncio
async def test_collect_git_diff_timeout(tmp_path: Path, monkeypatch):
    """If git diff hangs longer than the timeout, the tool catches it."""
    subprocess.run(["git", "init", "-q"], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "config", "user.email", "t@t.com"],
        cwd=tmp_path, check=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "t"],
        cwd=tmp_path, check=True,
    )
    (tmp_path / "f.txt").write_text("x")
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "commit", "-q", "-m", "init"],
        cwd=tmp_path, check=True,
    )

    router = _make_mock_router(response="APPROVE")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=tmp_path,
    )

    async def fake_wait_for(coro, timeout):
        raise asyncio.TimeoutError()

    monkeypatch.setattr(asyncio, "wait_for", fake_wait_for)

    arguments = RequestReviewInput(summary="x")
    ctx = _make_context(tmp_path)
    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is True
    assert "timed out" in result.output.lower() or "timeout" in result.output.lower()


# ---------------------------------------------------------------------------
# Input validation
# ---------------------------------------------------------------------------


def test_input_validation_empty_summary_rejected():
    from pydantic import ValidationError

    with pytest.raises(ValidationError):
        RequestReviewInput(summary="")
