import pytest
from fastapi.testclient import TestClient
from src.api_server import app


client = TestClient(app)


def test_health():
    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "ok"
    assert "sessions" in data


def test_delete_session_not_found():
    resp = client.delete("/api/sessions/nonexistent")
    assert resp.status_code == 404


def test_run_returns_202_shape():
    """Test the run endpoint returns correct response shape.

    Note: actual agent execution requires model API keys,
    so we just verify the fire-and-forget response shape.
    """
    resp = client.post("/api/run", json={
        "project_id": 1,
        "message": "hello",
    })
    assert resp.status_code == 200
    data = resp.json()
    assert "session_id" in data
    assert data["status"] == "accepted"
    assert "correlation_id" in data
