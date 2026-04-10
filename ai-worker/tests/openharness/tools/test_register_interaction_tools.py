"""Tests for register_interaction_tools helper.

Spec: §2.9.3.b.
"""

from __future__ import annotations

from pathlib import Path
from unittest.mock import AsyncMock

import pytest

from src.openharness.tools.base import ToolRegistry
from src.openharness.tools.interaction_tools import (
    RequestClarificationTool,
    RequestReviewTool,
    register_interaction_tools,
)


def test_registers_both_tools(tmp_path: Path):
    """A single call to register_interaction_tools registers both
    RequestClarificationTool and RequestReviewTool."""
    registry = ToolRegistry()
    router = AsyncMock()

    register_interaction_tools(
        registry=registry,
        model_router=router,
        workspace_dir=tmp_path,
    )

    clarification = registry.get("request_clarification")
    review = registry.get("request_review")

    assert clarification is not None
    assert review is not None
    assert isinstance(clarification, RequestClarificationTool)
    assert isinstance(review, RequestReviewTool)


def test_idempotent_registration_fails_loudly(tmp_path: Path):
    """A second call must raise — duplicate registration indicates
    a bug in the call site."""
    registry = ToolRegistry()
    router = AsyncMock()

    register_interaction_tools(
        registry=registry,
        model_router=router,
        workspace_dir=tmp_path,
    )

    with pytest.raises(ValueError):
        register_interaction_tools(
            registry=registry,
            model_router=router,
            workspace_dir=tmp_path,
        )


def test_review_tool_constructed_with_router_and_workspace(tmp_path: Path):
    """The RequestReviewTool instance must be constructed with the
    same router and workspace_dir passed into the helper."""
    registry = ToolRegistry()
    router = AsyncMock()

    register_interaction_tools(
        registry=registry,
        model_router=router,
        workspace_dir=tmp_path,
    )

    review = registry.get("request_review")
    assert review._router is router
    assert review._workspace_dir == tmp_path
