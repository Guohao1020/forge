"""Tests for ContextCache and ContextBuilder parallel fetch."""

import asyncio
import json
import pytest
from unittest.mock import AsyncMock, patch, MagicMock

from src.context.builder import ProjectContext, ContextBuilder
from src.context.cache import ContextCache, _context_to_json, _json_to_context, _cache_key


class TestProjectContext:
    def test_to_system_prompt_empty(self):
        ctx = ProjectContext()
        prompt = ctx.to_system_prompt()
        assert prompt == ""

    def test_to_system_prompt_with_standards(self):
        ctx = ProjectContext(
            coding_standards=["Always use error handling", "Follow PEP 8"],
        )
        prompt = ctx.to_system_prompt()
        assert "Coding Standards" in prompt
        assert "Always use error handling" in prompt
        assert "Follow PEP 8" in prompt

    def test_to_system_prompt_with_project(self):
        ctx = ProjectContext(
            project_name="test-project",
            project_description="A test project",
            tech_stack={"languages": ["Go"], "frameworks": ["Gin"]},
        )
        prompt = ctx.to_system_prompt()
        assert "test-project" in prompt
        assert "A test project" in prompt
        assert "Go" in prompt

    def test_to_system_prompt_with_profiles(self):
        ctx = ProjectContext(
            project_profiles={
                "api_catalog": {"endpoints": [{"path": "/api/users", "method": "GET"}]},
                "db_schema": {"tables": [{"name": "users"}]},
            },
        )
        prompt = ctx.to_system_prompt()
        assert "API 接口清单" in prompt
        assert "/api/users" in prompt
        assert "数据库结构" in prompt

    def test_to_system_prompt_truncates_large_profiles(self):
        ctx = ProjectContext(
            project_profiles={
                "api_catalog": {"data": "x" * 20000},
            },
        )
        prompt = ctx.to_system_prompt()
        assert "truncated" in prompt


class TestCacheSerialization:
    def test_context_to_json_roundtrip(self):
        ctx = ProjectContext(
            project_name="test",
            project_description="desc",
            tech_stack={"languages": ["Go"]},
            coding_standards=["rule1", "rule2"],
            review_rules=[{"id": 1, "name": "security"}],
            prompt_template_system="You are an AI",
            project_profiles={"api_catalog": {"endpoints": []}},
        )
        json_str = _context_to_json(ctx)
        restored = _json_to_context(json_str)

        assert restored.project_name == "test"
        assert restored.project_description == "desc"
        assert restored.tech_stack == {"languages": ["Go"]}
        assert restored.coding_standards == ["rule1", "rule2"]
        assert len(restored.review_rules) == 1
        assert restored.prompt_template_system == "You are an AI"
        assert "api_catalog" in restored.project_profiles
        # conversation_history is NOT cached
        assert restored.conversation_history == []

    def test_json_to_context_empty(self):
        ctx = _json_to_context("{}")
        assert ctx.project_name == ""
        assert ctx.coding_standards == []

    def test_cache_key(self):
        key = _cache_key(123)
        assert key == "ctx:project:123"


class TestContextBuilderParallel:
    @pytest.mark.asyncio
    async def test_build_parallel_fetch(self):
        """Verify that build() fetches all 4 APIs in parallel."""
        builder = ContextBuilder()

        # Mock all HTTP responses
        mock_responses = [
            ("/api/projects/1/profiles", {"data": {"profiles": [{"profileKey": "api_catalog", "profileValue": {"endpoints": []}}]}}),
            ("/api/projects/1", {"data": {"name": "test", "description": "desc", "techStack": {"languages": ["Go"]}}}),
            ("/api/specs/effective/1", {"data": {"standards": [{"content": "rule1"}], "rules": []}}),
            ("/api/specs/prompts", {"data": {"items": [{"isDefault": True, "systemPrompt": "You are AI", "userTemplate": ""}]}}),
        ]

        async def mock_get(url, **kwargs):
            url_str = str(url)
            # Match longest path first (profiles before projects)
            for path, response in mock_responses:
                if path in url_str:
                    mock_resp = MagicMock()
                    mock_resp.status_code = 200
                    mock_resp.json.return_value = response
                    return mock_resp
            mock_resp = MagicMock()
            mock_resp.status_code = 404
            return mock_resp

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=mock_get)
        builder._client.aclose = AsyncMock()

        ctx = await builder.build(project_id=1, purpose="code-generation")

        assert ctx.project_name == "test"
        assert ctx.coding_standards == ["rule1"]
        assert ctx.prompt_template_system == "You are AI"
        assert "api_catalog" in ctx.project_profiles

        await builder.close()

    @pytest.mark.asyncio
    async def test_build_handles_failures_gracefully(self):
        """Build should return partial context even if some APIs fail."""
        builder = ContextBuilder()

        async def mock_get(url, **kwargs):
            if "projects/1" in url and "profiles" not in url and "code" not in url:
                mock_resp = MagicMock()
                mock_resp.status_code = 200
                mock_resp.json.return_value = {"data": {"name": "test", "description": "", "techStack": {}}}
                return mock_resp
            raise ConnectionError("API down")

        builder._client = MagicMock()
        builder._client.get = AsyncMock(side_effect=mock_get)
        builder._client.aclose = AsyncMock()

        ctx = await builder.build(project_id=1, purpose="code-generation")

        # Project name should be populated even though other calls failed
        assert ctx.project_name == "test"
        assert ctx.coding_standards == []
        assert ctx.project_profiles == {}

        await builder.close()
