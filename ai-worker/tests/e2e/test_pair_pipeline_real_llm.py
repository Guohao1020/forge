"""End-to-end smoke test for the agent base loop with a REAL LLM.

This is the critical regression test mandated by TASK 11 of
docs/plans/2026-04-07-agent-base-loop-reduction.md. It exercises
every component of the base loop end-to-end:

  1. Real DASHSCOPE LLM call via ModelRouter (qwen3-coder-plus)
  2. Coder QueryEngine streams an AssistantTurnComplete with fenced
     code blocks
  3. _extract_code_files parses the fenced blocks into a {path:
     content} dict
  4. BuildVerifyHook preflight allows the build (go.mod is present
     because we seed it before invoking the pipeline)
  5. Real `go build ./...` runs against a tmp_path Go module
  6. Reviewer QueryEngine evaluates the code and emits APPROVE
  7. PairPipelineResult.success is True

Why a tmp_path Go module instead of self-hosting against forge-core:
- Zero working tree pollution; pytest cleans tmp_path automatically.
- Smallest possible code surface (one Go file + go.mod) so the LLM
  fenced-block parser is not the bottleneck of the test.
- The build hook still runs a real `go build`, so this validates
  the entire toolchain integration path that an e2e is supposed to
  cover. Self-hosting would add nothing testable.

Skip conditions:
- DASHSCOPE_API_KEY missing (the test cannot make real LLM calls).
- `go` binary not on PATH (the test cannot run a real build).

Run manually:
    cd ai-worker
    pytest -m e2e tests/e2e/test_pair_pipeline_real_llm.py -v -s

CI is configured (via pyproject.toml addopts) to skip e2e tests by
default — opt in with `-m e2e`.
"""

from __future__ import annotations

import os
import shutil
from pathlib import Path

import pytest

from src.openharness.engine.pair_pipeline import (
    PairPipelineConfig,
    PairPipelineResult,
    run_pair_pipeline,
)

pytestmark = pytest.mark.e2e


def _skip_if_unconfigured() -> None:
    """Skip the test if the live LLM credentials or local toolchain
    are not available. Returns nothing — calls pytest.skip() directly."""
    if not os.environ.get("DASHSCOPE_API_KEY"):
        pytest.skip("DASHSCOPE_API_KEY not set — e2e test requires a real LLM key")
    if shutil.which("go") is None:
        pytest.skip("go binary not on PATH — e2e test requires a Go toolchain")


def _build_query_engine(model: str, system_prompt: str):
    """Construct a QueryEngine wired to the real ModelRouter via the
    DASHSCOPE provider. Mirrors api_server._create_engine without the
    fallback-to-mock branch — this test must hit a real LLM or skip."""
    from src.models.router import ModelRouter, Purpose
    from src.openharness.api.providers.router_adapter import ModelRouterAdapter
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.hooks.executor import HookExecutor
    from src.openharness.permissions.checker import PermissionChecker
    from src.openharness.permissions.modes import PermissionMode

    router = ModelRouter()
    api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)

    return QueryEngine(
        api_client=api_client,
        tool_registry=ToolRegistry(),
        model=model,
        system_prompt=system_prompt,
        hook_executor=HookExecutor(HookRegistry()),
        permission_checker=PermissionChecker(mode=PermissionMode.FULL_AUTO),
    )


@pytest.mark.asyncio
async def test_real_llm_coder_build_reviewer_loop(tmp_path: Path) -> None:
    """End-to-end: real LLM generates Go code, real `go build` verifies
    it, real LLM reviewer approves. Asserts the entire pair pipeline
    completes successfully within 3 cycles.

    This is the e2e gate from TASK 11. If it fails, the base loop is
    broken and the PR cannot ship. Common failure modes the test
    surfaces:
    - LLM returned text without fenced code blocks
    - Fenced block parser tripped on the LLM's exact format
    - go build failed because go.mod is missing or detection picked
      the wrong toolchain
    - Reviewer never emitted APPROVE within max_cycles
    """
    _skip_if_unconfigured()

    # Seed a minimal Go module in tmp_path so the BuildVerifyHook
    # preflight (TASK 8) is satisfied and `go build` has somewhere to
    # resolve packages from.
    (tmp_path / "go.mod").write_text(
        "module example.com/forge-e2e\n\ngo 1.22\n",
        encoding="utf-8",
    )

    coder_system_prompt = (
        "You are a Go developer. When asked to write code, respond ONLY "
        "with fenced code blocks in the format:\n\n"
        "```go:path/to/file.go\n<file contents>\n```\n\n"
        "Each block must include the file path after the language. "
        "Write minimal, idiomatic Go. Do not include explanations outside "
        "the code blocks."
    )

    reviewer_system_prompt = (
        "You are a strict Go code reviewer. Respond with exactly one of:\n"
        "- APPROVE if the code is correct, idiomatic, and ready to merge\n"
        "- REVISE: <specific changes needed>\n"
        "- REJECT if the approach is fundamentally wrong\n"
        "Be lenient on style; demand correctness."
    )

    coder = _build_query_engine(
        model="qwen3-coder-plus",
        system_prompt=coder_system_prompt,
    )
    reviewer = _build_query_engine(
        model="qwen-max",
        system_prompt=reviewer_system_prompt,
    )

    config = PairPipelineConfig(
        max_cycles=3,
        project_dir=tmp_path,
        # build_command intentionally None — Stream 4c language
        # detection (TASK 7) should resolve `go build ./...` from
        # the seeded go.mod.
    )

    prompt = (
        "Add an IsEven(n int) bool function to a file at "
        "internal/util/even.go. The function returns true when n is "
        "divisible by 2. Also create internal/util/even_test.go with "
        "a Go test that covers IsEven(0)=true, IsEven(1)=false, "
        "IsEven(-2)=true. Both files must be in package util."
    )

    results = []
    async for event in run_pair_pipeline(config, coder, reviewer, prompt):
        results.append(event)

    # Detection must have populated build_command from the seeded go.mod
    assert config.build_command, (
        "Stream 4c detection failed: build_command still None. "
        f"detected_language={config.detected_language!r}"
    )
    assert "go" in config.build_command, (
        f"detected build_command={config.build_command!r} is not a go command"
    )
    assert config.detected_language is not None

    # Final pipeline result must exist and be successful
    pipeline_results = [r for r in results if isinstance(r, PairPipelineResult)]
    assert len(pipeline_results) == 1, (
        f"expected exactly 1 PairPipelineResult, got {len(pipeline_results)}"
    )
    final = pipeline_results[0]
    assert final.success, (
        f"pipeline failed after {final.total_cycles} cycles: {final.reason}\n"
        f"cycles: {[(c.cycle, c.build_success, c.review_decision) for c in final.cycles]}"
    )
    assert final.total_cycles <= 3, f"used too many cycles: {final.total_cycles}"

    # The generated code must include the requested file
    assert any(
        "even.go" in path for path in final.final_code
    ), f"LLM did not produce even.go; got files: {list(final.final_code.keys())}"

    # And `go build` must have actually passed at least once
    last_cycle = final.cycles[-1]
    assert last_cycle.build_success, (
        f"final cycle build did not pass: {last_cycle.build_output[:500]}"
    )
