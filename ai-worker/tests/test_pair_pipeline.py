import pytest
from unittest.mock import AsyncMock, MagicMock
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
