from __future__ import annotations

import asyncio
import json
import logging
from dataclasses import dataclass, field

import httpx

from src.config import settings

logger = logging.getLogger(__name__)
TOKEN_BUDGET = 180_000  # 200k max - 20k output reserve


@dataclass
class ProjectContext:
    project_name: str = ""
    project_description: str = ""
    tech_stack: dict = field(default_factory=dict)
    coding_standards: list[str] = field(default_factory=list)
    review_rules: list[dict] = field(default_factory=list)
    prompt_template_system: str = ""
    prompt_template_user: str = ""
    conversation_history: list[dict] = field(default_factory=list)
    project_profiles: dict = field(default_factory=dict)  # key -> JSONB value

    def to_system_prompt(self) -> str:
        """Assemble full system prompt from all layers."""
        parts = []
        # Layer 1: Prompt template system prompt
        if self.prompt_template_system:
            parts.append(self.prompt_template_system)
        # Layer 2: Standards injection
        if self.coding_standards:
            parts.append("\n## Coding Standards (MUST follow)\n")
            for std in self.coding_standards:
                parts.append(std)
        # Layer 3: Project context
        if self.project_name:
            parts.append(f"\n## Project\nName: {self.project_name}")
            if self.project_description:
                parts.append(f"Description: {self.project_description}")
            if self.tech_stack:
                parts.append(f"## Tech Stack Constraints\n{json.dumps(self.tech_stack, indent=2)}")
        # Layer 4: Project profiles (AI memory)
        if self.project_profiles:
            profile_labels = {
                "api_catalog": "API 接口清单",
                "db_schema": "数据库结构",
                "module_graph": "模块依赖图",
                "architecture": "技术架构",
                "business_rules": "业务规则",
                "coding_habits": "编码习惯",
                "quality_trends": "质量趋势",
            }
            profile_parts = []
            for key, value in self.project_profiles.items():
                label = profile_labels.get(key, key)
                value_str = json.dumps(value, ensure_ascii=False, indent=2)
                # Truncate individual profile if too large (keep under 10k chars)
                if len(value_str) > 10_000:
                    value_str = value_str[:10_000] + "\n... (truncated)"
                profile_parts.append(f"### {label}\n{value_str}")
            if profile_parts:
                parts.append(
                    "## 项目画像（AI 记忆）\n" + "\n\n".join(profile_parts)
                )
        return "\n\n".join(parts)


class ContextBuilder:
    def __init__(self):
        self._client = httpx.AsyncClient(
            base_url=settings.forge_api_url,
            headers={"Authorization": f"Bearer {settings.forge_api_token}"},
            timeout=10.0,
        )

    async def build(
        self,
        project_id: int,
        purpose: str,
        conversation_history: list[dict] | None = None,
    ) -> ProjectContext:
        ctx = ProjectContext()
        ctx.conversation_history = conversation_history or []

        # Parallel fetch all 4 APIs (SX-2 optimization: ~4x faster context loading)
        project_task = self._fetch_project(project_id)
        specs_task = self._fetch_effective_specs(project_id)
        prompts_task = self._fetch_prompt_template(purpose)
        profiles_task = self._fetch_profiles(project_id)

        project_data, specs_data, prompts_data, profiles_data = await asyncio.gather(
            project_task, specs_task, prompts_task, profiles_task,
            return_exceptions=True,
        )

        # Apply project data
        if isinstance(project_data, dict):
            ctx.project_name = project_data.get("name", "")
            ctx.project_description = project_data.get("description", "")
            ctx.tech_stack = project_data.get("techStack") or {}

        # Apply specs data
        if isinstance(specs_data, dict):
            standards = specs_data.get("standards", [])
            ctx.coding_standards = [
                s.get("content", "") for s in standards if s.get("content")
            ]
            ctx.review_rules = specs_data.get("rules", [])

        # Apply prompt template
        if isinstance(prompts_data, dict):
            ctx.prompt_template_system = prompts_data.get("systemPrompt", "")
            ctx.prompt_template_user = prompts_data.get("userTemplate", "")

        # Apply profiles
        if isinstance(profiles_data, dict):
            for key, value in profiles_data.items():
                if key and value:
                    ctx.project_profiles[key] = value

        logger.info(
            "Context built for project %d: standards=%d, rules=%d, profiles=%s, prompt_template=%s",
            project_id,
            len(ctx.coding_standards),
            len(ctx.review_rules),
            list(ctx.project_profiles.keys()),
            bool(ctx.prompt_template_system),
        )

        return ctx

    async def _fetch_project(self, project_id: int) -> dict:
        try:
            resp = await self._client.get(f"/api/projects/{project_id}")
            if resp.status_code == 200:
                return resp.json().get("data", {})
        except Exception as e:
            logger.warning("Failed to fetch project %d: %s", project_id, e)
        return {}

    async def _fetch_effective_specs(self, project_id: int) -> dict:
        try:
            resp = await self._client.get(f"/api/specs/effective/{project_id}")
            if resp.status_code == 200:
                return resp.json().get("data", {})
        except Exception as e:
            logger.warning("Failed to fetch specs for project %d: %s", project_id, e)
        return {}

    async def _fetch_prompt_template(self, purpose: str) -> dict:
        try:
            resp = await self._client.get(
                "/api/specs/prompts", params={"purpose": purpose}
            )
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                items = data.get("items", [])
                template = None
                for item in items:
                    if item.get("isDefault"):
                        template = item
                        break
                if not template and items:
                    template = items[0]
                if template:
                    return {
                        "systemPrompt": template.get("systemPrompt", ""),
                        "userTemplate": template.get("userTemplate", ""),
                    }
        except Exception as e:
            logger.warning("Failed to fetch prompt template for %s: %s", purpose, e)
        return {}

    async def _fetch_profiles(self, project_id: int) -> dict:
        try:
            resp = await self._client.get(f"/api/projects/{project_id}/profiles")
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                profiles_list = data.get("profiles", [])
                result = {}
                for p in profiles_list:
                    key = p.get("profileKey", "")
                    value = p.get("profileValue", {})
                    if key and value:
                        result[key] = value
                return result
        except Exception as e:
            logger.warning("Failed to fetch profiles for project %d: %s", project_id, e)
        return {}

    async def close(self):
        await self._client.aclose()
