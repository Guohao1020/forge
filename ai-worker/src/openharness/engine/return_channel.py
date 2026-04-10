"""Redis pub/sub return channel — per-session subscriber that delivers
clarification responses to ClarificationCoordinator.

Spec reference: §2.9.2.c (subscriber lifecycle).

Channel name: agent:return:{session_id}
Message shape: §2.9.2.b — JSON with type, session_id, tool_use_id, response.
"""

from __future__ import annotations

import asyncio
import json
import logging
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    import redis.asyncio as aioredis

    from .agent_hooks import ClarificationCoordinator

logger = logging.getLogger(__name__)


class ReturnChannel:
    """Per-session Redis pub/sub subscriber.

    One instance per session, owned by QueryEngine. Spawns a listener
    task that dispatches incoming messages to the ClarificationCoordinator.

    Usage:
        rc = await ReturnChannel.open(session_id, redis_client, coordinator)
        # ... session runs, tools await coordinator.wait_for() ...
        await rc.close()
    """

    def __init__(
        self,
        session_id: str,
        pubsub: aioredis.client.PubSub,
        coordinator: ClarificationCoordinator,
        listener_task: asyncio.Task[None],
    ) -> None:
        self._session_id = session_id
        self._pubsub = pubsub
        self._coordinator = coordinator
        self._listener_task = listener_task
        self._closed = False
        self._channel_name = f"agent:return:{session_id}"

    @classmethod
    async def open(
        cls,
        session_id: str,
        redis_client: aioredis.Redis,
        coordinator: ClarificationCoordinator,
    ) -> ReturnChannel:
        """Subscribe to the session's return channel and spawn the listener.

        Returns a ReturnChannel instance. The listener runs until close()
        is called.
        """
        pubsub = redis_client.pubsub()
        channel_name = f"agent:return:{session_id}"
        await pubsub.subscribe(channel_name)

        instance = cls.__new__(cls)
        instance._session_id = session_id
        instance._pubsub = pubsub
        instance._coordinator = coordinator
        instance._closed = False
        instance._channel_name = channel_name

        # Spawn listener task — the instance reference is captured in the closure.
        instance._listener_task = asyncio.create_task(
            instance._listen(),
            name=f"return-channel-{session_id}",
        )
        return instance

    async def _listen(self) -> None:
        """Main listener loop — reads from pubsub and dispatches."""
        try:
            async for message in self._pubsub.listen():
                if self._closed:
                    break
                if message["type"] != "message":
                    # subscription confirmations, etc.
                    continue

                data = message.get("data")
                if isinstance(data, bytes):
                    data = data.decode("utf-8", errors="replace")

                self._dispatch(data)
        except asyncio.CancelledError:
            # Normal shutdown path
            return
        except Exception:
            logger.exception(
                "ReturnChannel listener failed for session %s — "
                "cancelling all pending futures",
                self._session_id,
            )
            # Cancel all pending futures with ReturnChannelError context
            self._coordinator.cancel_all()

    def _dispatch(self, raw: str) -> None:
        """Parse and validate a single message, then deliver to coordinator."""
        # Parse JSON
        try:
            payload = json.loads(raw)
        except (json.JSONDecodeError, TypeError):
            logger.warning(
                "ReturnChannel[%s]: malformed JSON discarded: %.200s",
                self._session_id,
                raw,
            )
            return

        if not isinstance(payload, dict):
            logger.warning(
                "ReturnChannel[%s]: payload is not a dict, discarded",
                self._session_id,
            )
            return

        # Validate type
        msg_type = payload.get("type")
        if msg_type != "clarification_response":
            logger.warning(
                "ReturnChannel[%s]: unexpected message type %r, discarded",
                self._session_id,
                msg_type,
            )
            return

        # Validate session_id
        msg_session_id = payload.get("session_id")
        if msg_session_id != self._session_id:
            logger.warning(
                "ReturnChannel[%s]: session_id mismatch (got %r), discarded",
                self._session_id,
                msg_session_id,
            )
            return

        # Extract tool_use_id and response
        tool_use_id = payload.get("tool_use_id")
        if not tool_use_id or not isinstance(tool_use_id, str):
            logger.warning(
                "ReturnChannel[%s]: missing or invalid tool_use_id, discarded",
                self._session_id,
            )
            return

        response = payload.get("response")
        if response is None or not isinstance(response, str):
            logger.warning(
                "ReturnChannel[%s]: missing or non-string response, discarded",
                self._session_id,
            )
            return

        # Deliver to coordinator
        self._coordinator.deliver(tool_use_id, response)

    async def close(self) -> None:
        """Tear down the subscriber. Idempotent — safe to call twice.

        1. Cancel the listener task.
        2. Unsubscribe from the channel.
        3. Close the pubsub handle.
        4. Cancel all pending coordinator futures.
        """
        if self._closed:
            return
        self._closed = True

        # Cancel listener task
        self._listener_task.cancel()
        try:
            await self._listener_task
        except (asyncio.CancelledError, Exception):
            pass

        # Unsubscribe and close pubsub
        try:
            await self._pubsub.unsubscribe(self._channel_name)
        except Exception:
            logger.debug(
                "ReturnChannel[%s]: unsubscribe failed (connection may be closed)",
                self._session_id,
            )
        try:
            await self._pubsub.aclose()
        except Exception:
            logger.debug(
                "ReturnChannel[%s]: pubsub close failed",
                self._session_id,
            )

        # Cancel any still-pending clarification futures
        self._coordinator.cancel_all()
