from __future__ import annotations

from dataclasses import dataclass
from typing import List, Optional, Set

from .modes import PermissionMode


@dataclass(frozen=True)
class PermissionDecision:
    allowed: bool
    requires_confirmation: bool = False
    reason: str = ""


class PermissionChecker:
    def __init__(
        self,
        mode: PermissionMode = PermissionMode.DEFAULT,
        allowed_tools: Optional[List[str]] = None,
        denied_tools: Optional[List[str]] = None,
    ) -> None:
        self._mode = mode
        self._allowed: Set[str] = set(allowed_tools or [])
        self._denied: Set[str] = set(denied_tools or [])

    def evaluate(
        self, tool_name: str, *, is_read_only: bool = False, **kw,
    ) -> PermissionDecision:
        # Denied always wins
        if tool_name in self._denied:
            return PermissionDecision(
                allowed=False, reason=f"Tool '{tool_name}' denied",
            )
        # Explicit allow
        if tool_name in self._allowed:
            return PermissionDecision(allowed=True)
        # Read-only is always safe
        if is_read_only:
            return PermissionDecision(allowed=True)
        # Full auto mode allows everything
        if self._mode == PermissionMode.FULL_AUTO:
            return PermissionDecision(allowed=True)
        # Default mode: mutating tools need confirmation
        return PermissionDecision(
            allowed=False,
            requires_confirmation=True,
            reason="Mutating tool requires confirmation",
        )
