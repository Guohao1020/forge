"""Hook executor — runs lifecycle hooks with subprocess_exec (no shell by default)."""

from __future__ import annotations

import asyncio
import json
import logging
import os
from dataclasses import dataclass, field
from typing import Any

from .events import HookEvent
from .loader import HookRegistry
from .schemas import HookMatcher

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class HookResult:
    hook_type: str
    success: bool
    output: str
    blocked: bool = False
    reason: str = ""
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class AggregatedHookResult:
    results: list[HookResult]

    @property
    def blocked(self) -> bool:
        return any(r.blocked for r in self.results)

    @property
    def reason(self) -> str:
        for r in self.results:
            if r.blocked:
                return r.reason
        return ""

    @property
    def all_reasons(self) -> list[str]:
        return [r.reason for r in self.results if r.blocked and r.reason]


class HookExecutor:
    def __init__(self, registry: HookRegistry) -> None:
        self._registry = registry

    async def execute(
        self, event: HookEvent, payload: dict[str, Any],
    ) -> AggregatedHookResult:
        hooks = self._registry.get(event)
        results: list[HookResult] = []
        for hook in hooks:
            if hook.matcher:
                m = HookMatcher(**hook.matcher)
                if not m.matches(payload):
                    continue
            result = await self._run_hook(hook, payload)
            results.append(result)
            if result.blocked:
                break
        return AggregatedHookResult(results=results)

    async def _run_hook(self, hook: Any, payload: dict[str, Any]) -> HookResult:
        if hook.type == "command":
            return await self._run_command_hook(hook, payload)
        return HookResult(hook_type=hook.type, success=False, output="Unknown hook type")

    async def _run_command_hook(self, hook: Any, payload: dict[str, Any]) -> HookResult:
        env = os.environ.copy()
        env["FORGE_HOOK_EVENT"] = str(payload.get("event", ""))
        env["FORGE_HOOK_PAYLOAD"] = json.dumps(payload, default=str)

        try:
            if isinstance(hook.command, list):
                proc = await asyncio.create_subprocess_exec(
                    *hook.command,
                    stdout=asyncio.subprocess.PIPE,
                    stderr=asyncio.subprocess.PIPE,
                    env=env,
                )
            elif hook.shell:
                proc = await asyncio.create_subprocess_shell(
                    hook.command,
                    stdout=asyncio.subprocess.PIPE,
                    stderr=asyncio.subprocess.PIPE,
                    env=env,
                )
            else:
                # Default: split command string and use exec (no shell)
                parts = hook.command.split()
                proc = await asyncio.create_subprocess_exec(
                    *parts,
                    stdout=asyncio.subprocess.PIPE,
                    stderr=asyncio.subprocess.PIPE,
                    env=env,
                )

            stdout, stderr = await asyncio.wait_for(
                proc.communicate(), timeout=hook.timeout_seconds,
            )
            output = stdout.decode() if stdout else ""
            success = proc.returncode == 0
            blocked = not success and hook.block_on_failure
            return HookResult(
                hook_type="command",
                success=success,
                output=output,
                blocked=blocked,
                reason=stderr.decode() if blocked else "",
            )
        except asyncio.TimeoutError:
            return HookResult(
                hook_type="command",
                success=False,
                output="",
                blocked=hook.block_on_failure,
                reason="Hook timed out",
            )
        except FileNotFoundError as e:
            logger.warning("Hook command not found: %s", e)
            return HookResult(
                hook_type="command",
                success=False,
                output="",
                blocked=hook.block_on_failure,
                reason=f"Command not found: {e}",
            )
        except PermissionError as e:
            logger.warning("Hook permission denied: %s", e)
            return HookResult(
                hook_type="command",
                success=False,
                output="",
                blocked=hook.block_on_failure,
                reason=f"Permission denied: {e}",
            )
