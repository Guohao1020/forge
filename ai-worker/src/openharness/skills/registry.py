from __future__ import annotations

import logging
from typing import Dict, List, Optional

from .types import SkillDefinition

logger = logging.getLogger(__name__)


class SkillRegistry:
    def __init__(self) -> None:
        self._skills: Dict[str, SkillDefinition] = {}

    def register(self, skill: SkillDefinition) -> None:
        if skill.name in self._skills:
            logger.warning(
                "Skill '%s' already registered (source: %s), overwriting with source: %s",
                skill.name, self._skills[skill.name].source, skill.source,
            )
        self._skills[skill.name] = skill

    def get(self, name: str) -> Optional[SkillDefinition]:
        return self._skills.get(name)

    def list_skills(self) -> List[SkillDefinition]:
        return sorted(self._skills.values(), key=lambda s: s.name)
