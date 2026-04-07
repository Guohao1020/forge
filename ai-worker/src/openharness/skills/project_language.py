"""Project language detection + build command routing.

Loads YAML language profiles from `skills/languages/*.yaml` and
provides a detector that returns the right build/test/lint commands
for a given project working directory. The detection runs on project
profile scans and on every pair_pipeline cycle so generated code is
verified with the correct toolchain.

Profile shape (see skills/languages/java-project.yaml for an example):

    name: java-project
    detect:
      files: [pom.xml, build.gradle]
      extensions: [.java]
    build:
      commands:
        - marker: pom.xml
          command: mvn compile -q
        - marker: build.gradle
          command: ./gradlew compileJava --quiet
      timeout_seconds: 180
"""

from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

import yaml

logger = logging.getLogger(__name__)


@dataclass
class CommandSpec:
    """A single build/test/lint command with the marker file that
    enables it."""
    marker: str
    command: str


@dataclass
class LanguageProfile:
    """Language-specific build/test/lint configuration loaded from a
    YAML file under skills/languages/."""
    name: str
    description: str
    detect_files: List[str] = field(default_factory=list)
    detect_extensions: List[str] = field(default_factory=list)
    build_commands: List[CommandSpec] = field(default_factory=list)
    build_timeout: int = 120
    test_commands: List[CommandSpec] = field(default_factory=list)
    test_timeout: int = 300
    lint_commands: List[CommandSpec] = field(default_factory=list)
    lint_timeout: int = 120

    def matches(self, project_dir: Path) -> bool:
        """True if any detect marker file exists in project_dir."""
        for f in self.detect_files:
            if (project_dir / f).exists():
                return True
        # Fall back to extension scan — slower but catches projects
        # without any build file (e.g. ad-hoc Python scripts).
        if self.detect_extensions:
            for ext in self.detect_extensions:
                if any(project_dir.rglob(f"*{ext}")):
                    return True
        return False

    def build_command_for(self, project_dir: Path) -> Optional[str]:
        """Return the first build command whose marker exists in the
        project dir. None if no marker matched (e.g. detection was
        extension-only)."""
        return _first_matching(self.build_commands, project_dir)

    def test_command_for(self, project_dir: Path) -> Optional[str]:
        return _first_matching(self.test_commands, project_dir)

    def lint_command_for(self, project_dir: Path) -> Optional[str]:
        return _first_matching(self.lint_commands, project_dir)


def _first_matching(
    specs: List[CommandSpec], project_dir: Path
) -> Optional[str]:
    for spec in specs:
        if (project_dir / spec.marker).exists():
            return spec.command
    # No marker matched — return the first spec if any (caller's
    # detect_language already promised this language matched).
    if specs:
        return specs[0].command
    return None


def _parse_commands(raw: Optional[dict]) -> List[CommandSpec]:
    if not raw:
        return []
    commands = raw.get("commands") or []
    out: List[CommandSpec] = []
    for c in commands:
        if isinstance(c, dict) and "command" in c:
            out.append(CommandSpec(
                marker=str(c.get("marker", "")),
                command=str(c["command"]),
            ))
    return out


def load_language_profile(yaml_path: Path) -> Optional[LanguageProfile]:
    """Load a single language profile from a YAML file. Returns None
    if the file is malformed so the caller can skip it and move on."""
    try:
        raw = yaml.safe_load(yaml_path.read_text(encoding="utf-8"))
    except Exception as e:
        logger.warning("failed to load language profile %s: %s", yaml_path, e)
        return None
    if not isinstance(raw, dict):
        return None

    detect = raw.get("detect") or {}
    build = raw.get("build") or {}
    test = raw.get("test") or {}
    lint = raw.get("lint") or {}

    return LanguageProfile(
        name=str(raw.get("name") or yaml_path.stem),
        description=str(raw.get("description") or ""),
        detect_files=list(detect.get("files") or []),
        detect_extensions=list(detect.get("extensions") or []),
        build_commands=_parse_commands(build),
        build_timeout=int(build.get("timeout_seconds") or 120),
        test_commands=_parse_commands(test),
        test_timeout=int(test.get("timeout_seconds") or 300),
        lint_commands=_parse_commands(lint),
        lint_timeout=int(lint.get("timeout_seconds") or 120),
    )


def load_all_language_profiles(
    skills_dir: str = "skills/languages",
) -> Dict[str, LanguageProfile]:
    """Load every YAML file under skills/languages/. Returns a name ->
    profile map. Missing directory returns an empty dict so callers
    don't crash in dev."""
    out: Dict[str, LanguageProfile] = {}
    d = Path(skills_dir)
    if not d.exists():
        logger.debug("language profiles dir %s not found", skills_dir)
        return out
    for f in sorted(d.glob("*.yaml")):
        profile = load_language_profile(f)
        if profile is not None:
            out[profile.name] = profile
    return out


def detect_language(
    project_dir: Path,
    profiles: Dict[str, LanguageProfile],
) -> Optional[LanguageProfile]:
    """Return the first language profile whose detect markers match
    the project dir. Priority order is the sorted file name of the
    profile YAML (so java-project.yaml before node-project.yaml).

    Caller is responsible for project_dir existence; we silently
    return None on any OS error so the ai-worker engine can still run
    against a missing working directory during unit tests."""
    if not project_dir.exists():
        return None
    for profile in profiles.values():
        try:
            if profile.matches(project_dir):
                return profile
        except OSError:
            continue
    return None
