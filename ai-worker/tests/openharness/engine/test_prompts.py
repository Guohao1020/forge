"""Tests for build_system_prompt.

The system prompt is just a string, so tests are assertions over
substring presence. Goal: if someone accidentally drops the
'set_phase' instruction or the 'no network' constraint, the test
catches it before the agent silently starts misbehaving.
"""

import pytest

from src.openharness.engine.prompts import build_system_prompt


@pytest.mark.asyncio
async def test_prompt_mentions_all_seven_phases():
    prompt = await build_system_prompt(language="go", workspace_path="/ws/project")
    for phase in ("Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"):
        assert phase in prompt, f"phase {phase} missing from system prompt"


@pytest.mark.asyncio
async def test_prompt_mentions_set_phase_tool():
    prompt = await build_system_prompt(language="python", workspace_path="/ws/p")
    assert "set_phase" in prompt


@pytest.mark.asyncio
async def test_prompt_mentions_all_six_file_tools():
    prompt = await build_system_prompt(language=None, workspace_path="/ws/p")
    for tool in ("read_file", "write_file", "edit_file", "glob", "grep", "list_directory"):
        assert tool in prompt, f"tool {tool} missing from system prompt"


@pytest.mark.asyncio
async def test_prompt_mentions_bash_and_sandbox_constraints():
    prompt = await build_system_prompt(language="go", workspace_path="/ws/p")
    assert "bash" in prompt
    assert "no network" in prompt.lower() or "NO network" in prompt
    assert "120" in prompt  # default timeout


@pytest.mark.asyncio
async def test_prompt_mentions_edit_file_preference():
    """The agent should prefer edit_file over write_file for small changes."""
    prompt = await build_system_prompt(language="go", workspace_path="/ws/p")
    # Some variation of "prefer edit_file" or "use edit_file for small"
    assert "edit_file" in prompt
    assert "prefer" in prompt.lower() or "preferred" in prompt.lower()


@pytest.mark.asyncio
async def test_prompt_mentions_line_number_stripping():
    """read_file returns content with line-number prefix; agent must
    strip before passing into edit_file."""
    prompt = await build_system_prompt(language="go", workspace_path="/ws/p")
    assert "line-number" in prompt.lower() or "line number" in prompt.lower()
    assert "strip" in prompt.lower()


@pytest.mark.asyncio
async def test_prompt_with_known_language():
    prompt = await build_system_prompt(language="python", workspace_path="/data/ws/py-project")
    assert "python" in prompt.lower()
    assert "/data/ws/py-project" in prompt


@pytest.mark.asyncio
async def test_prompt_with_unknown_language():
    """Language=None should still produce a valid prompt — it just
    doesn't make a language-specific claim."""
    prompt = await build_system_prompt(language=None, workspace_path="/ws/p")
    assert "unknown" in prompt.lower() or "inspect" in prompt.lower()
    assert "/ws/p" in prompt


@pytest.mark.asyncio
async def test_prompt_tells_agent_to_stop_when_done():
    prompt = await build_system_prompt(language="go", workspace_path="/ws/p")
    assert "stop" in prompt.lower() or "done" in prompt.lower()
    # Explicit anti-overengineering guidance
    assert "over-engineer" in prompt.lower() or "not ask" in prompt.lower()


@pytest.mark.asyncio
async def test_prompt_is_nonempty_and_reasonable_length():
    prompt = await build_system_prompt(language="go", workspace_path="/ws/p")
    # Expect 1000-6000 chars — enough to cover tools + phases +
    # constraints, not so much that token budget explodes
    assert 1000 < len(prompt) < 6000, f"prompt length {len(prompt)} is outside 1000-6000"


# ---------------------------------------------------------------------------
# Round 2: slot substitution + Round 2 instruction bullets
# ---------------------------------------------------------------------------


from src.openharness.engine.agent_hooks import AgentHookContext
from pathlib import Path


@pytest.mark.asyncio
async def test_language_substitution():
    """The {language} f-string substitution from Round 1 still works
    after the async refactor."""
    prompt = await build_system_prompt(
        language="python",
        workspace_path="/data/ws/py",
    )
    assert "python" in prompt.lower()


@pytest.mark.asyncio
async def test_workspace_path_substitution():
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/data/ws/go-project",
    )
    assert "/data/ws/go-project" in prompt


@pytest.mark.asyncio
async def test_unregistered_slot_stripped():
    """When no slot filler is registered for {{project_specs}}, the
    regex cleanup strips it. The agent must never see literal
    {{project_specs}} in the rendered prompt."""
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/ws",
    )
    assert "{{project_specs}}" not in prompt
    assert "{{" not in prompt


@pytest.mark.asyncio
async def test_registered_slot_replaces_placeholder(tmp_path):
    """A registered slot filler's return value replaces its
    {{slot_name}} placeholder via plain str.replace."""
    async def filler(ctx):
        return "## Project specs\n- spec one\n- spec two"

    ctx = AgentHookContext(
        project_id=1,
        session_id="s",
        workspace_dir=tmp_path,
        system_prompt_buffer=[],
    )
    prompt = await build_system_prompt(
        language="go",
        workspace_path=str(tmp_path),
        slots={"project_specs": filler},
        hook_context=ctx,
    )
    assert "## Project specs" in prompt
    assert "- spec one" in prompt
    assert "{{project_specs}}" not in prompt


@pytest.mark.asyncio
async def test_filler_exception_propagates(tmp_path):
    """A slot filler that raises propagates the exception (fail-fast
    per §2.8 — silent suppression would hide context loader bugs)."""
    async def boom(ctx):
        raise RuntimeError("filler boom")

    ctx = AgentHookContext(
        project_id=1,
        session_id="s",
        workspace_dir=tmp_path,
        system_prompt_buffer=[],
    )
    with pytest.raises(RuntimeError, match="filler boom"):
        await build_system_prompt(
            language="go",
            workspace_path=str(tmp_path),
            slots={"project_specs": boom},
            hook_context=ctx,
        )


@pytest.mark.asyncio
async def test_request_review_instruction_present():
    """Round 2 §5.2 bullet 7: the agent must be told when to invoke
    request_review at major milestones."""
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/ws",
    )
    assert "request_review" in prompt
    # The verdict vocabulary must be mentioned so the agent knows
    # how to interpret the response
    assert "APPROVE" in prompt
    assert "REVISE" in prompt
    assert "REJECT" in prompt


@pytest.mark.asyncio
async def test_request_clarification_instruction_present():
    """Round 2 §5.2 bullet 1: the agent must be told to call
    request_clarification when the request is ambiguous."""
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/ws",
    )
    assert "request_clarification" in prompt
    assert "ambiguous" in prompt.lower() or "clarification" in prompt.lower()
