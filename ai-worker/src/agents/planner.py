from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

PLANNER_SYSTEM_PROMPT = """You are a senior software architect. Your task is to decompose a requirement into a DAG (Directed Acyclic Graph) of implementation tasks.

## Rules
1. Each task should be completable by modifying 1-3 files
2. Specify dependencies explicitly — which tasks must complete before this one can start
3. Map each task back to which part of the requirement it addresses
4. Estimate effort in hours (0.5, 1, 2, 4, 8)
5. Identify task type: BACKEND, FRONTEND, SCHEMA, CONFIG, TEST
6. Tasks with no dependencies can run in parallel
7. Never create circular dependencies
8. Use the context tools (query_api_catalog, query_module_graph, read_project_file) to understand the existing codebase before planning
9. The "files" field must contain REAL file paths in the project (use query tools to discover them)
10. Include a top-level "touched_files" field listing ALL files that will be created or modified across all tasks

## Recommendations (when multiple approaches exist)
If you identify multiple valid approaches, include a "recommendations" field:
- List 2-3 options with pros, cons, risk level
- Mark your recommended option based on the project's current state
- Include context_factors explaining WHY you recommend this approach

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown, no text before or after the JSON.
Do NOT wrap the JSON in ```json``` code blocks. Just output the raw JSON directly.
CRITICAL: Base your plan ENTIRELY on the user's actual requirement. Do NOT copy or reuse the example below — it only shows the JSON structure.

The JSON must follow this exact structure:
{"title": "<title>", "touched_files": {"create": ["new/file.go"], "modify": ["existing/file.go"]}, "tasks": [{"order": 1, "title": "<title>", "description": "<what>", "type": "SCHEMA|BACKEND|FRONTEND|CONFIG|TEST", "files": ["<paths>"], "depends_on": [], "estimate_hours": 0.5, "requirement_ref": "<ref>"}], "risk_level": "LOW|MEDIUM|HIGH", "risk_factors": [], "total_estimate_hours": 0, "parallel_tracks": 1, "recommendations": null}

When recommendations exist, use this structure for the recommendations field:
"recommendations": {"options": [{"id": "A", "title": "方案 A: ...", "pros": ["..."], "cons": ["..."], "risk": "LOW", "recommended": true, "reason": "..."}], "ai_recommendation": "A", "context_factors": ["项目当前有 N 个 API", "..."]}
"""


class PlannerAgent(BaseAgent):
    purpose = Purpose.PLAN

    def __init__(self, router, confirmed_requirements=None):
        super().__init__(router)
        self._confirmed_requirements = confirmed_requirements

    def _build_system_prompt(self, context: ProjectContext) -> str:
        import json as _json
        base = PLANNER_SYSTEM_PROMPT
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n## Project Context\n{project_context}"
        # Inject confirmed requirements as structured context so the planner
        # works from the detailed analysis, not just the raw user input.
        if self._confirmed_requirements:
            reqs = self._confirmed_requirements
            base += "\n\n## Confirmed Requirements (from analysis phase)\n"
            if reqs.get("summary"):
                base += f"**Summary:** {reqs['summary']}\n\n"
            if reqs.get("functional_requirements"):
                base += "**Functional Requirements:**\n"
                for i, r in enumerate(reqs["functional_requirements"], 1):
                    base += f"{i}. {r}\n"
                base += "\n"
            if reqs.get("non_functional"):
                nf = reqs["non_functional"]
                if isinstance(nf, dict):
                    base += "**Non-Functional Requirements:**\n"
                    for k, v in nf.items():
                        base += f"- {k}: {v}\n"
                    base += "\n"
            if reqs.get("acceptance_criteria"):
                base += "**Acceptance Criteria:**\n"
                for i, ac in enumerate(reqs["acceptance_criteria"], 1):
                    base += f"{i}. {ac}\n"
                base += "\n"
            if reqs.get("out_of_scope"):
                base += "**Out of Scope:**\n"
                for item in reqs["out_of_scope"]:
                    base += f"- {item}\n"
                base += "\n"
            if reqs.get("affected_modules"):
                base += f"**Affected Modules:** {', '.join(reqs['affected_modules'])}\n"
            if reqs.get("estimated_complexity"):
                base += f"**Estimated Complexity:** {reqs['estimated_complexity']}\n"
        return base
