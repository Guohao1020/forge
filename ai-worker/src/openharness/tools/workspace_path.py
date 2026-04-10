"""WorkspacePath -- a path guaranteed to be inside a workspace sandbox.

This is the silicon-valley-grade answer to "how do we prevent path
escapes in file tools?". Rather than making every tool author remember
to call a sanitize_path() helper, path safety is a type contract:
the only way to construct a WorkspacePath is via
WorkspacePath.resolve(workspace_root, user_path), which raises
PathEscapeError if the input would escape the workspace.

Escape cases covered:
- Absolute paths: "/etc/passwd"
- Parent traversal: "../other"
- Deep parent traversal: "a/b/../../../etc"
- Null bytes: "foo\\x00bar"
- Symlink out: symlink inside workspace pointing outside
"""

from __future__ import annotations

from pathlib import Path


class PathEscapeError(ValueError):
    """Raised when a user-provided path would escape the workspace sandbox."""


class WorkspacePath:
    """A path guaranteed to be inside a workspace sandbox.

    Never construct directly. Use WorkspacePath.resolve() which
    enforces escape checks at construction time.

    Attributes:
        workspace_root: The absolute, resolved workspace root directory.
        relative: The safe relative path from workspace_root to the target.
    """

    __slots__ = ("workspace_root", "relative")

    def __init__(self, workspace_root: Path, relative: Path) -> None:
        self.workspace_root = workspace_root
        self.relative = relative

    @classmethod
    def resolve(cls, workspace_root: Path, user_path: str) -> "WorkspacePath":
        """Resolve a user-provided path against the workspace root.

        Raises PathEscapeError if the path:
          - is absolute (starts with /)
          - contains a null byte
          - resolves to a location outside workspace_root
          - contains '..' segments in the resolved relative path
        """
        if user_path is None:
            raise PathEscapeError("empty path (None)")
        if user_path == "":
            raise PathEscapeError("empty path")
        if "\x00" in user_path:
            raise PathEscapeError(f"path contains null byte: {user_path!r}")

        p = Path(user_path)
        if p.is_absolute():
            raise PathEscapeError(f"absolute path not allowed: {user_path}")

        root_abs = workspace_root.resolve()
        target_abs = (root_abs / p).resolve()

        try:
            relative = target_abs.relative_to(root_abs)
        except ValueError:
            raise PathEscapeError(
                f"path escapes workspace: {user_path!r} resolved to {target_abs}"
            )

        if any(part == ".." for part in relative.parts):
            raise PathEscapeError(
                f"path contains '..' segments after resolve: {user_path!r}"
            )

        return cls(root_abs, relative)

    @property
    def absolute(self) -> Path:
        """Full absolute path for filesystem operations."""
        return self.workspace_root / self.relative

    def __repr__(self) -> str:
        return f"WorkspacePath(root={self.workspace_root!r}, rel={self.relative!r})"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, WorkspacePath):
            return NotImplemented
        return (
            self.workspace_root == other.workspace_root
            and self.relative == other.relative
        )

    def __hash__(self) -> int:
        return hash((self.workspace_root, self.relative))
