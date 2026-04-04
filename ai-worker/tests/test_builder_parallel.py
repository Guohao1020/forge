"""Tests for ContextBuilder parallel fetching and error resilience."""

import asyncio
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from src.context.builder import ContextBuilder, ProjectContext


class TestParallelFetch:
    """Verify that the 4 API calls run in parallel, not sequentially."""

    @pytest.mark.asyncio
    async def test_parallel_fetch_timing(self):
        """All 4 fetches should complete in ~1 delay period, not 4x."""
        builder = ContextBuilder()
        call_times = []

        async def slow_get(url, **kwargs):
            import time
            call_times.append(time.monotonic())
            await asyncio.sleep(0.05)  # 50ms simulated latency
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            if "specs/effective" in str(url):
                mock_resp.json.return_value = {"data": {"standards": [], "rules": []}}
            elif "specs/prompts" in str(url):
                mock_resp.json.return_value = {"data": {"items": []}}
            elif "profiles" in str(url):
                mock_resp.json.return_value = {"data": {"profiles": []}}
            else:
                mock_resp.json.return_value = {"data": {"name": "test", "description": "", "techStack": {}}}
            return mock_resp

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=slow_get)
        builder._client.aclose = AsyncMock()

        start = asyncio.get_event_loop().time()
        ctx = await builder.build(project_id=1, purpose="test")
        elapsed = asyncio.get_event_loop().time() - start

        # Should take ~50ms (parallel), not ~200ms (serial)
        assert elapsed < 0.15, f"Parallel fetch took {elapsed:.3f}s (should be <0.15s)"
        assert len(call_times) == 4, f"Expected 4 API calls, got {len(call_times)}"
        assert ctx.project_name == "test"

        await builder.close()

    @pytest.mark.asyncio
    async def test_partial_failure_returns_partial_context(self):
        """If 2 of 4 APIs fail, the other 2 should still populate context."""
        builder = ContextBuilder()

        call_count = 0
        async def mixed_get(url, **kwargs):
            nonlocal call_count
            call_count += 1
            url_str = str(url)
            if "specs/effective" in url_str:
                raise ConnectionError("specs service down")
            if "profiles" in url_str:
                raise TimeoutError("profiles timeout")
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            if "specs/prompts" in url_str:
                mock_resp.json.return_value = {"data": {"items": [{"isDefault": True, "systemPrompt": "AI", "userTemplate": ""}]}}
            else:
                mock_resp.json.return_value = {"data": {"name": "resilient", "description": "test", "techStack": {"languages": ["Go"]}}}
            return mock_resp

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=mixed_get)
        builder._client.aclose = AsyncMock()

        ctx = await builder.build(project_id=1, purpose="test")

        # Project info should be populated (succeeded)
        assert ctx.project_name == "resilient"
        assert ctx.tech_stack == {"languages": ["Go"]}
        # Prompt template should be populated (succeeded)
        assert ctx.prompt_template_system == "AI"
        # Standards should be empty (failed)
        assert ctx.coding_standards == []
        # Profiles should be empty (failed)
        assert ctx.project_profiles == {}
        # All 4 calls were attempted
        assert call_count == 4

        await builder.close()

    @pytest.mark.asyncio
    async def test_all_apis_fail_returns_empty_context(self):
        """Even if all APIs fail, build() should return an empty context, not crash."""
        builder = ContextBuilder()

        async def failing_get(url, **kwargs):
            raise ConnectionError("everything is down")

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=failing_get)
        builder._client.aclose = AsyncMock()

        ctx = await builder.build(project_id=1, purpose="test")

        assert ctx.project_name == ""
        assert ctx.coding_standards == []
        assert ctx.project_profiles == {}
        assert ctx.prompt_template_system == ""

        await builder.close()

    @pytest.mark.asyncio
    async def test_conversation_history_not_affected_by_cache(self):
        """conversation_history should be per-call, not shared across builds."""
        builder = ContextBuilder()

        async def simple_get(url, **kwargs):
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            mock_resp.json.return_value = {"data": {"name": "test", "description": "", "techStack": {}, "standards": [], "rules": [], "items": [], "profiles": []}}
            return mock_resp

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=simple_get)
        builder._client.aclose = AsyncMock()

        history1 = [{"role": "user", "content": "hello"}]
        history2 = [{"role": "user", "content": "world"}]

        ctx1 = await builder.build(1, "test", conversation_history=history1)
        ctx2 = await builder.build(1, "test", conversation_history=history2)

        assert len(ctx1.conversation_history) == 1
        assert ctx1.conversation_history[0]["content"] == "hello"
        assert len(ctx2.conversation_history) == 1
        assert ctx2.conversation_history[0]["content"] == "world"

        await builder.close()

    @pytest.mark.asyncio
    async def test_http_404_treated_as_empty(self):
        """Non-200 responses should result in empty data, not errors."""
        builder = ContextBuilder()

        async def not_found_get(url, **kwargs):
            mock_resp = MagicMock()
            mock_resp.status_code = 404
            return mock_resp

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=not_found_get)
        builder._client.aclose = AsyncMock()

        ctx = await builder.build(project_id=999, purpose="test")

        assert ctx.project_name == ""
        assert ctx.coding_standards == []

        await builder.close()
