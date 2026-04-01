"""LLM client wrappers for 4 providers with standardized response."""

from __future__ import annotations

import time
import logging
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

import anthropic
import openai

logger = logging.getLogger(__name__)

MAX_TOKENS = 8192


@dataclass
class LLMResponse:
    content: str
    model: str
    provider: str
    input_tokens: int
    output_tokens: int
    latency_ms: int


async def call_anthropic(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
) -> LLMResponse:
    """Call Anthropic Claude API."""
    client = anthropic.AsyncAnthropic(api_key=api_key)
    start = time.monotonic()
    response = await client.messages.create(
        model=model,
        max_tokens=MAX_TOKENS,
        system=system,
        messages=messages,
    )
    latency_ms = int((time.monotonic() - start) * 1000)
    return LLMResponse(
        content=response.content[0].text,
        model=response.model,
        provider="anthropic",
        input_tokens=response.usage.input_tokens,
        output_tokens=response.usage.output_tokens,
        latency_ms=latency_ms,
    )


async def _call_openai_compatible(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    provider: str,
    base_url: Optional[str] = None,
) -> LLMResponse:
    """Shared implementation for OpenAI-compatible APIs."""
    kwargs: dict[str, Any] = {"api_key": api_key}
    if base_url:
        kwargs["base_url"] = base_url
    client = openai.AsyncOpenAI(**kwargs)

    full_messages = [{"role": "system", "content": system}] + messages
    start = time.monotonic()
    response = await client.chat.completions.create(
        model=model,
        max_tokens=MAX_TOKENS,
        messages=full_messages,
    )
    latency_ms = int((time.monotonic() - start) * 1000)
    choice = response.choices[0]
    usage = response.usage
    return LLMResponse(
        content=choice.message.content or "",
        model=response.model,
        provider=provider,
        input_tokens=usage.prompt_tokens if usage else 0,
        output_tokens=usage.completion_tokens if usage else 0,
        latency_ms=latency_ms,
    )


async def call_openai(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
) -> LLMResponse:
    """Call OpenAI API."""
    return await _call_openai_compatible(api_key, model, system, messages, "openai")


async def call_dashscope(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
) -> LLMResponse:
    """Call Alibaba DashScope API (OpenAI-compatible)."""
    return await _call_openai_compatible(
        api_key,
        model,
        system,
        messages,
        "dashscope",
        base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",
    )


async def call_deepseek(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
) -> LLMResponse:
    """Call DeepSeek API (OpenAI-compatible)."""
    return await _call_openai_compatible(
        api_key,
        model,
        system,
        messages,
        "deepseek",
        base_url="https://api.deepseek.com",
    )


PROVIDER_CALLERS = {
    "anthropic": call_anthropic,
    "openai": call_openai,
    "dashscope": call_dashscope,
    "deepseek": call_deepseek,
}
