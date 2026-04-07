from __future__ import annotations

from .events import HookEvent
from .schemas import CommandHookDefinition

HookDefinition = CommandHookDefinition


class HookRegistry:
    def __init__(self) -> None:
        self._hooks: dict[HookEvent, list[HookDefinition]] = {}

    def register(self, event: HookEvent, hook: HookDefinition) -> None:
        self._hooks.setdefault(event, []).append(hook)

    def get(self, event: HookEvent) -> list[HookDefinition]:
        return self._hooks.get(event, [])
