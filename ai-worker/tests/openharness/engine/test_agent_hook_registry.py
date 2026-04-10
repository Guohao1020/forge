"""Tests for AgentHookRegistry — the in-process Python hook system.

These are pure construction-and-shape tests. End-to-end behavior
(hooks actually firing inside run_agent_loop) lives in
test_hooks_integration.py (Task 5.10).

Spec: §2.9.1.a-e.
"""

from __future__ import annotations

import dataclasses
from pathlib import Path

import pytest

from src.openharness.engine.agent_hooks import (
    AgentHookContext,
    AgentHookRegistry,
    PostTurnHook,
    PreToolCallBlock,
    PreToolCallHook,
    PreTurnHook,
    PromptSlotFiller,
)


# ---------------------------------------------------------------------------
# AgentHookRegistry construction
# ---------------------------------------------------------------------------


def test_empty_registry_construction():
    """Default construction yields four empty containers — no hooks
    registered. Round 2 ships this empty default; downstream projects
    populate it via a project-scoped factory."""
    registry = AgentHookRegistry()
    assert registry.pre_turn == []
    assert registry.pre_tool_call == []
    assert registry.post_turn == []
    assert registry.system_prompt_slots == {}
    # Each registry instance has independent containers (no shared
    # default mutable state — that bug class is one of Python's most
    # famous). Construct a second instance and confirm.
    other = AgentHookRegistry()
    other.pre_turn.append(lambda: None)
    assert registry.pre_turn == []  # not aliased


def test_register_pre_turn_hook():
    """pre_turn hooks accumulate in registration order."""
    registry = AgentHookRegistry()

    async def hook_a(ctx, messages):
        return messages

    async def hook_b(ctx, messages):
        return messages

    registry.pre_turn.append(hook_a)
    registry.pre_turn.append(hook_b)

    assert len(registry.pre_turn) == 2
    assert registry.pre_turn[0] is hook_a
    assert registry.pre_turn[1] is hook_b


def test_register_pre_tool_call_hook():
    registry = AgentHookRegistry()

    async def block_bash(ctx, tool_name, arguments):
        if tool_name == "bash":
            return PreToolCallBlock(reason="bash blocked by test")
        return arguments

    registry.pre_tool_call.append(block_bash)
    assert len(registry.pre_tool_call) == 1
    assert registry.pre_tool_call[0] is block_bash


def test_register_post_turn_hook():
    registry = AgentHookRegistry()
    seen = []

    async def record(ctx, final_message):
        seen.append(final_message)

    registry.post_turn.append(record)
    assert len(registry.post_turn) == 1
    assert registry.post_turn[0] is record


def test_register_slot_filler():
    """system_prompt_slots is a dict — slot name -> async filler."""
    registry = AgentHookRegistry()

    async def project_specs_filler(ctx):
        return "spec content goes here"

    registry.system_prompt_slots["project_specs"] = project_specs_filler
    assert "project_specs" in registry.system_prompt_slots
    assert registry.system_prompt_slots["project_specs"] is project_specs_filler


# ---------------------------------------------------------------------------
# PreToolCallBlock dataclass
# ---------------------------------------------------------------------------


def test_pre_tool_call_block_dataclass_frozen():
    """PreToolCallBlock is frozen — reason cannot be mutated after
    construction. This is enforced by @dataclass(frozen=True)."""
    block = PreToolCallBlock(reason="bash blocked")
    assert block.reason == "bash blocked"
    with pytest.raises((AttributeError, dataclasses.FrozenInstanceError)):
        block.reason = "different reason"  # type: ignore


# ---------------------------------------------------------------------------
# AgentHookContext shape
# ---------------------------------------------------------------------------


def test_agent_hook_context_mutable_buffer():
    """AgentHookContext.system_prompt_buffer is a mutable list that
    pre_turn hooks can append to. Construction takes project_id,
    session_id, workspace_dir, and the initial (usually empty) buffer.
    """
    ctx = AgentHookContext(
        project_id=42,
        session_id="sess-abc",
        workspace_dir=Path("/data/forge/workspaces/tenant-1/project-42"),
        system_prompt_buffer=[],
    )
    assert ctx.project_id == 42
    assert ctx.session_id == "sess-abc"
    assert ctx.workspace_dir == Path(
        "/data/forge/workspaces/tenant-1/project-42"
    )
    assert ctx.system_prompt_buffer == []

    # The buffer is mutable — pre_turn hooks append to it
    ctx.system_prompt_buffer.append("extra context line")
    assert ctx.system_prompt_buffer == ["extra context line"]


def test_agent_hook_context_buffers_are_per_instance():
    """Two separate context instances must not share the same buffer
    object — each session gets a fresh list."""
    ctx_a = AgentHookContext(
        project_id=1,
        session_id="a",
        workspace_dir=Path("/ws/a"),
        system_prompt_buffer=[],
    )
    ctx_b = AgentHookContext(
        project_id=1,
        session_id="b",
        workspace_dir=Path("/ws/b"),
        system_prompt_buffer=[],
    )
    ctx_a.system_prompt_buffer.append("a-only")
    assert ctx_b.system_prompt_buffer == []
