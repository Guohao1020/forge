"""Tests for multi-model router and circuit breaker."""

import pytest
from unittest.mock import AsyncMock, patch

from src.models.client import LLMResponse
from src.models.router import CircuitBreaker, ModelRouter, Purpose


class TestCircuitBreaker:
    def test_circuit_breaker_opens_after_threshold(self):
        cb = CircuitBreaker(threshold=3)
        assert cb.is_available()

        cb.record_failure()
        cb.record_failure()
        assert cb.is_available()

        cb.record_failure()
        assert not cb.is_available()

    def test_circuit_breaker_resets_on_success(self):
        cb = CircuitBreaker(threshold=3)
        cb.record_failure()
        cb.record_failure()
        cb.record_failure()
        assert not cb.is_available()

        cb.record_success()
        assert cb.is_available()
        assert cb.failures == 0


def _make_response(provider: str, model: str = "test-model") -> LLMResponse:
    return LLMResponse(
        content="test response",
        model=model,
        provider=provider,
        input_tokens=10,
        output_tokens=20,
        latency_ms=100,
    )


class TestModelRouter:
    @pytest.mark.asyncio
    async def test_router_returns_first_successful_model(self):
        router = ModelRouter()
        mock_caller = AsyncMock(return_value=_make_response("anthropic"))

        with (
            patch.dict(
                "src.models.router.PROVIDER_CALLERS",
                {"anthropic": mock_caller},
            ),
            patch.object(router, "_get_api_key", return_value="sk-test"),
        ):
            result = await router.chat("system", [{"role": "user", "content": "hi"}], Purpose.GENERATE)

        assert result.provider == "anthropic"
        mock_caller.assert_called_once()

    @pytest.mark.asyncio
    async def test_router_falls_back_on_failure(self):
        router = ModelRouter()
        fail_caller = AsyncMock(side_effect=Exception("API error"))
        ok_caller = AsyncMock(return_value=_make_response("openai"))

        with (
            patch.dict(
                "src.models.router.PROVIDER_CALLERS",
                {"anthropic": fail_caller, "openai": ok_caller},
            ),
            patch.object(router, "_get_api_key", return_value="sk-test"),
        ):
            result = await router.chat("system", [{"role": "user", "content": "hi"}], Purpose.GENERATE)

        assert result.provider == "openai"
        fail_caller.assert_called_once()
        ok_caller.assert_called_once()

    @pytest.mark.asyncio
    async def test_router_raises_when_all_fail(self):
        router = ModelRouter()
        fail_caller = AsyncMock(side_effect=Exception("API error"))

        with (
            patch.dict(
                "src.models.router.PROVIDER_CALLERS",
                {
                    "anthropic": fail_caller,
                    "openai": fail_caller,
                    "dashscope": fail_caller,
                    "deepseek": fail_caller,
                },
            ),
            patch.object(router, "_get_api_key", return_value="sk-test"),
        ):
            with pytest.raises(RuntimeError, match="All models failed"):
                await router.chat("system", [{"role": "user", "content": "hi"}], Purpose.GENERATE)
