"""Tests for LLM client utility functions.

Covers:
- _convert_messages_for_openai: Anthropic-format → OpenAI-compatible message conversion
"""

from __future__ import annotations

import json

from src.models.client import _convert_messages_for_openai


class TestConvertMessagesForOpenAI:
    """Tests for _convert_messages_for_openai() which converts Anthropic-style
    messages into OpenAI-compatible format for multi-round tool conversations."""

    def test_string_content_passthrough(self):
        """Regular string content messages pass through unchanged."""
        messages = [
            {"role": "user", "content": "Hello, world!"},
            {"role": "assistant", "content": "Hi there!"},
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 2
        assert result[0] == {"role": "user", "content": "Hello, world!"}
        assert result[1] == {"role": "assistant", "content": "Hi there!"}

    def test_assistant_text_blocks_concatenated(self):
        """Assistant message with text content blocks joins them with newlines."""
        messages = [
            {
                "role": "assistant",
                "content": [
                    {"type": "text", "text": "Let me analyze this."},
                    {"type": "text", "text": "Here is my finding."},
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 1
        assert result[0]["role"] == "assistant"
        assert result[0]["content"] == "Let me analyze this.\nHere is my finding."

    def test_assistant_tool_use_blocks_converted(self):
        """Assistant content with tool_use blocks converts to OpenAI tool_calls format."""
        messages = [
            {
                "role": "assistant",
                "content": [
                    {"type": "text", "text": "I'll check the database."},
                    {
                        "type": "tool_use",
                        "id": "toolu_123",
                        "name": "query_db_schema",
                        "input": {"table_name": "users"},
                    },
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 1
        msg = result[0]
        assert msg["role"] == "assistant"
        assert msg["content"] == "I'll check the database."
        assert len(msg["tool_calls"]) == 1
        tc = msg["tool_calls"][0]
        assert tc["id"] == "toolu_123"
        assert tc["type"] == "function"
        assert tc["function"]["name"] == "query_db_schema"
        assert json.loads(tc["function"]["arguments"]) == {"table_name": "users"}

    def test_assistant_tool_use_only_no_text(self):
        """Assistant message with only tool_use (no text) sets content to None."""
        messages = [
            {
                "role": "assistant",
                "content": [
                    {
                        "type": "tool_use",
                        "id": "toolu_abc",
                        "name": "read_file",
                        "input": {"path": "main.go"},
                    },
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 1
        msg = result[0]
        assert msg["role"] == "assistant"
        assert msg["content"] is None  # no text parts -> None
        assert len(msg["tool_calls"]) == 1

    def test_user_tool_result_blocks_to_tool_messages(self):
        """User message with tool_result blocks becomes separate tool role messages."""
        messages = [
            {
                "role": "user",
                "content": [
                    {
                        "type": "tool_result",
                        "tool_use_id": "toolu_123",
                        "content": '{"columns": ["id", "name"]}',
                    },
                    {
                        "type": "tool_result",
                        "tool_use_id": "toolu_456",
                        "content": "file content here",
                    },
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 2
        assert result[0] == {
            "role": "tool",
            "tool_call_id": "toolu_123",
            "content": '{"columns": ["id", "name"]}',
        }
        assert result[1] == {
            "role": "tool",
            "tool_call_id": "toolu_456",
            "content": "file content here",
        }

    def test_user_list_with_non_tool_content(self):
        """User message with list content containing non-tool blocks converts to text."""
        messages = [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": "Additional context"},
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 1
        assert result[0]["role"] == "user"
        # Non-tool content is stringified
        assert isinstance(result[0]["content"], str)

    def test_fallback_non_string_non_list_content(self):
        """Content that is neither string nor list gets str() converted."""
        messages = [
            {"role": "user", "content": 12345},
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 1
        assert result[0]["role"] == "user"
        assert result[0]["content"] == "12345"

    def test_mixed_conversation_full_round(self):
        """Full multi-round conversation with all message types."""
        messages = [
            # Round 1: user text
            {"role": "user", "content": "Generate a user API"},
            # Round 1: assistant with tool call
            {
                "role": "assistant",
                "content": [
                    {"type": "text", "text": "Let me check the schema."},
                    {
                        "type": "tool_use",
                        "id": "tc1",
                        "name": "query_db",
                        "input": {"table": "users"},
                    },
                ],
            },
            # Round 1: tool results
            {
                "role": "user",
                "content": [
                    {
                        "type": "tool_result",
                        "tool_use_id": "tc1",
                        "content": "schema data",
                    },
                ],
            },
            # Round 2: assistant final text
            {"role": "assistant", "content": "Here is the generated code."},
        ]

        result = _convert_messages_for_openai(messages)

        assert len(result) == 4
        # User text passthrough
        assert result[0] == {"role": "user", "content": "Generate a user API"}
        # Assistant with tool_calls
        assert result[1]["role"] == "assistant"
        assert "tool_calls" in result[1]
        assert result[1]["tool_calls"][0]["function"]["name"] == "query_db"
        # Tool result
        assert result[2]["role"] == "tool"
        assert result[2]["tool_call_id"] == "tc1"
        # Final assistant text passthrough
        assert result[3] == {"role": "assistant", "content": "Here is the generated code."}

    def test_empty_messages_list(self):
        """Empty input returns empty output."""
        result = _convert_messages_for_openai([])
        assert result == []

    def test_assistant_multiple_tool_calls(self):
        """Assistant with multiple tool_use blocks creates multiple tool_calls."""
        messages = [
            {
                "role": "assistant",
                "content": [
                    {
                        "type": "tool_use",
                        "id": "tc1",
                        "name": "read_file",
                        "input": {"path": "a.go"},
                    },
                    {
                        "type": "tool_use",
                        "id": "tc2",
                        "name": "read_file",
                        "input": {"path": "b.go"},
                    },
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        assert len(result) == 1
        assert len(result[0]["tool_calls"]) == 2
        assert result[0]["tool_calls"][0]["id"] == "tc1"
        assert result[0]["tool_calls"][1]["id"] == "tc2"

    def test_tool_use_missing_fields_default(self):
        """Tool use blocks with missing fields use empty defaults."""
        messages = [
            {
                "role": "assistant",
                "content": [
                    {"type": "tool_use"},  # missing id, name, input
                ],
            }
        ]
        result = _convert_messages_for_openai(messages)

        tc = result[0]["tool_calls"][0]
        assert tc["id"] == ""
        assert tc["function"]["name"] == ""
        assert tc["function"]["arguments"] == "{}"
