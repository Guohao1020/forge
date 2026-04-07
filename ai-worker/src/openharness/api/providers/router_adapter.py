"""ModelRouterAdapter — bridges the existing ModelRouter to QueryEngine.

Wraps ModelRouter.chat() (which supports tools + fallback chains) and
yields ApiStreamEvents that QueryEngine consumes.

Note: This currently yields the full response as a single text delta event
because ModelRouter.chat() returns a complete LLMResponse. Real token-by-token
streaming requires wrapping stream_llm() directly, which doesn't support tools.
When tool support is added to stream_llm, this adapter should be updated.
"""

from __future__ import annotations

import logging
from typing import Any, AsyncIterator, Dict, List, Optional

from ..client import (
    ApiMessageCompleteEvent,
    ApiMessageRequest,
    ApiStreamEvent,
    ApiTextDeltaEvent,
)
from ..usage import UsageSnapshot
from ...engine.messages import (
    ConversationMessage,
    ContentBlock,
    TextBlock,
    ToolUseBlock,
)

logger = logging.getLogger(__name__)


class ModelRouterAdapter:
    """Adapts ModelRouter to the SupportsStreamingMessages protocol."""

    def __init__(self, router: Any, purpose: Any = None) -> None:
        self._router = router
        self._purpose = purpose

    async def stream_message(
        self, request: ApiMessageRequest,
    ) -> AsyncIterator[ApiStreamEvent]:
        """Call ModelRouter.chat() and yield events."""
        # Convert OpenHarness messages to router format
        messages = [msg.to_api_param() for msg in request.messages]

        kwargs: Dict[str, Any] = {}
        if self._purpose is not None:
            kwargs["purpose"] = self._purpose
        if request.tools:
            kwargs["tools"] = request.tools

        try:
            response = await self._router.chat(
                system=request.system_prompt or "",
                messages=messages,
                **kwargs,
            )
        except Exception as e:
            logger.error("ModelRouter.chat() failed: %s", e)
            raise

        # Build usage
        usage = UsageSnapshot(
            input_tokens=getattr(response, "input_tokens", 0),
            output_tokens=getattr(response, "output_tokens", 0),
        )

        # Build content blocks
        content_blocks: List[ContentBlock] = []

        # Text content
        text = getattr(response, "content", "") or ""
        if text:
            content_blocks.append(TextBlock(text=text))
            yield ApiTextDeltaEvent(text=text)

        # Tool calls
        tool_calls = getattr(response, "tool_calls", []) or []
        for tc in tool_calls:
            content_blocks.append(ToolUseBlock(
                id=getattr(tc, "id", f"toolu_{id(tc)}"),
                name=tc.name,
                input=tc.input if isinstance(tc.input, dict) else {},
            ))

        # Build assistant message
        stop_reason = getattr(response, "stop_reason", "end_turn") or "end_turn"
        assistant_msg = ConversationMessage(
            role="assistant",
            content=content_blocks if content_blocks else [TextBlock(text="")],
        )

        yield ApiMessageCompleteEvent(
            message=assistant_msg,
            usage=usage,
            stop_reason=stop_reason,
        )
