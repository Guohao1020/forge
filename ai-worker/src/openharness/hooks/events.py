from enum import Enum


class HookEvent(str, Enum):
    PRE_TOOL_USE = "pre_tool_use"
    POST_TOOL_USE = "post_tool_use"
    PRE_GENERATION = "pre_generation"
    POST_GENERATION = "post_generation"
    POST_PUSH = "post_push"
    POST_CI = "post_ci"
