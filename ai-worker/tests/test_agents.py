from __future__ import annotations

from src.agents.analyst import AnalystAgent
from src.agents.base import BaseAgent
from src.agents.coder import CoderAgent
from src.agents.reviewer import ReviewerAgent
from src.context.builder import ProjectContext


def test_parse_json_direct():
    agent = BaseAgent(router=None)
    result = agent._parse_json('{"status": "confirmed"}')
    assert result == {"status": "confirmed"}


def test_parse_json_from_code_block():
    agent = BaseAgent(router=None)
    text = 'Here is the result:\n```json\n{"status": "clarify", "questions": ["Q1"]}\n```'
    result = agent._parse_json(text)
    assert result["status"] == "clarify"
    assert result["questions"] == ["Q1"]


def test_parse_json_fallback_empty():
    agent = BaseAgent(router=None)
    result = agent._parse_json("This is not JSON at all")
    assert result == {}


def test_analyst_system_prompt_includes_context():
    agent = AnalystAgent(router=None)
    ctx = ProjectContext(project_name="test-project", coding_standards=["Use camelCase"])
    prompt = agent._build_system_prompt(ctx)
    assert "product analyst" in prompt
    assert "test-project" in prompt


def test_reviewer_injects_review_rules():
    agent = ReviewerAgent(router=None)
    ctx = ProjectContext(
        review_rules=[
            {"name": "no-empty-catch", "category": "CODING", "severity": "ERROR"},
            {"name": "no-select-star", "category": "DATABASE", "severity": "ERROR"},
        ]
    )
    prompt = agent._build_system_prompt(ctx)
    assert "no-empty-catch" in prompt
    assert "no-select-star" in prompt
    assert "[ERROR] CODING" in prompt


def test_coder_system_prompt_has_standards():
    agent = CoderAgent(router=None)
    ctx = ProjectContext(coding_standards=["## Java Rules\n- Use camelCase"])
    prompt = agent._build_system_prompt(ctx)
    assert "Java Rules" in prompt
    assert "STRICTLY follow" in prompt
