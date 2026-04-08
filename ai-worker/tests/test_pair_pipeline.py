from pathlib import Path

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from src.openharness.engine.pair_pipeline import (
    PairPipelineConfig,
    PairPipelineResult,
    CycleResult,
    ReviewDecision,
    run_pair_pipeline,
    _parse_review_decision,
    _extract_code_files,
    _build_fix_prompt,
)
from src.openharness.skills.project_language import (
    CommandSpec,
    LanguageProfile,
)


def test_parse_review_approve():
    assert _parse_review_decision("APPROVE - looks good") == ReviewDecision.APPROVE


def test_parse_review_revise():
    assert _parse_review_decision("REVISE: fix the error handling") == ReviewDecision.REVISE


def test_parse_review_reject():
    assert _parse_review_decision("REJECT - wrong approach") == ReviewDecision.REJECT


def test_parse_review_default_revise():
    assert _parse_review_decision("some feedback without keyword") == ReviewDecision.REVISE


def test_extract_code_files():
    response = '''Here is the code:

```python:src/main.py
print("hello")
```

And the test:

```python:tests/test_main.py
def test_hello():
    assert True
```
'''
    files = _extract_code_files(response)
    assert "src/main.py" in files
    assert "tests/test_main.py" in files
    assert 'print("hello")' in files["src/main.py"]


def test_extract_no_files():
    files = _extract_code_files("Just some text without code blocks")
    assert files == {}


def test_build_fix_prompt_build_failure():
    cycle = CycleResult(
        cycle=1, build_success=False,
        build_output="error: undefined variable x",
    )
    prompt = _build_fix_prompt(cycle)
    assert "build failed" in prompt.lower()
    assert "undefined variable x" in prompt


def test_build_fix_prompt_review_revise():
    cycle = CycleResult(
        cycle=1, build_success=True,
        build_output="",
        review_decision=ReviewDecision.REVISE,
        review_feedback="Add error handling for null input",
    )
    prompt = _build_fix_prompt(cycle)
    assert "reviewer" in prompt.lower()
    assert "null input" in prompt


@pytest.mark.asyncio
async def test_pair_pipeline_approve_first_cycle():
    """Pipeline should succeed when reviewer approves on first cycle."""
    from src.openharness.engine.messages import ConversationMessage, TextBlock
    from src.openharness.engine.stream_events import AssistantTurnComplete
    from src.openharness.api.usage import UsageSnapshot

    # Mock coder engine
    coder = MagicMock()
    coder_msg = ConversationMessage(
        role="assistant",
        content=[TextBlock(text="```python:app.py\nprint('hello')\n```")],
    )

    async def coder_submit(prompt):
        yield AssistantTurnComplete(
            message=coder_msg,
            usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        )
    coder.submit_message = coder_submit

    # Mock reviewer engine
    reviewer = MagicMock()
    reviewer_msg = ConversationMessage(
        role="assistant",
        content=[TextBlock(text="APPROVE - code is correct")],
    )

    async def reviewer_submit(prompt):
        yield AssistantTurnComplete(
            message=reviewer_msg,
            usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        )
    reviewer.submit_message = reviewer_submit

    config = PairPipelineConfig(max_cycles=3)  # No build command = skip build
    results = []
    async for event in run_pair_pipeline(config, coder, reviewer, "Write hello world"):
        results.append(event)

    pipeline_results = [r for r in results if isinstance(r, PairPipelineResult)]
    assert len(pipeline_results) == 1
    assert pipeline_results[0].success
    assert pipeline_results[0].total_cycles == 1


# ---- Stream 4c language detection wiring (TASK 7) -------------------------


def _make_engines():
    """Build a minimal coder/reviewer pair: coder emits a fenced file,
    reviewer immediately APPROVEs. Used by the language-detection tests
    so they can focus on the detection wiring instead of the loop."""
    from src.openharness.engine.messages import ConversationMessage, TextBlock
    from src.openharness.engine.stream_events import AssistantTurnComplete
    from src.openharness.api.usage import UsageSnapshot

    coder = MagicMock()

    async def coder_submit(prompt):
        yield AssistantTurnComplete(
            message=ConversationMessage(
                role="assistant",
                content=[TextBlock(text="```go:main.go\npackage main\n```")],
            ),
            usage=UsageSnapshot(input_tokens=1, output_tokens=1),
        )

    coder.submit_message = coder_submit

    reviewer = MagicMock()

    async def reviewer_submit(prompt):
        yield AssistantTurnComplete(
            message=ConversationMessage(
                role="assistant",
                content=[TextBlock(text="APPROVE")],
            ),
            usage=UsageSnapshot(input_tokens=1, output_tokens=1),
        )

    reviewer.submit_message = reviewer_submit
    return coder, reviewer


@pytest.mark.asyncio
async def test_detect_language_populates_config_when_project_dir_set(tmp_path):
    """When project_dir is provided and build_command is left None,
    run_pair_pipeline must call detect_language and write the result
    back into config.build_command + config.detected_language."""
    # Build a fake go project: empty go.mod is enough for the marker
    # check in LanguageProfile.matches.
    (tmp_path / "go.mod").write_text("module example.com/test\n", encoding="utf-8")

    fake_profile = LanguageProfile(
        name="go-project",
        description="Go test profile",
        detect_files=["go.mod"],
        build_commands=[CommandSpec(marker="go.mod", command="go build ./...")],
        build_timeout=180,
    )

    coder, reviewer = _make_engines()
    config = PairPipelineConfig(
        max_cycles=1,
        project_dir=tmp_path,
        # build_command intentionally left None — detection should fill it
    )

    # Patch the loader so the test does not depend on YAML files on disk.
    with patch(
        "src.openharness.engine.pair_pipeline.load_all_language_profiles",
        return_value={"go-project": fake_profile},
    ):
        # Drive the pipeline to completion (single cycle, reviewer APPROVES).
        # We don't actually want to run `go build` in the unit test, so the
        # mock for BuildVerifyHook below short-circuits the subprocess call.
        with patch(
            "src.openharness.engine.pair_pipeline.BuildVerifyHook"
        ) as mock_hook_cls:
            from src.openharness.hooks.builtin.build_verify_hook import (
                BuildVerifyResult,
            )
            mock_hook = MagicMock()

            async def fake_run(code_files, project_dir=None):
                return BuildVerifyResult(
                    success=True,
                    output="ok",
                    command="go build ./...",
                )

            mock_hook.run = fake_run
            mock_hook_cls.return_value = mock_hook

            results = []
            async for event in run_pair_pipeline(config, coder, reviewer, "build it"):
                results.append(event)

    # Detection wrote the resolved command + language back into config.
    assert config.build_command == "go build ./..."
    assert config.detected_language == "go-project"
    assert config.build_timeout == 180

    # And the pipeline actually ran a cycle that hit the build hook.
    pipeline_results = [r for r in results if isinstance(r, PairPipelineResult)]
    assert len(pipeline_results) == 1
    assert pipeline_results[0].success


@pytest.mark.asyncio
async def test_explicit_build_command_overrides_detection(tmp_path):
    """If the caller pins build_command, detection must NOT run — the
    explicit value wins so e2e tests can lock down a known toolchain."""
    (tmp_path / "go.mod").write_text("module example.com/test\n", encoding="utf-8")

    coder, reviewer = _make_engines()
    config = PairPipelineConfig(
        max_cycles=1,
        project_dir=tmp_path,
        build_command="echo overridden",  # explicit
    )

    with patch(
        "src.openharness.engine.pair_pipeline.load_all_language_profiles"
    ) as mock_loader:
        with patch(
            "src.openharness.engine.pair_pipeline.BuildVerifyHook"
        ) as mock_hook_cls:
            from src.openharness.hooks.builtin.build_verify_hook import (
                BuildVerifyResult,
            )
            mock_hook = MagicMock()

            async def fake_run(code_files, project_dir=None):
                return BuildVerifyResult(success=True, output="", command="")

            mock_hook.run = fake_run
            mock_hook_cls.return_value = mock_hook

            async for _ in run_pair_pipeline(config, coder, reviewer, "build it"):
                pass

        mock_loader.assert_not_called()  # detection skipped entirely

    assert config.build_command == "echo overridden"
    assert config.detected_language is None  # never set


@pytest.mark.asyncio
async def test_detect_language_no_match_skips_build(tmp_path):
    """When project_dir contains no recognizable markers, detection
    returns None and the pipeline runs without a build command (skips
    BuildVerify). The pipeline must still complete successfully on
    reviewer APPROVE — language detection is best-effort."""
    # Empty tmp_path: no go.mod, no pom.xml, nothing.
    coder, reviewer = _make_engines()
    config = PairPipelineConfig(
        max_cycles=1,
        project_dir=tmp_path,
    )

    with patch(
        "src.openharness.engine.pair_pipeline.load_all_language_profiles",
        return_value={},
    ):
        results = []
        async for event in run_pair_pipeline(config, coder, reviewer, "build it"):
            results.append(event)

    assert config.build_command is None
    assert config.detected_language is None
    pipeline_results = [r for r in results if isinstance(r, PairPipelineResult)]
    assert len(pipeline_results) == 1
    assert pipeline_results[0].success  # reviewer APPROVE still wins


@pytest.mark.asyncio
async def test_detect_language_loader_failure_does_not_crash(tmp_path):
    """Detection must be best-effort: a malformed YAML or filesystem
    error must not crash the pipeline. We swallow exceptions and fall
    through to build_command=None (BuildVerify skipped)."""
    coder, reviewer = _make_engines()
    config = PairPipelineConfig(
        max_cycles=1,
        project_dir=tmp_path,
    )

    with patch(
        "src.openharness.engine.pair_pipeline.load_all_language_profiles",
        side_effect=RuntimeError("YAML on fire"),
    ):
        results = []
        async for event in run_pair_pipeline(config, coder, reviewer, "build it"):
            results.append(event)

    assert config.build_command is None
    pipeline_results = [r for r in results if isinstance(r, PairPipelineResult)]
    assert len(pipeline_results) == 1
    assert pipeline_results[0].success


@pytest.mark.asyncio
async def test_pair_pipeline_max_cycles_exhausted():
    """Pipeline should report failure when max cycles exhausted."""
    from src.openharness.engine.messages import ConversationMessage, TextBlock
    from src.openharness.engine.stream_events import AssistantTurnComplete
    from src.openharness.api.usage import UsageSnapshot

    coder = MagicMock()
    coder_msg = ConversationMessage(
        role="assistant",
        content=[TextBlock(text="some code")],
    )

    async def coder_submit(prompt):
        yield AssistantTurnComplete(
            message=coder_msg,
            usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        )
    coder.submit_message = coder_submit

    reviewer = MagicMock()
    reviewer_msg = ConversationMessage(
        role="assistant",
        content=[TextBlock(text="REVISE: needs more error handling")],
    )

    async def reviewer_submit(prompt):
        yield AssistantTurnComplete(
            message=reviewer_msg,
            usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        )
    reviewer.submit_message = reviewer_submit

    config = PairPipelineConfig(max_cycles=2)
    results = []
    async for event in run_pair_pipeline(config, coder, reviewer, "Write code"):
        results.append(event)

    pipeline_results = [r for r in results if isinstance(r, PairPipelineResult)]
    assert len(pipeline_results) == 1
    assert not pipeline_results[0].success
    assert "exhausted" in pipeline_results[0].reason.lower()
