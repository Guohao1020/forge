"""Tests for Context Tools and ContextToolExecutor."""

import json
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from src.context.builder import ProjectContext
from src.context.tools import (
    CONTEXT_TOOLS,
    ContextToolExecutor,
    build_profile_availability_hint,
    _count_profile_items,
)


class TestToolDefinitions:
    def test_all_tools_have_required_fields(self):
        for tool in CONTEXT_TOOLS:
            assert "name" in tool
            assert "description" in tool
            assert "input_schema" in tool
            assert tool["input_schema"]["type"] == "object"
            assert "properties" in tool["input_schema"]
            assert "required" in tool["input_schema"]

    def test_tool_names(self):
        names = [t["name"] for t in CONTEXT_TOOLS]
        assert "query_api_catalog" in names
        assert "query_db_schema" in names
        assert "query_business_rules" in names
        assert "query_module_graph" in names
        assert "read_project_file" in names
        assert len(names) == 5

    def test_all_tools_have_chinese_descriptions(self):
        for tool in CONTEXT_TOOLS:
            # Each tool description should contain Chinese characters
            assert any("\u4e00" <= c <= "\u9fff" for c in tool["description"])


class TestProfileAvailabilityHint:
    def test_empty_profiles(self):
        hint = build_profile_availability_hint({})
        assert "not yet scanned" in hint or "未扫描" in hint or "empty" in hint.lower()

    def test_with_data(self):
        profiles = {
            "api_catalog": {"endpoints": [{"path": "/api/users"}, {"path": "/api/orders"}]},
            "db_schema": {"tables": [{"name": "users"}]},
            "module_graph": {},
            "business_rules": {"rules": [{"domain": "auth", "rule": "JWT expires"}]},
        }
        hint = build_profile_availability_hint(profiles)
        assert "api_catalog (2 endpoints)" in hint
        assert "db_schema (1 tables)" in hint
        assert "module_graph (empty)" in hint
        assert "business_rules (1 rules)" in hint

    def test_count_profile_items(self):
        assert _count_profile_items("api_catalog", {"endpoints": [1, 2, 3]}) == 3
        assert _count_profile_items("db_schema", {"tables": [1]}) == 1
        assert _count_profile_items("module_graph", {"modules": []}) == 0
        assert _count_profile_items("business_rules", {"rules": [1, 2]}) == 2
        assert _count_profile_items("unknown", {}) == 0


class TestContextToolExecutor:
    def setup_method(self):
        self.context = ProjectContext(
            project_profiles={
                "api_catalog": {
                    "endpoints": [
                        {"path": "/api/users", "method": "GET", "handler": "UserController.list"},
                        {"path": "/api/users/:id", "method": "GET", "handler": "UserController.get"},
                        {"path": "/api/orders", "method": "POST", "handler": "OrderController.create"},
                    ]
                },
                "db_schema": {
                    "tables": [
                        {"name": "users", "columns": [{"name": "id", "type": "bigint"}]},
                        {"name": "orders", "columns": [{"name": "id", "type": "bigint"}]},
                    ]
                },
                "business_rules": {
                    "rules": [
                        {"domain": "auth", "rule": "JWT expires after 24h", "source": "service.go:45"},
                        {"domain": "order", "rule": "Orders auto-cancel after 30min", "source": "service.go:120"},
                    ]
                },
                "module_graph": {
                    "modules": [
                        {"name": "auth", "path": "internal/auth", "depends_on": []},
                        {"name": "user", "path": "internal/user", "depends_on": ["auth"]},
                    ]
                },
            }
        )
        self.executor = ContextToolExecutor(self.context, project_id=1)

    @pytest.mark.asyncio
    async def test_query_api_catalog(self):
        result = await self.executor.execute({
            "name": "query_api_catalog",
            "input": {"keyword": "user"},
        })
        parsed = json.loads(result)
        assert len(parsed) == 2  # /api/users and /api/users/:id
        assert any("/api/users" in str(e) for e in parsed)

    @pytest.mark.asyncio
    async def test_query_api_catalog_no_match(self):
        result = await self.executor.execute({
            "name": "query_api_catalog",
            "input": {"keyword": "payment"},
        })
        assert "未找到" in result

    @pytest.mark.asyncio
    async def test_query_db_schema(self):
        result = await self.executor.execute({
            "name": "query_db_schema",
            "input": {"table_name": "users"},
        })
        parsed = json.loads(result)
        assert len(parsed) == 1
        assert parsed[0]["name"] == "users"

    @pytest.mark.asyncio
    async def test_query_business_rules(self):
        result = await self.executor.execute({
            "name": "query_business_rules",
            "input": {"domain": "auth"},
        })
        parsed = json.loads(result)
        assert len(parsed) == 1
        assert "JWT" in parsed[0]["rule"]

    @pytest.mark.asyncio
    async def test_query_module_graph(self):
        result = await self.executor.execute({
            "name": "query_module_graph",
            "input": {"module_name": "user"},
        })
        parsed = json.loads(result)
        assert len(parsed) == 1
        assert parsed[0]["name"] == "user"

    @pytest.mark.asyncio
    async def test_query_empty_dimension(self):
        """When a dimension has no data, return helpful message."""
        executor = ContextToolExecutor(ProjectContext(), project_id=1)
        result = await executor.execute({
            "name": "query_api_catalog",
            "input": {"keyword": "user"},
        })
        assert "没有" in result or "empty" in result.lower()

    @pytest.mark.asyncio
    async def test_unknown_tool(self):
        result = await self.executor.execute({
            "name": "nonexistent_tool",
            "input": {},
        })
        assert "Unknown tool" in result

    @pytest.mark.asyncio
    async def test_read_project_file_success(self):
        """Test file reading via mock HTTP."""
        with patch("src.context.tools.httpx.AsyncClient") as MockClient:
            mock_cm = AsyncMock()
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            mock_resp.json.return_value = {"data": {"content": "package main\n\nfunc main() {}"}}
            mock_cm.get = AsyncMock(return_value=mock_resp)
            mock_cm.__aenter__ = AsyncMock(return_value=mock_cm)
            mock_cm.__aexit__ = AsyncMock(return_value=False)
            MockClient.return_value = mock_cm

            result = await self.executor.execute({
                "name": "read_project_file",
                "input": {"path": "main.go"},
            })
            assert "package main" in result

    @pytest.mark.asyncio
    async def test_read_project_file_not_found(self):
        with patch("src.context.tools.httpx.AsyncClient") as MockClient:
            mock_cm = AsyncMock()
            mock_resp = MagicMock()
            mock_resp.status_code = 404
            mock_cm.get = AsyncMock(return_value=mock_resp)
            mock_cm.__aenter__ = AsyncMock(return_value=mock_cm)
            mock_cm.__aexit__ = AsyncMock(return_value=False)
            MockClient.return_value = mock_cm

            result = await self.executor.execute({
                "name": "read_project_file",
                "input": {"path": "nonexistent.go"},
            })
            assert "不存在" in result

    @pytest.mark.asyncio
    async def test_read_project_file_truncation(self):
        with patch("src.context.tools.httpx.AsyncClient") as MockClient:
            mock_cm = AsyncMock()
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            mock_resp.json.return_value = {"data": {"content": "x" * 25000}}
            mock_cm.get = AsyncMock(return_value=mock_resp)
            mock_cm.__aenter__ = AsyncMock(return_value=mock_cm)
            mock_cm.__aexit__ = AsyncMock(return_value=False)
            MockClient.return_value = mock_cm

            result = await self.executor.execute({
                "name": "read_project_file",
                "input": {"path": "big_file.go"},
            })
            assert "截断" in result
            assert len(result) < 25000


class TestProfileAvailabilityHint:
    """Tests for build_profile_availability_hint edge cases."""

    def test_empty_profiles(self):
        from src.context.tools import build_profile_availability_hint
        result = build_profile_availability_hint({})
        assert "empty" in result  # all dimensions show as empty
        assert "api_catalog" in result

    def test_profiles_with_data(self):
        from src.context.tools import build_profile_availability_hint
        profiles = {
            "api_catalog": {"endpoints": ["/api/v1/users", "/api/v1/tasks"]},
            "db_schema": {"tables": ["users", "tasks", "projects"]},
        }
        result = build_profile_availability_hint(profiles)
        assert "api_catalog" in result
        assert "2" in result  # 2 endpoints
        assert "db_schema" in result

    def test_profiles_with_empty_data(self):
        from src.context.tools import build_profile_availability_hint
        profiles = {"api_catalog": {}}
        result = build_profile_availability_hint(profiles)
        assert "empty" in result

    def test_count_profile_items_all_types(self):
        from src.context.tools import _count_profile_items
        assert _count_profile_items("api_catalog", {"endpoints": [1, 2, 3]}) == 3
        assert _count_profile_items("db_schema", {"tables": [1]}) == 1
        assert _count_profile_items("module_graph", {"modules": []}) == 0
        assert _count_profile_items("business_rules", {"rules": [1, 2]}) == 2
        assert _count_profile_items("architecture", {"services": [1, 2, 3, 4]}) == 4

    def test_count_profile_items_unknown_key(self):
        from src.context.tools import _count_profile_items
        result = _count_profile_items("unknown_key", {"foo": [1, 2]})
        assert result == 0

    def test_count_profile_items_non_dict(self):
        from src.context.tools import _count_profile_items
        result = _count_profile_items("api_catalog", "just a string")
        assert result == 0
