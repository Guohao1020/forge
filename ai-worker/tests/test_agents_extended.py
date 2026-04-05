"""Extended tests for all AI agent classes."""

import pytest
from unittest.mock import AsyncMock, MagicMock

from src.agents.base import BaseAgent, AgentResult
from src.agents.planner import PlannerAgent, PLANNER_SYSTEM_PROMPT
from src.agents.coder import CoderAgent, CODER_SYSTEM_PROMPT, _build_language_constraints
from src.agents.test_writer import TestWriterAgent
from src.agents.reviewer import ReviewerAgent
from src.context.builder import ProjectContext
from src.models.client import LLMResponse
from src.models.router import ModelRouter, Purpose


def make_response(content, stop_reason="end_turn"):
    return LLMResponse(
        content=content, model="test", provider="test",
        input_tokens=100, output_tokens=50, latency_ms=500,
        stop_reason=stop_reason,
    )


class TestPlannerAgent:
    def test_purpose(self):
        assert PlannerAgent.purpose == Purpose.PLAN

    def test_system_prompt_contains_rules(self):
        assert "DAG" in PLANNER_SYSTEM_PROMPT
        assert "depends_on" in PLANNER_SYSTEM_PROMPT
        assert "touched_files" in PLANNER_SYSTEM_PROMPT
        assert "recommendations" in PLANNER_SYSTEM_PROMPT

    def test_system_prompt_includes_project_context(self):
        agent = PlannerAgent(MagicMock())
        ctx = ProjectContext(
            project_name="test-api",
            tech_stack={"languages": ["Go"]},
            coding_standards=["Use error handling"],
        )
        prompt = agent._build_system_prompt(ctx)
        assert "test-api" in prompt
        assert "Coding Standards" in prompt

    @pytest.mark.asyncio
    async def test_run_returns_structured_plan(self):
        router = MagicMock(spec=ModelRouter)
        router.chat = AsyncMock(return_value=make_response(
            '{"title":"Add user API","tasks":[{"order":1,"title":"Create model","type":"SCHEMA","files":["model.go"],"depends_on":[],"estimate_hours":1}],"risk_level":"LOW","risk_factors":[],"total_estimate_hours":1,"parallel_tracks":1,"touched_files":{"create":["model.go"],"modify":[]}}'
        ))
        agent = PlannerAgent(router)
        result = await agent.run("Add user management", ProjectContext())
        assert result.structured["title"] == "Add user API"
        assert len(result.structured["tasks"]) == 1
        assert result.structured["touched_files"]["create"] == ["model.go"]


class TestCoderAgent:
    def test_purpose(self):
        assert CoderAgent.purpose == Purpose.GENERATE

    def test_system_prompt_contains_rules(self):
        assert "Dockerfile" in CODER_SYSTEM_PROMPT
        assert "compilable" in CODER_SYSTEM_PROMPT

    def test_language_constraints_go(self):
        constraints = _build_language_constraints({"languages": ["Go"], "frameworks": ["Gin"]})
        assert "Go" in constraints
        assert "gofmt" in constraints
        assert "Gin" in constraints

    def test_language_constraints_typescript(self):
        constraints = _build_language_constraints({"languages": ["TypeScript"]})
        assert "TypeScript" in constraints
        assert "strict mode" in constraints

    def test_language_constraints_empty(self):
        constraints = _build_language_constraints({})
        assert constraints == ""

    def test_language_constraints_python(self):
        constraints = _build_language_constraints({"languages": ["Python"]})
        assert "PEP 8" in constraints
        assert "type hints" in constraints

    def test_language_alias_js(self):
        constraints = _build_language_constraints({"languages": ["js"]})
        assert "JavaScript" in constraints or "ESLint" in constraints or len(constraints) > 0

    def test_language_alias_ts(self):
        constraints = _build_language_constraints({"languages": ["ts"]})
        assert "TypeScript" in constraints or "strict" in constraints or len(constraints) > 0

    def test_language_alias_golang(self):
        constraints = _build_language_constraints({"languages": ["golang"]})
        assert "Go" in constraints or "gofmt" in constraints

    def test_language_alias_py(self):
        constraints = _build_language_constraints({"languages": ["py"]})
        assert "PEP" in constraints or "Python" in constraints or "type" in constraints

    @pytest.mark.asyncio
    async def test_run_returns_files(self):
        router = MagicMock(spec=ModelRouter)
        router.chat = AsyncMock(return_value=make_response(
            '{"files":[{"path":"main.go","content":"package main","action":"create","language":"go"}],"commit_message":"feat: add main","files_changed":1,"lines_added":1,"lines_deleted":0}'
        ))
        agent = CoderAgent(router)
        ctx = ProjectContext(tech_stack={"languages": ["Go"]})
        result = await agent.run("Create main.go", ctx)
        assert len(result.structured["files"]) == 1
        assert result.structured["files"][0]["path"] == "main.go"


class TestTestWriterAgent:
    def test_purpose(self):
        assert TestWriterAgent.purpose == Purpose.TEST_WRITING

    def test_system_prompt_contains_framework_detection(self):
        agent = TestWriterAgent(MagicMock())
        ctx = ProjectContext(tech_stack={"languages": ["Go"]})
        prompt = agent._build_system_prompt(ctx)
        assert "test" in prompt.lower()

    def test_system_prompt_with_project_context(self):
        agent = TestWriterAgent(MagicMock())
        ctx = ProjectContext(
            project_name="forge",
            project_description="AI platform",
            tech_stack={"languages": ["Go"], "frameworks": ["Gin"]},
        )
        prompt = agent._build_system_prompt(ctx)
        assert "forge" in prompt
        assert "Go" in prompt

    @pytest.mark.asyncio
    async def test_run_returns_test_files(self):
        router = MagicMock(spec=ModelRouter)
        router.chat = AsyncMock(return_value=make_response(
            '{"test_files":[{"path":"main_test.go","content":"package main","language":"go","framework":"go_test","covers_task":1}],"test_count":1,"framework":"go_test","coverage_targets":["main"]}'
        ))
        agent = TestWriterAgent(router)
        result = await agent.run("Write tests", ProjectContext(tech_stack={"languages": ["Go"]}))
        assert result.structured["framework"] == "go_test"
        assert len(result.structured["test_files"]) == 1


class TestReviewerAgent:
    def test_purpose(self):
        assert ReviewerAgent.purpose == Purpose.REVIEW

    @pytest.mark.asyncio
    async def test_review_pass(self):
        router = MagicMock(spec=ModelRouter)
        router.chat = AsyncMock(return_value=make_response(
            '{"passed":true,"score":95,"findings":[],"summary":"Good code","fix_instructions":""}'
        ))
        agent = ReviewerAgent(router)
        result = await agent.run("Review this code", ProjectContext())
        assert result.structured["passed"] is True
        assert result.structured["score"] == 95

    @pytest.mark.asyncio
    async def test_review_fail(self):
        router = MagicMock(spec=ModelRouter)
        router.chat = AsyncMock(return_value=make_response(
            '{"passed":false,"score":60,"findings":[{"severity":"ERROR","message":"Missing error handling"}],"summary":"Needs fixes","fix_instructions":"Add error handling to all functions"}'
        ))
        agent = ReviewerAgent(router)
        result = await agent.run("Review this code", ProjectContext())
        assert result.structured["passed"] is False
        assert len(result.structured["findings"]) == 1
        assert result.structured["fix_instructions"] != ""

    def test_system_prompt_with_project_context(self):
        agent = ReviewerAgent(MagicMock())
        ctx = ProjectContext(
            project_name="forge",
            project_description="AI platform",
            review_rules=[{"name": "no-console-log", "category": "style", "severity": "warning"}],
        )
        prompt = agent._build_system_prompt(ctx)
        assert "no-console-log" in prompt
        assert "forge" in prompt  # project context included

    def test_system_prompt_with_empty_rules(self):
        agent = ReviewerAgent(MagicMock())
        ctx = ProjectContext(review_rules=[])
        prompt = agent._build_system_prompt(ctx)
        assert len(prompt) > 0  # base prompt still present
