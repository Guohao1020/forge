"""Shared test fixtures for Forge AI Worker tests."""

import pytest
from unittest.mock import AsyncMock, MagicMock

from src.context.builder import ProjectContext
from src.models.client import LLMResponse
from src.models.router import ModelRouter


@pytest.fixture
def empty_context():
    """Empty ProjectContext with no data."""
    return ProjectContext()


@pytest.fixture
def sample_context():
    """ProjectContext with realistic sample data."""
    return ProjectContext(
        project_name="test-project",
        project_description="A sample Go API project for testing",
        tech_stack={
            "languages": ["Go"],
            "frameworks": ["Gin"],
            "projectType": "backend_api",
            "subType": "go_api",
            "branchStrategy": "trunk_based",
        },
        coding_standards=[
            "All functions must handle errors explicitly",
            "Use constructor injection for dependencies",
        ],
        review_rules=[
            {"id": 1, "name": "error-handling", "severity": "ERROR"},
        ],
        project_profiles={
            "api_catalog": {
                "endpoints": [
                    {"path": "/api/users", "method": "GET", "handler": "UserHandler.List"},
                    {"path": "/api/users/:id", "method": "GET", "handler": "UserHandler.Get"},
                    {"path": "/api/users", "method": "POST", "handler": "UserHandler.Create"},
                ]
            },
            "db_schema": {
                "tables": [
                    {
                        "name": "users",
                        "columns": [
                            {"name": "id", "type": "bigint", "primary": True},
                            {"name": "name", "type": "varchar(100)"},
                            {"name": "email", "type": "varchar(200)", "unique": True},
                            {"name": "created_at", "type": "timestamptz"},
                        ],
                    }
                ]
            },
            "business_rules": {
                "rules": [
                    {"domain": "user", "rule": "Email must be unique", "source": "service.go:42"},
                    {"domain": "auth", "rule": "JWT expires after 24h", "source": "auth.go:15"},
                ]
            },
            "module_graph": {
                "modules": [
                    {"name": "user", "path": "internal/user", "depends_on": ["auth"]},
                    {"name": "auth", "path": "internal/auth", "depends_on": []},
                ]
            },
        },
    )


@pytest.fixture
def mock_router():
    """ModelRouter mock that returns a simple response."""
    router = MagicMock(spec=ModelRouter)
    router.chat = AsyncMock(return_value=LLMResponse(
        content='{"status": "ok"}',
        model="test-model",
        provider="test",
        input_tokens=100,
        output_tokens=50,
        latency_ms=500,
    ))
    return router


@pytest.fixture
def mock_llm_response():
    """Factory for creating LLMResponse objects."""
    def _make(content='{"status": "ok"}', stop_reason="end_turn", tool_calls=None):
        return LLMResponse(
            content=content,
            model="test-model",
            provider="test",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            stop_reason=stop_reason,
            tool_calls=tool_calls or [],
            raw_content=content,
        )
    return _make
