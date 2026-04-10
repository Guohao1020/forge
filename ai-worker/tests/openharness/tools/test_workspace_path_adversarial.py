"""Adversarial path-escape tests for WorkspacePath.resolve.

Each test is a named attack vector from spec section 7.1. A failure in this
file is a P0 security regression — these tests are the mechanical
guarantee that WorkspacePath prevents sandbox escapes.
"""

import os
import sys
from pathlib import Path

import pytest

from src.openharness.tools.workspace_path import PathEscapeError, WorkspacePath

_IS_WINDOWS = sys.platform == "win32"
_SYMLINK_REASON = "symlinks need admin/developer-mode on Windows"


# ---------------------------------------------------------------------------
# The 8 adversarial cases from spec section 7.1
# ---------------------------------------------------------------------------


def test_reject_absolute_path(tmp_path: Path):
    """Absolute paths to anywhere outside the workspace must be rejected."""
    if _IS_WINDOWS:
        # On Windows, Path.is_absolute() requires a drive letter prefix.
        # '/etc/passwd' is NOT absolute on Windows, so use a real Windows path.
        with pytest.raises(PathEscapeError, match="absolute"):
            WorkspacePath.resolve(tmp_path, "C:\\Windows\\System32")
    else:
        with pytest.raises(PathEscapeError, match="absolute"):
            WorkspacePath.resolve(tmp_path, "/etc/passwd")


def test_reject_parent_traversal(tmp_path: Path):
    """A single '..' that climbs out must be rejected."""
    with pytest.raises(PathEscapeError, match="escapes workspace"):
        WorkspacePath.resolve(tmp_path, "../other")


def test_reject_nested_parent_traversal(tmp_path: Path):
    """Nested '..' that resolves to outside the workspace must be rejected."""
    with pytest.raises(PathEscapeError, match="escapes workspace"):
        WorkspacePath.resolve(tmp_path, "a/b/../../../etc")


@pytest.mark.skipif(_IS_WINDOWS, reason=_SYMLINK_REASON)
def test_reject_symlink_pointing_outside(tmp_path: Path):
    """A symlink inside the workspace that points to /etc must be caught."""
    workspace = tmp_path / "ws"
    workspace.mkdir()
    link_path = workspace / "escape"

    target = tmp_path / "outside_file"
    target.write_text("secret")
    os.symlink(target, link_path)

    with pytest.raises(PathEscapeError, match="escapes workspace"):
        WorkspacePath.resolve(workspace, "escape")


def test_reject_null_byte(tmp_path: Path):
    """Null bytes in paths break pathname-based security — must be rejected."""
    with pytest.raises(PathEscapeError, match="null byte"):
        WorkspacePath.resolve(tmp_path, "foo\x00.txt")


def test_reject_empty_string(tmp_path: Path):
    """Empty string is not a valid path."""
    with pytest.raises(PathEscapeError, match="empty"):
        WorkspacePath.resolve(tmp_path, "")


def test_reject_none(tmp_path: Path):
    """None input must be rejected with a clear error, not a TypeError."""
    with pytest.raises(PathEscapeError, match="empty"):
        WorkspacePath.resolve(tmp_path, None)  # type: ignore[arg-type]


def test_reject_deep_relative_that_points_home(tmp_path: Path):
    """A deeply nested relative path that resolves to $HOME or / must be rejected."""
    with pytest.raises(PathEscapeError, match="escapes workspace"):
        WorkspacePath.resolve(tmp_path, "../../../../etc")


# ---------------------------------------------------------------------------
# Defense-in-depth: multiple escape shapes in one path
# ---------------------------------------------------------------------------


def test_reject_mixed_escape(tmp_path: Path):
    """Combination: leading './' + '..' climb + final 'passwd'."""
    with pytest.raises(PathEscapeError):
        WorkspacePath.resolve(tmp_path, "./foo/../../../../../etc/passwd")


@pytest.mark.skipif(_IS_WINDOWS, reason="backslash semantics only meaningful on POSIX")
def test_reject_backslash_escape_on_posix(tmp_path: Path):
    """Backslashes on POSIX are literal chars, not separators.
    On Windows this test is skipped."""
    (tmp_path / "foo\\bar").touch()
    wp = WorkspacePath.resolve(tmp_path, "foo\\bar")
    assert wp.absolute.exists()


# ---------------------------------------------------------------------------
# Symlink-to-root edge: workspace root itself is a symlink
# ---------------------------------------------------------------------------


@pytest.mark.skipif(_IS_WINDOWS, reason=_SYMLINK_REASON)
def test_workspace_root_as_symlink_still_works(tmp_path: Path):
    """If workspace_root is a symlink pointing to a real directory,
    resolve must canonicalize it and still accept legitimate paths."""
    real_ws = tmp_path / "real_workspace"
    real_ws.mkdir()
    (real_ws / "file.txt").write_text("hello")

    link_ws = tmp_path / "link_workspace"
    os.symlink(real_ws, link_ws)

    wp = WorkspacePath.resolve(link_ws, "file.txt")
    assert wp.absolute.read_text() == "hello"
    assert wp.workspace_root == real_ws.resolve()
