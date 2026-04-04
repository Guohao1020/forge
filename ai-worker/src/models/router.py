"""Multi-model router with circuit breaker and fallback chain."""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any

from src.config import settings
from src.models.client import PROVIDER_CALLERS, LLMResponse, stream_llm

logger = logging.getLogger(__name__)


class Purpose(Enum):
    ANALYZE = "analyze"
    PLAN = "plan"
    TEST_WRITING = "test_writing"
    GENERATE = "generate"
    REVIEW = "review"


@dataclass
class CircuitBreaker:
    failures: int = 0
    last_failure: float = 0.0
    is_open: bool = False
    threshold: int = 3
    window_seconds: float = 30.0
    recovery_seconds: float = 60.0

    def record_failure(self) -> None:
        now = time.monotonic()
        # Reset counter if outside failure window
        if now - self.last_failure > self.window_seconds:
            self.failures = 0
        self.failures += 1
        self.last_failure = now
        if self.failures >= self.threshold:
            self.is_open = True
            logger.warning("Circuit breaker opened after %d failures", self.failures)

    def record_success(self) -> None:
        self.failures = 0
        self.is_open = False

    def is_available(self) -> bool:
        if not self.is_open:
            return True
        # Half-open: allow retry after recovery period
        if time.monotonic() - self.last_failure > self.recovery_seconds:
            return True
        return False


# Each Purpose maps to a fallback chain of (provider, model) pairs.
# The router tries each in order, skipping providers without API keys
# or with open circuit breakers.
#
# Model selection per step:
#   ANALYZE:      qwen3-max (strongest reasoning, for requirement understanding)
#   PLAN:         qwen3-max (needs strong reasoning for architecture decisions)
#   TEST_WRITING: qwen3-coder-plus (code-specialized)
#   GENERATE:     qwen3-coder-plus (code-specialized)
#   REVIEW:       qwen3-max (needs reasoning to evaluate code quality)
ROUTING_RULES: dict[Purpose, list[tuple[str, str]]] = {
    Purpose.ANALYZE: [
        ("dashscope", "qwen3-max"),
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.PLAN: [
        ("dashscope", "qwen3-max"),
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.TEST_WRITING: [
        ("dashscope", "qwen3-coder-plus"),
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.GENERATE: [
        ("dashscope", "qwen3-coder-plus"),
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.REVIEW: [
        ("dashscope", "qwen3-max"),
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
    ],
}

# Map provider names to settings attribute names
_API_KEY_MAP = {
    "anthropic": "anthropic_api_key",
    "openai": "openai_api_key",
    "dashscope": "dashscope_api_key",
    "deepseek": "deepseek_api_key",
}


class ModelRouter:
    """Routes LLM calls through a fallback chain with circuit breakers."""

    def __init__(self) -> None:
        self._breakers: dict[str, CircuitBreaker] = {}

    def _get_breaker(self, key: str) -> CircuitBreaker:
        if key not in self._breakers:
            self._breakers[key] = CircuitBreaker()
        return self._breakers[key]

    def _get_api_key(self, provider: str) -> str:
        attr = _API_KEY_MAP.get(provider, "")
        return getattr(settings, attr, "")

    async def chat(
        self,
        system: str,
        messages: list[dict[str, Any]],
        purpose: Purpose = Purpose.GENERATE,
        tools: list[dict] | None = None,
    ) -> LLMResponse:
        """Route a chat request through the fallback chain for the given purpose.

        Args:
            tools: Optional tool definitions. When provided, the LLM may return
                   tool_use stop_reason with tool_calls in the response.
                   Providers that don't support tools are skipped.
        """
        chain = ROUTING_RULES[purpose]
        errors: list[str] = []

        for provider, model in chain:
            api_key = self._get_api_key(provider)
            if not api_key:
                logger.debug("Skipping %s: no API key configured", provider)
                continue

            breaker_key = f"{provider}:{model}"
            breaker = self._get_breaker(breaker_key)
            if not breaker.is_available():
                logger.debug("Skipping %s/%s: circuit breaker open", provider, model)
                continue

            try:
                caller = PROVIDER_CALLERS[provider]
                call_kwargs: dict[str, Any] = {}
                # For ANALYZE purpose on OpenAI-compatible providers, enforce JSON output
                # (but NOT when tools are active — tool responses use different format)
                if purpose == Purpose.ANALYZE and provider in ("dashscope", "openai", "deepseek") and not tools:
                    call_kwargs["response_format"] = {"type": "json_object"}
                # Pass tools through to provider callers
                if tools:
                    call_kwargs["tools"] = tools
                response = await caller(api_key, model, system, messages, **call_kwargs)
                breaker.record_success()
                logger.info(
                    "LLM call succeeded: provider=%s model=%s latency=%dms",
                    provider,
                    model,
                    response.latency_ms,
                )
                return response
            except Exception as exc:
                breaker.record_failure()
                error_msg = f"{provider}/{model}: {exc}"
                errors.append(error_msg)
                logger.warning("LLM call failed: %s", error_msg)

        raise RuntimeError(
            f"All models failed for purpose={purpose.value}: {'; '.join(errors)}"
        )

    async def chat_stream(
        self,
        system: str,
        messages: list[dict[str, Any]],
        purpose: Purpose = Purpose.GENERATE,
        task_id: int = 0,
    ) -> LLMResponse:
        """Stream a chat request, publishing chunks to Redis when task_id is set.

        Falls back through the model chain just like ``chat()``.  Each text
        chunk is published to ``code:stream:{task_id}`` so the Go SSE handler
        can forward it to the browser in real time.
        """
        import redis.asyncio as aioredis
        from src.config import settings as _settings

        chain = ROUTING_RULES[purpose]
        errors: list[str] = []

        redis_client = None
        channel = f"code:stream:{task_id}" if task_id else None
        if task_id:
            try:
                redis_client = aioredis.from_url(
                    f"redis://:{_settings.redis_password}@{_settings.redis_host}:{_settings.redis_port}"
                )
            except Exception as exc:
                logger.warning("Redis unavailable for streaming: %s", exc)

        try:
            for provider, model in chain:
                api_key = self._get_api_key(provider)
                if not api_key:
                    continue

                breaker_key = f"{provider}:{model}"
                breaker = self._get_breaker(breaker_key)
                if not breaker.is_available():
                    continue

                try:
                    import time as _time

                    start = _time.monotonic()
                    full_text = ""

                    async for chunk in stream_llm(
                        api_key, model, provider, system, messages
                    ):
                        full_text += chunk
                        if redis_client and channel:
                            try:
                                await redis_client.publish(channel, chunk)
                            except Exception:
                                pass  # fire-and-forget

                    latency_ms = int((_time.monotonic() - start) * 1000)
                    breaker.record_success()
                    logger.info(
                        "LLM stream succeeded: provider=%s model=%s latency=%dms",
                        provider,
                        model,
                        latency_ms,
                    )
                    return LLMResponse(
                        content=full_text,
                        model=model,
                        provider=provider,
                        input_tokens=0,
                        output_tokens=0,
                        latency_ms=latency_ms,
                    )
                except Exception as exc:
                    breaker.record_failure()
                    error_msg = f"{provider}/{model}: {exc}"
                    errors.append(error_msg)
                    logger.warning("LLM stream failed: %s", error_msg)
        finally:
            if redis_client:
                await redis_client.aclose()

        raise RuntimeError(
            f"All models failed (stream) for purpose={purpose.value}: {'; '.join(errors)}"
        )
