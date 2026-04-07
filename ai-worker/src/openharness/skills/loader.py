from __future__ import annotations

import re
from pathlib import Path
from typing import Dict, List, Tuple

import yaml

from .registry import SkillRegistry
from .types import SkillDefinition


def parse_skill_markdown(
    default_name: str, content: str,
) -> Tuple[str, str, Dict]:
    metadata: Dict = {}
    body = content
    fm_match = re.match(r"^---\s*\n(.*?)\n---\s*\n", content, re.DOTALL)
    if fm_match:
        try:
            metadata = yaml.safe_load(fm_match.group(1)) or {}
        except yaml.YAMLError:
            pass
        body = content[fm_match.end():]
    name = metadata.get("name", default_name)
    description = metadata.get("description", "")
    if not description:
        h1 = re.search(r"^#\s+(.+)$", body, re.MULTILINE)
        if h1:
            description = h1.group(1).strip()
    return name, description, metadata


def load_skills_from_dir(
    directory: str, namespace: str = "forge",
) -> List[SkillDefinition]:
    d = Path(directory)
    if not d.exists():
        return []
    skills: List[SkillDefinition] = []
    for f in sorted(d.glob("*.md")):
        content = f.read_text(encoding="utf-8")
        name, desc, metadata = parse_skill_markdown(f.stem, content)
        # Add namespace prefix if not already present
        if ":" not in name:
            name = f"{namespace}:{name}"
        skills.append(SkillDefinition(
            name=name, description=desc, content=content,
            source="file", path=str(f), metadata=metadata,
        ))
    return skills


def load_skill_registry(
    skills_dir: str = "skills/", namespace: str = "forge",
) -> SkillRegistry:
    registry = SkillRegistry()
    for skill in load_skills_from_dir(skills_dir, namespace=namespace):
        registry.register(skill)
    return registry
