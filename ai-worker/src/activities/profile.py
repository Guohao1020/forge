import json
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

import httpx
from temporalio import activity

from src.agents.profiler import ProfilerAgent
from src.config import settings
from src.context.builder import ContextBuilder, ProjectContext
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)

ALL_DIMENSIONS = [
    "api_catalog",
    "db_schema",
    "module_graph",
    "architecture",
    "business_rules",
]

# File selection patterns per dimension
DIMENSION_FILE_PATTERNS: Dict[str, List[str]] = {
    "api_catalog": [
        "router.go", "handler.go", "controller", "routes",
        "router.ts", "controller.ts", "api.go",
    ],
    "db_schema": [
        "migration", "schema", "model.go", "models.go",
        "entity", ".sql", "model.ts", "prisma",
    ],
    "module_graph": [
        "go.mod", "go.sum", "package.json", "import",
        "main.go", "cmd/", "internal/",
    ],
    "architecture": [
        "main.go", "main.ts", "docker-compose", "Dockerfile",
        "config.go", "config.ts", ".env.example", "Makefile",
        "cmd/", "app.go", "server.go",
    ],
    "business_rules": [
        "service.go", "service.ts", "usecase", "domain",
        "validator", "middleware", "policy", "rule",
    ],
}

# Max files to fetch per dimension to stay within token budget
MAX_FILES_PER_DIMENSION = 15
MAX_FILE_SIZE = 50_000  # 50KB per file


@dataclass
class ScanProfileInput:
    project_id: int
    user_id: int
    keys: Optional[List[str]] = None  # Dimensions to scan; None = all


@dataclass
class ScanProfileOutput:
    results: Dict[str, Any] = field(default_factory=dict)
    errors: Dict[str, str] = field(default_factory=dict)
    dimensions_scanned: int = 0
    dimensions_failed: int = 0


def _matches_dimension(file_path: str, patterns: List[str]) -> bool:
    """Check if a file path matches any pattern for a dimension."""
    path_lower = file_path.lower()
    for pattern in patterns:
        if pattern.lower() in path_lower:
            return True
    return False


def _select_files_for_dimension(
    file_tree: List[Dict[str, Any]], dimension: str
) -> List[str]:
    """Select relevant files from the file tree for a given dimension."""
    patterns = DIMENSION_FILE_PATTERNS.get(dimension, [])
    if not patterns:
        return []

    matched = []
    for entry in file_tree:
        # Handle both dict ({"path": "..."}) and string ("...") entries
        if isinstance(entry, dict):
            path = entry.get("path", entry.get("name", ""))
        else:
            path = str(entry)
        # Skip non-code files
        if any(
            path.endswith(ext)
            for ext in [".png", ".jpg", ".gif", ".svg", ".ico", ".woff", ".woff2",
                        ".ttf", ".eot", ".mp4", ".mp3", ".zip", ".tar", ".gz",
                        ".lock", ".sum"]
        ):
            continue
        if _matches_dimension(path, patterns):
            matched.append(path)

    return matched[:MAX_FILES_PER_DIMENSION]


async def _fetch_file_tree(client: httpx.AsyncClient, project_id: int) -> List[Dict[str, Any]]:
    """Fetch the file tree from forge-core API."""
    resp = await client.get(f"/api/projects/{project_id}/code/tree")
    if resp.status_code != 200:
        logger.warning(f"Failed to fetch file tree: status={resp.status_code}")
        return []
    data = resp.json().get("data", {})
    # data may be a list directly or have a "files" key
    if isinstance(data, list):
        return data
    return data.get("files", data.get("tree", []))


async def _fetch_file_content(
    client: httpx.AsyncClient, project_id: int, path: str
) -> str:
    """Fetch a single file's content from forge-core API."""
    resp = await client.get(
        f"/api/projects/{project_id}/code/file",
        params={"path": path},
    )
    if resp.status_code != 200:
        return ""
    data = resp.json().get("data", {})
    content = data.get("content", "")
    # Truncate very large files
    if len(content) > MAX_FILE_SIZE:
        content = content[:MAX_FILE_SIZE] + "\n... (truncated)"
    return content


async def _save_profile(
    client: httpx.AsyncClient,
    project_id: int,
    key: str,
    value: dict,
) -> bool:
    """Save a profile dimension back to forge-core API via PUT upsert."""
    try:
        resp = await client.put(
            f"/api/projects/{project_id}/profiles/{key}",
            json=value,  # Send raw value, handler expects json.RawMessage body
        )
        if resp.status_code in (200, 201):
            logger.info(f"Profile {key} saved for project {project_id}")
            return True
        logger.warning(f"Profile save failed: {key} status={resp.status_code}")
        return False
    except Exception as e:
        logger.warning(f"Failed to save profile {key}: {e}")
        return False


@activity.defn(name="scan_project_profile")
async def scan_project_profile_activity(input: ScanProfileInput) -> ScanProfileOutput:
    """Scan a project's codebase and extract structured profiles per dimension."""
    logger.info(
        f"Starting profile scan for project {input.project_id}, "
        f"dimensions: {input.keys or 'all'}"
    )

    dimensions = input.keys or ALL_DIMENSIONS
    output = ScanProfileOutput()

    client = httpx.AsyncClient(
        base_url=settings.forge_api_url,
        headers={"Authorization": f"Bearer {settings.forge_api_token}"},
        timeout=30.0,
    )

    try:
        # Step 1: Fetch file tree
        file_tree = await _fetch_file_tree(client, input.project_id)
        if not file_tree:
            logger.warning(f"Empty file tree for project {input.project_id}")
            output.errors["_general"] = "Could not fetch file tree from repository"
            return output

        # Build a minimal context for the agent
        builder = ContextBuilder()
        try:
            ctx = await builder.build(
                project_id=input.project_id,
                purpose="profile-scan",
            )
        finally:
            await builder.close()

        router = ModelRouter()

        # Step 2: Process each dimension
        for dimension in dimensions:
            if dimension not in DIMENSION_FILE_PATTERNS:
                logger.warning(f"Unknown dimension: {dimension}, skipping")
                output.errors[dimension] = f"Unknown dimension: {dimension}"
                output.dimensions_failed += 1
                continue

            try:
                # Select relevant files
                selected_files = _select_files_for_dimension(file_tree, dimension)
                if not selected_files:
                    logger.info(f"No matching files for dimension {dimension}")
                    output.errors[dimension] = "No matching files found"
                    output.dimensions_failed += 1
                    continue

                # Fetch file contents
                file_contents = []
                for path in selected_files:
                    content = await _fetch_file_content(client, input.project_id, path)
                    if content:
                        file_contents.append(f"### File: {path}\n```\n{content}\n```")

                if not file_contents:
                    logger.info(f"No file contents retrieved for dimension {dimension}")
                    output.errors[dimension] = "Could not read file contents"
                    output.dimensions_failed += 1
                    continue

                # Build user prompt with file contents
                user_prompt = (
                    f"Analyze the following source code files and extract "
                    f"structured {dimension} information.\n\n"
                    + "\n\n".join(file_contents)
                )

                # Run ProfilerAgent
                agent = ProfilerAgent(router, dimension=dimension)
                result = await agent.run(user_prompt, ctx)

                if result.structured:
                    output.results[dimension] = result.structured
                    output.dimensions_scanned += 1

                    # Save back to forge-core
                    await _save_profile(
                        client, input.project_id, dimension, result.structured
                    )

                    logger.info(
                        f"Dimension {dimension} scanned successfully "
                        f"(model={result.model}, tokens={result.tokens_used})"
                    )
                else:
                    output.errors[dimension] = "AI returned empty structured data"
                    output.dimensions_failed += 1
                    logger.warning(f"Empty result for dimension {dimension}")

            except Exception as e:
                logger.error(f"Failed to scan dimension {dimension}: {e}", exc_info=True)
                output.errors[dimension] = str(e)
                output.dimensions_failed += 1

    finally:
        await client.aclose()

    logger.info(
        f"Profile scan complete: {output.dimensions_scanned} scanned, "
        f"{output.dimensions_failed} failed"
    )
    return output
