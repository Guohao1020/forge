import pytest
from src.openharness.engine.messages import (
    TextBlock, ToolUseBlock, ToolResultBlock, ConversationMessage
)


def test_text_block_creation():
    block = TextBlock(text="hello")
    assert block.type == "text"
    assert block.text == "hello"


def test_tool_use_block_creation():
    block = ToolUseBlock(name="bash", input={"command": "ls"})
    assert block.type == "tool_use"
    assert block.name == "bash"
    assert block.id.startswith("toolu_")


def test_conversation_message_from_user():
    msg = ConversationMessage.from_user_text("hello")
    assert msg.role == "user"
    assert msg.text == "hello"
    assert len(msg.content) == 1


def test_conversation_message_tool_uses():
    msg = ConversationMessage(role="assistant", content=[
        TextBlock(text="I'll run that"),
        ToolUseBlock(name="bash", input={"command": "ls"}),
    ])
    assert len(msg.tool_uses) == 1
    assert msg.tool_uses[0].name == "bash"


def test_tool_result_block():
    block = ToolResultBlock(tool_use_id="toolu_abc", content="file1.py\nfile2.py")
    assert block.type == "tool_result"
    assert not block.is_error


def test_message_to_api_param():
    msg = ConversationMessage.from_user_text("test")
    param = msg.to_api_param()
    assert param["role"] == "user"
    assert isinstance(param["content"], list)


# --- UsageSnapshot tests ---

from src.openharness.api.usage import UsageSnapshot


def test_usage_snapshot():
    u = UsageSnapshot(input_tokens=100, output_tokens=50)
    assert u.total_tokens == 150


# --- CostTracker tests ---

from src.openharness.engine.cost_tracker import CostTracker


def test_cost_tracker():
    ct = CostTracker()
    ct.add(UsageSnapshot(input_tokens=10, output_tokens=5))
    ct.add(UsageSnapshot(input_tokens=20, output_tokens=10))
    assert ct.total.input_tokens == 30
    assert ct.total.output_tokens == 15
    assert ct.total.total_tokens == 45


def test_cost_tracker_reset():
    ct = CostTracker()
    ct.add(UsageSnapshot(input_tokens=10, output_tokens=5))
    ct.reset()
    assert ct.total.total_tokens == 0


# --- StreamEvent tests ---

from src.openharness.engine.stream_events import (
    AssistantTextDelta, ToolExecutionStarted, ToolExecutionCompleted, ErrorEvent
)


def test_stream_event_types():
    delta = AssistantTextDelta(text="hello")
    assert delta.text == "hello"

    started = ToolExecutionStarted(tool_use_id="t1", tool_name="bash", tool_input={"command": "ls"})
    assert started.tool_name == "bash"
    assert started.tool_use_id == "t1"

    completed = ToolExecutionCompleted(tool_use_id="t1", tool_name="bash", output="file.py", is_error=False)
    assert not completed.is_error

    error = ErrorEvent(message="oops", recoverable=True)
    assert error.recoverable
