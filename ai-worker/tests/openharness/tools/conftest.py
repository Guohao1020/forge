"""Shared fixtures for tests under tests/openharness/tools/."""

from pathlib import Path
from typing import Any

import pytest

from src.openharness.tools.base import ToolExecutionContext


@pytest.fixture
def workspace(tmp_path: Path) -> Path:
    """A clean, empty workspace directory unique to each test."""
    return tmp_path


@pytest.fixture
def tool_context(workspace: Path) -> ToolExecutionContext:
    """A default ToolExecutionContext rooted at the per-test workspace."""
    return ToolExecutionContext(cwd=workspace, metadata={})
