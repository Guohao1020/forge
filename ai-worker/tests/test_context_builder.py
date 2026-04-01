from src.context.builder import ProjectContext


def test_system_prompt_assembly():
    ctx = ProjectContext(
        project_name="forge",
        project_description="AI platform",
        tech_stack="Go + Python",
        coding_standards=["## Java Rules\n- Use camelCase"],
        prompt_template_system="You are a senior engineer.",
    )
    prompt = ctx.to_system_prompt()
    assert "You are a senior engineer." in prompt
    assert "Java Rules" in prompt
    assert "forge" in prompt
    assert "Go + Python" in prompt


def test_system_prompt_empty_context():
    ctx = ProjectContext()
    prompt = ctx.to_system_prompt()
    assert prompt == ""


def test_system_prompt_standards_only():
    ctx = ProjectContext(coding_standards=["Rule 1", "Rule 2"])
    prompt = ctx.to_system_prompt()
    assert "Coding Standards" in prompt
    assert "Rule 1" in prompt
    assert "Rule 2" in prompt
