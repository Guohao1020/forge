"""BuildVerifyHook — POST_GENERATION hook that runs real compilation.

Reads the build command from project skill config (detect.yaml).
Creates a temp directory, writes generated code, runs the compiler,
returns pass/fail with the full compiler output.

Preflight (TASK 8 of agent-base-loop-reduction): before invoking the
subprocess we sanity-check that the work directory looks plausible for
the chosen build command. The flagship check is "if the build command
starts with `go`, the work directory MUST contain go.mod". Without
this guard, a misconfigured pipeline (e.g. detection went wrong, or
the LLM emitted code into the wrong subdir) silently runs `go build`
in an empty dir and surfaces opaque "no Go files in /tmp/foo" errors
that look like LLM mistakes. The preflight catches this class of
operator error early and reports it as an explicit
`PreflightFailed` outcome.
"""

from __future__ import annotations

import asyncio
import logging
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

from ..events import HookEvent
from ..executor import HookResult

logger = logging.getLogger(__name__)


# Marker files we expect to see when a given build tool runs. The
# preflight check below uses this map to refuse running `go build` in
# a directory without go.mod, `mvn` without pom.xml, etc. Plan TASK 8
# explicitly requires the go entry; the others are zero-cost coverage
# for the same class of bug across the rest of the language profiles.
_BUILD_MARKERS: Dict[str, Tuple[str, ...]] = {
    "go": ("go.mod",),
    "mvn": ("pom.xml",),
    "gradle": ("build.gradle", "build.gradle.kts", "settings.gradle"),
    "./gradlew": ("build.gradle", "build.gradle.kts", "settings.gradle"),
    "npm": ("package.json",),
    "yarn": ("package.json",),
    "pnpm": ("package.json",),
    "cargo": ("Cargo.toml",),
}


def _required_markers_for(build_command: str) -> Tuple[str, ...]:
    """Return the marker files a given build_command needs in its work
    directory, or () if we don't recognize the tool (in which case the
    preflight is a no-op and we let the subprocess speak for itself)."""
    if not build_command:
        return ()
    first = build_command.strip().split()[0]
    return _BUILD_MARKERS.get(first, ())


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
            # Preflight: refuse to launch the build tool if its required
            # marker is missing from the work dir (after the LLM-generated
            # files have been written, in case the LLM emitted the marker
            # itself). Plan TASK 8 / G4: catches misconfigured pipelines
            # that would otherwise produce opaque tool errors.
            #
            # The check runs against the union of (existing files in
            # work_dir) ∪ (files we are about to write), so an LLM that
            # emits go.mod alongside main.go is allowed through.
            required = _required_markers_for(self.build_command)
            if required:
                generated_paths = set(code_files.keys())
                marker_satisfied = False
                for marker in required:
                    if (work_dir / marker).exists() or marker in generated_paths:
                        marker_satisfied = True
                        break
                if not marker_satisfied:
                    msg = (
                        f"BuildVerify preflight failed: build command "
                        f"'{self.build_command}' requires one of "
                        f"{list(required)} in {work_dir}, but none were "
                        f"found and the generated files do not include any. "
                        f"Pipeline misconfigured — language detection may "
                        f"have returned the wrong toolchain, or the work "
                        f"directory points at the wrong place."
                    )
                    logger.warning(msg)
                    return BuildVerifyResult(
                        success=False,
                        output=msg,
                        command=self.build_command,
                    )

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
