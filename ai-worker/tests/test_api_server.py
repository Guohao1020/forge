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

    Note: actual agent execution requires model API keys and a real
    workspace, so we just verify the fire-and-forget response shape.
    The workspace_path is required but the background task will error
    silently since the directory doesn't exist.
    """
    resp = client.post("/api/run", json={
        "project_id": 1,
        "workspace_path": "test/fake",
        "message": "hello",
    })
    assert resp.status_code == 200
    data = resp.json()
    assert "session_id" in data
    assert data["status"] == "accepted"
    assert "correlation_id" in data


# ---------------------------------------------------------------------------
# Workspace prep tests (Phase 5 Task 5.7)
# ---------------------------------------------------------------------------


def test_workspace_prep_missing_workspace_returns_error(monkeypatch, tmp_path):
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(tmp_path))

    resp = client.post(
        "/api/workspace/prep",
        json={
            "tenant_id": 1,
            "project_id": 1,
            "workspace_path": "does-not-exist",
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "error"
    assert "does not exist" in body["error"]


def test_workspace_prep_unknown_language_returns_skipped(monkeypatch, tmp_path):
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(tmp_path))

    ws = tmp_path / "empty"
    ws.mkdir()
    (ws / "README.md").write_text("just a readme")

    resp = client.post(
        "/api/workspace/prep",
        json={
            "tenant_id": 1,
            "project_id": 1,
            "workspace_path": "empty",
        },
    )
    body = resp.json()
    assert body["status"] == "skipped"
