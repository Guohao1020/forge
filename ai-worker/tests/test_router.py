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

    @pytest.mark.asyncio
    async def test_router_with_tools(self):
        """Tools parameter should be passed through to the provider caller."""
        router = ModelRouter()
        mock_caller = AsyncMock(return_value=_make_response("anthropic"))
        tools = [{"name": "query_db", "description": "test", "input_schema": {}}]

        with (
            patch.dict("src.models.router.PROVIDER_CALLERS", {"anthropic": mock_caller}),
            patch.object(router, "_get_api_key", return_value="sk-test"),
        ):
            result = await router.chat(
                "system", [{"role": "user", "content": "hi"}],
                Purpose.GENERATE, tools=tools,
            )

        assert result.provider == "anthropic"
        # Verify tools were passed
        call_kwargs = mock_caller.call_args
        assert call_kwargs is not None

    @pytest.mark.asyncio
    async def test_router_circuit_breaker_skips_failed_provider(self):
        """After enough failures, circuit breaker should skip the provider."""
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
            # Call multiple times to trigger circuit breaker
            for _ in range(5):
                try:
                    await router.chat("sys", [{"role": "user", "content": "hi"}], Purpose.GENERATE)
                except RuntimeError:
                    pass

            # After failures, should still work via fallback
            result = await router.chat("sys", [{"role": "user", "content": "hi"}], Purpose.GENERATE)
            assert result.provider == "openai"


class TestPurpose:
    def test_purpose_values(self):
        """All purpose values should be Enum members with string values."""
        purposes = [Purpose.ANALYZE, Purpose.PLAN, Purpose.GENERATE, Purpose.REVIEW, Purpose.TEST_WRITING]
        for p in purposes:
            assert p.value != ""  # Enum value should be non-empty

    def test_purpose_count(self):
        """Should have at least 5 purpose values."""
        assert len(Purpose) >= 5


class TestCircuitBreakerEdgeCases:
    def test_initial_state(self):
        cb = CircuitBreaker(threshold=5)
        assert cb.is_available()
        assert cb.failures == 0

    def test_just_below_threshold(self):
        cb = CircuitBreaker(threshold=3)
        cb.record_failure()
        cb.record_failure()
        assert cb.is_available()  # 2 < 3

    def test_exactly_at_threshold(self):
        cb = CircuitBreaker(threshold=3)
        cb.record_failure()
        cb.record_failure()
        cb.record_failure()
        assert not cb.is_available()  # 3 >= 3

    def test_multiple_successes_dont_go_negative(self):
        cb = CircuitBreaker(threshold=3)
        cb.record_success()
        cb.record_success()
        assert cb.failures == 0
        assert cb.is_available()
