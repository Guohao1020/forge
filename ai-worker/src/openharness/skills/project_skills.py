"""ProjectSkillLoader — loads platform skill YAML configs for project-specific context."""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any, Dict, Optional

import yaml

logger = logging.getLogger(__name__)


class ProjectSkillConfig:
    """Holds parsed platform skill configuration for a project."""

    def __init__(self, detect: Dict, dockerfile: Dict, agent_loop: Dict) -> None:
        self.detect = detect
        self.dockerfile = dockerfile
        self.agent_loop = agent_loop

    def get_build_command(self, language: str) -> Optional[str]:
        for rule in self.detect.get("rules", []):
            if rule.get("language") == language:
                return rule.get("build_command")
        return None

    def get_test_command(self, language: str) -> Optional[str]:
        for rule in self.detect.get("rules", []):
            if rule.get("language") == language:
                return rule.get("test_command")
        return None

    def get_lint_command(self, language: str) -> Optional[str]:
        for rule in self.detect.get("rules", []):
            if rule.get("language") == language:
                return rule.get("lint_command")
        return None

    def get_dockerfile_template(self, language: str) -> Optional[Dict[str, str]]:
        return self.dockerfile.get("templates", {}).get(language)

    def get_agent_loop_params(self, purpose: str) -> Dict[str, Any]:
        defaults = dict(self.agent_loop.get("defaults", {}))
        overrides = self.agent_loop.get("purpose_overrides", {}).get(purpose, {})
        defaults.update(overrides)
        return defaults

    def get_build_verify_config(self) -> Dict[str, Any]:
        return dict(self.agent_loop.get("build_verify", {
            "max_retries": 3,
            "timeout_seconds": 120,
            "cleanup_temp": True,
        }))


def _load_yaml(path: Path) -> Dict:
    if not path.exists():
        logger.warning("Skill YAML not found: %s", path)
        return {}
    try:
        return yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    except yaml.YAMLError as e:
        logger.error("Failed to parse %s: %s", path, e)
        return {}


def load_project_skills(skills_dir: str = "skills/") -> ProjectSkillConfig:
    d = Path(skills_dir)
    return ProjectSkillConfig(
        detect=_load_yaml(d / "detect.yaml"),
        dockerfile=_load_yaml(d / "dockerfile.yaml"),
        agent_loop=_load_yaml(d / "agent-loop.yaml"),
    )
