from __future__ import annotations

import uuid
from typing import Literal, Union

from pydantic import BaseModel, Field


class TextBlock(BaseModel):
    type: Literal["text"] = "text"
    text: str


class ToolUseBlock(BaseModel):
    type: Literal["tool_use"] = "tool_use"
    id: str = Field(default_factory=lambda: f"toolu_{uuid.uuid4().hex[:24]}")
    name: str
    input: dict


class ToolResultBlock(BaseModel):
    type: Literal["tool_result"] = "tool_result"
    tool_use_id: str
    content: str
    is_error: bool = False


ContentBlock = Union[TextBlock, ToolUseBlock, ToolResultBlock]


class ConversationMessage(BaseModel):
    role: Literal["user", "assistant"]
    content: list[ContentBlock]

    @classmethod
    def from_user_text(cls, text: str) -> ConversationMessage:
        return cls(role="user", content=[TextBlock(text=text)])

    @property
    def text(self) -> str:
        return "".join(
            block.text for block in self.content if isinstance(block, TextBlock)
        )

    @property
    def tool_uses(self) -> list[ToolUseBlock]:
        return [
            block for block in self.content if isinstance(block, ToolUseBlock)
        ]

    def to_api_param(self) -> dict:
        return {
            "role": self.role,
            "content": [block.model_dump() for block in self.content],
        }
