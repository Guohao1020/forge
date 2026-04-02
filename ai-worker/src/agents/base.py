from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass

from src.context.builder import ProjectContext
from src.models.client import LLMResponse
from src.models.router import ModelRouter, Purpose

logger = logging.getLogger(__name__)


@dataclass
class AgentResult:
    content: str  # Raw text response
    structured: dict  # Parsed JSON data
    tokens_used: int
    model: str
    provider: str
    latency_ms: int


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

    async def run(self, user_input: str, context: ProjectContext) -> AgentResult:
        system = self._build_system_prompt(context)
        messages = self._build_messages(user_input, context)
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

    def _parse_json(self, text: str) -> dict:
        text = text.strip()
        # Try direct parse
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            pass
        # Try markdown code block (various formats)
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
        # Try to find balanced {...} block (greedy from first { to last })
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
