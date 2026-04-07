import pytest
from src.openharness.skills.project_skills import load_project_skills


def test_load_project_skills_from_core():
    """Load skills from forge-core/skills/ directory."""
    config = load_project_skills("../forge-core/skills/")
    assert config.get_build_command("go") == "go build ./..."
    assert config.get_build_command("python") == "python -m py_compile"
    assert config.get_test_command("typescript") == "npm test"
    assert config.get_build_command("unknown") is None


def test_dockerfile_templates():
    config = load_project_skills("../forge-core/skills/")
    tmpl = config.get_dockerfile_template("go")
    assert tmpl is not None
    assert "golang" in tmpl.get("base_image", "")


def test_agent_loop_params():
    config = load_project_skills("../forge-core/skills/")
    params = config.get_agent_loop_params("generate")
    assert params["max_tokens"] == 8192
    assert params["max_turns"] == 30


def test_agent_loop_defaults():
    config = load_project_skills("../forge-core/skills/")
    params = config.get_agent_loop_params("unknown_purpose")
    assert params["max_tokens"] == 4096  # defaults


def test_build_verify_config():
    config = load_project_skills("../forge-core/skills/")
    bv = config.get_build_verify_config()
    assert bv["max_retries"] == 3
    assert bv["timeout_seconds"] == 120


def test_missing_dir():
    config = load_project_skills("/nonexistent/path/")
    assert config.get_build_command("go") is None
