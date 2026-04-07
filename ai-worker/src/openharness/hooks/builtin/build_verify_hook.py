"""BuildVerifyHook — POST_GENERATION hook that runs real compilation.

Reads the build command from project skill config (detect.yaml).
Creates a temp directory, writes generated code, runs the compiler,
returns pass/fail with the full compiler output.
"""

from __future__ import annotations

import asyncio
import logging
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Optional

from ..events import HookEvent
from ..executor import HookResult

logger = logging.getLogger(__name__)


@dataclass
class BuildVerifyResult:
    success: bool
    output: str
    command: str
    duration_ms: int = 0


class BuildVerifyHook:
    """Runs compilation verification after code generation."""

    event = HookEvent.POST_GENERATION

    def __init__(
        self,
        build_command: str,
        timeout_seconds: int = 120,
        max_retries: int = 3,
        cwd: Optional[Path] = None,
    ) -> None:
        self.build_command = build_command
        self.timeout_seconds = timeout_seconds
        self.max_retries = max_retries
        self.cwd = cwd

    async def run(
        self,
        code_files: Dict[str, str],
        project_dir: Optional[Path] = None,
    ) -> BuildVerifyResult:
        """Run build verification on the given code files.

        Args:
            code_files: Dict of {relative_path: content} to write before building.
            project_dir: If provided, write files here. Otherwise use a temp dir.
        """
        work_dir = project_dir or Path(tempfile.mkdtemp(prefix="forge-build-"))
        cleanup = project_dir is None

        try:
            # Write generated files
            for rel_path, content in code_files.items():
                full_path = work_dir / rel_path
                full_path.parent.mkdir(parents=True, exist_ok=True)
                full_path.write_text(content, encoding="utf-8")

            # Run build command
            import time
            start = time.monotonic()

            parts = self.build_command.split()
            try:
                proc = await asyncio.create_subprocess_exec(
                    *parts,
                    stdout=asyncio.subprocess.PIPE,
                    stderr=asyncio.subprocess.PIPE,
                    cwd=str(work_dir),
                )
                stdout, stderr = await asyncio.wait_for(
                    proc.communicate(), timeout=self.timeout_seconds,
                )
            except FileNotFoundError:
                return BuildVerifyResult(
                    success=False,
                    output=f"Build command not found: {parts[0]}. "
                           f"Is the compiler installed?",
                    command=self.build_command,
                )
            except asyncio.TimeoutError:
                return BuildVerifyResult(
                    success=False,
                    output=f"Build timed out after {self.timeout_seconds}s",
                    command=self.build_command,
                )

            duration_ms = int((time.monotonic() - start) * 1000)
            output = stdout.decode(errors="replace") + stderr.decode(errors="replace")
            success = proc.returncode == 0

            return BuildVerifyResult(
                success=success,
                output=output.strip(),
                command=self.build_command,
                duration_ms=duration_ms,
            )

        except OSError as e:
            return BuildVerifyResult(
                success=False,
                output=f"OS error during build: {e}",
                command=self.build_command,
            )
        finally:
            if cleanup:
                try:
                    shutil.rmtree(work_dir, ignore_errors=True)
                except Exception:
                    pass

    def to_hook_result(self, result: BuildVerifyResult) -> HookResult:
        """Convert a BuildVerifyResult to a HookResult for the hook pipeline."""
        return HookResult(
            hook_type="build_verify",
            success=result.success,
            output=result.output,
            blocked=not result.success,
            reason=result.output if not result.success else "",
            metadata={
                "command": result.command,
                "duration_ms": result.duration_ms,
            },
        )
