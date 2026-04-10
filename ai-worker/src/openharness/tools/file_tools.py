"""T2 file-operating tools for the Variant B agent.

Six tools:
  - ReadFileTool      (SimpleTool, read-only)
  - WriteFileTool     (SimpleTool)
  - EditFileTool      (SimpleTool)
  - GlobTool          (SimpleTool, read-only)
  - GrepTool          (SimpleTool, read-only)
  - ListDirectoryTool (SimpleTool, read-only)

All tools are scoped to a workspace root at construction time and
consume WorkspacePath.resolve() for path input. Path escapes become
ToolResult(is_error=True, ...) -- tools never raise.
"""

from __future__ import annotations

import asyncio
import io
import json as _json
import os as _os
import re as _re
import shutil as _shutil
from pathlib import Path
from typing import Iterable, List, Optional

from pydantic import BaseModel, Field

from .base import SimpleTool, ToolExecutionContext, ToolRegistry, ToolResult
from .workspace_path import PathEscapeError, WorkspacePath

try:
    import pathspec
except ImportError as e:
    raise ImportError(
        "pathspec is required for GlobTool. Run: pip install pathspec>=0.12"
    ) from e


# ---------------------------------------------------------------------------
# Shared constants
# ---------------------------------------------------------------------------

# Directories whose basename matches any of these are skipped during
# recursive operations (glob, grep, list_directory). No configuration
# knob in Phase 3.
DEFAULT_IGNORE_DIRS = frozenset(
    {
        ".git",
        "node_modules",
        ".venv",
        "venv",
        "__pycache__",
        "dist",
        "build",
        "target",
        ".next",
        ".gradle",
        ".cache",
    }
)

# Per-call caps for read_file output
READ_FILE_MAX_LINES = 2000
READ_FILE_MAX_BYTES = 200_000
# First N bytes we sniff for null bytes to detect binaries
BINARY_SNIFF_BYTES = 8192


# ---------------------------------------------------------------------------
# ReadFileTool
# ---------------------------------------------------------------------------


class ReadFileInput(BaseModel):
    path: str = Field(..., description="File path relative to workspace root")
    start_line: Optional[int] = Field(
        None,
        description="1-indexed first line to read. Default: 1 (start of file).",
        ge=1,
    )
    limit: Optional[int] = Field(
        None,
        description=(
            f"Max lines to read. Default and max: {READ_FILE_MAX_LINES}. "
            "Use this plus start_line to page through large files."
        ),
        ge=1,
    )


class ReadFileTool(SimpleTool):
    name = "read_file"
    description = (
        "Read a file from the project workspace. Returns the file contents "
        "with cat -n-style line numbers (strip them before passing content "
        "to edit_file). Use start_line and limit to read portions of large "
        "files. Rejects binary files. Default cap: 2000 lines or 200 KB."
    )
    input_model = ReadFileInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True

    async def _execute_simple(
        self,
        arguments: ReadFileInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        # 1. Path resolution
        try:
            wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
        except PathEscapeError as e:
            return ToolResult(is_error=True, output=f"Path escape: {e}")

        abs_path = wp.absolute

        # 2. Existence and type check
        if not abs_path.exists():
            return ToolResult(
                is_error=True,
                output=f"File not found: {arguments.path}",
            )
        if abs_path.is_dir():
            return ToolResult(
                is_error=True,
                output=(
                    f"{arguments.path} is a directory, not a file. "
                    "Use list_directory or glob to explore directories."
                ),
            )

        # 3. Binary-file detection (first 8KB sniff)
        try:
            with abs_path.open("rb") as fb:
                head = fb.read(BINARY_SNIFF_BYTES)
        except OSError as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot open {arguments.path}: {e}",
            )
        if b"\x00" in head:
            return ToolResult(
                is_error=True,
                output=(
                    f"{arguments.path} appears to be a binary file "
                    "(null byte detected in first 8KB). "
                    "read_file only supports text files."
                ),
            )

        # 4. Read as text
        try:
            text = abs_path.read_text(encoding="utf-8", errors="replace")
        except OSError as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot read {arguments.path}: {e}",
            )

        # 5. Empty file early return
        if text == "":
            return ToolResult(output="(empty file)")

        # 6. Slicing by start_line + limit
        all_lines = text.splitlines(keepends=True)
        total_lines = len(all_lines)

        start = (arguments.start_line or 1) - 1  # convert 1-indexed to 0-indexed
        limit = arguments.limit or READ_FILE_MAX_LINES
        # Clamp limit to the hard cap
        effective_limit = min(limit, READ_FILE_MAX_LINES)

        # Range: [start, start + effective_limit)
        end = min(start + effective_limit, total_lines)
        selected = all_lines[start:end]

        # 7. Apply byte cap as well (secondary safety net)
        out_buf = io.StringIO()
        bytes_used = 0
        lines_used = 0
        hit_byte_cap = False

        for offset, raw_line in enumerate(selected):
            line_num = start + offset + 1
            # Right-align line number in 6-char field, then tab, then content
            prefix = f"{line_num:>6}\t"
            line_out = prefix + raw_line
            line_bytes = len(line_out.encode("utf-8"))
            if bytes_used + line_bytes > READ_FILE_MAX_BYTES:
                hit_byte_cap = True
                break
            out_buf.write(line_out)
            bytes_used += line_bytes
            lines_used += 1

        # 8. Truncation notes
        output = out_buf.getvalue()

        if hit_byte_cap:
            output += (
                f"\n... [truncated at {READ_FILE_MAX_BYTES} bytes, "
                f"showing {lines_used} lines starting at line {start + 1} "
                f"of {total_lines} total]"
            )
        elif end < total_lines:
            output += (
                f"\n... [truncated, showing lines {start + 1}-{end} "
                f"of {total_lines} total; use start_line and limit to page]"
            )

        return ToolResult(output=output)


# ---------------------------------------------------------------------------
# WriteFileTool
# ---------------------------------------------------------------------------


class WriteFileInput(BaseModel):
    path: str = Field(..., description="File path relative to workspace root")
    content: str = Field(
        ...,
        description=(
            "Full contents to write. Overwrites any existing file. "
            "Parent directories are created automatically."
        ),
    )


class WriteFileTool(SimpleTool):
    name = "write_file"
    description = (
        "Create a new file or overwrite an existing file. Parent "
        "directories are created automatically. For small modifications "
        "to existing files, prefer edit_file (less error-prone than "
        "rewriting entire files)."
    )
    input_model = WriteFileInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        return False

    async def _execute_simple(
        self,
        arguments: WriteFileInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        try:
            wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
        except PathEscapeError as e:
            return ToolResult(is_error=True, output=f"Path escape: {e}")

        abs_path = wp.absolute

        # Can't write to workspace root itself (it's a dir)
        if abs_path == self._workspace_root.resolve() or abs_path.is_dir():
            return ToolResult(
                is_error=True,
                output=(
                    f"{arguments.path} is a directory, not a file. "
                    "write_file can only create or overwrite files."
                ),
            )

        # Create parent dirs
        try:
            abs_path.parent.mkdir(parents=True, exist_ok=True)
        except OSError as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot create parent directory for {arguments.path}: {e}",
            )

        # Write the file
        try:
            abs_path.write_text(arguments.content, encoding="utf-8")
        except OSError as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot write {arguments.path}: {e}",
            )

        line_count = arguments.content.count("\n")
        if arguments.content and not arguments.content.endswith("\n"):
            line_count += 1
        byte_count = len(arguments.content.encode("utf-8"))

        return ToolResult(
            output=f"Wrote {line_count} line(s), {byte_count} byte(s) to {arguments.path}"
        )


# ---------------------------------------------------------------------------
# EditFileTool
# ---------------------------------------------------------------------------


class EditFileInput(BaseModel):
    path: str = Field(..., description="File path relative to workspace root")
    old_string: str = Field(
        ...,
        description=(
            "Exact text to replace. Must appear exactly once in the file "
            "unless replace_all=True. No regex, no fuzzy match -- this is "
            "a literal string comparison. Include surrounding context to "
            "disambiguate if the core text you want to change appears "
            "multiple times."
        ),
    )
    new_string: str = Field(..., description="Replacement text.")
    replace_all: bool = Field(
        False,
        description=(
            "If True, replace ALL occurrences of old_string. If False "
            "(default), replace exactly one and error if there are more."
        ),
    )


def _count_lines(s: str) -> int:
    """Count lines in a string. An empty string has 0 lines.
    A string with content but no trailing newline has N+1 lines
    where N is the count of embedded newlines.
    """
    if not s:
        return 0
    n = s.count("\n")
    if not s.endswith("\n"):
        n += 1
    return n


class EditFileTool(SimpleTool):
    name = "edit_file"
    description = (
        "Replace an exact string in an existing file. The old_string must "
        "appear exactly once in the file unless replace_all is True. This "
        "is the preferred way to modify code -- it's less error-prone than "
        "rewriting entire files. Use read_file first to see exact content "
        "(strip the line-number prefix before passing into old_string)."
    )
    input_model = EditFileInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        return False

    async def _execute_simple(
        self,
        arguments: EditFileInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        # Path resolution
        try:
            wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
        except PathEscapeError as e:
            return ToolResult(is_error=True, output=f"Path escape: {e}")

        abs_path = wp.absolute

        if not abs_path.exists():
            return ToolResult(
                is_error=True,
                output=(
                    f"File not found: {arguments.path}. "
                    "edit_file requires an existing file. "
                    "Use write_file to create a new file."
                ),
            )
        if abs_path.is_dir():
            return ToolResult(
                is_error=True,
                output=(
                    f"{arguments.path} is a directory, not a file. "
                    "edit_file only works on files."
                ),
            )

        try:
            original = abs_path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError) as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot read {arguments.path}: {e}",
            )

        # Count occurrences of old_string
        occurrences = original.count(arguments.old_string)

        if occurrences == 0:
            return ToolResult(
                is_error=True,
                output=(
                    f"old_string not found in {arguments.path}. "
                    "Use read_file to see the exact current content "
                    "(remember to strip the line-number prefix)."
                ),
            )

        if occurrences > 1 and not arguments.replace_all:
            return ToolResult(
                is_error=True,
                output=(
                    f"old_string appears {occurrences} times in {arguments.path}. "
                    "Either add more surrounding context to make it unique, "
                    "or set replace_all=True to replace every occurrence."
                ),
            )

        # Perform the replacement
        if arguments.replace_all:
            new_content = original.replace(arguments.old_string, arguments.new_string)
            num_replaced = occurrences
        else:
            new_content = original.replace(arguments.old_string, arguments.new_string, 1)
            num_replaced = 1

        # Write back
        try:
            abs_path.write_text(new_content, encoding="utf-8")
        except OSError as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot write {arguments.path}: {e}",
            )

        # Report the delta
        old_lines = _count_lines(arguments.old_string) * num_replaced
        new_lines = _count_lines(arguments.new_string) * num_replaced

        if arguments.replace_all and num_replaced > 1:
            return ToolResult(
                output=(
                    f"Replaced {num_replaced} occurrences in {arguments.path} "
                    f"(+{new_lines} -{old_lines} line(s))"
                )
            )
        else:
            return ToolResult(
                output=(
                    f"Replaced in {arguments.path} "
                    f"(+{new_lines} -{old_lines} line(s))"
                )
            )


# ---------------------------------------------------------------------------
# GlobTool
# ---------------------------------------------------------------------------


GLOB_MAX_RESULTS = 200


class GlobInput(BaseModel):
    pattern: str = Field(
        ...,
        description=(
            "Glob pattern like '**/*.go', 'src/**/*.{ts,tsx}', or 'main.go'. "
            "Uses gitignore-style matching (pathspec library). Brace "
            "expansion ({ts,tsx}) is supported. Recursive patterns use **."
        ),
    )
    path: Optional[str] = Field(
        None,
        description=(
            "Subdirectory (relative to workspace root) to search from. "
            "Default: workspace root."
        ),
    )


def _expand_braces(pattern: str) -> List[str]:
    """Expand brace patterns like '*.{ts,tsx}' into ['*.ts', '*.tsx'].

    Handles nested braces simplistically (only one layer). Returns
    [pattern] unchanged if there are no braces.
    """
    m = _re.search(r"\{([^{}]*)\}", pattern)
    if not m:
        return [pattern]
    options = m.group(1).split(",")
    expanded = []
    for opt in options:
        sub = pattern[: m.start()] + opt.strip() + pattern[m.end() :]
        expanded.extend(_expand_braces(sub))  # recurse for more braces
    return expanded


def _iter_files(root: Path) -> Iterable[Path]:
    """Walk root recursively, skipping DEFAULT_IGNORE_DIRS at any depth.
    Yields file paths (not directories).
    """
    for dirpath, dirnames, filenames in _os.walk(root):
        # Prune ignore dirs so we don't recurse into them
        dirnames[:] = [d for d in dirnames if d not in DEFAULT_IGNORE_DIRS]
        for name in filenames:
            yield Path(dirpath) / name


class GlobTool(SimpleTool):
    name = "glob"
    description = (
        "Find files matching a glob pattern. Returns paths sorted by "
        "modification time (most recent first). Supports gitignore-style "
        "patterns with ** recursion and {a,b} brace expansion. Ignores "
        ".git/, node_modules/, .venv/, __pycache__/, dist/, build/, "
        "target/, .next/, .gradle/, .cache/ automatically. Result cap: "
        "200 matches."
    )
    input_model = GlobInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True

    async def _execute_simple(
        self,
        arguments: GlobInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        # Resolve the search root (workspace root or a subdirectory)
        if arguments.path:
            try:
                wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
            except PathEscapeError as e:
                return ToolResult(is_error=True, output=f"Path escape: {e}")
            search_root = wp.absolute
        else:
            search_root = self._workspace_root.resolve()

        if not search_root.exists():
            return ToolResult(
                is_error=True,
                output=f"Search path not found: {arguments.path or '.'}",
            )
        if not search_root.is_dir():
            return ToolResult(
                is_error=True,
                output=f"Search path is not a directory: {arguments.path}",
            )

        # Build the pathspec matcher from possibly-brace-expanded patterns
        expanded = _expand_braces(arguments.pattern)
        try:
            spec = pathspec.PathSpec.from_lines("gitignore", expanded)
        except Exception as e:
            return ToolResult(
                is_error=True,
                output=f"Invalid glob pattern {arguments.pattern!r}: {e}",
            )

        # Walk and filter
        matches: List[tuple[float, Path]] = []
        for file_path in _iter_files(search_root):
            # pathspec wants a path RELATIVE to the search root
            try:
                rel = file_path.relative_to(search_root)
            except ValueError:
                continue
            rel_str = str(rel).replace("\\", "/")  # normalize for pathspec on Windows
            if spec.match_file(rel_str):
                try:
                    mtime = file_path.stat().st_mtime
                except OSError:
                    mtime = 0.0
                matches.append((mtime, file_path))

        if not matches:
            return ToolResult(output=f"No matches for pattern {arguments.pattern!r}")

        # Sort by mtime descending, newest first
        matches.sort(key=lambda pair: pair[0], reverse=True)
        total = len(matches)

        # Cap at GLOB_MAX_RESULTS
        capped = matches[:GLOB_MAX_RESULTS]

        # Render paths relative to WORKSPACE ROOT (not search root) for
        # consistent output
        lines: List[str] = []
        for _, abs_p in capped:
            try:
                rel_to_ws = abs_p.relative_to(self._workspace_root.resolve())
                lines.append(str(rel_to_ws).replace("\\", "/"))
            except ValueError:
                lines.append(str(abs_p))

        output = "\n".join(lines)
        if total > GLOB_MAX_RESULTS:
            output += f"\n... ({total - GLOB_MAX_RESULTS} more matches truncated, showing first {GLOB_MAX_RESULTS} by mtime)"

        return ToolResult(output=output)


# ---------------------------------------------------------------------------
# GrepTool
# ---------------------------------------------------------------------------


GREP_MAX_RESULT_LINES = 500
GREP_MAX_RESULT_BYTES = 200_000
GREP_TIMEOUT_SECS = 30


class GrepInput(BaseModel):
    pattern: str = Field(..., description="Regex pattern (Rust regex syntax)")
    path: Optional[str] = Field(
        None,
        description="Subdirectory (relative to workspace root) to search from",
    )
    file_glob: Optional[str] = Field(
        None,
        description=(
            "Optional glob to limit which files are searched "
            "(e.g. '*.go', '**/*.py'). Passed to ripgrep as -g <glob>."
        ),
    )
    case_insensitive: bool = Field(
        False,
        description="If True, match case-insensitively (ripgrep -i).",
    )


class GrepTool(SimpleTool):
    name = "grep"
    description = (
        "Search file contents using regex. Returns matching lines in "
        "'path:line:content' format. Uses ripgrep under the hood -- "
        "respects .gitignore by default, skips hidden directories, and "
        "is fast on large trees. Cap: 500 result lines or 200 KB."
    )
    input_model = GrepInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True

    async def _execute_simple(
        self,
        arguments: GrepInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        rg_path = _shutil.which("rg")
        if rg_path is None:
            return ToolResult(
                is_error=True,
                output=(
                    "ripgrep (rg) not found on PATH. The ai-worker "
                    "container installs it in Phase 0 Dockerfile update; "
                    "if you're running outside the container, install "
                    "ripgrep and retry."
                ),
            )

        # Resolve the search root
        if arguments.path:
            try:
                wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
            except PathEscapeError as e:
                return ToolResult(is_error=True, output=f"Path escape: {e}")
            search_root = wp.absolute
        else:
            search_root = self._workspace_root.resolve()

        if not search_root.exists():
            return ToolResult(
                is_error=True,
                output=f"Search path not found: {arguments.path or '.'}",
            )

        # Build the rg command
        cmd = [
            rg_path,
            "--json",
            "--no-heading",
            "--color=never",
            "--no-messages",
        ]
        if arguments.case_insensitive:
            cmd.append("-i")
        if arguments.file_glob:
            cmd.extend(["-g", arguments.file_glob])
        cmd.append(arguments.pattern)
        cmd.append(str(search_root))

        # Run rg
        try:
            proc = await asyncio.create_subprocess_exec(
                *cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(), timeout=GREP_TIMEOUT_SECS
            )
        except asyncio.TimeoutError:
            return ToolResult(
                is_error=True,
                output=f"grep timed out after {GREP_TIMEOUT_SECS} seconds",
            )
        except OSError as e:
            return ToolResult(is_error=True, output=f"rg failed to start: {e}")

        # ripgrep exit codes: 0 = matches found, 1 = no matches, 2 = error
        if proc.returncode == 1:
            return ToolResult(output=f"No matches for pattern {arguments.pattern!r}")
        if proc.returncode != 0:
            err = stderr.decode("utf-8", errors="replace").strip()
            return ToolResult(
                is_error=True,
                output=f"rg error (exit {proc.returncode}): {err or '(no stderr)'}",
            )

        # Parse JSON stream: one object per line
        result_lines: List[str] = []
        for raw_line in stdout.splitlines():
            if not raw_line.strip():
                continue
            try:
                obj = _json.loads(raw_line)
            except _json.JSONDecodeError:
                continue
            if obj.get("type") != "match":
                continue
            data = obj.get("data", {})
            file_text = data.get("path", {}).get("text", "")
            line_num = data.get("line_number", 0)
            line_text = data.get("lines", {}).get("text", "").rstrip("\n")

            # Normalize file_text to workspace-relative
            try:
                rel = Path(file_text).relative_to(self._workspace_root.resolve())
                display_path = str(rel).replace("\\", "/")
            except ValueError:
                display_path = file_text

            result_lines.append(f"{display_path}:{line_num}:{line_text}")

        if not result_lines:
            return ToolResult(output=f"No matches for pattern {arguments.pattern!r}")

        total = len(result_lines)
        # Apply caps
        capped = result_lines[:GREP_MAX_RESULT_LINES]
        output = "\n".join(capped)

        if len(output.encode("utf-8")) > GREP_MAX_RESULT_BYTES:
            # Trim by lines until under byte cap
            while len(output.encode("utf-8")) > GREP_MAX_RESULT_BYTES and capped:
                capped.pop()
                output = "\n".join(capped)
            output += f"\n... [truncated at {GREP_MAX_RESULT_BYTES} bytes, showing {len(capped)} of {total} matches]"
        elif total > GREP_MAX_RESULT_LINES:
            output += f"\n... ({total - GREP_MAX_RESULT_LINES} more matches truncated, showing first {GREP_MAX_RESULT_LINES})"

        return ToolResult(output=output)


# ---------------------------------------------------------------------------
# ListDirectoryTool
# ---------------------------------------------------------------------------


LIST_DIR_MAX_ENTRIES = 500


class ListDirectoryInput(BaseModel):
    path: str = Field(
        ".",
        description=(
            "Directory path relative to workspace root. Default: "
            "workspace root itself."
        ),
    )


class ListDirectoryTool(SimpleTool):
    name = "list_directory"
    description = (
        "List the contents of a directory, one level deep. Directories "
        "appear first with a trailing slash; files are alphabetical "
        "within each group. Skips .git/, node_modules/, .venv/, and "
        "other common build/cache directories. For recursive exploration "
        "use glob instead."
    )
    input_model = ListDirectoryInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True

    async def _execute_simple(
        self,
        arguments: ListDirectoryInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        # Resolve path
        try:
            wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
        except PathEscapeError as e:
            return ToolResult(is_error=True, output=f"Path escape: {e}")

        abs_path = wp.absolute
        if not abs_path.exists():
            return ToolResult(
                is_error=True,
                output=f"Directory not found: {arguments.path}",
            )
        if not abs_path.is_dir():
            return ToolResult(
                is_error=True,
                output=f"{arguments.path} is not a directory (it's a file).",
            )

        # List entries
        try:
            entries = list(abs_path.iterdir())
        except OSError as e:
            return ToolResult(
                is_error=True,
                output=f"Cannot list {arguments.path}: {e}",
            )

        # Filter out ignored names + split dirs vs files
        dirs: List[str] = []
        files: List[str] = []
        for entry in entries:
            if entry.name in DEFAULT_IGNORE_DIRS:
                continue
            if entry.is_dir():
                dirs.append(entry.name + "/")
            else:
                files.append(entry.name)

        dirs.sort()
        files.sort()
        all_lines = dirs + files

        total = len(all_lines)
        capped = all_lines[:LIST_DIR_MAX_ENTRIES]
        output = "\n".join(capped)

        if total == 0:
            return ToolResult(output="(empty directory)")
        if total > LIST_DIR_MAX_ENTRIES:
            output += (
                f"\n... ({total - LIST_DIR_MAX_ENTRIES} more entries truncated, "
                f"showing first {LIST_DIR_MAX_ENTRIES} sorted dirs-then-files)"
            )

        return ToolResult(output=output)


# ---------------------------------------------------------------------------
# Registration helper
# ---------------------------------------------------------------------------


def register_file_tools(registry: ToolRegistry, workspace_root: Path) -> None:
    """Register all six T2 file tools against the given ToolRegistry.

    The workspace_root is the directory all tools will be scoped to.
    Typically computed by the agent service (Phase 5) from the
    workspace.Manager.ProjectDir plus FORGE_WORKSPACE_ROOT env var.
    """
    registry.register(ReadFileTool(workspace_root))
    registry.register(WriteFileTool(workspace_root))
    registry.register(EditFileTool(workspace_root))
    registry.register(GlobTool(workspace_root))
    registry.register(GrepTool(workspace_root))
    registry.register(ListDirectoryTool(workspace_root))
