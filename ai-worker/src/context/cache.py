"""
ContextCache — Workflow-level context caching via Redis.

One workflow execution (e.g., TaskWorkflow) calls 5+ activities (analyze, plan,
test_write, generate, review). Each activity needs the same ProjectContext.

Without cache: 5 activities × 4 HTTP calls = 20 round trips.
With cache:    1st activity builds + caches; 4 subsequent read from Redis = 4 calls total.

Cache key:   ctx:{workflow_id}
TTL:         30 minutes (one workflow execution)
Serialized:  JSON (ProjectContext dataclass → dict → JSON string)
"""

from __future__ import annotations

import json
import logging
from typing import Optional

import redis.asyncio as aioredis

from src.config import settings
from src.context.builder import ContextBuilder, ProjectContext

logger = logging.getLogger(__name__)

CACHE_TTL = 600  # 10 minutes — short enough for freshness, long enough for one workflow
CACHE_KEY_PREFIX = "ctx"


def _cache_key(project_id: int) -> str:
    """Cache key by project_id — all activities for same project share context."""
    return f"{CACHE_KEY_PREFIX}:project:{project_id}"


def _context_to_json(ctx: ProjectContext) -> str:
    """Serialize ProjectContext to JSON string for Redis storage."""
    return json.dumps(
        {
            "project_name": ctx.project_name,
            "project_description": ctx.project_description,
            "tech_stack": ctx.tech_stack,
            "coding_standards": ctx.coding_standards,
            "review_rules": ctx.review_rules,
            "prompt_template_system": ctx.prompt_template_system,
            "prompt_template_user": ctx.prompt_template_user,
            "project_profiles": ctx.project_profiles,
            # conversation_history is NOT cached — it changes per activity call
        },
        ensure_ascii=False,
    )


def _json_to_context(data: str) -> ProjectContext:
    """Deserialize JSON string back to ProjectContext."""
    d = json.loads(data)
    return ProjectContext(
        project_name=d.get("project_name", ""),
        project_description=d.get("project_description", ""),
        tech_stack=d.get("tech_stack", {}),
        coding_standards=d.get("coding_standards", []),
        review_rules=d.get("review_rules", []),
        prompt_template_system=d.get("prompt_template_system", ""),
        prompt_template_user=d.get("prompt_template_user", ""),
        project_profiles=d.get("project_profiles", {}),
        conversation_history=[],  # always fresh per call
    )


class ContextCache:
    """
    Workflow-level ProjectContext cache backed by Redis.

    Usage in activities:
        cache = ContextCache()
        ctx = await cache.get_or_build(workflow_id, project_id, purpose)
        ctx.conversation_history = conversation_history  # set per-call
    """

    def __init__(self) -> None:
        self._redis: Optional[aioredis.Redis] = None
        self._builder = ContextBuilder()

    async def _get_redis(self) -> aioredis.Redis:
        if self._redis is None:
            self._redis = aioredis.from_url(
                f"redis://{settings.redis_host}:{settings.redis_port}",
                password=settings.redis_password or None,
                decode_responses=True,
            )
        return self._redis

    async def get_or_build(
        self,
        project_id: int,
        purpose: str,
        conversation_history: list[dict] | None = None,
    ) -> ProjectContext:
        """
        Get cached ProjectContext for this project, or build and cache it.

        Cache key is project_id (not workflow_id) — all activities for the same
        project share context. TTL is 10 minutes, which covers a typical workflow
        execution while keeping data reasonably fresh.

        Args:
            project_id: Forge project ID
            purpose: Agent purpose string (e.g., "code-generation")
            conversation_history: Per-call conversation history (NOT cached)

        Returns:
            ProjectContext with cached project data + fresh conversation_history
        """
        key = _cache_key(project_id)

        # Try cache first
        try:
            r = await self._get_redis()
            cached = await r.get(key)
            if cached:
                ctx = _json_to_context(cached)
                ctx.conversation_history = conversation_history or []
                logger.info(
                    "ContextCache HIT: project=%d (%s), standards=%d, profiles=%s",
                    project_id,
                    ctx.project_name,
                    len(ctx.coding_standards),
                    list(ctx.project_profiles.keys()),
                )
                return ctx
        except Exception as e:
            logger.warning("ContextCache read failed (will rebuild): %s", e)

        # Cache miss — build from API (parallel fetch)
        logger.info("ContextCache MISS: project=%d, building from API", project_id)
        ctx = await self._builder.build(project_id, purpose, conversation_history)

        # Store in cache (fire-and-forget, don't block on cache write failure)
        try:
            r = await self._get_redis()
            await r.setex(key, CACHE_TTL, _context_to_json(ctx))
            logger.info("ContextCache SET: project=%d, ttl=%ds", project_id, CACHE_TTL)
        except Exception as e:
            logger.warning("ContextCache write failed (non-fatal): %s", e)

        return ctx

    async def invalidate(self, project_id: int) -> None:
        """Explicitly invalidate cache for a project (e.g., after profile scan)."""
        try:
            r = await self._get_redis()
            await r.delete(_cache_key(project_id))
            logger.info("ContextCache INVALIDATED: project=%d", project_id)
        except Exception as e:
            logger.warning("ContextCache invalidation failed: %s", e)

    async def close(self) -> None:
        """Clean up connections."""
        if self._redis:
            await self._redis.close()
        await self._builder.close()
