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
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, AsyncIterator, Dict, List, Optional

from ..hooks.builtin.build_verify_hook import BuildVerifyHook, BuildVerifyResult
from .messages import ConversationMessage, TextBlock
from .stream_events import StreamEvent, ErrorEvent

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

    Yields StreamEvents and CycleResults as they occur.
    """
    current_code = dict(code_files or {})
    cycles: List[CycleResult] = []

    for cycle_num in range(1, config.max_cycles + 1):
        logger.info("Pair pipeline cycle %d/%d", cycle_num, config.max_cycles)

        # 1. Coder generates/fixes code
        prompt = initial_prompt if cycle_num == 1 else _build_fix_prompt(cycles[-1])
        coder_response = ""

        async for event in coder_engine.submit_message(prompt):
            yield event
            if hasattr(event, "message") and hasattr(event.message, "text"):
                coder_response = event.message.text

        # Extract code files from coder response (simplified: look for fenced blocks)
        new_files = _extract_code_files(coder_response)
        current_code.update(new_files)

        # 2. Build verification
        build_result = BuildVerifyResult(success=True, output="No build command", command="")
        if config.build_command:
            hook = BuildVerifyHook(
                build_command=config.build_command,
                timeout_seconds=config.build_timeout,
            )
            build_result = await hook.run(code_files=current_code)

        cycle = CycleResult(
            cycle=cycle_num,
            build_success=build_result.success,
            build_output=build_result.output,
            code_files=dict(current_code),
        )

        if not build_result.success:
            cycle.review_decision = None
            cycle.review_feedback = f"Build failed: {build_result.output}"
            cycles.append(cycle)
            yield cycle
            logger.info("Build failed on cycle %d, retrying", cycle_num)
            continue

        # 3. Reviewer reviews (only if build passed)
        review_prompt = _build_review_prompt(coder_response, current_code)
        review_response = ""

        async for event in reviewer_engine.submit_message(review_prompt):
            yield event
            if hasattr(event, "message") and hasattr(event.message, "text"):
                review_response = event.message.text

        decision = _parse_review_decision(review_response)
        cycle.review_decision = decision
        cycle.review_feedback = review_response
        cycles.append(cycle)
        yield cycle

        if decision == ReviewDecision.APPROVE:
            logger.info("Reviewer approved on cycle %d", cycle_num)
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
    yield PairPipelineResult(
        success=False,
        cycles=cycles,
        final_code=current_code,
        total_cycles=config.max_cycles,
        reason="Max cycles exhausted",
    )


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
