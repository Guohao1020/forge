"""
BaseAgent — Core agent class with multi-round tool-use agent loop.

Based on learn-claude-code s01 pattern:
  while True:
      response = llm.chat(system, messages, tools)
      if response.stop_reason != "tool_use":
          return parse(response)
      for tool_call in response.tool_calls:
          result = execute_tool(tool_call)
      messages.append(assistant_msg + tool_results)

When tools=None, behavior is identical to the original single-round call.
"""

from __future__ import annotations

import hashlib
import json
import logging
import re
import time
from dataclasses import dataclass, field
from typing import Any, Callable, Awaitable, Optional

from src.context.builder import ProjectContext
from src.models.client import LLMResponse
from src.models.router import ModelRouter, Purpose

logger = logging.getLogger(__name__)

# Agent loop constraints
MAX_TOOL_ROUNDS = 5          # Maximum tool-use iterations before forcing final output
TOOL_TIMEOUT_SECONDS = 10    # Per-tool-call timeout
TOKEN_BUDGET_RATIO = 0.8     # Stop tool loop when 80% of budget consumed
TOKEN_BUDGET = 180_000       # Same as ContextBuilder


@dataclass
class AgentResult:
    content: str  # Raw text response
    structured: dict  # Parsed JSON data
    tokens_used: int
    model: str
    provider: str
    latency_ms: int
    tool_calls_made: int = 0    # Number of tool calls executed
    parse_failed: bool = False  # True if final JSON parsing failed


def _normalize_assistant_content(response: LLMResponse) -> list:
    """Normalize assistant response content to Anthropic-style content blocks.

    This handles the difference between providers:
    - Anthropic raw_content: list of ContentBlock objects (TextBlock, ToolUseBlock)
    - OpenAI raw_content: ChatCompletionMessage with .content and .tool_calls

    Returns a list of dicts in Anthropic format:
    [{"type": "text", "text": "..."}, {"type": "tool_use", "id": "...", "name": "...", "input": {...}}]
    """
    raw = response.raw_content

    # Already a list (Anthropic format) — convert to dicts if needed
    if isinstance(raw, list):
        result = []
        for block in raw:
            if isinstance(block, dict):
                result.append(block)
            elif hasattr(block, "type"):
                # Anthropic ContentBlock object
                if block.type == "text":
                    result.append({"type": "text", "text": block.text})
                elif block.type == "tool_use":
                    result.append({"type": "tool_use", "id": block.id, "name": block.name, "input": block.input})
            else:
                result.append({"type": "text", "text": str(block)})
        return result

    # OpenAI ChatCompletionMessage — convert to Anthropic format
    result = []
    if hasattr(raw, "content") and raw.content:
        result.append({"type": "text", "text": raw.content})
    if hasattr(raw, "tool_calls") and raw.tool_calls:
        for tc in raw.tool_calls:
            result.append({
                "type": "tool_use",
                "id": tc.id,
                "name": tc.function.name,
                "input": json.loads(tc.function.arguments) if tc.function.arguments else {},
            })
    if not result:
        # Fallback: use the text content from the response
        result.append({"type": "text", "text": response.content or ""})
    return result


class BaseAgent:
    purpose: Purpose = Purpose.ANALYZE

    def __init__(self, router: ModelRouter) -> None:
        self.router = router

    def _build_system_prompt(self, context: ProjectContext) -> str:
        return context.to_system_prompt()

    def _build_messages(self, user_input: str, context: ProjectContext) -> list[dict]:
        messages = []
        for msg in context.conversation_history:
            messages.append(
                {"role": msg.get("role", "user"), "content": msg.get("content", "")}
            )
        messages.append({"role": "user", "content": user_input})
        return messages

    async def run(
        self,
        user_input: str,
        context: ProjectContext,
        tools: list[dict] | None = None,
        tool_executor: Optional[Any] = None,
    ) -> AgentResult:
        """
        Execute agent with optional multi-round tool use.

        Args:
            user_input: The user's request text
            context: Project context (standards, profiles, tech stack)
            tools: Optional list of tool definitions (JSON schema format).
                   When None, behaves as a single-round LLM call (backward compatible).
            tool_executor: Object with async execute(tool_call) -> str method.
                   Required when tools is not None.

        Returns:
            AgentResult with structured output and metadata.
        """
        system = self._build_system_prompt(context)
        messages = self._build_messages(user_input, context)

        # Spec injection verification logging
        logger.info(
            "Agent %s context: project=%s, standards=%d, rules=%d, profiles=%s, prompt_len=%d, tools=%s",
            self.__class__.__name__,
            context.project_name,
            len(context.coding_standards),
            len(context.review_rules),
            list(context.project_profiles.keys()),
            len(system),
            len(tools) if tools else 0,
        )

        # --- Single-round path (backward compatible, tools=None) ---
        if not tools:
            response: LLMResponse = await self.router.chat(
                system=system, messages=messages, purpose=self.purpose
            )
            structured = self._parse_json(response.content)
            return AgentResult(
                content=response.content,
                structured=structured,
                tokens_used=response.input_tokens + response.output_tokens,
                model=response.model,
                provider=response.provider,
                latency_ms=response.latency_ms,
            )

        # --- Multi-round agent loop (with tools) ---
        total_tokens = 0
        total_latency = 0
        tool_calls_made = 0
        tool_cache: dict[str, str] = {}  # dedup cache: hash(name+args) -> result
        last_model = ""
        last_provider = ""

        for round_num in range(MAX_TOOL_ROUNDS + 1):
            response = await self.router.chat(
                system=system,
                messages=messages,
                purpose=self.purpose,
                tools=tools,
            )
            total_tokens += response.input_tokens + response.output_tokens
            total_latency += response.latency_ms
            last_model = response.model
            last_provider = response.provider

            # Check if model wants to produce final output (no tool calls)
            if response.stop_reason != "tool_use" or not response.tool_calls:
                structured = self._parse_json(response.content)
                return AgentResult(
                    content=response.content,
                    structured=structured,
                    tokens_used=total_tokens,
                    model=last_model,
                    provider=last_provider,
                    latency_ms=total_latency,
                    tool_calls_made=tool_calls_made,
                )

            # Process tool calls
            tool_results = []
            for tc in response.tool_calls:
                tool_calls_made += 1
                cache_key = self._tool_cache_key(tc)

                if cache_key in tool_cache:
                    # Dedup: same tool + same args → return cached result
                    logger.debug("Tool call dedup hit: %s", tc.get("name"))
                    result = tool_cache[cache_key]
                else:
                    # Execute tool with timeout
                    try:
                        import asyncio
                        result = await asyncio.wait_for(
                            tool_executor.execute(tc),
                            timeout=TOOL_TIMEOUT_SECONDS,
                        )
                    except asyncio.TimeoutError:
                        result = f"Tool call timed out after {TOOL_TIMEOUT_SECONDS}s"
                        logger.warning("Tool %s timed out", tc.get("name"))
                    except Exception as e:
                        result = f"Tool execution error: {str(e)}"
                        logger.warning("Tool %s failed: %s", tc.get("name"), e)

                    tool_cache[cache_key] = result

                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tc.get("id", ""),
                    "content": result,
                })

            # Append assistant response and tool results in normalized format.
            # We use Anthropic-style format internally (content blocks), and the
            # client layer converts to OpenAI format when needed.
            #
            # For Anthropic: raw_content is a list of ContentBlock objects
            # For OpenAI: raw_content is a ChatCompletionMessage Pydantic model
            # We normalize both to Anthropic-style content blocks.
            assistant_content = _normalize_assistant_content(response)
            messages.append({"role": "assistant", "content": assistant_content})
            messages.append({"role": "user", "content": tool_results})

            # Token budget check: stop early if consuming too much
            if total_tokens > TOKEN_BUDGET * TOKEN_BUDGET_RATIO:
                logger.warning(
                    "Agent %s token budget threshold reached (%d/%d), forcing final output",
                    self.__class__.__name__,
                    total_tokens,
                    TOKEN_BUDGET,
                )
                break

        # Exceeded max rounds or token budget — force final output
        logger.warning(
            "Agent %s reached max tool rounds (%d) or budget, requesting final output",
            self.__class__.__name__,
            MAX_TOOL_ROUNDS,
        )
        messages.append({
            "role": "user",
            "content": "请立即输出最终 JSON 结果，不要再调用工具。直接输出 JSON。",
        })
        response = await self.router.chat(
            system=system, messages=messages, purpose=self.purpose
            # No tools passed — force text output
        )
        total_tokens += response.input_tokens + response.output_tokens
        total_latency += response.latency_ms

        structured = self._parse_json(response.content)
        parse_failed = not structured  # empty dict means parse failed

        return AgentResult(
            content=response.content,
            structured=structured,
            tokens_used=total_tokens,
            model=response.model or last_model,
            provider=response.provider or last_provider,
            latency_ms=total_latency,
            tool_calls_made=tool_calls_made,
            parse_failed=parse_failed,
        )

    @staticmethod
    def _tool_cache_key(tool_call: dict) -> str:
        """Generate a dedup cache key from tool name + input args."""
        name = tool_call.get("name", "")
        args = json.dumps(tool_call.get("input", {}), sort_keys=True)
        return hashlib.md5(f"{name}:{args}".encode()).hexdigest()

    def _parse_json(self, text: str) -> dict:
        text = text.strip()
        # Strategy 1: Direct parse
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            pass
        # Strategy 2: Markdown code block
        for pattern in [
            r"```(?:json)?\s*\n(.*?)\n\s*```",
            r"```(?:json)?\s*\n?(.*?)\n?\s*```",
        ]:
            match = re.search(pattern, text, re.DOTALL)
            if match:
                try:
                    return json.loads(match.group(1).strip())
                except json.JSONDecodeError:
                    pass
        # Strategy 3: Greedy brace match
        first_brace = text.find("{")
        last_brace = text.rfind("}")
        if first_brace != -1 and last_brace > first_brace:
            candidate = text[first_brace : last_brace + 1]
            try:
                return json.loads(candidate)
            except json.JSONDecodeError:
                pass
        logger.warning("Failed to parse JSON from agent response")
        logger.debug("Raw response (first 300 chars): %s", text[:300])
        return {}
