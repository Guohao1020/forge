# chronos · Phase 2 — BaseTool Refactor + WorkspacePath Type

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 2 of 7 · **Tasks:** 6 · **Depends on:** [Phase 0](phase-0-infrastructure.md) · **Unblocks:** Phase 3, Phase 4
> **Spec reference:** [Design spec §4.1 (BaseTool refactor), §4.2 (WorkspacePath type), §4.11 (SetPhaseTool)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Land the load-bearing abstractions the T2 tool set will build on:

1. **`BaseTool` refactored to an async generator contract.** Tools yield zero or more `StreamEvent`s during execution and exactly one `ToolResult` at the end. `SimpleTool` is a convenience subclass for the common case of "no mid-execution events, just return a result".
2. **`WorkspacePath` type.** A path guaranteed to be inside the workspace sandbox, constructed only via `WorkspacePath.resolve(root, user_path)`. Path escapes (`..`, absolute paths, symlink-out, null bytes) raise `PathEscapeError` at construction time — so downstream file tools never hit a raw escape bug.
3. **`context_tools.py` migrated to `SimpleTool`.** Six existing tools (`query_api_catalog`, `query_db_schema`, `query_business_rules`, `query_module_graph`, `read_project_file`) adapted to the new contract.
4. **Contract test suite.** Parametrized test that runs against every registered tool class: asserts exactly one `ToolResult` is yielded, tools return errors rather than raising, etc.
5. **Path adversarial suite.** 8 escape attempts verifying `WorkspacePath.resolve()` rejects them.
6. **`query.py` adapted to the new contract** in the same phase (not Phase 5) so the build never goes red between phases. The agent loop's `_execute_tool_call` function switches from `await tool.execute(...)` to `async for item in tool.execute(...)`.

**Completion gate:**
- `pytest ai-worker/tests/openharness/tools/ -v` passes (includes new `test_base_tool_contract.py` + `test_workspace_path.py` + `test_workspace_path_adversarial.py` + the migrated `test_context_tools.py`)
- `pytest ai-worker/tests/test_query_engine.py -v` still passes (with adapted `query.py`)
- `pytest ai-worker/tests/test_agent_loop.py -v` still passes
- No test in the repo raises `TypeError: object async_generator can't be used in 'await' expression`
- `python -c "from src.openharness.tools.base import BaseTool, SimpleTool; from src.openharness.tools.workspace_path import WorkspacePath; print('ok')"` prints `ok`

## Why this phase matters

Phase 2 is **the hinge**. Every other phase either:
- (Phase 3, 4) relies on the new `BaseTool` / `SimpleTool` contract to write new tools
- (Phase 5) calls `async for` on tools through the adapted agent loop

If Phase 2 is wrong, everything downstream has to re-ship. If Phase 2 is right but lands without adapting `query.py`, the build stays red for ~4 phases and every new tool's test suite has to isolate itself from transitive imports of `query.py`. **Task 2.6 (query.py adapt) is non-negotiable — it keeps main green.**

**Silicon-valley rule:** no hardcoded `if tool_name == "bash"` special cases in `query.py`. Every tool is treated identically by the agent loop; BashTool's ability to emit `ThinkingStarted` mid-execution comes from the tool itself yielding the event, not from the agent loop sniffing the tool name. This is the whole reason we're refactoring `BaseTool` — see [`feedback-silicon-valley-infra.md`](../../../.claude/projects/D--shulex-work-forge/memory/feedback-silicon-valley-infra.md) in the session memory.

---

### Task 2.1: Refactor `BaseTool` to async-generator contract + add `SimpleTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/base.py`
- Modify: `ai-worker/tests/test_tool_registry.py` (adapt existing tests to new signature if needed)

**Context:** The current `BaseTool.execute` is `async def execute(args, ctx) -> ToolResult`. The new signature is `def execute(args, ctx) -> AsyncIterator[StreamEvent | ToolResult]` — i.e., a standard Python async generator. Tools yield zero or more `StreamEvent` instances for mid-execution progress (Thinking, PhaseChanged, etc.) and then yield exactly one `ToolResult` as the final value.

`SimpleTool` is a convenience subclass that owns the async generator plumbing for the common case. Subclasses override `_execute_simple(args, ctx) -> ToolResult` (returns a value, no yield) and `SimpleTool.execute` wraps it in a one-shot async generator that yields only the result.

Rationale for async generator over a two-method API (e.g., `execute_stream` + `execute_simple`):
- Pythonic: async generators are the standard idiom for "produce a stream of values over time"
- Single code path in the caller: `query.py` doesn't need to branch on "is this a streaming tool or a simple tool"
- Tests can treat all tools uniformly via the contract test (Task 2.4)

**New type imports:** `AsyncIterator` from `collections.abc`, `StreamEvent` from `..engine.stream_events`. Note the import cycle risk — `tools/base.py` importing from `engine/stream_events.py` while `engine/query.py` imports from `tools/base.py`. This is fine because `stream_events.py` has no dependencies on `tools/` or `engine/query.py`; it's a leaf module with only dataclass definitions.

- [ ] **Step 1: Verify no cycle by checking `stream_events.py` imports**

Run:
```bash
grep -n "^from\|^import" ai-worker/src/openharness/engine/stream_events.py
```
Expected: imports are only `dataclasses`, `..api.usage`, `.messages`. No `tools` imports. Safe to import `stream_events` from `base.py`.

- [ ] **Step 2: Write the new `base.py`**

Replace the entire content of `ai-worker/src/openharness/tools/base.py` with:

```python
"""Tool abstractions — BaseTool, SimpleTool, ToolRegistry, ToolResult, ToolExecutionContext.

BaseTool is an async-generator-shaped abstraction: subclasses yield zero
or more StreamEvent instances during execution (to report progress,
thinking indicators, phase transitions, etc.) and then yield exactly
one ToolResult as the final value. The agent loop consumes the
generator, forwards StreamEvents to its own stream, and uses the
ToolResult to build the ToolResultBlock that goes back to the model.

SimpleTool is a convenience subclass for the common case of "tool
completes in one step, no mid-execution events". Subclasses override
_execute_simple and get the async-generator wrapping for free.

This file deliberately imports StreamEvent from engine.stream_events —
stream_events.py is a leaf module (no tools imports) so no cycle.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import AsyncIterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Union

from pydantic import BaseModel

# Forward reference to avoid a runtime import cycle. StreamEvent is
# defined in engine.stream_events which does not import from tools;
# importing at module top level is safe.
from ..engine.stream_events import StreamEvent


@dataclass
class ToolExecutionContext:
    """Runtime context passed to every tool invocation.

    cwd is the resolved workspace root for the current session. Tools
    that accept paths (read_file, write_file, etc.) must resolve them
    relative to cwd via WorkspacePath.resolve().
    """

    cwd: Path
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class ToolResult:
    """Immutable result returned by a tool execution.

    Tools must yield exactly one of these as the final item in their
    execute() generator. is_error=True signals that the tool caught a
    recoverable failure (invalid input, file not found, etc.) — the
    agent sees this as a ToolResultBlock(is_error=True) and can react.
    Uncaught Python exceptions are a bug; tools should catch them
    internally and return ToolResult(is_error=True, output=...).
    """

    output: str
    is_error: bool = False
    metadata: dict[str, Any] = field(default_factory=dict)


# The item type yielded by BaseTool.execute. A stream of zero or more
# StreamEvents, terminated by exactly one ToolResult.
ToolItem = Union[StreamEvent, ToolResult]


class BaseTool(ABC):
    """Abstract base for all OpenHarness tools.

    Subclasses MUST implement execute() as an async generator. They
    yield StreamEvents freely during execution and yield ToolResult
    exactly once as the final item.

    For the common case of "simple tool with no mid-execution events",
    subclass SimpleTool instead — it handles the generator wrapping.
    """

    name: str
    description: str
    input_model: type[BaseModel]

    @abstractmethod
    def execute(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> AsyncIterator[ToolItem]:
        """Run the tool. Yields zero or more StreamEvents followed by
        exactly one ToolResult. Must not raise on expected errors —
        return ToolResult(is_error=True, output=...) instead.

        This is a method that returns an AsyncIterator, which in Python
        means: subclasses define it with 'async def' and 'yield', and
        callers use 'async for item in tool.execute(...)'.
        """
        ...

    def is_read_only(self, arguments: BaseModel) -> bool:
        """Whether this invocation is purely read-only (safe to run in parallel)."""
        return False

    def to_api_schema(self) -> dict[str, Any]:
        """Serialize to Anthropic-compatible tool definition."""
        return {
            "name": self.name,
            "description": self.description,
            "input_schema": self.input_model.model_json_schema(),
        }


class SimpleTool(BaseTool):
    """Convenience subclass for tools that don't emit mid-execution events.

    Subclasses override _execute_simple() which returns a ToolResult
    directly (no yield). SimpleTool.execute wraps it in a one-shot
    async generator so the BaseTool contract is preserved without
    every subclass having to write `yield await self._execute_simple(...)`
    boilerplate.

    Example:
        class MyTool(SimpleTool):
            name = "my_tool"
            description = "..."
            input_model = MyInput

            async def _execute_simple(self, arguments, context):
                return ToolResult(output="done")
    """

    @abstractmethod
    async def _execute_simple(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> ToolResult:
        """Run the tool and return a ToolResult. No yielding."""
        ...

    async def execute(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> AsyncIterator[ToolItem]:
        """Adapt _execute_simple into the async-generator contract."""
        result = await self._execute_simple(arguments, context)
        yield result


class ToolRegistry:
    """Registry that holds named tool instances.

    Not thread-safe — intended to be built once at engine construction
    and read concurrently thereafter.
    """

    def __init__(self) -> None:
        self._tools: dict[str, BaseTool] = {}

    def register(self, tool: BaseTool) -> None:
        self._tools[tool.name] = tool

    def get(self, name: str) -> BaseTool | None:
        return self._tools.get(name)

    def list_tools(self) -> list[BaseTool]:
        return list(self._tools.values())

    def to_api_schema(self) -> list[dict[str, Any]]:
        return [t.to_api_schema() for t in self._tools.values()]
```

- [ ] **Step 3: Check existing `test_tool_registry.py` for breakage**

Run:
```bash
cd ai-worker && python -m pytest tests/test_tool_registry.py -v 2>&1 | tail -30
```

Expected: some tests may fail because they construct `BaseTool` subclasses with `async def execute() -> ToolResult` (old signature). Those subclasses must be updated to either yield or inherit from `SimpleTool`.

If there are failing tests, locate the subclass definitions inside `test_tool_registry.py` and update each one. Typical fix:

```python
# BEFORE
class FakeTool(BaseTool):
    name = "fake"
    description = "test"
    input_model = FakeInput

    async def execute(self, args, ctx):
        return ToolResult(output="fake")

# AFTER
class FakeTool(SimpleTool):
    name = "fake"
    description = "test"
    input_model = FakeInput

    async def _execute_simple(self, args, ctx):
        return ToolResult(output="fake")
```

- [ ] **Step 4: Run the tool_registry tests again until green**

```bash
cd ai-worker && python -m pytest tests/test_tool_registry.py -v
```
Expected: all tests pass.

- [ ] **Step 5: Verify the base.py module imports cleanly**

```bash
cd ai-worker && python -c "
from src.openharness.tools.base import BaseTool, SimpleTool, ToolRegistry, ToolResult, ToolExecutionContext, ToolItem
print('base.py imports:', BaseTool, SimpleTool, ToolRegistry, ToolResult, ToolExecutionContext, ToolItem)
"
```
Expected: one line of output naming all six symbols. If this fails with a circular import error, the `stream_events` import in `base.py` is wrong — double-check that `stream_events.py` has no `tools` imports.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/tools/base.py ai-worker/tests/test_tool_registry.py
git commit -m "refactor(tools): BaseTool is now an async generator contract

BaseTool.execute changes from 'async def ... -> ToolResult' to
'def ... -> AsyncIterator[StreamEvent | ToolResult]'. Subclasses
yield zero or more StreamEvents during execution (for mid-flight
progress like ThinkingStarted, PhaseChanged) and yield exactly one
ToolResult as the final item.

SimpleTool is a convenience subclass for the common case where the
tool just returns a result — subclasses override _execute_simple()
and get the async-generator wrapping for free. This keeps
context_tools and all read-only tools free of yield boilerplate.

Rationale: eliminates hardcoded 'if tool_name == bash' special cases
in the agent loop (query.py). Every tool is treated identically;
BashTool's ability to emit ThinkingStarted during execution comes
from the tool itself yielding the event, not from the loop sniffing
the tool name. Silicon-valley standard: no hardcoded special cases.

test_tool_registry.py's in-test BaseTool subclasses updated to
SimpleTool where applicable. query.py adaptation lands in Task 2.6
of this phase (not deferred to Phase 5) so the build stays green."
```

---

### Task 2.2: Create `WorkspacePath` type + happy-path tests

**Files:**
- Create: `ai-worker/src/openharness/tools/workspace_path.py`
- Create: `ai-worker/tests/openharness/tools/__init__.py`
- Create: `ai-worker/tests/openharness/tools/test_workspace_path.py`

**Context:** Every file-operating tool needs a path that's guaranteed to stay inside the workspace. Doing this with a helper function (`resolve_path(cwd, user_path)`) that each tool calls means every tool author must remember to call it — a drift risk. The silicon-valley rule says "make the contract a type". So path escape protection is enforced by the `WorkspacePath` class: the only way to get one is via `WorkspacePath.resolve(root, user_path)`, which raises `PathEscapeError` on any escape attempt.

Tools then accept `path: str` at the Pydantic input layer, and in their `execute` they call `WorkspacePath.resolve(context.cwd, arguments.path)`. If that raises, they return `ToolResult(is_error=True, ...)`.

Why not make the Pydantic field itself a `WorkspacePath`? Because the workspace root isn't available at Pydantic validation time (it's in the execution context). Pydantic validators receive no external state. So `str → WorkspacePath` happens one layer later, at the start of `execute()`.

The path-resolution algorithm handles these cases (spec §4.2):
- Absolute path → reject (`/etc/passwd`)
- Relative path containing `..` → reject (`../other`)
- Nested `..` that climbs out → reject (`a/b/../../../etc`)
- Null byte → reject (`foo\x00.txt`)
- Normal relative path → accept (`src/main.go`)
- Deep relative path → accept (`a/b/c/d/e.txt`)
- Workspace root → accept (`.`)
- Symlink pointing outside → reject (detected via `Path.resolve()` checking the final absolute location is under the workspace root)

This task covers the happy path and the class itself. The adversarial tests (8 escape attempts) live in Task 2.5.

- [ ] **Step 1: Write the happy-path tests first**

Create the test directory structure (the `openharness/tools/` subdir doesn't exist yet):

```bash
mkdir -p ai-worker/tests/openharness/tools
touch ai-worker/tests/openharness/__init__.py
touch ai-worker/tests/openharness/tools/__init__.py
```

Create `ai-worker/tests/openharness/tools/test_workspace_path.py`:

```python
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
    # relative should be empty or Path(".") depending on impl — both acceptable
    assert wp.relative in (Path("."), Path(""))


def test_resolve_current_dir_prefix(workspace: Path):
    """'./src/main.go' should normalize to 'src/main.go'."""
    wp = WorkspacePath.resolve(workspace, "./src/main.go")
    assert wp.relative == Path("src/main.go")


def test_resolve_nonexistent_file_still_works(workspace: Path):
    """Path resolution doesn't require the file to exist — the tool
    that gets the WorkspacePath handles file-existence checks."""
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
    """PathEscapeError should subclass ValueError so callers can
    catch it via the broader exception hierarchy if they want."""
    assert issubclass(PathEscapeError, ValueError)
```

- [ ] **Step 2: Run the tests — expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_workspace_path.py -v
```
Expected: `ModuleNotFoundError: src.openharness.tools.workspace_path`. That's the intended TDD failure state.

- [ ] **Step 3: Implement `workspace_path.py`**

Create `ai-worker/src/openharness/tools/workspace_path.py`:

```python
"""WorkspacePath — a path guaranteed to be inside a workspace sandbox.

This is the silicon-valley-grade answer to "how do we prevent path
escapes in file tools?". Rather than making every tool author remember
to call a sanitize_path() helper (drift risk), path safety is a type
contract: the only way to construct a WorkspacePath is via
WorkspacePath.resolve(workspace_root, user_path), which raises
PathEscapeError if the input would escape the workspace.

Design notes:
- The workspace_root is resolved (via Path.resolve) once at construction
  time so the comparison uses canonical absolute paths — this catches
  symlink escapes (a link inside the workspace pointing to /etc).
- Path resolution does NOT require the file to exist. Tools check file
  existence themselves after getting a WorkspacePath.
- Null bytes are rejected explicitly because some filesystems accept
  them in path syntax but they break pathname-based security checks.
- '..' segments in the resolved path are rejected even when resolve()
  eliminates them, as a defense-in-depth check.

Escape cases covered:
- Absolute paths: "/etc/passwd"
- Parent traversal: "../other"
- Deep parent traversal: "a/b/../../../etc"
- Null bytes: "foo\\x00bar"
- Symlink out: a symlink inside the workspace pointing outside it
  (caught by Path.resolve() returning an absolute path outside the
  workspace_root.resolve())
"""

from __future__ import annotations

from pathlib import Path


class PathEscapeError(ValueError):
    """Raised when a user-provided path would escape the workspace sandbox.

    Subclasses ValueError so callers can catch it via either the
    specific or the broader exception type.
    """


class WorkspacePath:
    """A path guaranteed to be inside a workspace sandbox.

    Never construct directly. Use WorkspacePath.resolve() which
    enforces escape checks at construction time.

    Attributes:
        workspace_root: The absolute, resolved (symlinks followed)
            workspace root directory. All WorkspacePaths from the
            same resolve() call share the same workspace_root.
        relative: The safe relative path from workspace_root to the
            target. Always a proper relative Path (no leading slash,
            no '..' segments).
    """

    __slots__ = ("workspace_root", "relative")

    def __init__(self, workspace_root: Path, relative: Path) -> None:
        # Intentionally no validation here — callers must go through
        # resolve(). Direct construction is considered a bug but not
        # explicitly prevented (Python doesn't do private constructors).
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

        Does NOT require the target file to exist. Symlinks are
        followed during resolution so a symlink inside the workspace
        pointing to /etc will be caught.
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

        # Canonicalize the workspace root once. Path.resolve() also
        # follows symlinks on the root itself, which handles the case
        # where workspace_root is /var/symlink -> /data/forge/ws.
        root_abs = workspace_root.resolve()

        # Build the target absolute path and resolve it. Path.resolve
        # with strict=False (the default) does not require the target
        # to exist but does resolve any existing intermediate symlinks.
        target_abs = (root_abs / p).resolve()

        # Check that target_abs is inside root_abs. We use the
        # canonical strings + separator to avoid a subtle bug where
        # a workspace /foo and a target /foo2 would match by prefix.
        try:
            relative = target_abs.relative_to(root_abs)
        except ValueError:
            raise PathEscapeError(
                f"path escapes workspace: {user_path!r} resolved to {target_abs}"
            )

        # Defense-in-depth: even if relative_to succeeded, reject any
        # '..' in the relative parts. This is unreachable in practice
        # (relative_to would have failed first) but belt-and-braces.
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
```

- [ ] **Step 4: Run the tests and confirm they pass**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_workspace_path.py -v
```
Expected: 8 tests pass (`test_resolve_simple_file`, `test_resolve_deep_file`, `test_resolve_workspace_root`, `test_resolve_current_dir_prefix`, `test_resolve_nonexistent_file_still_works`, `test_resolve_is_idempotent_for_valid_paths`, `test_resolve_preserves_workspace_root_reference`, `test_path_escape_error_is_value_error`).

If any fail, fix the implementation before moving on. Do not proceed to the adversarial suite (Task 2.5) on a broken happy path.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/tools/workspace_path.py ai-worker/tests/openharness/
git commit -m "feat(tools): WorkspacePath type enforces sandbox boundary

WorkspacePath is a type-level contract: the only way to construct
one is via WorkspacePath.resolve(workspace_root, user_path), which
raises PathEscapeError on any escape attempt. File tools in Phase 3
consume this type and never see unsafe paths.

Escape detection:
- absolute paths (/etc/passwd) — rejected
- parent traversal (../other) — rejected
- deep climb (a/b/../../../etc) — rejected via Path.resolve
- null bytes (foo\\x00bar) — rejected explicitly
- symlink out — rejected because Path.resolve follows symlinks
  and the resolved target is checked for containment under
  workspace_root.resolve()

Workspace root is canonicalized via Path.resolve at construction
time, so a symlinked workspace_root (/var/symlink -> /data/ws)
still works.

Happy-path tests in tests/openharness/tools/test_workspace_path.py
(8 tests). Adversarial escape tests land in Task 2.5."
```

---

### Task 2.3: Migrate `context_tools.py` to `SimpleTool`

**Files:**
- Modify: `ai-worker/src/openharness/tools/context_tools.py`
- Modify: `ai-worker/tests/test_context_tools.py` (if existing tests need updates)

**Context:** Six existing tools (`QueryApiCatalogTool`, `QueryDbSchemaTool`, `QueryBusinessRulesTool`, `QueryModuleGraphTool`, `ReadProjectFileTool`) currently subclass `BaseTool` with the old `async def execute() -> ToolResult` signature. They all do "return a result, no mid-execution events" — the textbook case for `SimpleTool`. This task switches their base class and renames `execute` → `_execute_simple`.

No behavior changes. The regression suite in `test_context_tools.py` should still pass without modification (unless tests directly call `tool.execute(...)` and expect an awaitable — in which case they need to update to `async for`).

- [ ] **Step 1: Check how existing tests call `execute()`**

```bash
grep -n "\.execute(" ai-worker/tests/test_context_tools.py
```
Expected: if tests await `tool.execute(...)`, they break. Update them to consume the async generator. If tests hit the tool via a higher-level wrapper (e.g., the agent loop or a test helper), check the wrapper is compatible.

- [ ] **Step 2: Update each tool class**

Edit `ai-worker/src/openharness/tools/context_tools.py`. For each of the six tool classes, change:

1. `from .base import BaseTool, ToolExecutionContext, ToolRegistry, ToolResult` → add `SimpleTool` to the import list
2. `class QueryApiCatalogTool(BaseTool):` → `class QueryApiCatalogTool(SimpleTool):`
3. `async def execute(self, arguments, context) -> ToolResult:` → `async def _execute_simple(self, arguments, context) -> ToolResult:`

Do the same for the other five: `QueryDbSchemaTool`, `QueryBusinessRulesTool`, `QueryModuleGraphTool`, `ReadProjectFileTool`.

The final import line in `context_tools.py` should look like:

```python
from src.openharness.tools.base import (
    BaseTool,
    SimpleTool,
    ToolExecutionContext,
    ToolRegistry,
    ToolResult,
)
```

(Keep `BaseTool` in the import list — `register_context_tools`'s type annotations may still reference it, and pytest import-time type-checkers may want it.)

- [ ] **Step 3: Run the context_tools tests**

```bash
cd ai-worker && python -m pytest tests/test_context_tools.py -v
```
Expected: all tests pass (or skip for the usual infrastructure reasons — missing forge-core for the HTTP-backed `ReadProjectFileTool`). If a test fails because it awaits `tool.execute()` directly, update it to:

```python
# BEFORE
result = await tool.execute(args, ctx)
assert result.output == "..."

# AFTER
results = []
async for item in tool.execute(args, ctx):
    results.append(item)
# ToolResult is the last item; earlier items (if any) would be StreamEvents
tool_result = results[-1]
assert isinstance(tool_result, ToolResult)
assert tool_result.output == "..."
```

For SimpleTool instances there will be exactly one item (the ToolResult), so `results[0]` and `results[-1]` are equivalent.

- [ ] **Step 4: Verify `register_context_tools` still works**

```bash
cd ai-worker && python -c "
from unittest.mock import MagicMock
from src.openharness.tools.base import ToolRegistry
from src.openharness.tools.context_tools import register_context_tools
reg = ToolRegistry()
register_context_tools(reg, profiles={'api_catalog': {'endpoints': []}}, project_id=1)
print('registered tools:', [t.name for t in reg.list_tools()])
"
```
Expected: `registered tools: ['query_api_catalog', 'query_db_schema', 'query_business_rules', 'query_module_graph', 'read_project_file']`.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/tools/context_tools.py ai-worker/tests/test_context_tools.py
git commit -m "refactor(tools): migrate context_tools to SimpleTool

All six context tools (query_api_catalog, query_db_schema,
query_business_rules, query_module_graph, read_project_file) change
from 'BaseTool with async execute' to 'SimpleTool with async
_execute_simple'. No behavior change — SimpleTool handles the
async-generator wrapping and yields the result as a one-shot stream.

Existing test_context_tools.py tests adapted where they called
tool.execute() directly: now they consume the async generator
and assert on the last item (the ToolResult).

This completes the in-place migration of every existing BaseTool
subclass to the new contract. Phase 3/4 new tools write to the
new signature from the start."
```

---

### Task 2.4: Parametrized contract test for every registered tool

**Files:**
- Create: `ai-worker/tests/openharness/tools/test_base_tool_contract.py`
- Create: `ai-worker/tests/openharness/tools/conftest.py`

**Context:** Every tool must satisfy the BaseTool contract:
1. `execute()` yields zero or more `StreamEvent`s followed by exactly one `ToolResult`
2. `execute()` does not raise on expected failure modes — it returns `ToolResult(is_error=True, ...)` instead
3. The `ToolResult` is the LAST item yielded (nothing yields after it)
4. The `name` / `description` / `input_model` class attributes are set

This test is **parametrized over every tool class the codebase knows about** — the parametrization list is built at test collection time. When Phase 3/4 add new tools (`ReadFileTool`, `WriteFileTool`, `BashTool`, etc.), they're auto-added to the contract suite just by being importable. No new test code needed per tool.

The contract test runs against "valid-enough" arguments — it doesn't test business logic correctness, only the contract itself. Each tool provides a minimal fake argument via a test-local factory pattern.

- [ ] **Step 1: Create a shared conftest for tool tests**

Create `ai-worker/tests/openharness/tools/conftest.py`:

```python
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
```

- [ ] **Step 2: Write the contract test**

Create `ai-worker/tests/openharness/tools/test_base_tool_contract.py`:

```python
"""Parametrized contract tests for every registered BaseTool subclass.

This test suite runs against every importable tool class in the code-
base. When a new tool is added in Phase 3/4, it's auto-included by
appending it to ALL_TOOL_SPECS below. The parametrization ensures
every tool satisfies:

  1. execute() is an async generator
  2. It yields zero or more StreamEvents then exactly one ToolResult
  3. The ToolResult is the LAST item yielded
  4. name / description / input_model class attrs are set

Contract tests do NOT verify business logic — each tool's dedicated
test file (test_file_tools.py, test_bash_adversarial.py, etc.) does
that. Contract tests verify the wire protocol.
"""

from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any, Callable

import pytest

from src.openharness.engine.stream_events import StreamEvent
from src.openharness.tools.base import (
    BaseTool,
    ToolExecutionContext,
    ToolResult,
)


# ---------------------------------------------------------------------------
# Tool spec table — auto-extended as new tools land in Phase 3/4.
# ---------------------------------------------------------------------------

ToolSpec = tuple[
    type[BaseTool],                          # class
    Callable[[Path], BaseTool],              # factory (workspace-aware)
    Callable[[], Any],                        # argument factory
]


def _make_context_profiles() -> dict[str, Any]:
    """Minimal profiles dict for context tools."""
    return {
        "api_catalog": {"endpoints": []},
        "db_schema": {"tables": []},
        "business_rules": {"rules": []},
        "module_graph": {"modules": []},
    }


# Phase 2 populates the existing tools from context_tools.py. Phase 3/4
# will append new rows for ReadFileTool, WriteFileTool, etc.
def _all_tool_specs() -> list[ToolSpec]:
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
    # NOTE: ReadProjectFileTool deliberately omitted because its
    # execute path makes an HTTP request to forge-core — it's tested
    # in test_context_tools.py with a mock HTTP client. The contract
    # test here doesn't mock HTTP; it runs tools as-is and assumes
    # "no network needed" for the contract test.

    profiles = _make_context_profiles()

    specs: list[ToolSpec] = [
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
    ]

    # Phase 3/4 will append additional specs here for ReadFileTool,
    # WriteFileTool, EditFileTool, GlobTool, GrepTool, ListDirectoryTool,
    # BashTool, SetPhaseTool. When adding a new tool, add its spec to
    # this function and the contract suite runs against it automatically.

    return specs


ALL_TOOL_SPECS = _all_tool_specs()


# ---------------------------------------------------------------------------
# Contract tests
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "tool_class,factory,arg_factory",
    ALL_TOOL_SPECS,
    ids=[spec[0].__name__ for spec in ALL_TOOL_SPECS],
)
async def test_tool_yields_exactly_one_tool_result(
    tool_class: type[BaseTool],
    factory: Callable[[Path], BaseTool],
    arg_factory: Callable[[], Any],
    workspace: Path,
    tool_context: ToolExecutionContext,
):
    """Every tool must yield exactly one ToolResult. StreamEvents are
    fine and can be any count."""
    tool = factory(workspace)
    arguments = arg_factory()

    items: list[Any] = []
    async for item in tool.execute(arguments, tool_context):
        items.append(item)

    tool_results = [i for i in items if isinstance(i, ToolResult)]
    stream_events = [i for i in items if isinstance(i, StreamEvent)]
    other = [i for i in items if not isinstance(i, (ToolResult, StreamEvent))]

    assert len(tool_results) == 1, (
        f"{tool_class.__name__} yielded {len(tool_results)} ToolResults, "
        f"expected exactly 1"
    )
    assert not other, (
        f"{tool_class.__name__} yielded non-StreamEvent non-ToolResult items: "
        f"{[type(x).__name__ for x in other]}"
    )
    # Stream events before result is fine; it's how BashTool signals Thinking
    # (in Phase 4). Log the count for debug visibility.
    print(f"{tool_class.__name__}: {len(stream_events)} stream events, 1 result")


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "tool_class,factory,arg_factory",
    ALL_TOOL_SPECS,
    ids=[spec[0].__name__ for spec in ALL_TOOL_SPECS],
)
async def test_tool_result_is_last_item(
    tool_class: type[BaseTool],
    factory: Callable[[Path], BaseTool],
    arg_factory: Callable[[], Any],
    workspace: Path,
    tool_context: ToolExecutionContext,
):
    """The ToolResult must be yielded LAST. Nothing comes after."""
    tool = factory(workspace)
    arguments = arg_factory()

    items: list[Any] = []
    async for item in tool.execute(arguments, tool_context):
        items.append(item)

    assert len(items) > 0, f"{tool_class.__name__} yielded nothing"
    assert isinstance(items[-1], ToolResult), (
        f"{tool_class.__name__}'s last yielded item is "
        f"{type(items[-1]).__name__}, expected ToolResult"
    )


@pytest.mark.parametrize(
    "tool_class",
    [spec[0] for spec in ALL_TOOL_SPECS],
    ids=[spec[0].__name__ for spec in ALL_TOOL_SPECS],
)
def test_tool_class_attrs_set(tool_class: type[BaseTool]):
    """Every tool class must have name, description, input_model."""
    assert hasattr(tool_class, "name") and isinstance(tool_class.name, str)
    assert tool_class.name, f"{tool_class.__name__}.name is empty"

    assert hasattr(tool_class, "description") and isinstance(tool_class.description, str)
    assert tool_class.description, f"{tool_class.__name__}.description is empty"

    assert hasattr(tool_class, "input_model"), (
        f"{tool_class.__name__} missing input_model attr"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "tool_class,factory,arg_factory",
    ALL_TOOL_SPECS,
    ids=[spec[0].__name__ for spec in ALL_TOOL_SPECS],
)
async def test_tool_execute_returns_async_iterator(
    tool_class: type[BaseTool],
    factory: Callable[[Path], BaseTool],
    arg_factory: Callable[[], Any],
    workspace: Path,
    tool_context: ToolExecutionContext,
):
    """tool.execute(...) must return an AsyncIterator (generator object).

    Calling it must NOT raise — raising at call time means the tool
    made execute a normal async function that returns a coroutine
    rather than a generator. That's the old contract and is a bug.
    """
    tool = factory(workspace)
    arguments = arg_factory()

    gen = tool.execute(arguments, tool_context)
    assert hasattr(gen, "__anext__"), (
        f"{tool_class.__name__}.execute did not return an async iterator; "
        f"got {type(gen).__name__}. Did you use 'async def ... return' "
        f"instead of 'async def ... yield'?"
    )
    # Consume the generator so it doesn't leak an unclosed resource
    async for _ in gen:
        pass
```

- [ ] **Step 3: Run the contract tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_base_tool_contract.py -v
```
Expected: 16 tests pass (4 test functions × 4 tool classes). If any fail, the failing tool has a contract bug — fix the tool before proceeding.

- [ ] **Step 4: Verify the parametrization IDs are readable**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_base_tool_contract.py --collect-only -q 2>&1 | head -25
```
Expected: test IDs include tool class names like `test_tool_yields_exactly_one_tool_result[QueryApiCatalogTool]`.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/tests/openharness/tools/conftest.py ai-worker/tests/openharness/tools/test_base_tool_contract.py
git commit -m "test(tools): parametrized BaseTool contract suite

Four contract tests parametrized over every tool class in
ALL_TOOL_SPECS:
  1. yields exactly one ToolResult
  2. ToolResult is the last item
  3. class attrs (name/description/input_model) are set
  4. execute() returns an async iterator (not a coroutine)

Phase 2 populates specs for the four context tools that don't need
HTTP (ReadProjectFileTool is tested separately in test_context_tools
with a mocked client). Phase 3/4 will append specs for
ReadFileTool, WriteFileTool, EditFileTool, GlobTool, GrepTool,
ListDirectoryTool, BashTool, SetPhaseTool as those land.

The parametrization is a single list in _all_tool_specs(); adding
a new tool to the contract suite is a one-line change when the
tool is added. This is the silicon-valley rule 'contract as
mechanical gate' in action — BaseTool is not a documented
convention, it's a tested constraint."
```

---

### Task 2.5: Adversarial WorkspacePath tests

**Files:**
- Create: `ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py`

**Context:** Happy-path tests in Task 2.2 verified `WorkspacePath.resolve` works on normal input. This task verifies it **rejects all known escape shapes** — the defensive side of the contract. Each test is a specific attack vector.

Spec §7.1 lists 8 cases as P0. The test names map 1:1 to the spec's adversarial test matrix.

Why separate from the happy-path file? So the CI log makes it obvious when an adversarial check fails — adversarial tests failing is a P0 security regression, not a general test failure. Separating the files lets the team/agent filter.

- [ ] **Step 1: Write the adversarial tests**

Create `ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py`:

```python
"""Adversarial path-escape tests for WorkspacePath.resolve.

Each test is a named attack vector from spec §7.1. A failure in this
file is a P0 security regression — these tests are the mechanical
guarantee that WorkspacePath prevents sandbox escapes.

Related unit tests (happy path only) are in test_workspace_path.py.
The file split is intentional: CI failures under
test_workspace_path_adversarial.py mean the sandbox is broken;
failures under test_workspace_path.py usually mean normal path
logic is broken.
"""

import os
from pathlib import Path

import pytest

from src.openharness.tools.workspace_path import PathEscapeError, WorkspacePath


# ---------------------------------------------------------------------------
# The 8 adversarial cases from spec §7.1
# ---------------------------------------------------------------------------


def test_reject_absolute_path(tmp_path: Path):
    """Absolute paths to anywhere outside the workspace must be rejected."""
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


def test_reject_symlink_pointing_outside(tmp_path: Path):
    """A symlink inside the workspace that points to /etc must be caught.

    This test creates a workspace with a symlink entry, then asks
    resolve() for that symlink's path. Because Path.resolve() follows
    symlinks, the target becomes an absolute path outside the workspace,
    and the containment check fails.
    """
    workspace = tmp_path / "ws"
    workspace.mkdir()
    link_path = workspace / "escape"

    # Point the symlink at an absolute path outside the workspace
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
    """A deeply nested relative path that resolves to $HOME or / must be rejected.

    On a workspace at /tmp/pytest-xxx/workspace, the path '../../../../etc'
    might end up at /etc. Resolve must catch this.
    """
    with pytest.raises(PathEscapeError, match="escapes workspace"):
        WorkspacePath.resolve(tmp_path, "../../../../etc")


# ---------------------------------------------------------------------------
# Defense-in-depth: multiple escape shapes in one path
# ---------------------------------------------------------------------------


def test_reject_mixed_escape(tmp_path: Path):
    """Combination: leading './' + '..' climb + final 'passwd'."""
    with pytest.raises(PathEscapeError):
        WorkspacePath.resolve(tmp_path, "./foo/../../../../../etc/passwd")


def test_reject_backslash_escape_on_posix(tmp_path: Path):
    """Backslashes in paths on POSIX are not directory separators but
    users sometimes type them expecting Windows behavior. They should
    be treated as literal characters, and if the resulting literal
    path doesn't escape, it should be accepted — we're just checking
    resolve doesn't crash on them."""
    if os.name != "posix":
        pytest.skip("backslash semantics only meaningful on POSIX")
    # Creates a workspace and a file literally named "foo\\bar"
    (tmp_path / "foo\\bar").touch()
    wp = WorkspacePath.resolve(tmp_path, "foo\\bar")
    assert wp.absolute.exists()


# ---------------------------------------------------------------------------
# Symlink-to-root edge: workspace root itself is a symlink
# ---------------------------------------------------------------------------


def test_workspace_root_as_symlink_still_works(tmp_path: Path):
    """If workspace_root is a symlink pointing to a real directory,
    resolve must canonicalize it and still accept legitimate paths
    inside it."""
    real_ws = tmp_path / "real_workspace"
    real_ws.mkdir()
    (real_ws / "file.txt").write_text("hello")

    link_ws = tmp_path / "link_workspace"
    os.symlink(real_ws, link_ws)

    wp = WorkspacePath.resolve(link_ws, "file.txt")
    assert wp.absolute.read_text() == "hello"
    # workspace_root is canonicalized to the real path, not the link
    assert wp.workspace_root == real_ws.resolve()
```

- [ ] **Step 2: Run the adversarial tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_workspace_path_adversarial.py -v
```
Expected: **11 tests pass** (8 from the spec + 3 defense-in-depth additions). If any fail, the WorkspacePath implementation has a security gap — fix it and re-run. **Do not commit this file if any adversarial test fails; that means the feature is not shipping with the safety net claimed.**

- [ ] **Step 3: Manually inspect each failure message to ensure they're informative**

For each adversarial test that passes, the `PathEscapeError` message should name the specific reason (absolute, null byte, escape). Debug the messages by running a single test with `-s`:

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_workspace_path_adversarial.py::test_reject_null_byte -v -s
```

The exception message appears in the test output. Good messages guide debugging if an agent ever hits these in practice.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py
git commit -m "test(tools): 11 adversarial path-escape tests for WorkspacePath

Spec §7.1 lists 8 P0 cases; this file implements all of them plus
3 defense-in-depth additions:
  - absolute path rejection
  - parent traversal ('../other')
  - nested parent traversal ('a/b/../../../etc')
  - symlink inside workspace pointing outside
  - null byte in path
  - empty string
  - None input (no bare TypeError)
  - deep '../../../../etc' climb
  - mixed shape './foo/../../etc/passwd'
  - backslash literal on POSIX (not a separator, treat as filename)
  - workspace root itself as a symlink (canonicalized correctly)

Failure in this file is a P0 security regression — the sandbox
is broken. Separation from the happy-path test_workspace_path.py
lets CI log readers distinguish 'normal bug' from 'security
regression' at a glance."
```

---

### Task 2.6: Adapt `query.py` to the new async-generator contract

**Files:**
- Modify: `ai-worker/src/openharness/engine/query.py`

**Context:** This is the task that keeps the build green. `query.py:124` currently has:

```python
result = await tool.execute(parsed, exec_ctx)
```

After Task 2.1, `tool.execute(...)` returns an async generator. Awaiting it raises `TypeError: object async_generator can't be used in 'await' expression`. Every test that transitively imports `query.py` fails.

This task rewrites `_execute_tool_call` to consume the generator with `async for`, forwarding any intermediate `StreamEvent`s back to the caller (`run_agent_loop`). Because `_execute_tool_call` is currently a regular `async def` that returns `ToolResultBlock`, it must become an async generator too (yields `StreamEvent`s then the final `ToolResultBlock`). `run_agent_loop` is updated accordingly.

Spec §4.1 shows the exact code shape. This is a small targeted change — no new logic, just the control-flow update.

- [ ] **Step 1: Read the current `_execute_tool_call` and the calling `run_agent_loop` tool loop**

```bash
sed -n '70,150p' ai-worker/src/openharness/engine/query.py
```
Expected: you see `_execute_tool_call` ending at ~line 147 with `return ToolResultBlock(...)`, and the tool loop in `run_agent_loop` at ~line 204-217 that calls `result = await _execute_tool_call(...)`.

- [ ] **Step 2: Update `_execute_tool_call` to be an async generator**

Edit `ai-worker/src/openharness/engine/query.py`. Find the function definition:

```python
async def _execute_tool_call(
    context: QueryContext,
    tool_name: str,
    tool_use_id: str,
    tool_input: Dict[str, Any],
) -> ToolResultBlock:
    """Execute a single tool call with hooks and permission checks."""
```

Change the signature and body to an async generator. Replace the entire function with:

```python
async def _execute_tool_call(
    context: QueryContext,
    tool_name: str,
    tool_use_id: str,
    tool_input: Dict[str, Any],
) -> AsyncIterator[Any]:  # yields StreamEvents then a ToolResultBlock
    """Execute a single tool call with hooks and permission checks.

    Consumes the tool's async-generator execute() and yields:
      - zero or more StreamEvents (forwarded as-is to the caller)
      - exactly one ToolResultBlock as the final item

    Hook failures and permission denials short-circuit with a single
    ToolResultBlock(is_error=True) and no StreamEvents.
    """

    # 1. Pre-tool hook
    if context.hook_executor:
        payload = {"tool_name": tool_name, "tool_input": tool_input}
        hook_result = await context.hook_executor.execute(HookEvent.PRE_TOOL_USE, payload)
        if hook_result.blocked:
            reasons = hook_result.all_reasons
            yield ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"BLOCKED by hook: {'; '.join(reasons)}",
                is_error=True,
            )
            return

    # 2. Permission check
    if context.permission_checker:
        tool_obj = context.tool_registry.get(tool_name)
        is_ro = tool_obj.is_read_only(tool_input) if tool_obj else False
        decision = context.permission_checker.evaluate(tool_name, is_read_only=is_ro)
        if not decision.allowed:
            yield ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"Permission denied: {decision.reason}",
                is_error=True,
            )
            return

    # 3. Tool lookup
    tool = context.tool_registry.get(tool_name)
    if not tool:
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Unknown tool: {tool_name}",
            is_error=True,
        )
        return

    # 4. Input validation
    try:
        parsed = tool.input_model.model_validate(tool_input)
    except Exception as e:
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Invalid input: {e}",
            is_error=True,
        )
        return

    # 5. Tool execution — consume the async generator
    exec_ctx = ToolExecutionContext(cwd=context.cwd)
    tool_result: ToolResult | None = None
    try:
        async for item in tool.execute(parsed, exec_ctx):
            if isinstance(item, ToolResult):
                if tool_result is not None:
                    raise RuntimeError(
                        f"tool {tool_name} yielded multiple ToolResults"
                    )
                tool_result = item
            elif isinstance(item, StreamEvent):
                # Forward mid-execution events up to run_agent_loop
                yield item
            else:
                raise TypeError(
                    f"tool {tool_name} yielded unexpected type: {type(item).__name__}"
                )
    except Exception as e:
        logger.exception("Tool execution failed: %s", tool_name)
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Tool execution error: {e}",
            is_error=True,
        )
        return

    if tool_result is None:
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Tool {tool_name} did not yield a ToolResult",
            is_error=True,
        )
        return

    # 6. Post-tool hook
    if context.hook_executor:
        payload = {
            "tool_name": tool_name,
            "tool_input": tool_input,
            "tool_output": tool_result.output,
            "is_error": tool_result.is_error,
        }
        await context.hook_executor.execute(HookEvent.POST_TOOL_USE, payload)

    yield ToolResultBlock(
        tool_use_id=tool_use_id,
        content=tool_result.output,
        is_error=tool_result.is_error,
    )
```

Add the necessary imports at the top of `query.py`:

```python
from collections.abc import AsyncIterator
# (ToolResult is likely already imported via the tools/base module;
# if not, add: from ..tools.base import ToolResult)
```

Verify `ToolResult` and `StreamEvent` are both imported. Both likely already are (`StreamEvent` for yielding text deltas, `ToolResult` indirectly). If not, add them.

- [ ] **Step 3: Update the caller in `run_agent_loop`**

Find the tool execution block in `run_agent_loop`, currently around line 200-217:

```python
        # Execute tool calls
        tool_results: List[ToolResultBlock] = []
        for tu in tool_uses:
            yield ToolExecutionStarted(tool_name=tu.name, tool_input=tu.input)
            result = await _execute_tool_call(
                context=context,
                tool_name=tu.name,
                tool_use_id=tu.id,
                tool_input=tu.input,
            )
            tool_results.append(result)
            yield ToolExecutionCompleted(
                tool_name=tu.name,
                output=result.content,
                is_error=result.is_error,
            )
```

Replace with:

```python
        # Execute tool calls
        tool_results: List[ToolResultBlock] = []
        for tu in tool_uses:
            yield ToolExecutionStarted(tool_name=tu.name, tool_input=tu.input)

            # _execute_tool_call is an async generator that yields
            # zero or more StreamEvents followed by exactly one
            # ToolResultBlock. Forward StreamEvents upstream; capture
            # the ToolResultBlock as the tool's final result.
            final_block: ToolResultBlock | None = None
            async for item in _execute_tool_call(
                context=context,
                tool_name=tu.name,
                tool_use_id=tu.id,
                tool_input=tu.input,
            ):
                if isinstance(item, ToolResultBlock):
                    final_block = item
                else:
                    # Mid-execution StreamEvent (e.g., ThinkingStarted
                    # from BashTool in Phase 4). Pass through.
                    yield item

            assert final_block is not None, (
                f"_execute_tool_call yielded no ToolResultBlock for {tu.name}"
            )
            tool_results.append(final_block)
            yield ToolExecutionCompleted(
                tool_name=tu.name,
                output=final_block.content,
                is_error=final_block.is_error,
            )
```

- [ ] **Step 4: Run the query_engine tests**

```bash
cd ai-worker && python -m pytest tests/test_query_engine.py tests/test_agent_loop.py -v 2>&1 | tail -30
```
Expected: all existing tests pass. If a test specifically exercised the old `await tool.execute()` shape, update it to the new async-for pattern.

- [ ] **Step 5: Run the full ai-worker test suite as a smoke check**

```bash
cd ai-worker && python -m pytest tests/ -x --ignore=tests/e2e 2>&1 | tail -40
```
Expected: the suite runs to completion. Failures are acceptable only for tests that reach external dependencies (real LLM, PG without setup, etc.). No `TypeError: object async_generator can't be used in 'await' expression` anywhere.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/engine/query.py
git commit -m "refactor(query): consume BaseTool async-generator contract

_execute_tool_call becomes an async generator that yields zero or
more StreamEvents followed by exactly one ToolResultBlock. The
caller in run_agent_loop loops over it with async for, forwarding
StreamEvents to its own yield stream and capturing the final
ToolResultBlock for the tool_results list.

This is the third leg of Phase 2: without this, every tool call
at runtime would raise TypeError('object async_generator can't be
used in await expression'). Landing it inside Phase 2 (not Phase
5 as originally scoped) keeps main green between phases — Phase 3
and Phase 4 can write new tools against the contract and their
tests run against a working query.py.

No new logic; just control-flow adaptation to the new contract.
Hook and permission-check code paths are unchanged, they just
yield the ToolResultBlock with return instead of returning it."
```

---

## Phase 2 completion check

Before starting Phase 3:

- [ ] `python -c "from src.openharness.tools.base import BaseTool, SimpleTool, ToolRegistry, ToolResult, ToolExecutionContext; from src.openharness.tools.workspace_path import WorkspacePath, PathEscapeError; print('ok')"` prints `ok`
- [ ] `pytest ai-worker/tests/openharness/tools/test_workspace_path.py -v` — 8 tests pass
- [ ] `pytest ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py -v` — 11 tests pass (all P0)
- [ ] `pytest ai-worker/tests/openharness/tools/test_base_tool_contract.py -v` — 16 tests pass (4 contracts × 4 tools)
- [ ] `pytest ai-worker/tests/test_context_tools.py -v` — existing context_tools tests green
- [ ] `pytest ai-worker/tests/test_tool_registry.py -v` — registry tests green
- [ ] `pytest ai-worker/tests/test_query_engine.py tests/test_agent_loop.py -v` — agent loop tests green
- [ ] `grep -n "await tool.execute" ai-worker/src/openharness/engine/query.py` returns nothing (the old pattern is gone)
- [ ] `grep -rn "if tool_name ==" ai-worker/src/openharness/engine/` returns nothing (no hardcoded special cases)
- [ ] Branch has **6 new commits** from this phase (one per task)

## Phase 2 outputs unlock

- **Phase 3** can write `ReadFileTool`, `WriteFileTool`, `EditFileTool`, `GlobTool`, `GrepTool`, `ListDirectoryTool` as `SimpleTool` subclasses using `WorkspacePath` for all path resolution.
- **Phase 4** can write `BashTool` and `SetPhaseTool` as `BaseTool` subclasses that emit `ThinkingStarted`/`ThinkingStopped`/`PhaseChanged` during execution — the agent loop already forwards those events (thanks to Task 2.6) without any hardcoded tool-name checks.
- **Phase 5** does NOT need to re-touch `query.py`'s tool loop — that adaptation landed here. Phase 5's agent-loop work is limited to the `_create_engine` rewrite, the session collector wiring, and the system prompt.
