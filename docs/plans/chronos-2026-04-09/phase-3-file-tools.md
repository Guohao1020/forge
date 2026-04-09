# chronos · Phase 3 — T2 File Tools (Python)

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 3 of 7 · **Tasks:** 8 · **Depends on:** [Phase 2](phase-2-basetool.md) · **Unblocks:** Phase 5
> **Spec reference:** [Design spec §4.3 (File tools) + §4.2 (WorkspacePath integration) + §7.1 (adversarial tests)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Implement six file-operating tools that form the read/write/search surface of the Variant B agent:

1. **`ReadFileTool`** — read a file (or a byte range) with line numbers, reject binaries, truncate at cap
2. **`WriteFileTool`** — create or overwrite a file, auto-create parent dirs
3. **`EditFileTool`** — exact-string replacement with Claude Code's unique-match contract
4. **`GlobTool`** — find files by glob pattern (uses `pathspec`, mtime-sorted, ignores `.git/` etc)
5. **`GrepTool`** — content search via ripgrep subprocess
6. **`ListDirectoryTool`** — one-level directory listing, dirs first

All six are `SimpleTool` subclasses (no mid-execution events needed — simple input → result). All consume `WorkspacePath.resolve` on their path input, so any path escape becomes `ToolResult(is_error=True)`. All are read-only tools except `WriteFileTool` and `EditFileTool`.

**Completion gate:**
- `pytest ai-worker/tests/openharness/tools/test_file_tools.py -v` passes with full coverage (happy path + error paths + truncation + edge cases per tool)
- `pytest ai-worker/tests/openharness/tools/test_base_tool_contract.py -v` passes with **10 tool classes** in `ALL_TOOL_SPECS` (4 context tools from Phase 2 + 6 new file tools)
- `pytest ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py -v` still passes (nothing broken in the path sandbox)
- `pytest ai-worker/tests/test_tool_registry.py` still passes
- `grep -n "import.*file_tools" ai-worker/src/openharness/tools/` shows the new module is registered
- The agent loop (`query.py`) does not require changes in this phase — the contract (Phase 2) already absorbed the tool surface

## Why this phase matters

This is the **literal hands of the agent**. Without these six tools, the agent can only query profiles and context — it physically cannot read a `.go` file in the workspace, cannot write a new handler, cannot search for a symbol. Phase 1 built the workspace directory; Phase 2 built the contract; Phase 3 puts hands on the body.

Every tool here is small but has opinionated defaults that matter:
- `ReadFileTool` returns **line numbers** so `EditFileTool` gets a stable point-of-reference
- `EditFileTool` uses **unique-match semantics** (fail if old_string appears 0 or 2+ times) because LLMs produce clean edits that way — this is Claude Code's empirical finding and it's been battle-tested
- `GlobTool` uses `pathspec` over `fnmatch` because pathspec has real gitignore semantics (`!foo.go` negation, `**` meaning, etc.)
- `GrepTool` **requires** ripgrep — no Python fallback — because a fallback would be 100x slower and maintenance doubles
- `ListDirectoryTool` does **one level only** — if the agent wants deeper exploration, it uses `glob`; this keeps each tool focused

**Silicon-valley rule for this phase:** every tool has an adversarial mindset in its error handling. Paths are validated at the type level via `WorkspacePath`. File-not-found, permission-denied, binary-file-rejected are **returned as `ToolResult(is_error=True)`**, never raised. The agent sees the error message and can react; the agent loop never crashes because a tool threw.

---

## Shared conventions for all Phase 3 tools

These apply to every task below. Stating them once here instead of repeating in each task:

**Constructor signature:**
```python
class ReadFileTool(SimpleTool):
    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root
```
The workspace root is captured at construction time and used to resolve `WorkspacePath`s in `_execute_simple`. Passing it through `ToolExecutionContext.cwd` would also work, but the spec's decision is constructor injection — it's explicit about "this tool is scoped to this workspace".

**Ignore list (shared between Glob, Grep, ListDirectory):**
```python
DEFAULT_IGNORE_DIRS = frozenset({
    ".git", "node_modules", ".venv", "venv",
    "__pycache__", "dist", "build", "target",
    ".next", ".gradle", ".cache",
})
```
A directory whose **basename** is in this set is skipped entirely during recursion. No configuration knob in this phase — just the frozen set. When a future task needs a custom ignore list, it goes in a new version of this constant (not a mutable knob passed through context).

**Error response shape:**
All tools return `ToolResult(is_error=True, output="<actionable message>")` on expected failures. The agent sees the message in `ToolResultBlock.content` and can recover. Python exceptions are never allowed to escape `_execute_simple`; if one does, that's a bug and the contract test (Phase 2) catches it.

**Path handling:**
Every tool that takes a `path` input does this dance at the start of `_execute_simple`:
```python
try:
    wp = WorkspacePath.resolve(self._workspace_root, arguments.path)
except PathEscapeError as e:
    return ToolResult(is_error=True, output=f"Path escape: {e}")
```
The `WorkspacePath` result is then used for all subsequent filesystem operations.

---

### Task 3.1: `ReadFileTool`

**Files:**
- Create: `ai-worker/src/openharness/tools/file_tools.py` (starts with `ReadFileTool`; other tools added in subsequent tasks)
- Create: `ai-worker/tests/openharness/tools/test_file_tools.py` (starts with read_file tests)

**Context:** Reads a file, optionally a line range. Returns content with `cat -n`-style line numbers so `EditFileTool` has a stable reference frame. Rejects binary files (null byte in first 8KB is the heuristic — not perfect but catches common cases like images, native binaries, compiled .class files).

Caps: 2000 lines OR 200 KB, whichever fires first. On truncation, append a one-line note. The cap is **per-tool-call** — the agent can request `start_line=2001, limit=2000` to read the next 2000 lines of a 5000-line file.

**Line number format:**
```
   1	package main
   2	
   3	import "fmt"
```
Six-character right-aligned line number, tab, content. This is the exact format `cat -n` uses on Linux. LLMs recognize it and `EditFileTool` callers can produce `old_string`s that include line-number-stripped content.

Actually wait — if line numbers are in the output, the LLM might copy them into `old_string` verbatim and the match will fail. Let me look at what Claude Code does: it returns line numbers as a prefix but the LLM is prompted to strip them when using `edit_file`. That prompt instruction needs to live in the agent's system prompt (Phase 5).

**Decision for this phase:** output DOES include line numbers. System prompt will warn the agent to strip them when passing content to `edit_file`. This mirrors Claude Code's design exactly.

- [ ] **Step 1: Write the failing ReadFileTool tests**

Create `ai-worker/tests/openharness/tools/test_file_tools.py`:

```python
"""Unit tests for the T2 file-operating tools.

All six tools (read, write, edit, glob, grep, list_directory) live in
file_tools.py. Each has its own section below. Shared fixtures come
from conftest.py in the same directory.

Run only this file: pytest tests/openharness/tools/test_file_tools.py -v
"""

from pathlib import Path

import pytest

from src.openharness.tools.base import ToolExecutionContext, ToolResult
from src.openharness.tools.file_tools import ReadFileTool

# ---------------------------------------------------------------------------
# ReadFileTool
# ---------------------------------------------------------------------------


async def _run_tool(tool, arguments, context):
    """Helper: consume the SimpleTool generator and return the ToolResult."""
    results = []
    async for item in tool.execute(arguments, context):
        results.append(item)
    # SimpleTool yields exactly one ToolResult
    assert len(results) == 1
    assert isinstance(results[0], ToolResult)
    return results[0]


@pytest.fixture
def read_tool(workspace):
    return ReadFileTool(workspace)


@pytest.fixture
def sample_file(workspace):
    path = workspace / "src" / "main.go"
    path.parent.mkdir(parents=True)
    path.write_text(
        "package main\n"
        "\n"
        'import "fmt"\n'
        "\n"
        "func main() {\n"
        '\tfmt.Println("hello")\n'
        "}\n"
    )
    return path


@pytest.mark.asyncio
async def test_read_file_basic(read_tool, tool_context, sample_file):
    from src.openharness.tools.file_tools import ReadFileInput

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="src/main.go"),
        tool_context,
    )
    assert not result.is_error
    # Output includes line numbers
    assert "     1\tpackage main" in result.output
    assert "     3\timport \"fmt\"" in result.output
    # Last line number is 7
    assert "     7\t}" in result.output


@pytest.mark.asyncio
async def test_read_file_with_start_line(read_tool, tool_context, sample_file):
    from src.openharness.tools.file_tools import ReadFileInput

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="src/main.go", start_line=3),
        tool_context,
    )
    assert not result.is_error
    # First line in output should be line 3 of the file
    assert "     3\timport \"fmt\"" in result.output
    # Line 1 and 2 should not appear
    assert "     1\tpackage main" not in result.output


@pytest.mark.asyncio
async def test_read_file_with_limit(read_tool, tool_context, sample_file):
    from src.openharness.tools.file_tools import ReadFileInput

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="src/main.go", start_line=1, limit=2),
        tool_context,
    )
    assert not result.is_error
    assert "     1\tpackage main" in result.output
    assert "     2\t" in result.output  # line 2 is the blank line
    assert "     3\t" not in result.output  # line 3 should be truncated


@pytest.mark.asyncio
async def test_read_file_not_found(read_tool, tool_context):
    from src.openharness.tools.file_tools import ReadFileInput

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="does_not_exist.go"),
        tool_context,
    )
    assert result.is_error
    assert "not found" in result.output.lower() or "no such" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_is_directory(read_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ReadFileInput

    (workspace / "a_dir").mkdir()
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="a_dir"),
        tool_context,
    )
    assert result.is_error
    assert "director" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_rejects_binary(read_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ReadFileInput

    # Write a file with a null byte in the first 8KB
    binary = workspace / "image.png"
    binary.write_bytes(b"PNG\x00\x01\x02\x03" + b"x" * 100)
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="image.png"),
        tool_context,
    )
    assert result.is_error
    assert "binary" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_line_cap(read_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ReadFileInput

    # Write a file with 2500 lines (over the 2000-line default cap)
    big = workspace / "big.txt"
    big.write_text("".join(f"line {i}\n" for i in range(1, 2501)))

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="big.txt"),
        tool_context,
    )
    assert not result.is_error
    # Default cap is 2000 lines; should have truncation note
    assert "truncated" in result.output.lower()
    assert "2500" in result.output  # mentions total line count
    # Line 2001 should NOT be in the output (cap stops at 2000)
    assert "line 2001" not in result.output
    # Line 1 SHOULD be in the output
    assert "line 1\n" in result.output or "line 1 " in result.output


@pytest.mark.asyncio
async def test_read_file_byte_cap(read_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ReadFileInput

    # Write a file with a few long lines totalling > 200KB
    big = workspace / "bigline.txt"
    big.write_text("".join(f"line {i} {'x' * 2000}\n" for i in range(1, 200)))

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="bigline.txt"),
        tool_context,
    )
    assert not result.is_error
    assert "truncated" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_rejects_path_escape(read_tool, tool_context):
    from src.openharness.tools.file_tools import ReadFileInput

    result = await _run_tool(
        read_tool,
        ReadFileInput(path="../etc/passwd"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_empty_file(read_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ReadFileInput

    (workspace / "empty.txt").write_text("")
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="empty.txt"),
        tool_context,
    )
    assert not result.is_error
    # Empty file is OK; output is empty or a "(empty file)" note
    assert result.output == "" or "empty" in result.output.lower()
```

- [ ] **Step 2: Run tests, expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v
```
Expected: `ModuleNotFoundError: src.openharness.tools.file_tools`. Intended TDD failure.

- [ ] **Step 3: Implement `file_tools.py` with ReadFileTool**

Create `ai-worker/src/openharness/tools/file_tools.py`:

```python
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
ToolResult(is_error=True, ...) — tools never raise.

Phase 3 implements ReadFileTool first; subsequent tasks add
WriteFileTool (3.2), EditFileTool (3.3), GlobTool (3.4), GrepTool
(3.5), ListDirectoryTool (3.6). Task 3.7 adds a registration helper
and Task 3.8 extends the contract test to cover all six.
"""

from __future__ import annotations

import io
from pathlib import Path
from typing import Optional

from pydantic import BaseModel, Field

from .base import SimpleTool, ToolExecutionContext, ToolResult
from .workspace_path import PathEscapeError, WorkspacePath


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
            # 'replace' error handler so stray UTF-8 decoding errors don't
            # trigger an exception — the agent will see a few � characters
            # instead, which is still actionable.
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
        truncated_by_byte = hit_byte_cap
        truncated_by_line_cap = (end - start) < (total_lines - start) and lines_used == (end - start)

        if truncated_by_byte:
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
```

- [ ] **Step 4: Run the ReadFileTool tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v
```
Expected: **10 tests pass**. If any fail, fix the implementation — do not proceed until green.

Common failure modes and fixes:
- Line number format wrong: the `:>6` format specifier right-aligns in a 6-char field; the test asserts `     1\t` (5 spaces + `1` + tab)
- Truncation note test fails because the line count math is off: walk through the test's math manually (file has 2500 lines, default cap is 2000, so output should have lines 1-2000 and the truncation note)

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/tests/openharness/tools/test_file_tools.py
git commit -m "feat(tools): ReadFileTool with line numbers + binary reject

First of six file tools. Returns file contents with cat -n-style
line numbers so the agent can reference exact lines when calling
edit_file later. Rejects binary files via null-byte sniff in the
first 8KB. Caps: 2000 lines or 200 KB per call (whichever hits
first); start_line + limit let the agent page through larger files.

Path input validated via WorkspacePath.resolve; any escape becomes
ToolResult(is_error=True). All expected errors (not found, is a
directory, binary, open failed) returned as ToolResult not raised.

Tests: 10 cases covering happy path, start_line, limit, not found,
is directory, binary reject, line cap, byte cap, path escape,
empty file.

Shared DEFAULT_IGNORE_DIRS constant and READ_FILE_MAX_* constants
will be reused by WriteFileTool / GlobTool / GrepTool /
ListDirectoryTool in subsequent Phase 3 tasks."
```

---

### Task 3.2: `WriteFileTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/file_tools.py` (append `WriteFileTool`)
- Modify: `ai-worker/tests/openharness/tools/test_file_tools.py` (append WriteFileTool tests)

**Context:** Creates a new file or overwrites an existing one. No confirmation, no backup — the agent is expected to `read_file` first if it cares what's being overwritten. Parent directories are auto-created via `mkdir -p` semantics. Output message includes the line count and byte count so the agent can verify what landed.

Edge cases:
- Writing to a path where a parent segment is a file, not a directory — error (OSError bubbles up, we catch and return is_error=True)
- Writing an empty string — valid; creates a 0-byte file
- Writing to workspace root (path `.`) — error, that's not a file

- [ ] **Step 1: Add WriteFileTool tests to test_file_tools.py**

Append to `ai-worker/tests/openharness/tools/test_file_tools.py`:

```python
# ---------------------------------------------------------------------------
# WriteFileTool
# ---------------------------------------------------------------------------

from src.openharness.tools.file_tools import WriteFileTool  # noqa: E402


@pytest.fixture
def write_tool(workspace):
    return WriteFileTool(workspace)


@pytest.mark.asyncio
async def test_write_file_new(write_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import WriteFileInput

    result = await _run_tool(
        write_tool,
        WriteFileInput(path="new.txt", content="hello world\n"),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "new.txt").read_text() == "hello world\n"
    assert "1 line" in result.output or "line" in result.output
    assert "new.txt" in result.output


@pytest.mark.asyncio
async def test_write_file_creates_parent_dirs(write_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import WriteFileInput

    result = await _run_tool(
        write_tool,
        WriteFileInput(path="a/b/c/deep.txt", content="deep\n"),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "a" / "b" / "c" / "deep.txt").read_text() == "deep\n"


@pytest.mark.asyncio
async def test_write_file_overwrites_existing(write_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import WriteFileInput

    (workspace / "existing.txt").write_text("old content\n")
    result = await _run_tool(
        write_tool,
        WriteFileInput(path="existing.txt", content="new content\n"),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "existing.txt").read_text() == "new content\n"


@pytest.mark.asyncio
async def test_write_file_empty_content(write_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import WriteFileInput

    result = await _run_tool(
        write_tool,
        WriteFileInput(path="empty.txt", content=""),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "empty.txt").exists()
    assert (workspace / "empty.txt").read_text() == ""


@pytest.mark.asyncio
async def test_write_file_rejects_path_escape(write_tool, tool_context):
    from src.openharness.tools.file_tools import WriteFileInput

    result = await _run_tool(
        write_tool,
        WriteFileInput(path="/etc/passwd", content="pwned"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower() or "absolute" in result.output.lower()


@pytest.mark.asyncio
async def test_write_file_parent_is_file(write_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import WriteFileInput

    # Create a file where we want a directory
    (workspace / "blocker").write_text("I'm a file")
    result = await _run_tool(
        write_tool,
        WriteFileInput(path="blocker/child.txt", content="boom"),
        tool_context,
    )
    assert result.is_error
    # OSError should surface as a readable message mentioning the problem path
    assert "blocker" in result.output


@pytest.mark.asyncio
async def test_write_file_reports_counts(write_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import WriteFileInput

    content = "line 1\nline 2\nline 3\n"
    result = await _run_tool(
        write_tool,
        WriteFileInput(path="counted.txt", content=content),
        tool_context,
    )
    assert not result.is_error
    # Output mentions both line count and byte count
    assert "3" in result.output
    assert str(len(content.encode("utf-8"))) in result.output
```

- [ ] **Step 2: Implement WriteFileTool**

Append to `ai-worker/src/openharness/tools/file_tools.py`:

```python
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
        # If the file doesn't end with a newline, there's still one more
        # "line" at the tail (or the file is empty)
        if arguments.content and not arguments.content.endswith("\n"):
            line_count += 1
        byte_count = len(arguments.content.encode("utf-8"))

        return ToolResult(
            output=f"Wrote {line_count} line(s), {byte_count} byte(s) to {arguments.path}"
        )
```

- [ ] **Step 3: Run WriteFileTool tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v -k write_file
```
Expected: **7 tests pass** (test_write_file_new, _creates_parent_dirs, _overwrites_existing, _empty_content, _rejects_path_escape, _parent_is_file, _reports_counts).

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/tests/openharness/tools/test_file_tools.py
git commit -m "feat(tools): WriteFileTool — create or overwrite files

Auto-creates parent directories via mkdir(parents=True). Silently
overwrites existing files — the agent is expected to read_file
first if it cares about the current content. Empty-string content
produces a 0-byte file (valid).

Output reports both line count and byte count so the agent can
cross-check what landed. OSError on write (parent is a file,
permission denied, etc.) becomes ToolResult(is_error=True) with
the path mentioned in the message so the agent can debug.

Tests: 7 cases covering new file, nested parent creation,
overwrite, empty content, path escape, parent-is-file error,
and output count reporting."
```

---

### Task 3.3: `EditFileTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/file_tools.py` (append `EditFileTool`)
- Modify: `ai-worker/tests/openharness/tools/test_file_tools.py` (append EditFileTool tests)

**Context:** Claude Code's exact-match edit contract, adapted for Python:
- File must exist (this is edit, not create)
- `old_string` must appear **exactly once** in the file unless `replace_all=True`
- `old_string` not found → clear error asking the agent to `read_file` first
- `old_string` found N>1 times without `replace_all` → error asking for more context or to set `replace_all=True`
- Success → report `+X -Y` lines like a git diff
- No regex, no fuzzy match, no whitespace normalization — agent literally provides the bytes to replace

This is the most error-prone tool for LLMs because it's the one where the LLM has to reproduce a slice of file content verbatim. The uniqueness constraint is what saves the day: if the agent's `old_string` is ambiguous, it fails loudly with a message that tells the agent how to fix it.

**Subtle point:** the diff counts are **line-based delta**, not character delta. Computing them:
```python
before_lines = old_string.count("\n") + (1 if old_string and not old_string.endswith("\n") else 0)
after_lines  = new_string.count("\n") + (1 if new_string and not new_string.endswith("\n") else 0)
```
This gives "X lines removed, Y lines added" which is the familiar git diff notation. Not perfect (it doesn't know about shared prefix lines) but actionable.

- [ ] **Step 1: Add EditFileTool tests**

Append to `ai-worker/tests/openharness/tools/test_file_tools.py`:

```python
# ---------------------------------------------------------------------------
# EditFileTool
# ---------------------------------------------------------------------------

from src.openharness.tools.file_tools import EditFileTool  # noqa: E402


@pytest.fixture
def edit_tool(workspace):
    return EditFileTool(workspace)


@pytest.mark.asyncio
async def test_edit_file_unique_match(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    (workspace / "foo.py").write_text("def foo():\n    return 1\n")
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="foo.py",
            old_string="return 1",
            new_string="return 2",
        ),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "foo.py").read_text() == "def foo():\n    return 2\n"
    assert "foo.py" in result.output


@pytest.mark.asyncio
async def test_edit_file_not_found(edit_tool, tool_context):
    from src.openharness.tools.file_tools import EditFileInput

    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="ghost.py",
            old_string="x",
            new_string="y",
        ),
        tool_context,
    )
    assert result.is_error
    assert "not found" in result.output.lower()


@pytest.mark.asyncio
async def test_edit_file_old_string_not_in_file(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    (workspace / "foo.py").write_text("bar baz\n")
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="foo.py",
            old_string="not here",
            new_string="new",
        ),
        tool_context,
    )
    assert result.is_error
    # Error message should guide the agent to use read_file first
    assert "not found" in result.output.lower() or "no match" in result.output.lower()
    assert "read_file" in result.output


@pytest.mark.asyncio
async def test_edit_file_ambiguous_without_replace_all(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    (workspace / "foo.py").write_text("x = 1\ny = 1\nz = 1\n")
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="foo.py",
            old_string="= 1",
            new_string="= 2",
        ),
        tool_context,
    )
    assert result.is_error
    # Should mention count and suggest more context or replace_all
    assert "3 times" in result.output or "3" in result.output
    assert "replace_all" in result.output


@pytest.mark.asyncio
async def test_edit_file_replace_all(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    (workspace / "foo.py").write_text("x = 1\ny = 1\nz = 1\n")
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="foo.py",
            old_string="= 1",
            new_string="= 2",
            replace_all=True,
        ),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "foo.py").read_text() == "x = 2\ny = 2\nz = 2\n"


@pytest.mark.asyncio
async def test_edit_file_multiline(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    original = "def foo():\n    x = 1\n    return x\n"
    (workspace / "foo.py").write_text(original)
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="foo.py",
            old_string="    x = 1\n    return x\n",
            new_string="    x = 1\n    y = 2\n    return x + y\n",
        ),
        tool_context,
    )
    assert not result.is_error
    assert "y = 2" in (workspace / "foo.py").read_text()


@pytest.mark.asyncio
async def test_edit_file_diff_counts(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    (workspace / "foo.py").write_text("a\nb\nc\n")
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="foo.py",
            old_string="b\n",
            new_string="b1\nb2\nb3\n",
        ),
        tool_context,
    )
    assert not result.is_error
    # old_string has 1 line, new_string has 3 — delta: +3 -1
    assert "+3" in result.output or "+3 " in result.output or "3 added" in result.output
    assert "-1" in result.output or "1 removed" in result.output or "-1 " in result.output


@pytest.mark.asyncio
async def test_edit_file_is_directory(edit_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import EditFileInput

    (workspace / "a_dir").mkdir()
    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="a_dir",
            old_string="x",
            new_string="y",
        ),
        tool_context,
    )
    assert result.is_error
    assert "director" in result.output.lower()


@pytest.mark.asyncio
async def test_edit_file_rejects_path_escape(edit_tool, tool_context):
    from src.openharness.tools.file_tools import EditFileInput

    result = await _run_tool(
        edit_tool,
        EditFileInput(
            path="../etc/passwd",
            old_string="root",
            new_string="pwned",
        ),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()
```

- [ ] **Step 2: Implement EditFileTool**

Append to `ai-worker/src/openharness/tools/file_tools.py`:

```python
# ---------------------------------------------------------------------------
# EditFileTool
# ---------------------------------------------------------------------------


class EditFileInput(BaseModel):
    path: str = Field(..., description="File path relative to workspace root")
    old_string: str = Field(
        ...,
        description=(
            "Exact text to replace. Must appear exactly once in the file "
            "unless replace_all=True. No regex, no fuzzy match — this is "
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
        "is the preferred way to modify code — it's less error-prone than "
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
            # Replace exactly once
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
```

- [ ] **Step 3: Run EditFileTool tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v -k edit_file
```
Expected: **9 tests pass**. Common pitfall: `_count_lines("= 1")` returns 1 (no trailing newline but content present); verify with the unit test if a specific count looks off.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/tests/openharness/tools/test_file_tools.py
git commit -m "feat(tools): EditFileTool with Claude Code unique-match contract

Exact-string replacement with the uniqueness guard that makes LLM
edits reliable:
- old_string must appear exactly once (unless replace_all=True)
- 0 occurrences -> error asks agent to read_file first
- 2+ occurrences without replace_all -> error asks for more
  context or to set replace_all=True
- success -> reports line delta in +X -Y form

No regex, no fuzzy match, no whitespace normalization — the agent
literally provides the bytes to replace. This is Claude Code's
empirical finding that a strict contract produces better edits
than a forgiving one.

Error messages are actionable: every failure mode tells the agent
what to do next (read_file, add context, set replace_all, etc).
This is the 'style guide' side of silicon-valley infra — the
user-visible error shape matters as much as the safety.

Tests: 9 cases covering unique match, not found, old_string
missing, ambiguous, replace_all, multiline replace, diff counts,
is directory, path escape."
```

---

### Task 3.4: `GlobTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/file_tools.py` (append `GlobTool`)
- Modify: `ai-worker/tests/openharness/tools/test_file_tools.py` (append GlobTool tests)

**Context:** Pattern-based file discovery. Uses the `pathspec` library (installed in Phase 0 Task 0.2) for gitignore-style pattern matching, which handles `**`, `{ts,tsx}`, negation (`!foo.go`), and other real glob semantics that `fnmatch` doesn't.

Pattern syntax (pathspec's `GitWildMatchPattern`):
- `**/*.go` — any `.go` file at any depth
- `src/**/*.{ts,tsx}` — note: pathspec doesn't support brace expansion natively, so this needs preprocessing
- `main.go` — exact name match

The brace expansion limitation is worth noting: pathspec **does not** handle `{ts,tsx}` shorthand. For Phase 3 we'll either preprocess (split a brace-pattern into multiple patterns) or document the limitation. Decision: **preprocess**. Users expect gitignore-like patterns to work, and supporting brace expansion is ~15 lines of string manipulation.

Results are sorted by mtime (most recent first), capped at 200.

- [ ] **Step 1: Add GlobTool tests**

Append to `ai-worker/tests/openharness/tools/test_file_tools.py`:

```python
# ---------------------------------------------------------------------------
# GlobTool
# ---------------------------------------------------------------------------

import time
from src.openharness.tools.file_tools import GlobTool  # noqa: E402


@pytest.fixture
def glob_tool(workspace):
    return GlobTool(workspace)


@pytest.mark.asyncio
async def test_glob_simple_pattern(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    (workspace / "a.go").write_text("")
    (workspace / "b.go").write_text("")
    (workspace / "c.txt").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.go"),
        tool_context,
    )
    assert not result.is_error
    assert "a.go" in result.output
    assert "b.go" in result.output
    assert "c.txt" not in result.output


@pytest.mark.asyncio
async def test_glob_recursive_pattern(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    (workspace / "src").mkdir()
    (workspace / "src" / "main.go").write_text("")
    (workspace / "src" / "util").mkdir()
    (workspace / "src" / "util" / "helper.go").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="**/*.go"),
        tool_context,
    )
    assert not result.is_error
    assert "src/main.go" in result.output or "src\\main.go" in result.output
    assert "helper.go" in result.output


@pytest.mark.asyncio
async def test_glob_brace_expansion(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    (workspace / "a.ts").write_text("")
    (workspace / "b.tsx").write_text("")
    (workspace / "c.js").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.{ts,tsx}"),
        tool_context,
    )
    assert not result.is_error
    assert "a.ts" in result.output
    assert "b.tsx" in result.output
    assert "c.js" not in result.output


@pytest.mark.asyncio
async def test_glob_ignores_default_dirs(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    (workspace / ".git").mkdir()
    (workspace / ".git" / "HEAD").write_text("ref: refs/heads/main")
    (workspace / "node_modules").mkdir()
    (workspace / "node_modules" / "pkg.json").write_text("{}")
    (workspace / "src.go").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="**/*"),
        tool_context,
    )
    assert not result.is_error
    assert "HEAD" not in result.output
    assert "pkg.json" not in result.output
    assert "src.go" in result.output


@pytest.mark.asyncio
async def test_glob_sorted_by_mtime_desc(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    old = workspace / "old.txt"
    old.write_text("")
    # Force old mtime (2 seconds ago)
    import os as _os
    _os.utime(old, (time.time() - 2, time.time() - 2))

    new = workspace / "new.txt"
    new.write_text("")

    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.txt"),
        tool_context,
    )
    assert not result.is_error
    # new.txt should appear before old.txt in the output
    new_idx = result.output.index("new.txt")
    old_idx = result.output.index("old.txt")
    assert new_idx < old_idx


@pytest.mark.asyncio
async def test_glob_no_matches(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    (workspace / "a.go").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.nonexistent"),
        tool_context,
    )
    assert not result.is_error
    assert "no matches" in result.output.lower() or "0 matches" in result.output.lower()


@pytest.mark.asyncio
async def test_glob_subdirectory_path(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    (workspace / "src" / "a").mkdir(parents=True)
    (workspace / "src" / "a" / "main.go").write_text("")
    (workspace / "tests" / "b").mkdir(parents=True)
    (workspace / "tests" / "b" / "main.go").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="**/*.go", path="src"),
        tool_context,
    )
    assert not result.is_error
    assert "main.go" in result.output
    # Should only show src-scoped results, not tests
    assert "tests" not in result.output


@pytest.mark.asyncio
async def test_glob_result_cap(glob_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import GlobInput

    for i in range(250):
        (workspace / f"file_{i:04d}.txt").write_text("")
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.txt"),
        tool_context,
    )
    assert not result.is_error
    assert "truncated" in result.output.lower() or "more" in result.output.lower()


@pytest.mark.asyncio
async def test_glob_rejects_path_escape(glob_tool, tool_context):
    from src.openharness.tools.file_tools import GlobInput

    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.go", path="../outside"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()
```

- [ ] **Step 2: Implement GlobTool**

Append to `ai-worker/src/openharness/tools/file_tools.py`:

```python
# ---------------------------------------------------------------------------
# GlobTool
# ---------------------------------------------------------------------------

import re as _re
from typing import Iterable, List

try:
    import pathspec
    from pathspec.patterns.gitwildmatch import GitWildMatchPattern
except ImportError as e:
    raise ImportError(
        "pathspec is required for GlobTool. Run: pip install pathspec>=0.12"
    ) from e


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
    # Use os.walk so we can prune directories in-place
    import os as _os

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
            spec = pathspec.PathSpec.from_lines(
                GitWildMatchPattern, expanded
            )
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
        for _, abs_path in capped:
            try:
                rel_to_ws = abs_path.relative_to(self._workspace_root.resolve())
                lines.append(str(rel_to_ws).replace("\\", "/"))
            except ValueError:
                lines.append(str(abs_path))

        output = "\n".join(lines)
        if total > GLOB_MAX_RESULTS:
            output += f"\n... ({total - GLOB_MAX_RESULTS} more matches truncated, showing first {GLOB_MAX_RESULTS} by mtime)"

        return ToolResult(output=output)
```

- [ ] **Step 3: Run GlobTool tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v -k glob
```
Expected: **9 tests pass**. If `test_glob_ignores_default_dirs` fails, the ignore-list logic in `_iter_files` isn't pruning — check that `dirnames[:] = [...]` mutates in-place so `os.walk` doesn't descend into pruned dirs.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/tests/openharness/tools/test_file_tools.py
git commit -m "feat(tools): GlobTool with pathspec + brace expansion

File discovery using the pathspec library (gitignore-style
patterns, real ** recursion, negation semantics) with a small
brace-expansion preprocessor so patterns like '*.{ts,tsx}' work
even though pathspec itself doesn't grok braces.

Walk uses os.walk with in-place dirname pruning so ignored
directories (DEFAULT_IGNORE_DIRS: .git/, node_modules/, .venv/,
__pycache__/, dist/, build/, target/, .next/, .gradle/, .cache/)
are skipped at the descent level — no wasted IO.

Results sorted by mtime descending (most recent first). Cap 200
matches with a truncation note.

Tests: 9 cases covering simple pattern, recursive, brace
expansion, ignore-dir pruning, mtime sort, zero matches,
subdirectory path, 250-file cap, path escape."
```

---

### Task 3.5: `GrepTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/file_tools.py` (append `GrepTool`)
- Modify: `ai-worker/tests/openharness/tools/test_file_tools.py` (append GrepTool tests)

**Context:** Content search via a `rg` subprocess. ripgrep is installed in Phase 0 Task 0.1's Dockerfile change. **No Python fallback** — if `rg` is missing, the tool errors out with a clear message. One code path.

Command line we build:
```
rg --json --no-heading --color=never --no-messages <pattern> [path]
```
- `--json` gives structured output; parse line-by-line, each is a JSON object with `type: "match"`, `data.lines.text`, `data.line_number`, `data.path.text`
- `--no-heading` so results aren't grouped per file
- `--color=never` to avoid ANSI escape codes
- `--no-messages` to suppress "no such file" chatter
- Additional flags: `-i` for case insensitive, `-g <glob>` for file-glob filter (we pass through `file_glob` input), `-t <type>` not used (less predictable than explicit globs)

Ripgrep respects `.gitignore` by default, which aligns with our `DEFAULT_IGNORE_DIRS`. We also pass `--hidden=false` (default) to skip dot-directories.

Output format to the agent: `path:line:content` — simple, greppable, easy for the agent to reference in `edit_file`.

Output cap: 500 lines or 200 KB. On overflow, truncate with a note.

- [ ] **Step 1: Add GrepTool tests**

Append to `ai-worker/tests/openharness/tools/test_file_tools.py`:

```python
# ---------------------------------------------------------------------------
# GrepTool
# ---------------------------------------------------------------------------

import shutil
from src.openharness.tools.file_tools import GrepTool  # noqa: E402


@pytest.fixture
def grep_tool(workspace):
    return GrepTool(workspace)


def _require_ripgrep():
    if shutil.which("rg") is None:
        pytest.skip("ripgrep (rg) not installed; grep tests require it")


@pytest.mark.asyncio
async def test_grep_simple_match(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "a.go").write_text("package main\nfunc foo() {}\nfunc bar() {}\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="func foo"),
        tool_context,
    )
    assert not result.is_error
    assert "a.go:2:" in result.output
    assert "func foo" in result.output
    assert "func bar" not in result.output


@pytest.mark.asyncio
async def test_grep_multiple_files(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "a.go").write_text("TARGET here\n")
    (workspace / "b.go").write_text("also TARGET here\n")
    (workspace / "c.go").write_text("nothing interesting\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="TARGET"),
        tool_context,
    )
    assert not result.is_error
    assert "a.go" in result.output
    assert "b.go" in result.output
    assert "c.go" not in result.output


@pytest.mark.asyncio
async def test_grep_case_insensitive(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "a.go").write_text("HELLO WORLD\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="hello", case_insensitive=True),
        tool_context,
    )
    assert not result.is_error
    assert "HELLO WORLD" in result.output


@pytest.mark.asyncio
async def test_grep_case_sensitive_default(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "a.go").write_text("HELLO WORLD\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="hello"),
        tool_context,
    )
    assert not result.is_error
    # case-sensitive default means "hello" does not match "HELLO"
    assert "no matches" in result.output.lower() or "0 matches" in result.output.lower()


@pytest.mark.asyncio
async def test_grep_file_glob(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "a.go").write_text("TARGET in go\n")
    (workspace / "b.py").write_text("TARGET in python\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="TARGET", file_glob="*.go"),
        tool_context,
    )
    assert not result.is_error
    assert "a.go" in result.output
    assert "b.py" not in result.output


@pytest.mark.asyncio
async def test_grep_subdirectory(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "src").mkdir()
    (workspace / "src" / "a.go").write_text("TARGET in src\n")
    (workspace / "tests").mkdir()
    (workspace / "tests" / "b.go").write_text("TARGET in tests\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="TARGET", path="src"),
        tool_context,
    )
    assert not result.is_error
    assert "src/a.go" in result.output or "src\\a.go" in result.output
    assert "tests" not in result.output


@pytest.mark.asyncio
async def test_grep_no_matches(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / "a.go").write_text("nothing here\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="ABSENT"),
        tool_context,
    )
    assert not result.is_error
    assert "no matches" in result.output.lower() or "0 matches" in result.output.lower()


@pytest.mark.asyncio
async def test_grep_ignores_git_dir(grep_tool, tool_context, workspace):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    (workspace / ".git").mkdir()
    (workspace / ".git" / "HEAD").write_text("TARGET in .git\n")
    (workspace / "src.go").write_text("TARGET in src\n")
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="TARGET"),
        tool_context,
    )
    assert not result.is_error
    assert "src.go" in result.output
    assert ".git" not in result.output


@pytest.mark.asyncio
async def test_grep_rejects_path_escape(grep_tool, tool_context):
    _require_ripgrep()
    from src.openharness.tools.file_tools import GrepInput

    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="x", path="../outside"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()
```

- [ ] **Step 2: Implement GrepTool**

Append to `ai-worker/src/openharness/tools/file_tools.py`:

```python
# ---------------------------------------------------------------------------
# GrepTool
# ---------------------------------------------------------------------------

import asyncio
import json as _json
import shutil as _shutil


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
        "'path:line:content' format. Uses ripgrep under the hood — "
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
        matches: List[str] = []
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

            matches.append(f"{display_path}:{line_num}:{line_text}")

        if not matches:
            return ToolResult(output=f"No matches for pattern {arguments.pattern!r}")

        total = len(matches)
        # Apply caps
        capped = matches[:GREP_MAX_RESULT_LINES]
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
```

- [ ] **Step 3: Run GrepTool tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v -k grep
```
Expected: **9 tests pass** (or skip if rg isn't installed on the dev host). Inside the Phase 0 container rebuild, rg is guaranteed to be present.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/tests/openharness/tools/test_file_tools.py
git commit -m "feat(tools): GrepTool via ripgrep subprocess

Shells out to 'rg --json --no-heading --color=never --no-messages'
and parses the JSON event stream. One code path — no Python
fallback. If rg is missing, returns a clear error pointing at the
Phase 0 Dockerfile change.

Features:
- case_insensitive -> rg -i
- file_glob -> rg -g <pattern>
- path -> scope search to a subdirectory (with WorkspacePath guard)
- ripgrep respects .gitignore by default, aligning with our
  DEFAULT_IGNORE_DIRS without needing a second walk

Output format: 'path:line:content' one result per line, paths
normalized to workspace-relative.

Caps: 500 result lines or 200 KB. Cap application respects both
line count and byte total; the byte cap trims lines from the tail
until the output fits.

Timeout: 30 seconds (most greps finish in <1s on trees this
project is realistic for).

Tests: 9 cases with a _require_ripgrep skip for hosts without rg."
```

---

### Task 3.6: `ListDirectoryTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/file_tools.py` (append `ListDirectoryTool`)
- Modify: `ai-worker/tests/openharness/tools/test_file_tools.py` (append ListDirectoryTool tests)

**Context:** One-level `ls`, sorted with directories first (trailing `/`), then files (no trailing marker), both alphabetical. Cap 500 entries. Skips `DEFAULT_IGNORE_DIRS`. Simplest tool in the phase.

- [ ] **Step 1: Add tests**

Append to `ai-worker/tests/openharness/tools/test_file_tools.py`:

```python
# ---------------------------------------------------------------------------
# ListDirectoryTool
# ---------------------------------------------------------------------------

from src.openharness.tools.file_tools import ListDirectoryTool  # noqa: E402


@pytest.fixture
def list_tool(workspace):
    return ListDirectoryTool(workspace)


@pytest.mark.asyncio
async def test_list_directory_root(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    (workspace / "file.txt").write_text("")
    (workspace / "src").mkdir()
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="."),
        tool_context,
    )
    assert not result.is_error
    assert "src/" in result.output
    assert "file.txt" in result.output


@pytest.mark.asyncio
async def test_list_directory_dirs_before_files(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    (workspace / "zfile.txt").write_text("")
    (workspace / "adir").mkdir()
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="."),
        tool_context,
    )
    assert not result.is_error
    dir_idx = result.output.index("adir/")
    file_idx = result.output.index("zfile.txt")
    assert dir_idx < file_idx


@pytest.mark.asyncio
async def test_list_directory_alphabetical_within_type(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    (workspace / "b.txt").write_text("")
    (workspace / "a.txt").write_text("")
    (workspace / "c.txt").write_text("")
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="."),
        tool_context,
    )
    assert not result.is_error
    a_idx = result.output.index("a.txt")
    b_idx = result.output.index("b.txt")
    c_idx = result.output.index("c.txt")
    assert a_idx < b_idx < c_idx


@pytest.mark.asyncio
async def test_list_directory_subdirectory(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    (workspace / "src").mkdir()
    (workspace / "src" / "main.go").write_text("")
    (workspace / "src" / "util").mkdir()
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="src"),
        tool_context,
    )
    assert not result.is_error
    assert "main.go" in result.output
    assert "util/" in result.output


@pytest.mark.asyncio
async def test_list_directory_ignores_default_dirs(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    (workspace / ".git").mkdir()
    (workspace / "node_modules").mkdir()
    (workspace / "src").mkdir()
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="."),
        tool_context,
    )
    assert not result.is_error
    assert ".git" not in result.output
    assert "node_modules" not in result.output
    assert "src/" in result.output


@pytest.mark.asyncio
async def test_list_directory_not_found(list_tool, tool_context):
    from src.openharness.tools.file_tools import ListDirectoryInput

    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="does_not_exist"),
        tool_context,
    )
    assert result.is_error
    assert "not found" in result.output.lower()


@pytest.mark.asyncio
async def test_list_directory_is_file(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    (workspace / "a.txt").write_text("")
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="a.txt"),
        tool_context,
    )
    assert result.is_error
    assert "not a directory" in result.output.lower() or "is a file" in result.output.lower()


@pytest.mark.asyncio
async def test_list_directory_cap(list_tool, tool_context, workspace):
    from src.openharness.tools.file_tools import ListDirectoryInput

    for i in range(600):
        (workspace / f"f_{i:04d}.txt").write_text("")
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="."),
        tool_context,
    )
    assert not result.is_error
    assert "truncated" in result.output.lower() or "more" in result.output.lower()


@pytest.mark.asyncio
async def test_list_directory_rejects_path_escape(list_tool, tool_context):
    from src.openharness.tools.file_tools import ListDirectoryInput

    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="../outside"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()
```

- [ ] **Step 2: Implement ListDirectoryTool**

Append to `ai-worker/src/openharness/tools/file_tools.py`:

```python
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
```

- [ ] **Step 3: Run tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_file_tools.py -v -k list_directory
```
Expected: **9 tests pass**.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/tests/openharness/tools/test_file_tools.py
git commit -m "feat(tools): ListDirectoryTool one-level directory listing

Simplest of the Phase 3 tools. Lists entries in a directory with
dirs first (trailing /) then files, alphabetical within each group.
Skips DEFAULT_IGNORE_DIRS (.git/, node_modules/, .venv/, etc).
Cap 500 entries with a truncation note.

For recursive exploration the agent uses glob instead — spec §4.3
says list_directory stays one-level to keep tools focused and
force the agent to name its intent explicitly.

Tests: 9 cases covering root, dirs-before-files ordering,
alphabetical within type, subdirectory, ignored-dir filtering,
not found, is-file error, 600-entry cap, path escape."
```

---

### Task 3.7: Module registration helper

**Files:**
- Modify: `ai-worker/src/openharness/tools/file_tools.py` (append `register_file_tools`)
- Modify: `ai-worker/src/openharness/tools/__init__.py` (re-export the new symbols)

**Context:** A small helper `register_file_tools(registry, workspace_root)` that instantiates all six tools and registers them into a `ToolRegistry`. Mirrors the existing `register_context_tools` helper in `context_tools.py` so Phase 5's `_create_engine` can call both.

Also export the new tools from the package `__init__` so `from src.openharness.tools import ReadFileTool` works without having to go through the submodule path.

- [ ] **Step 1: Append `register_file_tools` to `file_tools.py`**

At the bottom of `ai-worker/src/openharness/tools/file_tools.py`, add:

```python
# ---------------------------------------------------------------------------
# Registration helper
# ---------------------------------------------------------------------------


def register_file_tools(registry, workspace_root: Path) -> None:
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
```

- [ ] **Step 2: Update `__init__.py`**

Read the current `ai-worker/src/openharness/tools/__init__.py`:

```bash
cat ai-worker/src/openharness/tools/__init__.py
```

Append to it (or create if empty):

```python
# Re-exports for convenience: `from src.openharness.tools import ReadFileTool` works.
from .file_tools import (
    EditFileTool,
    GlobTool,
    GrepTool,
    ListDirectoryTool,
    ReadFileTool,
    WriteFileTool,
    register_file_tools,
)
from .workspace_path import PathEscapeError, WorkspacePath

__all__ = [
    "EditFileTool",
    "GlobTool",
    "GrepTool",
    "ListDirectoryTool",
    "PathEscapeError",
    "ReadFileTool",
    "WorkspacePath",
    "WriteFileTool",
    "register_file_tools",
]
```

If the file already has a `__all__`, merge the new names into it rather than overwriting.

- [ ] **Step 3: Smoke-test the registration helper**

```bash
cd ai-worker && python -c "
from pathlib import Path
from src.openharness.tools.base import ToolRegistry
from src.openharness.tools import register_file_tools

reg = ToolRegistry()
register_file_tools(reg, Path('/tmp'))
print('registered:', [t.name for t in reg.list_tools()])
assert len(reg.list_tools()) == 6
print('ok')
"
```
Expected: `registered: ['read_file', 'write_file', 'edit_file', 'glob', 'grep', 'list_directory']` then `ok`.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/file_tools.py ai-worker/src/openharness/tools/__init__.py
git commit -m "feat(tools): register_file_tools helper + package exports

register_file_tools(registry, workspace_root) instantiates all six
T2 file tools and registers them into a ToolRegistry. Mirrors the
existing register_context_tools pattern so Phase 5's _create_engine
can call both helpers in sequence.

Package __init__.py re-exports ReadFileTool, WriteFileTool,
EditFileTool, GlobTool, GrepTool, ListDirectoryTool,
register_file_tools, WorkspacePath, PathEscapeError so callers
can do 'from src.openharness.tools import ReadFileTool' without
the submodule path."
```

---

### Task 3.8: Extend BaseTool contract tests to cover all six file tools

**Files:**
- Modify: `ai-worker/tests/openharness/tools/test_base_tool_contract.py` (add specs for the six new tools)

**Context:** Phase 2 built `test_base_tool_contract.py` with four context tools in `ALL_TOOL_SPECS`. Now that the file tools exist, add six more spec rows so the parametrized contract tests run against all ten.

Each spec is a `(tool_class, factory, arg_factory)` triple. The factories build instances with fake-but-valid arguments that let the tool complete a trivial execution. We pick "minimal valid" arguments:

- `ReadFileTool` — needs a real file in the workspace to read
- `WriteFileTool` — writes to a fresh path
- `EditFileTool` — needs an existing file with a known substring
- `GlobTool` — simple pattern `"*"` 
- `GrepTool` — skip on hosts without rg (the contract test will call `_require_ripgrep` pattern? — no, it's parametrized, we can't skip one param. Use `pytest.importorskip`-equivalent for rg). Actually the contract test just calls `execute()` and asserts the protocol — even if rg is missing, `GrepTool` returns `ToolResult(is_error=True, ...)`. That's a valid ToolResult yield. The contract still holds. No special handling needed.
- `ListDirectoryTool` — list `.`

The factories need a workspace path, which they get from the `workspace` fixture passed through the parametrize. The `factory` callable takes the workspace as input.

- [ ] **Step 1: Update `_all_tool_specs` in the contract test**

Edit `ai-worker/tests/openharness/tools/test_base_tool_contract.py`. Find the `_all_tool_specs()` function and add the six new specs:

Replace the existing `_all_tool_specs` with:

```python
def _all_tool_specs() -> list[ToolSpec]:
    # Context tools (Phase 2)
    from src.openharness.tools.context_tools import (
        QueryApiCatalogTool,
        QueryApiCatalogInput,
        QueryBusinessRulesTool,
        QueryBusinessRulesInput,
        QueryDbSchemaTool,
        QueryDbSchemaInput,
        QueryModuleGraphTool,
        QueryModuleGraphInput,
    )
    # File tools (Phase 3)
    from src.openharness.tools.file_tools import (
        EditFileInput,
        EditFileTool,
        GlobInput,
        GlobTool,
        GrepInput,
        GrepTool,
        ListDirectoryInput,
        ListDirectoryTool,
        ReadFileInput,
        ReadFileTool,
        WriteFileInput,
        WriteFileTool,
    )

    profiles = _make_context_profiles()

    def _seed_workspace_for_read(ws: Path) -> "ReadFileTool":
        (ws / "seed.txt").write_text("contract test seed\n")
        return ReadFileTool(ws)

    def _seed_workspace_for_edit(ws: Path) -> "EditFileTool":
        (ws / "seed.txt").write_text("contract test seed\n")
        return EditFileTool(ws)

    specs: list[ToolSpec] = [
        # Context tools
        (
            QueryApiCatalogTool,
            lambda _ws: QueryApiCatalogTool(profiles),
            lambda: QueryApiCatalogInput(keyword="nothing"),
        ),
        (
            QueryDbSchemaTool,
            lambda _ws: QueryDbSchemaTool(profiles),
            lambda: QueryDbSchemaInput(table_name="nothing"),
        ),
        (
            QueryBusinessRulesTool,
            lambda _ws: QueryBusinessRulesTool(profiles),
            lambda: QueryBusinessRulesInput(domain="nothing"),
        ),
        (
            QueryModuleGraphTool,
            lambda _ws: QueryModuleGraphTool(profiles),
            lambda: QueryModuleGraphInput(module_name="nothing"),
        ),
        # File tools
        (
            ReadFileTool,
            _seed_workspace_for_read,
            lambda: ReadFileInput(path="seed.txt"),
        ),
        (
            WriteFileTool,
            lambda ws: WriteFileTool(ws),
            lambda: WriteFileInput(path="contract.txt", content="contract test"),
        ),
        (
            EditFileTool,
            _seed_workspace_for_edit,
            lambda: EditFileInput(
                path="seed.txt",
                old_string="contract test seed",
                new_string="edited",
            ),
        ),
        (
            GlobTool,
            lambda ws: GlobTool(ws),
            lambda: GlobInput(pattern="*"),
        ),
        (
            GrepTool,
            lambda ws: GrepTool(ws),
            lambda: GrepInput(pattern="seed"),
        ),
        (
            ListDirectoryTool,
            lambda ws: ListDirectoryTool(ws),
            lambda: ListDirectoryInput(path="."),
        ),
    ]

    return specs
```

- [ ] **Step 2: Run the contract tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_base_tool_contract.py -v
```
Expected: **40 tests pass** (4 contract functions × 10 tool classes). If any fail, the failing tool has a contract bug. Common issues:
- A tool raises instead of returning `ToolResult(is_error=True)` — fix the tool
- A tool yields something that's neither `StreamEvent` nor `ToolResult` — fix the tool
- The factory produces invalid arguments that trip Pydantic — adjust the factory

- [ ] **Step 3: Run the full file tool test suite one more time**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/ -v 2>&1 | tail -40
```
Expected: all tests pass — `test_workspace_path.py` (8 tests), `test_workspace_path_adversarial.py` (11), `test_base_tool_contract.py` (40), `test_file_tools.py` (~53 across the six tools).

- [ ] **Step 4: Commit**

```bash
git add ai-worker/tests/openharness/tools/test_base_tool_contract.py
git commit -m "test(tools): extend contract suite to all six file tools

ALL_TOOL_SPECS grows from 4 to 10 entries. Four contract tests
(yields exactly one ToolResult / result is last / class attrs set
/ execute returns async iterator) run against each tool, giving
40 parametrized test cases total.

File tool factories seed the workspace fixture with minimal state
before constructing the tool (e.g. ReadFileTool needs a file to
read, EditFileTool needs a file with a known substring).

GrepTool factory makes no special provision for hosts without
ripgrep — the contract test just calls execute() and the tool
returns a valid ToolResult(is_error=True) even when rg is missing.
The contract is preserved whether or not rg is present."
```

---

## Phase 3 completion check

Before starting Phase 4:

- [ ] `pytest ai-worker/tests/openharness/tools/test_file_tools.py -v` — all file tool tests pass (approximately 53 tests: 10 read, 7 write, 9 edit, 9 glob, 9 grep, 9 list_directory)
- [ ] `pytest ai-worker/tests/openharness/tools/test_base_tool_contract.py -v` — 40 tests pass (4 × 10)
- [ ] `pytest ai-worker/tests/openharness/tools/test_workspace_path.py tests/openharness/tools/test_workspace_path_adversarial.py -v` — Phase 2 tests still pass (8 + 11)
- [ ] `pytest ai-worker/tests/test_context_tools.py tests/test_tool_registry.py -v` — older suites still pass
- [ ] `pytest ai-worker/tests/test_query_engine.py tests/test_agent_loop.py -v` — agent loop still works with the new tools
- [ ] `python -c "from src.openharness.tools import ReadFileTool, WriteFileTool, EditFileTool, GlobTool, GrepTool, ListDirectoryTool, register_file_tools, WorkspacePath, PathEscapeError; print('ok')"` — package exports clean
- [ ] `grep -c "SimpleTool" ai-worker/src/openharness/tools/file_tools.py` returns 6 — every file tool subclasses SimpleTool
- [ ] `grep -n "raise " ai-worker/src/openharness/tools/file_tools.py` returns nothing — no tool raises, they all return ToolResult
- [ ] Branch has **8 new commits** from this phase (one per task)

## Phase 3 outputs unlock

- **Phase 4** (Bash + SetPhase) can build on the same `BaseTool`/`SimpleTool` pattern. BashTool subclasses `BaseTool` directly (not SimpleTool) because it emits mid-execution `ThinkingStarted`/`ThinkingStopped` events. SetPhaseTool subclasses `BaseTool` because it emits `PhaseChanged`. All other file tools are settled; Phase 4 only adds two new tools.
- **Phase 5** can wire `register_file_tools(registry, workspace_root)` into `_create_engine` alongside `register_context_tools` and the new Phase 4 tools. The agent's `ToolRegistry` at that point will have 6 file tools + 6 context tools + 2 bash/phase tools = 14 total.
- **Phase 5's system prompt** can reference `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `list_directory` by name with the exact semantics documented here, including the "strip line numbers before edit_file" rule for `read_file` output.
