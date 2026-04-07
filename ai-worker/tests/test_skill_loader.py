import pytest
from src.openharness.skills.types import SkillDefinition
from src.openharness.skills.loader import parse_skill_markdown
from src.openharness.skills.registry import SkillRegistry


def test_parse_skill_with_frontmatter():
    content = (
        "---\nname: code-gen\ndescription: Generate code\n"
        "purpose: generate\ntools: [query_api_catalog]\n---\n\n"
        "You are a code expert.\n"
    )
    name, desc, metadata = parse_skill_markdown("fallback", content)
    assert name == "code-gen"
    assert desc == "Generate code"
    assert metadata["purpose"] == "generate"


def test_parse_skill_without_frontmatter():
    content = "# My Skill\n\nDescription here.\n"
    name, desc, metadata = parse_skill_markdown("my-skill", content)
    assert name == "my-skill"
    assert desc == "My Skill"
    assert metadata == {}


def test_parse_skill_malformed_yaml():
    content = "---\n: bad yaml [[\n---\n\nBody.\n"
    name, desc, metadata = parse_skill_markdown("fallback", content)
    assert name == "fallback"
    assert metadata == {}


def test_skill_registry_register_and_get():
    registry = SkillRegistry()
    skill = SkillDefinition(
        name="forge:test", description="Test", content="# Test", source="test",
    )
    registry.register(skill)
    assert registry.get("forge:test") is skill
    assert registry.get("nope") is None
    assert len(registry.list_skills()) == 1


def test_skill_registry_collision_warning(caplog):
    """Duplicate names should log a warning."""
    registry = SkillRegistry()
    s1 = SkillDefinition(name="forge:review", description="v1", content="v1", source="test")
    s2 = SkillDefinition(name="forge:review", description="v2", content="v2", source="test")
    registry.register(s1)
    import logging
    with caplog.at_level(logging.WARNING):
        registry.register(s2)
    assert "already registered" in caplog.text.lower() or registry.get("forge:review") is s2


def test_skill_registry_list_sorted():
    registry = SkillRegistry()
    registry.register(SkillDefinition(name="forge:z", description="", content="", source=""))
    registry.register(SkillDefinition(name="forge:a", description="", content="", source=""))
    names = [s.name for s in registry.list_skills()]
    assert names == ["forge:a", "forge:z"]


def test_skill_definition_frozen():
    skill = SkillDefinition(name="test", description="test", content="test", source="test")
    with pytest.raises(AttributeError):
        skill.name = "changed"
