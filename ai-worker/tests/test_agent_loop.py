"""Tests for BaseAgent multi-round tool-use agent loop."""

import pytest
from unittest.mock import AsyncMock, MagicMock

from src.agents.base import BaseAgent, AgentResult, MAX_TOOL_ROUNDS
from src.context.builder import ProjectContext
from src.models.client import LLMResponse
from src.models.router import ModelRouter, Purpose


def make_response(content="test", stop_reason="end_turn", tool_calls=None):
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


class TestAgentLoopBackwardCompat:
    """When tools=None, agent loop should behave exactly like the old single-round call."""

    @pytest.mark.asyncio
    async def test_single_round_no_tools(self):
        router = MagicMock(spec=ModelRouter)
        router.chat = AsyncMock(return_value=make_response('{"status": "ok"}'))

        agent = BaseAgent(router)
        ctx = ProjectContext()
        result = await agent.run("test input", ctx)

        assert result.structured == {"status": "ok"}
        assert result.tokens_used == 150
        assert result.tool_calls_made == 0
        router.chat.assert_called_once()

    @pytest.mark.asyncio
    async def test_json_parsing_strategies(self):
        router = MagicMock(spec=ModelRouter)

        # Markdown block
        router.chat = AsyncMock(return_value=make_response('```json\n{"key": "value"}\n```'))
        agent = BaseAgent(router)
        result = await agent.run("test", ProjectContext())
        assert result.structured == {"key": "value"}

        # Greedy brace
        router.chat = AsyncMock(return_value=make_response('Here is the result: {"a": 1} hope this helps'))
        result = await agent.run("test", ProjectContext())
        assert result.structured == {"a": 1}


class TestAgentLoopWithTools:
    """Test the multi-round tool use loop."""

    @pytest.mark.asyncio
    async def test_tool_call_then_final_response(self):
        """Agent makes one tool call, then produces final output."""
        router = MagicMock(spec=ModelRouter)

        # Round 1: tool call
        tool_response = make_response(
            content="",
            stop_reason="tool_use",
            tool_calls=[{"id": "tc1", "name": "query_db_schema", "input": {"table_name": "users"}}],
        )
        # Round 2: final output
        final_response = make_response('{"files": ["user.go"]}')

        router.chat = AsyncMock(side_effect=[tool_response, final_response])

        executor = AsyncMock()
        executor.execute = AsyncMock(return_value='[{"name": "users", "columns": []}]')

        agent = BaseAgent(router)
        tools = [{"name": "query_db_schema", "description": "test", "input_schema": {}}]
        result = await agent.run("generate code", ProjectContext(), tools=tools, tool_executor=executor)

        assert result.structured == {"files": ["user.go"]}
        assert result.tool_calls_made == 1
        assert router.chat.call_count == 2
        executor.execute.assert_called_once()

    @pytest.mark.asyncio
    async def test_max_rounds_enforced(self):
        """Agent should stop after MAX_TOOL_ROUNDS and force output."""
        router = MagicMock(spec=ModelRouter)

        # Always return tool calls
        tool_response = make_response(
            content="",
            stop_reason="tool_use",
            tool_calls=[{"id": "tc1", "name": "read_file", "input": {"path": "x.go"}}],
        )
        final_response = make_response('{"result": "forced"}')

        # MAX_TOOL_ROUNDS + 1 tool responses, then 1 forced final
        responses = [tool_response] * (MAX_TOOL_ROUNDS + 1) + [final_response]
        router.chat = AsyncMock(side_effect=responses)

        executor = AsyncMock()
        executor.execute = AsyncMock(return_value="file content")

        agent = BaseAgent(router)
        tools = [{"name": "read_file", "description": "test", "input_schema": {}}]
        result = await agent.run("test", ProjectContext(), tools=tools, tool_executor=executor)

        assert result.structured == {"result": "forced"}
        assert result.tool_calls_made == MAX_TOOL_ROUNDS + 1

    @pytest.mark.asyncio
    async def test_tool_timeout_handled(self):
        """Tool call that times out should return error string, not crash."""
        import asyncio

        router = MagicMock(spec=ModelRouter)

        tool_response = make_response(
            content="",
            stop_reason="tool_use",
            tool_calls=[{"id": "tc1", "name": "slow_tool", "input": {}}],
        )
        final_response = make_response('{"status": "ok"}')
        router.chat = AsyncMock(side_effect=[tool_response, final_response])

        executor = AsyncMock()
        # Simulate a very slow tool
        async def slow_execute(tc):
            await asyncio.sleep(100)
            return "should not reach"
        executor.execute = slow_execute

        agent = BaseAgent(router)
        tools = [{"name": "slow_tool", "description": "test", "input_schema": {}}]
        result = await agent.run("test", ProjectContext(), tools=tools, tool_executor=executor)

        # Should complete (timeout triggers after 10s in production, but test runs with mock)
        assert result.structured == {"status": "ok"}

    @pytest.mark.asyncio
    async def test_tool_dedup(self):
        """Same tool call with same args should return cached result."""
        router = MagicMock(spec=ModelRouter)

        # Two identical tool calls
        tool_response = make_response(
            content="",
            stop_reason="tool_use",
            tool_calls=[
                {"id": "tc1", "name": "query_db_schema", "input": {"table_name": "users"}},
                {"id": "tc2", "name": "query_db_schema", "input": {"table_name": "users"}},
            ],
        )
        final_response = make_response('{"ok": true}')
        router.chat = AsyncMock(side_effect=[tool_response, final_response])

        call_count = 0
        async def counting_execute(tc):
            nonlocal call_count
            call_count += 1
            return "result"

        executor = MagicMock()
        executor.execute = counting_execute

        agent = BaseAgent(router)
        tools = [{"name": "query_db_schema", "description": "test", "input_schema": {}}]
        result = await agent.run("test", ProjectContext(), tools=tools, tool_executor=executor)

        # Only 1 actual execution (second was deduped)
        assert call_count == 1
        assert result.tool_calls_made == 2  # Both counted, but only 1 executed

    @pytest.mark.asyncio
    async def test_parse_failed_flag(self):
        """When forced output also fails to parse, parse_failed should be True."""
        router = MagicMock(spec=ModelRouter)

        tool_response = make_response(
            content="",
            stop_reason="tool_use",
            tool_calls=[{"id": "tc1", "name": "tool", "input": {}}],
        )
        # Exceed max rounds with tool calls, then final output is not JSON
        responses = [tool_response] * (MAX_TOOL_ROUNDS + 1) + [make_response("not json at all")]
        router.chat = AsyncMock(side_effect=responses)

        executor = AsyncMock()
        executor.execute = AsyncMock(return_value="ok")

        agent = BaseAgent(router)
        tools = [{"name": "tool", "description": "test", "input_schema": {}}]
        result = await agent.run("test", ProjectContext(), tools=tools, tool_executor=executor)

        assert result.parse_failed is True
        assert result.structured == {}


class TestExtractJSON:
    """Tests for BaseAgent._parse_json edge cases."""

    def setup_method(self):
        router = MagicMock()
        self.agent = BaseAgent(router)

    def test_direct_json(self):
        result = self.agent._parse_json('{"key": "value"}')
        assert result == {"key": "value"}

    def test_markdown_code_block(self):
        text = 'Some explanation\n```json\n{"plan": "test"}\n```\nMore text'
        result = self.agent._parse_json(text)
        assert result == {"plan": "test"}

    def test_greedy_brace_match(self):
        text = 'Here is the output: {"result": 42} and some trailing text'
        result = self.agent._parse_json(text)
        assert result == {"result": 42}

    def test_no_json_found(self):
        result = self.agent._parse_json("This has no JSON at all")
        assert result == {}

    def test_invalid_json_in_braces(self):
        result = self.agent._parse_json("{not valid json}")
        assert result == {}

    def test_nested_json(self):
        text = '{"outer": {"inner": [1, 2, 3]}}'
        result = self.agent._parse_json(text)
        assert result["outer"]["inner"] == [1, 2, 3]

    def test_empty_string(self):
        result = self.agent._parse_json("")
        assert result == {}

    def test_code_block_without_json_tag(self):
        text = '```\n{"data": true}\n```'
        result = self.agent._parse_json(text)
        assert result == {"data": True}


class TestBuildMessages:
    """Tests for _build_messages with conversation history."""

    def test_with_conversation_history(self):
        router = MagicMock()
        agent = BaseAgent(router)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {"role": "user", "content": "I need a login page"},
            {"role": "assistant", "content": "Sure, let me plan that"},
        ]
        messages = agent._build_messages("Add password reset too", ctx)
        assert len(messages) == 3
        assert messages[0]["role"] == "user"
        assert messages[0]["content"] == "I need a login page"
        assert messages[1]["role"] == "assistant"
        assert messages[2]["content"] == "Add password reset too"

    def test_empty_conversation_history(self):
        router = MagicMock()
        agent = BaseAgent(router)
        ctx = ProjectContext()
        ctx.conversation_history = []
        messages = agent._build_messages("Hello", ctx)
        assert len(messages) == 1
        assert messages[0]["content"] == "Hello"

    def test_conversation_missing_fields(self):
        router = MagicMock()
        agent = BaseAgent(router)
        ctx = ProjectContext()
        ctx.conversation_history = [
            {"role": "user"},  # missing content
            {"content": "test"},  # missing role
        ]
        messages = agent._build_messages("end", ctx)
        assert len(messages) == 3
        assert messages[0]["content"] == ""  # default for missing
        assert messages[1]["role"] == "user"  # default for missing


class TestToolExceptionHandling:
    """Test tool execution error paths."""

    @pytest.mark.asyncio
    async def test_tool_execution_error(self):
        """Tool that raises should be caught and returned as error string."""
        router = MagicMock()
        router.chat = AsyncMock(side_effect=[
            make_response(
                content="",
                stop_reason="tool_use",
                tool_calls=[{"id": "1", "name": "bad_tool", "input": {}}],
            ),
            make_response(content='{"result": "done"}', stop_reason="end_turn"),
        ])

        async def failing_executor(tool_call):
            raise ValueError("tool crashed")

        executor = MagicMock()
        executor.execute = AsyncMock(side_effect=failing_executor)

        agent = BaseAgent(router)
        tools = [{"name": "bad_tool", "description": "test", "input_schema": {}}]
        result = await agent.run("test", ProjectContext(), tools=tools, tool_executor=executor)
        assert result is not None


# ============================================================
# _normalize_assistant_content tests
# ============================================================

class TestNormalizeAssistantContent:
    """Tests for _normalize_assistant_content() which converts provider-specific
    response formats into Anthropic-style content block dicts."""

    def test_anthropic_content_blocks(self):
        """Anthropic ContentBlock objects (with .type, .text, .id) convert to dicts."""
        from src.agents.base import _normalize_assistant_content

        # Simulate Anthropic ContentBlock objects
        text_block = MagicMock()
        text_block.type = "text"
        text_block.text = "Here is the analysis."

        tool_block = MagicMock()
        tool_block.type = "tool_use"
        tool_block.id = "toolu_123"
        tool_block.name = "query_db_schema"
        tool_block.input = {"table_name": "users"}

        response = LLMResponse(
            content="Here is the analysis.",
            model="claude-3",
            provider="anthropic",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            raw_content=[text_block, tool_block],
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 2
        assert result[0] == {"type": "text", "text": "Here is the analysis."}
        assert result[1] == {
            "type": "tool_use",
            "id": "toolu_123",
            "name": "query_db_schema",
            "input": {"table_name": "users"},
        }

    def test_openai_message_with_content_and_tool_calls(self):
        """OpenAI ChatCompletionMessage (with .content, .tool_calls) converts to Anthropic format."""
        from src.agents.base import _normalize_assistant_content

        # Simulate OpenAI tool call objects
        func_obj = MagicMock()
        func_obj.name = "read_file"
        func_obj.arguments = '{"path": "main.go"}'

        tool_call = MagicMock()
        tool_call.id = "call_abc"
        tool_call.function = func_obj

        # Simulate OpenAI ChatCompletionMessage
        oai_message = MagicMock()
        oai_message.content = "Let me read that file."
        oai_message.tool_calls = [tool_call]
        # Ensure it does NOT have .type attribute (OpenAI messages don't)
        del oai_message.type

        response = LLMResponse(
            content="Let me read that file.",
            model="gpt-4",
            provider="openai",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            raw_content=oai_message,
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 2
        assert result[0] == {"type": "text", "text": "Let me read that file."}
        assert result[1] == {
            "type": "tool_use",
            "id": "call_abc",
            "name": "read_file",
            "input": {"path": "main.go"},
        }

    def test_openai_message_no_tool_calls(self):
        """OpenAI message with content only (no tool_calls) produces single text block."""
        from src.agents.base import _normalize_assistant_content

        oai_message = MagicMock()
        oai_message.content = "The answer is 42."
        oai_message.tool_calls = None
        del oai_message.type

        response = LLMResponse(
            content="The answer is 42.",
            model="gpt-4",
            provider="openai",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            raw_content=oai_message,
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 1
        assert result[0] == {"type": "text", "text": "The answer is 42."}

    def test_already_dict_list_passthrough(self):
        """A list of dicts (already normalized) passes through unchanged."""
        from src.agents.base import _normalize_assistant_content

        blocks = [
            {"type": "text", "text": "Hello."},
            {"type": "tool_use", "id": "tc1", "name": "tool", "input": {}},
        ]

        response = LLMResponse(
            content="Hello.",
            model="test",
            provider="test",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            raw_content=blocks,
        )

        result = _normalize_assistant_content(response)

        assert result == blocks

    def test_empty_openai_message_fallback(self):
        """OpenAI message with empty content and no tool_calls uses response.content fallback."""
        from src.agents.base import _normalize_assistant_content

        oai_message = MagicMock()
        oai_message.content = None
        oai_message.tool_calls = None
        del oai_message.type

        response = LLMResponse(
            content="fallback text",
            model="gpt-4",
            provider="openai",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            raw_content=oai_message,
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 1
        assert result[0] == {"type": "text", "text": "fallback text"}

    def test_empty_openai_message_fallback_none_content(self):
        """OpenAI message with empty everything falls back to empty string."""
        from src.agents.base import _normalize_assistant_content

        oai_message = MagicMock()
        oai_message.content = ""
        oai_message.tool_calls = None
        del oai_message.type

        response = LLMResponse(
            content="",
            model="gpt-4",
            provider="openai",
            input_tokens=100,
            output_tokens=50,
            latency_ms=500,
            raw_content=oai_message,
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 1
        assert result[0] == {"type": "text", "text": ""}

    def test_list_with_unknown_objects_stringified(self):
        """List items without .type attribute get stringified as text blocks."""
        from src.agents.base import _normalize_assistant_content

        class UnknownBlock:
            """A plain object with no .type attribute."""
            def __str__(self):
                return "unknown content"

        response = LLMResponse(
            content="",
            model="test",
            provider="test",
            input_tokens=0,
            output_tokens=0,
            latency_ms=0,
            raw_content=[UnknownBlock()],
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 1
        assert result[0]["type"] == "text"
        assert result[0]["text"] == "unknown content"

    def test_openai_tool_call_empty_arguments(self):
        """OpenAI tool call with empty arguments string parses to empty dict."""
        from src.agents.base import _normalize_assistant_content

        func_obj = MagicMock()
        func_obj.name = "list_files"
        func_obj.arguments = ""

        tool_call = MagicMock()
        tool_call.id = "call_xyz"
        tool_call.function = func_obj

        oai_message = MagicMock()
        oai_message.content = None
        oai_message.tool_calls = [tool_call]
        del oai_message.type

        response = LLMResponse(
            content="",
            model="gpt-4",
            provider="openai",
            input_tokens=0,
            output_tokens=0,
            latency_ms=0,
            raw_content=oai_message,
        )

        result = _normalize_assistant_content(response)

        assert len(result) == 1
        assert result[0]["type"] == "tool_use"
        assert result[0]["input"] == {}
