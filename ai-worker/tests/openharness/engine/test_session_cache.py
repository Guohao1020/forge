"""Tests for LRUSessionCache — bounded session storage for api_server."""

from unittest.mock import MagicMock

import pytest

from src.openharness.engine.session_cache import LRUSessionCache


def _make_engine():
    """Fake engine — just needs a clear() method for eviction."""
    return MagicMock(clear=MagicMock())


def test_empty_cache_get_returns_none():
    cache = LRUSessionCache(max_size=3)
    assert cache.get("missing") is None
    assert len(cache) == 0


def test_put_then_get():
    cache = LRUSessionCache(max_size=3)
    engine = _make_engine()
    cache.put("s1", engine)
    assert cache.get("s1") is engine
    assert len(cache) == 1


def test_put_same_session_twice_does_not_grow():
    cache = LRUSessionCache(max_size=3)
    e1 = _make_engine()
    e2 = _make_engine()
    cache.put("s1", e1)
    cache.put("s1", e2)  # same id, new engine
    assert len(cache) == 1
    assert cache.get("s1") is e2


def test_lru_eviction_order():
    cache = LRUSessionCache(max_size=3)
    e1, e2, e3, e4 = [_make_engine() for _ in range(4)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    cache.put("s3", e3)
    assert len(cache) == 3

    # Inserting a 4th should evict the oldest (s1)
    cache.put("s4", e4)
    assert len(cache) == 3
    assert cache.get("s1") is None
    assert cache.get("s2") is e2
    assert cache.get("s3") is e3
    assert cache.get("s4") is e4


def test_eviction_calls_engine_clear():
    """Evicted engines get their clear() method called so message
    history is released."""
    cache = LRUSessionCache(max_size=2)
    e1, e2, e3 = [_make_engine() for _ in range(3)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    cache.put("s3", e3)  # evicts s1

    e1.clear.assert_called_once()
    e2.clear.assert_not_called()
    e3.clear.assert_not_called()


def test_get_refreshes_lru_position():
    """Accessing an entry via get() should move it to the most-
    recently-used position, preventing eviction on the next put."""
    cache = LRUSessionCache(max_size=3)
    e1, e2, e3, e4 = [_make_engine() for _ in range(4)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    cache.put("s3", e3)

    # Touch s1 — now s2 is the LRU
    assert cache.get("s1") is e1

    # Insert s4 — should evict s2, not s1
    cache.put("s4", e4)
    assert cache.get("s1") is e1  # still there
    assert cache.get("s2") is None  # evicted
    assert cache.get("s3") is e3
    assert cache.get("s4") is e4


def test_pop_removes_without_calling_clear():
    """pop() is for explicit session deletion via DELETE /api/sessions/{id}
    — the caller decides whether to clear the engine."""
    cache = LRUSessionCache(max_size=3)
    e1 = _make_engine()
    cache.put("s1", e1)

    popped = cache.pop("s1")
    assert popped is e1
    assert cache.get("s1") is None
    e1.clear.assert_not_called()


def test_pop_missing_returns_none():
    cache = LRUSessionCache(max_size=3)
    assert cache.pop("nonexistent") is None


def test_put_with_max_size_1():
    """Edge case: max_size=1 means every put evicts the previous."""
    cache = LRUSessionCache(max_size=1)
    e1, e2 = _make_engine(), _make_engine()

    cache.put("s1", e1)
    cache.put("s2", e2)

    assert cache.get("s1") is None
    assert cache.get("s2") is e2
    e1.clear.assert_called_once()


def test_refreshing_same_id_does_not_trigger_eviction():
    """Putting an existing id with a new engine replaces in place —
    no eviction happens even at cache capacity."""
    cache = LRUSessionCache(max_size=2)
    e1, e2, e3 = [_make_engine() for _ in range(3)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    # Replace s1 with e3 — no eviction
    cache.put("s1", e3)
    assert len(cache) == 2
    assert cache.get("s1") is e3
    assert cache.get("s2") is e2
    # e1 was REPLACED — its clear() should be called.
    e1.clear.assert_called_once()
