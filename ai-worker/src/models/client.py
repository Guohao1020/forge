"""LLM client wrappers for 4 providers with standardized response."""

from __future__ import annotations

import json
import time
import logging
from dataclasses import dataclass
from typing import Any, AsyncGenerator, Dict, List, Optional

import anthropic
import openai

logger = logging.getLogger(__name__)

MAX_TOKENS = 8192


@dataclass
class LLMResponse:
    content: str                         # Text content from the response
    model: str
    provider: str
    input_tokens: int
    output_tokens: int
    latency_ms: int
    stop_reason: str = "end_turn"        # "end_turn" or "tool_use"
    tool_calls: list = None              # List of tool call dicts [{name, id, input}]
    raw_content: Any = None              # Raw response content for multi-turn (assistant message)

    def __post_init__(self):
        if self.tool_calls is None:
            self.tool_calls = []


async def call_anthropic(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    **kwargs: Any,
) -> LLMResponse:
    """Call Anthropic Claude API with optional tool support."""
    client = anthropic.AsyncAnthropic(api_key=api_key)
    start = time.monotonic()

    create_kwargs: dict[str, Any] = {
        "model": model,
        "max_tokens": MAX_TOKENS,
        "system": system,
        "messages": messages,
    }
    # Pass tools if provided
    tools = kwargs.get("tools")
    if tools:
        create_kwargs["tools"] = tools

    response = await client.messages.create(**create_kwargs)
    latency_ms = int((time.monotonic() - start) * 1000)

    # Extract text content and tool calls
    content_text = ""
    tool_calls = []
    raw_content = []

    for block in response.content:
        raw_content.append(block)
        if block.type == "text":
            content_text = block.text
        elif block.type == "tool_use":
            tool_calls.append({
                "id": block.id,
                "name": block.name,
                "input": block.input,
            })

    return LLMResponse(
        content=content_text,
        model=response.model,
        provider="anthropic",
        input_tokens=response.usage.input_tokens,
        output_tokens=response.usage.output_tokens,
        latency_ms=latency_ms,
        stop_reason=response.stop_reason,  # "end_turn" or "tool_use"
        tool_calls=tool_calls,
        raw_content=raw_content,
    )


def _convert_messages_for_openai(messages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Convert Anthropic-format messages to OpenAI-compatible format.

    Handles:
    - assistant messages with content blocks → assistant with tool_calls
    - user messages with tool_result blocks → separate tool messages
    - Regular text messages pass through unchanged
    """
    result = []
    for msg in messages:
        role = msg.get("role", "user")
        content = msg.get("content", "")

        # Regular string content — pass through
        if isinstance(content, str):
            result.append(msg)
            continue

        # Assistant message with Anthropic-style content blocks
        if role == "assistant" and isinstance(content, list):
            text_parts = []
            oai_tool_calls = []
            for block in content:
                if isinstance(block, dict):
                    if block.get("type") == "text":
                        text_parts.append(block.get("text", ""))
                    elif block.get("type") == "tool_use":
                        oai_tool_calls.append({
                            "id": block.get("id", ""),
                            "type": "function",
                            "function": {
                                "name": block.get("name", ""),
                                "arguments": json.dumps(block.get("input", {})),
                            },
                        })
            oai_msg: dict[str, Any] = {"role": "assistant", "content": "\n".join(text_parts) or None}
            if oai_tool_calls:
                oai_msg["tool_calls"] = oai_tool_calls
            result.append(oai_msg)
            continue

        # User message with tool_result blocks → separate tool messages
        if role == "user" and isinstance(content, list):
            for block in content:
                if isinstance(block, dict) and block.get("type") == "tool_result":
                    result.append({
                        "role": "tool",
                        "tool_call_id": block.get("tool_use_id", ""),
                        "content": block.get("content", ""),
                    })
                else:
                    # Non-tool content in user message — pass as text
                    result.append({"role": "user", "content": str(block)})
            continue

        # Fallback: convert content to string
        result.append({"role": role, "content": str(content)})

    return result


async def _call_openai_compatible(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    provider: str,
    base_url: Optional[str] = None,
    response_format: Optional[Dict[str, Any]] = None,
    tools: Optional[list[dict]] = None,
) -> LLMResponse:
    """Shared implementation for OpenAI-compatible APIs with tool support."""
    kwargs: dict[str, Any] = {"api_key": api_key}
    if base_url:
        kwargs["base_url"] = base_url
    client = openai.AsyncOpenAI(**kwargs)

    # Convert Anthropic-format messages to OpenAI format (handles tool rounds)
    converted = _convert_messages_for_openai(messages)
    full_messages = [{"role": "system", "content": system}] + converted
    create_kwargs: dict[str, Any] = {
        "model": model,
        "max_tokens": MAX_TOKENS,
        "messages": full_messages,
    }
    if response_format:
        create_kwargs["response_format"] = response_format
    if tools:
        # Convert Anthropic-format tools to OpenAI function calling format
        openai_tools = []
        for t in tools:
            openai_tools.append({
                "type": "function",
                "function": {
                    "name": t["name"],
                    "description": t.get("description", ""),
                    "parameters": t.get("input_schema", {}),
                },
            })
        create_kwargs["tools"] = openai_tools

    start = time.monotonic()
    response = await client.chat.completions.create(**create_kwargs)
    latency_ms = int((time.monotonic() - start) * 1000)
    choice = response.choices[0]
    usage = response.usage

    # Extract tool calls if present
    tool_calls = []
    stop_reason = "end_turn"
    if choice.finish_reason == "tool_calls" or (choice.message.tool_calls and len(choice.message.tool_calls) > 0):
        stop_reason = "tool_use"
        for tc in choice.message.tool_calls:
            tool_calls.append({
                "id": tc.id,
                "name": tc.function.name,
                "input": json.loads(tc.function.arguments) if tc.function.arguments else {},
            })

    return LLMResponse(
        content=choice.message.content or "",
        model=response.model,
        provider=provider,
        input_tokens=usage.prompt_tokens if usage else 0,
        output_tokens=usage.completion_tokens if usage else 0,
        latency_ms=latency_ms,
        stop_reason=stop_reason,
        tool_calls=tool_calls,
        raw_content=choice.message,
    )


async def call_openai(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    response_format: Optional[Dict[str, Any]] = None,
    tools: Optional[list[dict]] = None,
) -> LLMResponse:
    """Call OpenAI API."""
    return await _call_openai_compatible(
        api_key, model, system, messages, "openai",
        response_format=response_format, tools=tools,
    )


async def call_dashscope(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    response_format: Optional[Dict[str, Any]] = None,
    tools: Optional[list[dict]] = None,
) -> LLMResponse:
    """Call Alibaba DashScope API (OpenAI-compatible)."""
    return await _call_openai_compatible(
        api_key, model, system, messages, "dashscope",
        base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",
        response_format=response_format, tools=tools,
    )


async def call_deepseek(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    response_format: Optional[Dict[str, Any]] = None,
    tools: Optional[list[dict]] = None,
) -> LLMResponse:
    """Call DeepSeek API (OpenAI-compatible)."""
    return await _call_openai_compatible(
        api_key, model, system, messages, "deepseek",
        base_url="https://api.deepseek.com",
        response_format=response_format, tools=tools,
    )


PROVIDER_CALLERS = {
    "anthropic": call_anthropic,
    "openai": call_openai,
    "dashscope": call_dashscope,
    "deepseek": call_deepseek,
}


# --- Streaming support ---

_OPENAI_BASE_URLS = {
    "dashscope": "https://dashscope.aliyuncs.com/compatible-mode/v1",
    "deepseek": "https://api.deepseek.com",
}


async def stream_llm(
    api_key: str,
    model: str,
    provider: str,
    system: str,
    messages: list[dict[str, Any]],
    max_tokens: int = MAX_TOKENS,
) -> AsyncGenerator[str, None]:
    """Stream LLM response tokens. Yields text chunks as they arrive."""
    if provider == "anthropic":
        client = anthropic.AsyncAnthropic(api_key=api_key)
        async with client.messages.stream(
            model=model,
            max_tokens=max_tokens,
            system=system,
            messages=messages,
        ) as stream:
            async for text in stream.text_stream:
                yield text
    elif provider in ("openai", "dashscope", "deepseek"):
        base_url = _OPENAI_BASE_URLS.get(provider)
        kwargs: dict[str, Any] = {"api_key": api_key}
        if base_url:
            kwargs["base_url"] = base_url
        client = openai.AsyncOpenAI(**kwargs)
        msgs = [{"role": "system", "content": system}] + messages
        stream = await client.chat.completions.create(
            model=model, messages=msgs, max_tokens=max_tokens, stream=True
        )
        async for chunk in stream:
            if chunk.choices and chunk.choices[0].delta.content:
                yield chunk.choices[0].delta.content
    else:
        raise ValueError(f"Unknown provider for streaming: {provider}")
