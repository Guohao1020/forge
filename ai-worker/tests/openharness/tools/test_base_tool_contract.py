"""Parametrized contract tests for every registered BaseTool subclass.

This test suite runs against every importable tool class in the code-
base. When a new tool is added in Phase 3/4, it's auto-included by
appending it to ALL_TOOL_SPECS below. The parametrization ensures
every tool satisfies:

  1. execute() is an async generator
  2. It yields zero or more StreamEvents then exactly one ToolResult
  3. The ToolResult is the LAST item yielded
  4. name / description / input_model class attrs are set

Contract tests do NOT verify business logic -- each tool's dedicated
test file does that. Contract tests verify the wire protocol.
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
# Tool spec table -- auto-extended as new tools land in Phase 3/4.
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

    # Exec tools (Phase 4)
    from src.openharness.tools.bash_tool import BashInput, BashTool
    from src.openharness.tools.phase_tool import SetPhaseInput, SetPhaseTool

    specs.extend([
        (
            BashTool,
            lambda ws: BashTool(ws),
            # 'echo ok' is not on the denylist and succeeds both in
            # bwrap and in the fallback-error path.
            lambda: BashInput(command="echo ok", timeout=30),
        ),
        (
            SetPhaseTool,
            lambda _ws: SetPhaseTool(),
            lambda: SetPhaseInput(phase="Analyze"),
        ),
    ])

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
    """Every tool must yield exactly one ToolResult."""
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
    """The ToolResult must be yielded LAST."""
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
    """tool.execute(...) must return an AsyncIterator (generator object)."""
    tool = factory(workspace)
    arguments = arg_factory()

    gen = tool.execute(arguments, tool_context)
    assert hasattr(gen, "__anext__"), (
        f"{tool_class.__name__}.execute did not return an async iterator; "
        f"got {type(gen).__name__}. Did you use 'async def ... return' "
        f"instead of 'async def ... yield'?"
    )
    # Consume the generator so it doesn't leak
    async for _ in gen:
        pass
