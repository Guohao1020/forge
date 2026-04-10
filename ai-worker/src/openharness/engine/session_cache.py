"""LRUSessionCache — bounded in-memory store for QueryEngine instances.

Replaces the unbounded `_sessions: Dict[str, Any] = {}` in
api_server.py. Bounds memory growth at `max_size` entries (default
100) by evicting the least-recently-used session when capacity is
reached. Evicted engines get their clear() method called so
message history is released.

Not thread-safe. api_server.py is single-process async; for true
multi-process session storage, swap this for a Redis-backed cache
and keep the same interface.

Spec: §5.8 Session cache (LRU).
"""

from __future__ import annotations

import logging
from collections import OrderedDict
from typing import Any, Optional

logger = logging.getLogger(__name__)


class LRUSessionCache:
    """LRU cache mapping session_id to QueryEngine.

    Interface:
      - get(session_id) -> engine or None; refreshes LRU position
      - put(session_id, engine); evicts oldest if at capacity, or
        replaces in place if session_id already present
      - pop(session_id) -> engine or None; explicit delete, does NOT
        call clear() on the returned engine (caller owns lifecycle)
      - __len__() -> current entry count

    Eviction (from put() overflow) and replacement (from put() with
    an existing id) both call engine.clear() on the departing engine
    so message history is released. pop() does not, because the
    caller is explicitly taking ownership and will clear it themselves
    or keep it alive.
    """

    def __init__(self, max_size: int = 100) -> None:
        if max_size < 1:
            raise ValueError(f"max_size must be >= 1, got {max_size}")
        self._max_size = max_size
        self._cache: OrderedDict[str, Any] = OrderedDict()

    def get(self, session_id: str) -> Optional[Any]:
        """Return the cached engine for session_id, or None. If
        found, refresh its LRU position (most recently used)."""
        if session_id in self._cache:
            self._cache.move_to_end(session_id)
            return self._cache[session_id]
        return None

    def put(self, session_id: str, engine: Any) -> None:
        """Insert or replace a session. Calls clear() on any
        departing engine (replaced or evicted)."""
        if session_id in self._cache:
            # In-place replacement — clear the old engine first
            old = self._cache[session_id]
            try:
                old.clear()
            except Exception as e:
                logger.warning(
                    "LRUSessionCache: clear() on replaced engine raised: %s",
                    e,
                )
            self._cache[session_id] = engine
            self._cache.move_to_end(session_id)
            return

        self._cache[session_id] = engine
        # Move-to-end is redundant for a fresh insert (OrderedDict
        # inserts at end), but explicit is fine.
        self._cache.move_to_end(session_id)

        # Enforce max_size
        while len(self._cache) > self._max_size:
            oldest_id, oldest_engine = self._cache.popitem(last=False)
            try:
                oldest_engine.clear()
            except Exception as e:
                logger.warning(
                    "LRUSessionCache: clear() on evicted engine raised: %s",
                    e,
                )
            logger.info(
                "LRUSessionCache: evicted session %s (size was %d)",
                oldest_id,
                self._max_size,
            )

    def pop(self, session_id: str) -> Optional[Any]:
        """Remove a session explicitly. Returns the engine if it
        existed, else None. Does NOT call clear() — the caller owns
        the returned engine's lifecycle."""
        return self._cache.pop(session_id, None)

    def __len__(self) -> int:
        return len(self._cache)
