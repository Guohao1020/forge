"""Pair Pipeline — Coder → BuildVerify → Review → iterate.

Implements the Worker/Reviewer pattern from OpenSwarm:
  1. Coder generates code
  2. BuildVerifyHook runs compilation
  3. If build fails → Coder fixes → loop (up to max_cycles)
  4. If build passes → Reviewer reviews
  5. If review says REVISE → Coder fixes → loop
  6. If review says APPROVE → done
"""

from __future__ import annotations

import logging
import re
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, AsyncIterator, Dict, List, Optional

from ..hooks.builtin.build_verify_hook import BuildVerifyHook, BuildVerifyResult
from .messages import ConversationMessage, TextBlock
from .stream_events import (
    ErrorEvent,
    FixLoopCompleted,
    FixLoopStarted,
    SessionComplete,
    ThinkingStarted,
    ThinkingStopped,
)

logger = logging.getLogger(__name__)


class ReviewDecision(str, Enum):
    APPROVE = "approve"
    REVISE = "revise"
    REJECT = "reject"


@dataclass
class PairPipelineConfig:
    max_cycles: int = 3
    build_command: Optional[str] = None
    build_timeout: int = 120
    coder_model: Optional[str] = None
    reviewer_model: Optional[str] = None


@dataclass
class CycleResult:
    cycle: int
    build_success: bool
    build_output: str
    review_decision: Optional[ReviewDecision] = None
    review_feedback: Optional[str] = None
    code_files: Dict[str, str] = field(default_factory=dict)


@dataclass
class PairPipelineResult:
    success: bool
    cycles: List[CycleResult]
    final_code: Dict[str, str]
    total_cycles: int
    reason: str = ""


async def run_pair_pipeline(
    config: PairPipelineConfig,
    coder_engine: Any,  # QueryEngine
    reviewer_engine: Any,  # QueryEngine
    initial_prompt: str,
    code_files: Optional[Dict[str, str]] = None,
) -> AsyncIterator[Any]:
    """Run the Coder/Reviewer pair pipeline.

    Yields StreamEvents (including the new FixLoopStarted/Completed and
    SessionComplete events for the frontend fix-loop visualization and
    SummaryCard) and CycleResults as they occur.
    """
    start_ts = time.monotonic()
    initial_code = dict(code_files or {})
    current_code = dict(initial_code)
    cycles: List[CycleResult] = []
    tokens_total = 0
    cost_total = 0.0
    final_build_status = "skipped"

    def _session_complete(reason: str) -> SessionComplete:
        created = sum(1 for p in current_code if p not in initial_code)
        modified = sum(
            1 for p, c in current_code.items()
            if p in initial_code and initial_code[p] != c
        )
        return SessionComplete(
            files_created=created,
            files_modified=modified,
            build_status=final_build_status,
            duration_ms=int((time.monotonic() - start_ts) * 1000),
            tokens_total=tokens_total,
            cost_usd=cost_total,
        )

    for cycle_num in range(1, config.max_cycles + 1):
        logger.info("Pair pipeline cycle %d/%d", cycle_num, config.max_cycles)

        # On retry cycles, announce the fix loop to the frontend.
        is_retry = cycle_num > 1 and cycles and not cycles[-1].build_success
        if is_retry:
            yield FixLoopStarted(
                cycle=cycle_num,
                max_cycles=config.max_cycles,
                errors=_count_compile_errors(cycles[-1].build_output),
            )

        # 1. Coder generates/fixes code
        prompt = initial_prompt if cycle_num == 1 else _build_fix_prompt(cycles[-1])
        yield ThinkingStarted(label="Generating code" if cycle_num == 1 else "Fixing code")
        coder_response = ""

        async for event in coder_engine.submit_message(prompt):
            yield event
            if hasattr(event, "message") and hasattr(event.message, "text"):
                coder_response = event.message.text
            # Best-effort token/cost accumulation for the SessionComplete event.
            usage = getattr(event, "usage", None)
            if usage is not None:
                tokens_total += (
                    getattr(usage, "input_tokens", 0)
                    + getattr(usage, "output_tokens", 0)
                )
                cost_total += float(getattr(usage, "total_cost_usd", 0.0) or 0.0)

        yield ThinkingStopped()

        # Extract code files from coder response (simplified: look for fenced blocks)
        new_files = _extract_code_files(coder_response)
        current_code.update(new_files)

        # 2. Build verification
        build_result = BuildVerifyResult(success=True, output="No build command", command="")
        if config.build_command:
            yield ThinkingStarted(label=f"Running {config.build_command}")
            hook = BuildVerifyHook(
                build_command=config.build_command,
                timeout_seconds=config.build_timeout,
            )
            build_result = await hook.run(code_files=current_code)
            yield ThinkingStopped()
        final_build_status = "passed" if build_result.success else "failed"

        cycle = CycleResult(
            cycle=cycle_num,
            build_success=build_result.success,
            build_output=build_result.output,
            code_files=dict(current_code),
        )

        if is_retry:
            # Wrap up the fix loop iteration — success means the build is
            # now green, failure means we'll continue to the next cycle or
            # exhaust the budget.
            yield FixLoopCompleted(cycle=cycle_num, success=build_result.success)

        if not build_result.success:
            cycle.review_decision = None
            cycle.review_feedback = f"Build failed: {build_result.output}"
            cycles.append(cycle)
            yield cycle
            logger.info("Build failed on cycle %d, retrying", cycle_num)
            continue

        # 3. Reviewer reviews (only if build passed)
        yield ThinkingStarted(label="Reviewing code")
        review_prompt = _build_review_prompt(coder_response, current_code)
        review_response = ""

        async for event in reviewer_engine.submit_message(review_prompt):
            yield event
            if hasattr(event, "message") and hasattr(event.message, "text"):
                review_response = event.message.text
            usage = getattr(event, "usage", None)
            if usage is not None:
                tokens_total += (
                    getattr(usage, "input_tokens", 0)
                    + getattr(usage, "output_tokens", 0)
                )
                cost_total += float(getattr(usage, "total_cost_usd", 0.0) or 0.0)

        yield ThinkingStopped()

        decision = _parse_review_decision(review_response)
        cycle.review_decision = decision
        cycle.review_feedback = review_response
        cycles.append(cycle)
        yield cycle

        if decision == ReviewDecision.APPROVE:
            logger.info("Reviewer approved on cycle %d", cycle_num)
            yield _session_complete("Reviewer approved")
            yield PairPipelineResult(
                success=True,
                cycles=cycles,
                final_code=current_code,
                total_cycles=cycle_num,
                reason="Reviewer approved",
            )
            return

        if decision == ReviewDecision.REJECT:
            logger.info("Reviewer rejected on cycle %d", cycle_num)
            yield _session_complete("Reviewer rejected")
            yield PairPipelineResult(
                success=False,
                cycles=cycles,
                final_code=current_code,
                total_cycles=cycle_num,
                reason="Reviewer rejected",
            )
            return

        # REVISE — continue loop
        logger.info("Reviewer requested revisions on cycle %d", cycle_num)

    # Max cycles exhausted
    yield ErrorEvent(
        message=f"Pair pipeline exhausted {config.max_cycles} cycles without approval",
        recoverable=False,
    )
    yield _session_complete("Max cycles exhausted")
    yield PairPipelineResult(
        success=False,
        cycles=cycles,
        final_code=current_code,
        total_cycles=config.max_cycles,
        reason="Max cycles exhausted",
    )


# Heuristic: match common compiler error patterns (Java/Python/Go/TS). The
# count is best-effort — it's for the FixLoopStarted `errors` field which
# is purely informational in the frontend banner.
_ERROR_PATTERNS = [
    re.compile(r"\[ERROR\]", re.MULTILINE),  # Maven
    re.compile(r"^\s*error:", re.MULTILINE | re.IGNORECASE),  # Go/TS/Rust
    re.compile(r"^.*\berror\[E\d+\]:", re.MULTILINE),  # Rust
    re.compile(r"^.*\.py:\d+:.*(?:Error|Exception)", re.MULTILINE),  # Python
    re.compile(r"^.*\.java:\d+: error:", re.MULTILINE),  # javac
    re.compile(r"FAILURE: Build failed", re.MULTILINE),  # Gradle
]


def _count_compile_errors(build_output: str) -> int:
    if not build_output:
        return 0
    total = 0
    for pat in _ERROR_PATTERNS:
        total += len(pat.findall(build_output))
    return total


def _build_fix_prompt(last_cycle: CycleResult) -> str:
    if not last_cycle.build_success:
        return (
            f"The build failed with this error:\n\n"
            f"```\n{last_cycle.build_output}\n```\n\n"
            f"Fix the code to resolve this compilation error."
        )
    return (
        f"The reviewer requested changes:\n\n"
        f"{last_cycle.review_feedback}\n\n"
        f"Apply the requested changes."
    )


def _build_review_prompt(coder_response: str, code_files: Dict[str, str]) -> str:
    files_section = "\n".join(
        f"### {path}\n```\n{content}\n```"
        for path, content in code_files.items()
    )
    return (
        f"Review the following code. Respond with exactly one of:\n"
        f"- APPROVE: if the code is correct and production-ready\n"
        f"- REVISE: followed by specific changes needed\n"
        f"- REJECT: if the approach is fundamentally wrong\n\n"
        f"## Generated Code\n\n{files_section}"
    )


def _parse_review_decision(response: str) -> ReviewDecision:
    upper = response.upper().strip()
    if upper.startswith("APPROVE"):
        return ReviewDecision.APPROVE
    if upper.startswith("REJECT"):
        return ReviewDecision.REJECT
    return ReviewDecision.REVISE


def _extract_code_files(response: str) -> Dict[str, str]:
    """Extract fenced code blocks with filenames from the response."""
    import re
    files: Dict[str, str] = {}
    # Match ```language:path or ```path
    pattern = r"```(?:\w+:)?([^\n`]+)\n(.*?)```"
    for match in re.finditer(pattern, response, re.DOTALL):
        path = match.group(1).strip()
        content = match.group(2)
        if "/" in path or "." in path:
            files[path] = content
    return files
