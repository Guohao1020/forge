"""Happy-path tests for WorkspacePath. Adversarial escape tests live
in test_workspace_path_adversarial.py."""

from pathlib import Path

import pytest

from src.openharness.tools.workspace_path import PathEscapeError, WorkspacePath


@pytest.fixture
def workspace(tmp_path: Path) -> Path:
    """Create a workspace root with a few sample files for path resolution."""
    (tmp_path / "src").mkdir()
    (tmp_path / "src" / "main.go").write_text("package main")
    (tmp_path / "a" / "b" / "c").mkdir(parents=True)
    (tmp_path / "a" / "b" / "c" / "deep.txt").write_text("deep")
    return tmp_path


def test_resolve_simple_file(workspace: Path):
    wp = WorkspacePath.resolve(workspace, "src/main.go")
    assert wp.relative == Path("src/main.go")
    assert wp.absolute == workspace / "src" / "main.go"


def test_resolve_deep_file(workspace: Path):
    wp = WorkspacePath.resolve(workspace, "a/b/c/deep.txt")
    assert wp.relative == Path("a/b/c/deep.txt")
    assert wp.absolute == workspace / "a" / "b" / "c" / "deep.txt"


def test_resolve_workspace_root(workspace: Path):
    """The string '.' should resolve to the workspace root itself."""
    wp = WorkspacePath.resolve(workspace, ".")
    assert wp.absolute == workspace
    assert wp.relative in (Path("."), Path(""))


def test_resolve_current_dir_prefix(workspace: Path):
    """'./src/main.go' should normalize to 'src/main.go'."""
    wp = WorkspacePath.resolve(workspace, "./src/main.go")
    assert wp.relative == Path("src/main.go")


def test_resolve_nonexistent_file_still_works(workspace: Path):
    """Path resolution doesn't require the file to exist."""
    wp = WorkspacePath.resolve(workspace, "src/does_not_exist.go")
    assert wp.absolute == workspace / "src" / "does_not_exist.go"


def test_resolve_is_idempotent_for_valid_paths(workspace: Path):
    wp1 = WorkspacePath.resolve(workspace, "src/main.go")
    wp2 = WorkspacePath.resolve(workspace, str(wp1.relative))
    assert wp1.absolute == wp2.absolute


def test_resolve_preserves_workspace_root_reference(workspace: Path):
    wp = WorkspacePath.resolve(workspace, "src/main.go")
    assert wp.workspace_root == workspace.resolve()


def test_path_escape_error_is_value_error():
    """PathEscapeError should subclass ValueError."""
    assert issubclass(PathEscapeError, ValueError)
