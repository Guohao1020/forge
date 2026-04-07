"""CI Auto-fix Hook — POST_PUSH hook that monitors CI and auto-repairs failures.

Circuit breaker: max 3 retry attempts with exponential backoff.
If all retries fail, gives up and reports the failure.
"""

from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass
from enum import Enum
from typing import Any, Optional

from ..events import HookEvent
from ..executor import HookResult

logger = logging.getLogger(__name__)


class CIStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    SUCCESS = "success"
    FAILURE = "failure"
    ERROR = "error"


@dataclass
class CIAutoFixConfig:
    max_retries: int = 3
    poll_interval_seconds: int = 30
    poll_timeout_seconds: int = 600
    backoff_multiplier: float = 2.0


@dataclass
class CIAutoFixResult:
    success: bool
    attempts: int
    ci_status: CIStatus
    last_error: str = ""
    fixed_on_attempt: int = 0


class CIAutoFixHook:
    """Monitors CI status after push, generates fixes for failures."""

    event = HookEvent.POST_PUSH

    def __init__(self, config: Optional[CIAutoFixConfig] = None) -> None:
        self.config = config or CIAutoFixConfig()
        self._attempts = 0

    async def run(
        self,
        ci_provider: Any,  # Protocol: poll_status(), get_logs(), push_fix()
        fix_engine: Any,  # QueryEngine for generating fixes
    ) -> CIAutoFixResult:
        """Monitor CI and auto-fix on failure. Circuit breaker after max_retries."""
        backoff = 1.0

        for attempt in range(1, self.config.max_retries + 1):
            self._attempts = attempt
            logger.info("CI auto-fix attempt %d/%d", attempt, self.config.max_retries)

            # 1. Poll CI status
            try:
                status = await self._poll_ci(ci_provider)
            except Exception as e:
                logger.error("CI polling failed: %s", e)
                return CIAutoFixResult(
                    success=False,
                    attempts=attempt,
                    ci_status=CIStatus.ERROR,
                    last_error=f"CI polling error: {e}",
                )

            if status == CIStatus.SUCCESS:
                return CIAutoFixResult(
                    success=True,
                    attempts=attempt,
                    ci_status=CIStatus.SUCCESS,
                    fixed_on_attempt=attempt if attempt > 1 else 0,
                )

            if status != CIStatus.FAILURE:
                return CIAutoFixResult(
                    success=False,
                    attempts=attempt,
                    ci_status=status,
                    last_error=f"Unexpected CI status: {status}",
                )

            # 2. Get CI logs
            try:
                logs = await ci_provider.get_logs()
            except Exception as e:
                logger.error("Failed to get CI logs: %s", e)
                logs = f"Failed to retrieve CI logs: {e}"

            # 3. Generate fix
            fix_prompt = (
                f"CI failed. Here are the logs:\n\n```\n{logs}\n```\n\n"
                f"Fix the code to make CI pass."
            )
            try:
                fix_response = ""
                async for event in fix_engine.submit_message(fix_prompt):
                    if hasattr(event, "message") and hasattr(event.message, "text"):
                        fix_response = event.message.text
            except Exception as e:
                logger.error("Fix generation failed: %s", e)
                return CIAutoFixResult(
                    success=False,
                    attempts=attempt,
                    ci_status=CIStatus.FAILURE,
                    last_error=f"Fix generation error: {e}",
                )

            # 4. Push fix
            try:
                await ci_provider.push_fix(fix_response)
            except Exception as e:
                logger.error("Push fix failed: %s", e)
                return CIAutoFixResult(
                    success=False,
                    attempts=attempt,
                    ci_status=CIStatus.FAILURE,
                    last_error=f"Push fix error: {e}",
                )

            # Exponential backoff before next poll
            logger.info("Fix pushed, waiting %.1fs before polling CI", backoff)
            await asyncio.sleep(backoff)
            backoff *= self.config.backoff_multiplier

        # Circuit breaker: max retries exhausted
        logger.warning("CI auto-fix exhausted %d retries", self.config.max_retries)
        return CIAutoFixResult(
            success=False,
            attempts=self.config.max_retries,
            ci_status=CIStatus.FAILURE,
            last_error=f"Max retries ({self.config.max_retries}) exhausted",
        )

    async def _poll_ci(self, ci_provider: Any) -> CIStatus:
        """Poll CI status until it finishes or times out."""
        elapsed = 0
        while elapsed < self.config.poll_timeout_seconds:
            status = await ci_provider.poll_status()
            if status in (CIStatus.SUCCESS, CIStatus.FAILURE, CIStatus.ERROR):
                return status
            await asyncio.sleep(self.config.poll_interval_seconds)
            elapsed += self.config.poll_interval_seconds
        return CIStatus.ERROR
