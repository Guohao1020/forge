from dataclasses import dataclass
from typing import Dict, Optional


@dataclass(frozen=True)
class SkillDefinition:
    name: str
    description: str
    content: str
    source: str
    path: Optional[str] = None
    metadata: Optional[Dict] = None
