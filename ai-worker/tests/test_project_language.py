"""Unit tests for project_language detection + command routing."""

from __future__ import annotations

import textwrap
from pathlib import Path

import pytest

from src.openharness.skills.project_language import (
    CommandSpec,
    LanguageProfile,
    detect_language,
    load_all_language_profiles,
    load_language_profile,
)


# ---- YAML loading ---------------------------------------------------------


# Skills dir resolution: pytest runs from repo root OR from ai-worker/,
# depending on how the test is invoked. Resolve relative to this test
# file so both paths work.
_SKILLS_DIR = (Path(__file__).parent.parent / "skills" / "languages").resolve()


def test_load_all_language_profiles_finds_the_five_defaults():
    profiles = load_all_language_profiles(str(_SKILLS_DIR))
    assert set(profiles.keys()) == {
        "java-project",
        "python-project",
        "go-project",
        "node-project",
        "rust-project",
    }


def test_java_profile_has_three_build_commands():
    profiles = load_all_language_profiles(str(_SKILLS_DIR))
    java = profiles["java-project"]
    assert len(java.build_commands) == 3
    # Marker order: pom.xml first, both Gradle variants after
    assert java.build_commands[0].marker == "pom.xml"
    assert java.build_commands[0].command == "mvn compile -q"


def test_load_language_profile_returns_none_for_malformed_yaml(tmp_path):
    bad = tmp_path / "bad.yaml"
    bad.write_text("{this is not: valid: yaml:", encoding="utf-8")
    assert load_language_profile(bad) is None


def test_load_language_profile_returns_none_for_non_dict_yaml(tmp_path):
    bad = tmp_path / "empty.yaml"
    bad.write_text("- just a list", encoding="utf-8")
    assert load_language_profile(bad) is None


# ---- LanguageProfile.matches ---------------------------------------------


def test_matches_via_marker_file(tmp_path):
    (tmp_path / "pom.xml").write_text("<project/>", encoding="utf-8")
    profile = LanguageProfile(
        name="java-project",
        description="",
        detect_files=["pom.xml"],
    )
    assert profile.matches(tmp_path) is True


def test_matches_via_extension_scan(tmp_path):
    (tmp_path / "main.go").write_text("package main", encoding="utf-8")
    profile = LanguageProfile(
        name="go-project",
        description="",
        detect_extensions=[".go"],
    )
    assert profile.matches(tmp_path) is True


def test_does_not_match_empty_directory(tmp_path):
    profile = LanguageProfile(
        name="rust-project",
        description="",
        detect_files=["Cargo.toml"],
        detect_extensions=[".rs"],
    )
    assert profile.matches(tmp_path) is False


# ---- Command routing ------------------------------------------------------


def test_build_command_for_picks_first_matching_marker(tmp_path):
    (tmp_path / "build.gradle").write_text("plugins {}", encoding="utf-8")
    profile = LanguageProfile(
        name="java-project",
        description="",
        build_commands=[
            CommandSpec(marker="pom.xml", command="mvn compile -q"),
            CommandSpec(marker="build.gradle", command="./gradlew compileJava --quiet"),
        ],
    )
    assert profile.build_command_for(tmp_path) == "./gradlew compileJava --quiet"


def test_build_command_for_falls_back_to_first_spec(tmp_path):
    profile = LanguageProfile(
        name="rust-project",
        description="",
        build_commands=[
            CommandSpec(marker="Cargo.toml", command="cargo build --quiet"),
        ],
    )
    # No Cargo.toml in tmp_path, but the spec list is non-empty so we
    # still return the first command as a best-effort default.
    assert profile.build_command_for(tmp_path) == "cargo build --quiet"


def test_build_command_for_returns_none_when_no_specs():
    profile = LanguageProfile(name="empty", description="")
    assert profile.build_command_for(Path(".")) is None


# ---- Top-level detect_language integration --------------------------------


@pytest.fixture
def real_profiles():
    return load_all_language_profiles(str(_SKILLS_DIR))


def test_detect_language_java_via_pom_xml(tmp_path, real_profiles):
    (tmp_path / "pom.xml").write_text("<project/>", encoding="utf-8")
    profile = detect_language(tmp_path, real_profiles)
    assert profile is not None
    assert profile.name == "java-project"


def test_detect_language_python_via_pyproject(tmp_path, real_profiles):
    (tmp_path / "pyproject.toml").write_text(
        "[project]\nname = 'x'", encoding="utf-8"
    )
    profile = detect_language(tmp_path, real_profiles)
    assert profile is not None
    assert profile.name == "python-project"


def test_detect_language_go_via_go_mod(tmp_path, real_profiles):
    (tmp_path / "go.mod").write_text("module x\n\ngo 1.22", encoding="utf-8")
    profile = detect_language(tmp_path, real_profiles)
    assert profile is not None
    assert profile.name == "go-project"


def test_detect_language_node_via_package_json(tmp_path, real_profiles):
    (tmp_path / "package.json").write_text('{"name":"x"}', encoding="utf-8")
    profile = detect_language(tmp_path, real_profiles)
    assert profile is not None
    assert profile.name == "node-project"


def test_detect_language_rust_via_cargo_toml(tmp_path, real_profiles):
    (tmp_path / "Cargo.toml").write_text(
        '[package]\nname = "x"', encoding="utf-8"
    )
    profile = detect_language(tmp_path, real_profiles)
    assert profile is not None
    assert profile.name == "rust-project"


def test_detect_language_empty_directory_returns_none(tmp_path, real_profiles):
    profile = detect_language(tmp_path, real_profiles)
    assert profile is None


def test_detect_language_missing_directory_returns_none(real_profiles):
    profile = detect_language(Path("/does/not/exist"), real_profiles)
    assert profile is None


def test_detect_language_build_command_end_to_end(tmp_path, real_profiles):
    """End-to-end: write a go.mod, detect language, get the right
    build command for BuildVerifyHook to run."""
    (tmp_path / "go.mod").write_text("module x\n\ngo 1.22", encoding="utf-8")
    profile = detect_language(tmp_path, real_profiles)
    assert profile is not None
    cmd = profile.build_command_for(tmp_path)
    assert cmd == "go build ./..."
