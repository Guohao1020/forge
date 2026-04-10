"""Unit tests for the T2 file-operating tools.

All six tools (read, write, edit, glob, grep, list_directory) live in
file_tools.py. Each has its own section below. Shared fixtures come
from conftest.py in the same directory.

Run only this file: pytest tests/openharness/tools/test_file_tools.py -v
"""

import os as _os
import shutil
import time
from pathlib import Path

import pytest

from src.openharness.tools.base import ToolExecutionContext, ToolResult
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


# ---------------------------------------------------------------------------
# Helper
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


# ---------------------------------------------------------------------------
# ReadFileTool
# ---------------------------------------------------------------------------


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
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="src/main.go"),
        tool_context,
    )
    assert not result.is_error
    # Output includes line numbers
    assert "     1\tpackage main" in result.output
    assert '     3\timport "fmt"' in result.output
    # Last line number is 7
    assert "     7\t}" in result.output


@pytest.mark.asyncio
async def test_read_file_with_start_line(read_tool, tool_context, sample_file):
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="src/main.go", start_line=3),
        tool_context,
    )
    assert not result.is_error
    # First line in output should be line 3 of the file
    assert '     3\timport "fmt"' in result.output
    # Line 1 and 2 should not appear
    assert "     1\tpackage main" not in result.output


@pytest.mark.asyncio
async def test_read_file_with_limit(read_tool, tool_context, sample_file):
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
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="does_not_exist.go"),
        tool_context,
    )
    assert result.is_error
    assert "not found" in result.output.lower() or "no such" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_is_directory(read_tool, tool_context, workspace):
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
    assert "\tline 2001\n" not in result.output
    # Line 1 SHOULD be in the output
    assert "\tline 1\n" in result.output


@pytest.mark.asyncio
async def test_read_file_byte_cap(read_tool, tool_context, workspace):
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
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="../etc/passwd"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()


@pytest.mark.asyncio
async def test_read_file_empty_file(read_tool, tool_context, workspace):
    (workspace / "empty.txt").write_text("")
    result = await _run_tool(
        read_tool,
        ReadFileInput(path="empty.txt"),
        tool_context,
    )
    assert not result.is_error
    # Empty file is OK; output is empty or a "(empty file)" note
    assert result.output == "" or "empty" in result.output.lower()


# ---------------------------------------------------------------------------
# WriteFileTool
# ---------------------------------------------------------------------------


@pytest.fixture
def write_tool(workspace):
    return WriteFileTool(workspace)


@pytest.mark.asyncio
async def test_write_file_new(write_tool, tool_context, workspace):
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
    result = await _run_tool(
        write_tool,
        WriteFileInput(path="a/b/c/deep.txt", content="deep\n"),
        tool_context,
    )
    assert not result.is_error
    assert (workspace / "a" / "b" / "c" / "deep.txt").read_text() == "deep\n"


@pytest.mark.asyncio
async def test_write_file_overwrites_existing(write_tool, tool_context, workspace):
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
    result = await _run_tool(
        write_tool,
        WriteFileInput(path="/etc/passwd", content="pwned"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower() or "absolute" in result.output.lower()


@pytest.mark.asyncio
async def test_write_file_parent_is_file(write_tool, tool_context, workspace):
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


# ---------------------------------------------------------------------------
# EditFileTool
# ---------------------------------------------------------------------------


@pytest.fixture
def edit_tool(workspace):
    return EditFileTool(workspace)


@pytest.mark.asyncio
async def test_edit_file_unique_match(edit_tool, tool_context, workspace):
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
    # old_string has 1 line, new_string has 3 -- delta: +3 -1
    assert "+3" in result.output or "3 added" in result.output
    assert "-1" in result.output or "1 removed" in result.output


@pytest.mark.asyncio
async def test_edit_file_is_directory(edit_tool, tool_context, workspace):
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


# ---------------------------------------------------------------------------
# GlobTool
# ---------------------------------------------------------------------------


@pytest.fixture
def glob_tool(workspace):
    return GlobTool(workspace)


@pytest.mark.asyncio
async def test_glob_simple_pattern(glob_tool, tool_context, workspace):
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
    old = workspace / "old.txt"
    old.write_text("")
    # Force old mtime (2 seconds ago)
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
    result = await _run_tool(
        glob_tool,
        GlobInput(pattern="*.go", path="../outside"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()


# ---------------------------------------------------------------------------
# GrepTool
# ---------------------------------------------------------------------------


@pytest.fixture
def grep_tool(workspace):
    return GrepTool(workspace)


def _require_ripgrep():
    if shutil.which("rg") is None:
        pytest.skip("ripgrep (rg) not installed; grep tests require it")


@pytest.mark.asyncio
async def test_grep_simple_match(grep_tool, tool_context, workspace):
    _require_ripgrep()
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
    result = await _run_tool(
        grep_tool,
        GrepInput(pattern="x", path="../outside"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()


@pytest.mark.asyncio
async def test_grep_rg_not_installed(workspace, tool_context, monkeypatch):
    """When rg is not found, GrepTool returns a clear error."""
    monkeypatch.setattr(shutil, "which", lambda _name: None)
    tool = GrepTool(workspace)
    result = await _run_tool(
        tool,
        GrepInput(pattern="x"),
        tool_context,
    )
    assert result.is_error
    assert "ripgrep" in result.output.lower()


# ---------------------------------------------------------------------------
# ListDirectoryTool
# ---------------------------------------------------------------------------


@pytest.fixture
def list_tool(workspace):
    return ListDirectoryTool(workspace)


@pytest.mark.asyncio
async def test_list_directory_root(list_tool, tool_context, workspace):
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
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="does_not_exist"),
        tool_context,
    )
    assert result.is_error
    assert "not found" in result.output.lower()


@pytest.mark.asyncio
async def test_list_directory_is_file(list_tool, tool_context, workspace):
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
    result = await _run_tool(
        list_tool,
        ListDirectoryInput(path="../outside"),
        tool_context,
    )
    assert result.is_error
    assert "escape" in result.output.lower()
