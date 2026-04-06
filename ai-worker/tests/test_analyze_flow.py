"""Tests for the requirement analysis conversation flow.

Covers:
1. normalize_clarify_response — legacy → new format conversion
2. format_human_response — structured data → Chinese markdown
3. AnalystAgent prompt structure
4. End-to-end flow simulation with mocked LLM
"""

from __future__ import annotations

import json
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from copy import deepcopy

from src.activities.analyze import (
    format_human_response,
    normalize_clarify_response,
    AnalyzeInput,
    analyze_requirement_activity,
)
from src.agents.analyst import AnalystAgent, ANALYST_SYSTEM_PROMPT
from src.agents.base import AgentResult
from src.context.builder import ProjectContext


# ============================================================
# 1. normalize_clarify_response tests
# ============================================================

class TestNormalizeClarifyResponse:
    """Ensure legacy AI output is converted to single-question format."""

    def test_already_new_format_unchanged(self):
        """New format with single question should pass through unchanged."""
        data = {
            "status": "clarify",
            "phase": "scenario",
            "understanding": "你想做一个计算器",
            "question": "支持哪些运算？",
            "options": ["基本运算", "科学计算", "金融计算"],
            "risks": [],
        }
        original = deepcopy(data)
        result = normalize_clarify_response(data)
        assert result["question"] == original["question"]
        assert result["options"] == original["options"]
        assert result["phase"] == "scenario"
        assert "questions" not in result  # no legacy field

    def test_legacy_questions_array_converted(self):
        """Legacy questions array should be converted to single question."""
        data = {
            "status": "clarify",
            "questions": [
                "支持哪些基本运算？",
                "预期的用户并发量是多少？",
                "目标浏览器兼容性要求？",
            ],
            "partial_summary": "你想实现一个网页端的计算器程序。",
            "risks": [],
        }
        result = normalize_clarify_response(data)

        # Should pick first question
        assert result["question"] == "支持哪些基本运算？"
        # Legacy field removed
        assert "questions" not in result
        # partial_summary → understanding
        assert result["understanding"] == "你想实现一个网页端的计算器程序。"
        assert "partial_summary" not in result
        # Phase auto-filled
        assert result["phase"] == "understanding"

    def test_confirmed_status_not_modified(self):
        """confirmed status should pass through without modification."""
        data = {
            "status": "confirmed",
            "summary": "完整需求摘要",
            "functional_requirements": ["功能1", "功能2"],
            "risks": [],
        }
        original = deepcopy(data)
        result = normalize_clarify_response(data)
        assert result == original  # unchanged

    def test_empty_questions_array(self):
        """Empty questions array with no question field."""
        data = {
            "status": "clarify",
            "questions": [],
            "partial_summary": "我的理解...",
            "risks": [],
        }
        result = normalize_clarify_response(data)
        # No question set since questions is empty
        assert "question" not in result or not result.get("question")
        assert result["understanding"] == "我的理解..."
        assert result["phase"] == "understanding"

    def test_phase_preserved_if_exists(self):
        """If phase already set, don't override."""
        data = {
            "status": "clarify",
            "phase": "constraints",
            "questions": ["性能要求？"],
            "partial_summary": "理解...",
            "risks": [],
        }
        result = normalize_clarify_response(data)
        assert result["phase"] == "constraints"  # not overridden

    def test_understanding_not_overwritten_by_partial_summary(self):
        """If both understanding and partial_summary exist, keep understanding."""
        data = {
            "status": "clarify",
            "understanding": "原始理解",
            "partial_summary": "旧理解",
            "questions": ["问题？"],
            "risks": [],
        }
        result = normalize_clarify_response(data)
        assert result["understanding"] == "原始理解"
        # partial_summary should still be there since understanding exists
        assert "partial_summary" in result


# ============================================================
# 2. format_human_response tests
# ============================================================

class TestFormatHumanResponse:
    """Verify structured JSON → Chinese markdown conversion."""

    # -- clarify responses --

    def test_clarify_new_format_with_options(self):
        """New format: single question + options → options NOT in text (rendered as buttons)."""
        data = {
            "status": "clarify",
            "phase": "understanding",
            "understanding": "你想做一个网页端的计算器应用。",
            "question": "这个计算器主要面向什么场景？",
            "options": [
                "日常简单计算（加减乘除）",
                "科学计算（三角函数、对数等）",
                "金融计算（利率、汇率等）",
            ],
            "risks": [],
        }
        result = format_human_response("clarify", data)

        assert "💡 我的理解" in result
        assert "你想做一个网页端的计算器应用" in result
        assert "📋 初步理解" in result
        assert "❓ 这个计算器主要面向什么场景" in result
        # Options are NOT in text — they're rendered as clickable buttons by frontend
        assert "**A.**" not in result
        # No risks section since empty
        assert "风险提示" not in result

    def test_clarify_with_risks(self):
        """Risks should render with emoji indicators."""
        data = {
            "status": "clarify",
            "phase": "scenario",
            "understanding": "理解内容",
            "question": "问题？",
            "risks": [
                {"level": "HIGH", "description": "高风险", "mitigation": "缓解措施"},
                {"level": "LOW", "description": "低风险"},
            ],
        }
        result = format_human_response("clarify", data)

        assert "🔴 **[HIGH]** 高风险" in result
        assert "└ 缓解：缓解措施" in result
        assert "🟢 **[LOW]** 低风险" in result

    def test_clarify_legacy_format_fallback(self):
        """Legacy questions array: after normalize, only single question renders."""
        data = {
            "status": "clarify",
            "partial_summary": "旧格式理解",
            "questions": ["问题1？", "问题2？"],
            "risks": [],
        }
        # First normalize (as the activity does)
        data = normalize_clarify_response(data)
        result = format_human_response("clarify", data)

        assert "旧格式理解" in result
        assert "❓ 问题1" in result
        # Second question should NOT appear as numbered list
        assert "2. 问题2" not in result

    def test_clarify_open_question_no_options(self):
        """Question without options (open-ended) should still render."""
        data = {
            "status": "clarify",
            "phase": "constraints",
            "understanding": "理解",
            "question": "请描述你期望的界面风格？",
            "risks": [],
        }
        result = format_human_response("clarify", data)

        assert "❓ 请描述你期望的界面风格" in result
        assert "**A.**" not in result  # no options

    def test_clarify_scenario_phase_label(self):
        """Phase 'scenario' should show correct Chinese label."""
        data = {
            "status": "clarify",
            "phase": "scenario",
            "understanding": "x",
            "question": "q?",
            "risks": [],
        }
        result = format_human_response("clarify", data)
        assert "🔍 场景澄清" in result

    def test_clarify_constraints_phase_label(self):
        data = {
            "status": "clarify",
            "phase": "constraints",
            "understanding": "x",
            "question": "q?",
            "risks": [],
        }
        result = format_human_response("clarify", data)
        assert "⚙️ 约束确认" in result

    def test_clarify_empty_structured(self):
        """Edge case: empty dict should not crash."""
        result = format_human_response("clarify", {})
        assert isinstance(result, str)

    # -- confirmed responses --

    def test_confirmed_full_output(self):
        """Full confirmed response with all fields."""
        data = {
            "status": "confirmed",
            "summary": "实现一个支持基本运算的网页计算器",
            "task_title": "网页计算器",
            "functional_requirements": [
                "支持加减乘除四则运算",
                "支持清除和回退操作",
                "实时显示计算结果",
            ],
            "non_functional": {
                "performance": "响应时间 < 100ms",
                "security": "无敏感数据存储",
                "compatibility": "支持 Chrome/Firefox/Safari",
            },
            "affected_modules": ["frontend", "calculator-engine"],
            "estimated_complexity": "LOW",
            "risks": [
                {"level": "LOW", "description": "浮点精度问题"},
            ],
            "acceptance_criteria": [
                "能正确计算 1+2=3",
                "连续运算不丢失精度",
                "移动端可正常使用",
            ],
            "out_of_scope": [
                "科学计算功能",
                "用户登录和历史记录",
            ],
        }
        result = format_human_response("confirmed", data)

        # Structure markers
        assert "✅ 需求确认" in result
        assert "功能需求" in result
        assert "非功能需求" in result
        assert "验收标准" in result
        assert "不在范围内" in result
        assert "风险识别" in result

        # Content
        assert "支持加减乘除四则运算" in result
        assert "响应时间 < 100ms" in result
        assert "🟢 LOW" in result
        assert "能正确计算 1+2=3" in result
        assert "科学计算功能" in result
        assert "frontend, calculator-engine" in result
        assert "请确认以上需求" in result

    def test_confirmed_minimal(self):
        """Confirmed with only summary and risks."""
        data = {
            "status": "confirmed",
            "summary": "简单需求",
            "risks": [],
        }
        result = format_human_response("confirmed", data)

        assert "✅ 需求确认" in result
        assert "简单需求" in result
        assert "请确认以上需求" in result

    def test_confirmed_complexity_emoji(self):
        """Complexity levels should have correct emoji."""
        for level, emoji in [("LOW", "🟢"), ("MEDIUM", "🟡"), ("HIGH", "🔴")]:
            data = {
                "status": "confirmed",
                "summary": "test",
                "estimated_complexity": level,
                "risks": [],
            }
            result = format_human_response("confirmed", data)
            assert emoji in result
            assert level in result

    # -- unknown status --

    def test_unknown_status_uses_summary(self):
        """Unknown status falls back to summary field."""
        data = {"summary": "回退摘要"}
        result = format_human_response("unknown", data)
        assert result == "回退摘要"

    def test_unknown_status_no_summary(self):
        """Unknown status without summary converts dict to string."""
        data = {"foo": "bar"}
        result = format_human_response("unknown", data)
        assert isinstance(result, str)


# ============================================================
# 3. Analyst prompt structure tests
# ============================================================

class TestAnalystPrompt:
    """Verify the system prompt follows Superpowers methodology."""

    def test_prompt_is_chinese(self):
        """Prompt must instruct AI to respond in Chinese."""
        assert "中文" in ANALYST_SYSTEM_PROMPT

    def test_prompt_one_question_at_a_time(self):
        """Core principle: one question at a time."""
        assert "一次只问一个问题" in ANALYST_SYSTEM_PROMPT

    def test_prompt_multiple_choice_preferred(self):
        """Core principle: multiple choice options preferred."""
        assert "多选题优先" in ANALYST_SYSTEM_PROMPT

    def test_prompt_progressive_phases(self):
        """Prompt defines progressive phases."""
        assert "Phase 1" in ANALYST_SYSTEM_PROMPT
        assert "Phase 2" in ANALYST_SYSTEM_PROMPT
        assert "Phase 3" in ANALYST_SYSTEM_PROMPT
        assert "Phase 4" in ANALYST_SYSTEM_PROMPT

    def test_prompt_single_question_field(self):
        """Prompt emphasizes question (singular) not questions (plural)."""
        assert '"question"' in ANALYST_SYSTEM_PROMPT
        assert "不是数组" in ANALYST_SYSTEM_PROMPT

    def test_prompt_includes_examples(self):
        """Prompt includes concrete JSON examples."""
        assert '"phase": "understanding"' in ANALYST_SYSTEM_PROMPT
        assert '"phase": "scenario"' in ANALYST_SYSTEM_PROMPT

    def test_prompt_confirmed_has_acceptance_criteria(self):
        """Confirmed output format includes acceptance criteria."""
        assert "acceptance_criteria" in ANALYST_SYSTEM_PROMPT
        assert "functional_requirements" in ANALYST_SYSTEM_PROMPT
        assert "out_of_scope" in ANALYST_SYSTEM_PROMPT

    def test_prompt_yagni(self):
        """YAGNI principle mentioned."""
        assert "YAGNI" in ANALYST_SYSTEM_PROMPT

    def test_prompt_when_to_confirm(self):
        """Prompt explains when to confirm (>= 3 rounds, etc.)."""
        assert ">= 3" in ANALYST_SYSTEM_PROMPT or "对话轮次" in ANALYST_SYSTEM_PROMPT

    def test_prompt_context_injection(self):
        """Analyst agent injects project context."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext(
            project_name="my-project",
            coding_standards=["Use TypeScript"],
        )
        prompt = agent._build_system_prompt(ctx)
        assert "my-project" in prompt
        assert ANALYST_SYSTEM_PROMPT in prompt


# ============================================================
# 4. End-to-end flow simulation
# ============================================================

class TestEndToEndFlow:
    """Simulate a multi-turn requirement analysis conversation."""

    def _make_clarify_response(
        self, phase: str, understanding: str, question: str,
        options: list[str] | None = None, risks: list | None = None,
    ) -> str:
        """Helper: build a clarify JSON response string."""
        data = {
            "status": "clarify",
            "phase": phase,
            "understanding": understanding,
            "question": question,
            "risks": risks or [],
        }
        if options:
            data["options"] = options
        return json.dumps(data, ensure_ascii=False)

    def _make_confirmed_response(self) -> str:
        """Helper: build a confirmed JSON response string."""
        data = {
            "status": "confirmed",
            "summary": "实现一个支持加减乘除的网页计算器，界面简洁，无需登录。",
            "task_title": "网页计算器",
            "functional_requirements": [
                "加减乘除四则运算",
                "清除和回退",
                "键盘输入支持",
            ],
            "non_functional": {
                "performance": "即时响应",
                "security": "无数据存储",
                "compatibility": "主流浏览器",
            },
            "affected_modules": ["frontend"],
            "estimated_complexity": "LOW",
            "risks": [{"level": "LOW", "description": "浮点精度"}],
            "acceptance_criteria": [
                "1+1=2 正确显示",
                "连续运算无异常",
            ],
            "out_of_scope": ["科学计算", "用户系统"],
        }
        return json.dumps(data, ensure_ascii=False)

    def test_turn1_initial_understanding(self):
        """Turn 1: AI receives requirement, responds with understanding + question."""
        ai_response = self._make_clarify_response(
            phase="understanding",
            understanding="你想做一个网页端的计算器应用。",
            question="这个计算器主要面向什么场景？",
            options=["日常简单计算", "科学计算", "金融计算"],
        )
        from src.agents.base import BaseAgent
        agent = BaseAgent(router=None)
        structured = agent._parse_json(ai_response)
        structured = normalize_clarify_response(structured)

        assert structured["status"] == "clarify"
        assert structured["phase"] == "understanding"
        assert structured["question"] == "这个计算器主要面向什么场景？"
        assert len(structured["options"]) == 3

        content = format_human_response("clarify", structured)
        assert "💡 我的理解" in content
        assert "❓" in content
        # Options are rendered as frontend buttons, not in text
        assert "**A.**" not in content

    def test_turn2_scenario_clarification(self):
        """Turn 2: After user answers, AI digs deeper into scenario."""
        ai_response = self._make_clarify_response(
            phase="scenario",
            understanding="你需要一个日常简单计算器，支持加减乘除。",
            question="需要保存计算历史记录吗？",
            options=["不需要", "本地保存最近20条", "云端同步"],
        )
        from src.agents.base import BaseAgent
        structured = BaseAgent(router=None)._parse_json(ai_response)
        structured = normalize_clarify_response(structured)

        assert structured["phase"] == "scenario"
        content = format_human_response("clarify", structured)
        assert "🔍 场景澄清" in content
        assert "保存计算历史" in content

    def test_turn3_constraints(self):
        """Turn 3: AI asks about constraints."""
        ai_response = self._make_clarify_response(
            phase="constraints",
            understanding="日常计算器，不需要历史记录。",
            question="有特定的浏览器兼容性要求吗？",
            options=["仅现代浏览器", "需要兼容 IE11", "移动端优先"],
        )
        from src.agents.base import BaseAgent
        structured = BaseAgent(router=None)._parse_json(ai_response)
        structured = normalize_clarify_response(structured)

        assert structured["phase"] == "constraints"
        content = format_human_response("clarify", structured)
        assert "⚙️ 约束确认" in content

    def test_turn4_confirmed(self):
        """Turn 4: AI confirms requirements with full document."""
        ai_response = self._make_confirmed_response()
        from src.agents.base import BaseAgent
        structured = BaseAgent(router=None)._parse_json(ai_response)

        assert structured["status"] == "confirmed"

        content = format_human_response("confirmed", structured)
        assert "✅ 需求确认" in content
        assert "功能需求" in content
        assert "验收标准" in content
        assert "不在范围内" in content
        assert "请确认以上需求" in content

    def test_full_flow_no_json_leak(self):
        """Full flow: ensure no raw JSON is ever shown to user."""
        from src.agents.base import BaseAgent
        agent = BaseAgent(router=None)

        responses = [
            self._make_clarify_response("understanding", "理解1", "问题1？", ["A", "B"]),
            self._make_clarify_response("scenario", "理解2", "问题2？", ["X", "Y", "Z"]),
            self._make_clarify_response("constraints", "理解3", "问题3？"),
            self._make_confirmed_response(),
        ]

        for resp in responses:
            structured = agent._parse_json(resp)
            structured = normalize_clarify_response(structured)
            status = structured.get("status", "clarify")
            content = format_human_response(status, structured)

            # Critical: no raw JSON should appear in output
            assert not content.strip().startswith("{"), \
                f"Output starts with JSON brace: {content[:100]}"
            assert '"status"' not in content, \
                f"Raw JSON key found in output: {content[:100]}"
            assert '"questions"' not in content, \
                f"Legacy questions key found in output: {content[:100]}"

    def test_legacy_response_normalized_and_formatted(self):
        """Legacy AI response (questions array) gets normalized then formatted."""
        legacy_response = json.dumps({
            "status": "clarify",
            "questions": [
                "支持哪些运算？",
                "预期用户量？",
                "兼容性要求？",
            ],
            "partial_summary": "你想做一个计算器。",
            "risks": [{"level": "MEDIUM", "description": "需求不明确"}],
        }, ensure_ascii=False)

        from src.agents.base import BaseAgent
        structured = BaseAgent(router=None)._parse_json(legacy_response)
        structured = normalize_clarify_response(structured)

        # Normalized
        assert structured["question"] == "支持哪些运算？"
        assert "questions" not in structured
        assert structured["understanding"] == "你想做一个计算器。"
        assert "partial_summary" not in structured

        # Formatted
        content = format_human_response("clarify", structured)
        assert "💡 我的理解" in content
        assert "❓ 支持哪些运算" in content
        assert "🟡 **[MEDIUM]** 需求不明确" in content
        # Should NOT have numbered list (legacy format)
        assert "1. 支持哪些运算" not in content
        assert "2. 预期用户量" not in content


# ============================================================
# 5. Edge cases and robustness
# ============================================================

class TestEdgeCases:
    """Test edge cases that could cause issues in production."""

    def test_format_handles_none_values(self):
        """None values in fields should not crash."""
        data = {
            "status": "clarify",
            "understanding": None,
            "question": "问题？",
            "phase": None,
            "risks": None,
        }
        result = format_human_response("clarify", data)
        assert "❓ 问题？" in result

    def test_format_handles_missing_fields(self):
        """Missing fields should not crash."""
        data = {"status": "clarify"}
        result = format_human_response("clarify", data)
        assert isinstance(result, str)

    def test_normalize_handles_non_clarify(self):
        """Non-clarify status passes through."""
        data = {"status": "other", "data": "value"}
        result = normalize_clarify_response(data)
        assert result == data

    def test_format_confirmed_empty_non_functional(self):
        """Empty non_functional dict should not show section."""
        data = {
            "status": "confirmed",
            "summary": "test",
            "non_functional": {},
            "risks": [],
        }
        result = format_human_response("confirmed", data)
        assert "非功能需求" not in result

    def test_format_confirmed_non_functional_with_empty_values(self):
        """Non-functional with empty string values should be skipped."""
        data = {
            "status": "confirmed",
            "summary": "test",
            "non_functional": {"performance": "", "security": "需要加密"},
            "risks": [],
        }
        result = format_human_response("confirmed", data)
        assert "安全" in result
        assert "性能" not in result  # empty value skipped

    def test_options_not_rendered_in_text(self):
        """Options should NOT appear in formatted text (rendered as buttons)."""
        data = {
            "status": "clarify",
            "question": "选择？",
            "options": ["选项A", "选项B", "选项C"],
            "risks": [],
        }
        result = format_human_response("clarify", data)
        assert "❓ 选择" in result
        assert "**A.**" not in result  # options are buttons, not text

    def test_risk_unknown_level(self):
        """Unknown risk level should use default emoji."""
        data = {
            "status": "clarify",
            "question": "q",
            "risks": [{"level": "CRITICAL", "description": "关键风险"}],
        }
        result = format_human_response("clarify", data)
        assert "⚪" in result  # default for unknown level
        assert "CRITICAL" in result


class TestGenerateFallbackOptions:
    """Tests for _generate_fallback_options pure logic."""

    def test_chinese_or_pattern(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("使用MySQL还是PostgreSQL？")
        assert len(opts) >= 2
        assert any("MySQL" in o for o in opts)
        assert any("PostgreSQL" in o for o in opts)

    def test_chinese_or_pattern_alternative(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("前端用React或者Vue？")
        assert len(opts) >= 2

    def test_yes_no_pattern(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("是否需要用户登录功能？")
        assert len(opts) == 3
        assert "是的，需要" in opts[0]

    def test_yes_no_pattern_variant(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("需不需要支持多语言？")
        assert len(opts) == 3

    def test_scale_pattern(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("预期多少用户同时使用？")
        assert len(opts) == 3
        assert "小规模" in opts[0]

    def test_feature_pattern(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("需要支持哪些功能？")
        assert len(opts) == 3
        assert "基础功能" in opts[0]

    def test_generic_fallback(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("你觉得怎么样？")
        assert len(opts) == 3
        assert "是的" in opts[0]

    def test_empty_question(self):
        from src.activities.analyze import _generate_fallback_options
        opts = _generate_fallback_options("")
        assert len(opts) == 3  # generic fallback


# ============================================================
# 6. _split_thinking_and_json tests
# ============================================================

class TestSplitThinkingAndJson:
    """Tests for _split_thinking_and_json() which separates AI output into
    thinking text (streamed to user) and JSON structure (parsed for data)."""

    def test_normal_two_part_format(self):
        """Standard format: thinking text + ---JSON--- + JSON object."""
        from src.activities.analyze import _split_thinking_and_json

        text = (
            "Let me analyze this requirement...\n\n"
            "The user wants a calendar app with notes.\n\n"
            "---JSON---\n"
            '{"status": "clarify", "question": "What is the main use case?"}'
        )
        thinking, json_text = _split_thinking_and_json(text)

        assert "Let me analyze this requirement" in thinking
        assert "calendar app" in thinking
        assert "---JSON---" not in thinking
        assert "---JSON---" not in json_text
        assert json_text.startswith('{"status"')
        # Verify JSON is parseable
        parsed = json.loads(json_text)
        assert parsed["status"] == "clarify"

    def test_no_separator_backward_compat(self):
        """Without separator, entire text is treated as JSON (backward compat)."""
        from src.activities.analyze import _split_thinking_and_json

        text = '{"status": "confirmed", "summary": "All clear"}'
        thinking, json_text = _split_thinking_and_json(text)

        assert thinking == ""
        assert json_text == text
        parsed = json.loads(json_text)
        assert parsed["status"] == "confirmed"

    def test_empty_thinking_separator_at_start(self):
        """Separator at the very beginning means empty thinking."""
        from src.activities.analyze import _split_thinking_and_json

        text = '---JSON---\n{"status": "clarify", "question": "Q?"}'
        thinking, json_text = _split_thinking_and_json(text)

        assert thinking == ""
        assert '"status": "clarify"' in json_text

    def test_multiline_thinking(self):
        """Multi-paragraph thinking text before the separator."""
        from src.activities.analyze import _split_thinking_and_json

        text = (
            "First paragraph of analysis.\n\n"
            "Second paragraph with more detail.\n"
            "- Point 1\n"
            "- Point 2\n\n"
            "---JSON---\n"
            '{"status": "clarify"}'
        )
        thinking, json_text = _split_thinking_and_json(text)

        assert "First paragraph" in thinking
        assert "Second paragraph" in thinking
        assert "Point 1" in thinking
        assert json_text == '{"status": "clarify"}'

    def test_only_separator_no_json(self):
        """Separator present but nothing after it."""
        from src.activities.analyze import _split_thinking_and_json

        text = "Some thinking\n\n---JSON---\n"
        thinking, json_text = _split_thinking_and_json(text)

        assert thinking == "Some thinking"
        assert json_text == ""

    def test_multiple_separators_only_first_used(self):
        """Only the first ---JSON--- is used as the split point."""
        from src.activities.analyze import _split_thinking_and_json

        text = (
            "Thinking\n"
            "---JSON---\n"
            '{"key": "value"}\n'
            "---JSON---\n"
            "extra stuff"
        )
        thinking, json_text = _split_thinking_and_json(text)

        assert thinking == "Thinking"
        # Everything after first separator, including the second one
        assert "---JSON---" in json_text
        assert '{"key": "value"}' in json_text

    def test_empty_string(self):
        """Empty string returns empty thinking and empty json_text."""
        from src.activities.analyze import _split_thinking_and_json

        thinking, json_text = _split_thinking_and_json("")
        assert thinking == ""
        assert json_text == ""


# ============================================================
# 7. AnalystAgent._build_messages tests
# ============================================================

class TestAnalystBuildMessages:
    """Tests for AnalystAgent._build_messages() which reconstructs
    assistant messages in the expected two-part format from DB-stored data."""

    def test_raw_response_in_metadata_used_directly(self):
        """When metadata contains raw_response, use it as-is for the assistant content."""
        agent = AnalystAgent(router=None)
        raw_ai_output = (
            "Some thinking...\n\n---JSON---\n"
            '{"status": "clarify", "question": "What?"}'
        )
        ctx = ProjectContext()
        ctx.conversation_history = [
            {"role": "user", "content": "I want a login page"},
            {
                "role": "assistant",
                "content": "Formatted human-readable text",
                "metadata": {
                    "status": "clarify",
                    "question": "What kind of login?",
                    "raw_response": raw_ai_output,
                },
            },
        ]
        messages = agent._build_messages("Add social login too", ctx)

        assert len(messages) == 3
        # Assistant message should use raw_response, not the formatted content
        assert messages[1]["role"] == "assistant"
        assert messages[1]["content"] == raw_ai_output
        assert "---JSON---" in messages[1]["content"]
        # Final user message
        assert messages[2]["content"] == "Add social login too"

    def test_metadata_with_status_no_raw_response(self):
        """When metadata has status but no raw_response, reconstruct thinking + JSON."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {"role": "user", "content": "Build a calculator"},
            {
                "role": "assistant",
                "content": "Some formatted text about the calculator",
                "metadata": {
                    "status": "clarify",
                    "understanding": "You want a calculator",
                    "question": "What type?",
                    "options": ["Basic", "Scientific"],
                },
            },
        ]
        messages = agent._build_messages("Basic please", ctx)

        assert len(messages) == 3
        assistant_content = messages[1]["content"]
        # Should contain the thinking (original content) and JSON separator
        assert "---JSON---" in assistant_content
        assert "Some formatted text about the calculator" in assistant_content
        # JSON part should contain the metadata
        json_part = assistant_content.split("---JSON---")[1].strip()
        parsed = json.loads(json_part)
        assert parsed["status"] == "clarify"
        assert parsed["question"] == "What type?"

    def test_metadata_with_status_empty_content(self):
        """When content is empty but metadata has status, use understanding as thinking."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {
                "role": "assistant",
                "content": "",
                "metadata": {
                    "status": "clarify",
                    "understanding": "I understand you want X",
                    "question": "Confirm?",
                },
            },
        ]
        messages = agent._build_messages("yes", ctx)

        assert len(messages) == 2
        assistant_content = messages[0]["content"]
        assert "---JSON---" in assistant_content
        # Should use understanding as the thinking part
        assert "I understand you want X" in assistant_content

    def test_no_metadata_passes_content_as_is(self):
        """Without metadata, assistant content passes through unchanged."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Original response text"},
        ]
        messages = agent._build_messages("Next question", ctx)

        assert len(messages) == 3
        assert messages[1]["content"] == "Original response text"

    def test_metadata_as_json_string(self):
        """Metadata stored as JSON string (not dict) gets parsed correctly."""
        agent = AnalystAgent(router=None)
        raw_ai_output = "thinking\n\n---JSON---\n{\"status\": \"clarify\"}"
        ctx = ProjectContext()
        ctx.conversation_history = [
            {
                "role": "assistant",
                "content": "Formatted text",
                "metadata": json.dumps({
                    "status": "clarify",
                    "raw_response": raw_ai_output,
                }),
            },
        ]
        messages = agent._build_messages("test", ctx)

        assert len(messages) == 2
        # Should parse JSON string metadata and find raw_response
        assert messages[0]["content"] == raw_ai_output

    def test_metadata_invalid_json_string_treated_as_no_metadata(self):
        """Invalid JSON string metadata falls back to raw content."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {
                "role": "assistant",
                "content": "The formatted response",
                "metadata": "not-valid-json",
            },
        ]
        messages = agent._build_messages("test", ctx)

        assert len(messages) == 2
        # Invalid JSON string -> meta becomes {}, no raw_response or status
        # Falls through to plain content
        assert messages[0]["content"] == "The formatted response"

    def test_user_messages_pass_through(self):
        """User messages are never modified, regardless of metadata."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {"role": "user", "content": "First message", "metadata": {"some": "data"}},
            {"role": "user", "content": "Second message"},
        ]
        messages = agent._build_messages("Third", ctx)

        assert len(messages) == 3
        assert messages[0]["content"] == "First message"
        assert messages[1]["content"] == "Second message"
        assert messages[2]["content"] == "Third"

    def test_empty_history(self):
        """Empty conversation history produces only the new user message."""
        agent = AnalystAgent(router=None)
        ctx = ProjectContext()
        ctx.conversation_history = []
        messages = agent._build_messages("Hello", ctx)

        assert len(messages) == 1
        assert messages[0] == {"role": "user", "content": "Hello"}
