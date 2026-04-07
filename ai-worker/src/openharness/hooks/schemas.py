from __future__ import annotations

from typing import Any, Dict, List, Optional, Union

from pydantic import BaseModel


class HookMatcher(BaseModel):
    tool_name: Optional[str] = None
    agent_name: Optional[str] = None

    def matches(self, payload: Dict[str, Any]) -> bool:
        if self.tool_name and payload.get("tool_name") != self.tool_name:
            return False
        if self.agent_name and payload.get("agent_name") != self.agent_name:
            return False
        return True


class CommandHookDefinition(BaseModel):
    type: str = "command"
    command: Union[str, List[str]]
    timeout_seconds: int = 30
    matcher: Optional[Dict[str, str]] = None
    block_on_failure: bool = False
    shell: bool = False
