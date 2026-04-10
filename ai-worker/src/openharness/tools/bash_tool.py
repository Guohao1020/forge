"""BashTool -- sandboxed shell execution via bubblewrap.

This is the highest-risk tool in the T2 set. The only things
standing between the agent and the host filesystem are:
  1. bubblewrap's namespace isolation (--unshare-all, workspace
     as the only read-write bind)
  2. an environment whitelist (parent env is filtered; only safe
     vars are forwarded)
  3. output caps and timeout with process-group kill
  4. a cheap regex denylist as a UX hint (NOT a security boundary)

Layers 1-3 are the actual security. Layer 4 just gives the agent
a clean error message for common mistakes (sudo, apt install,
curl | sh) instead of letting bwrap produce a cryptic failure
after spawning the subprocess.

The tool yields ThinkingStarted before the subprocess and
ThinkingStopped in a try/finally so the frontend pulses an
indicator even if the command crashes. Both are StreamEvents
passed verbatim through the agent loop to the frontend.
"""

from __future__ import annotations

import asyncio
import logging
import os
import re
import shutil
import signal
from pathlib import Path
from typing import List, Optional, Tuple

from pydantic import BaseModel, Field

from ..engine.stream_events import ThinkingStarted, ThinkingStopped
from .base import BaseTool, ToolExecutionContext, ToolResult

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

BASH_OUTPUT_CAP_BYTES = 100_000
BASH_DEFAULT_TIMEOUT = 120
BASH_MAX_TIMEOUT = 600


# Intent denylist. NOT a security boundary -- bubblewrap is.
# This exists so the agent gets a clean error message for common
# mistakes instead of a cryptic sandbox failure. Bypassing the
# denylist (e.g., via env-var indirection) succeeds at the regex
# layer but the resulting command still fails inside bwrap.
_INTENT_DENYLIST = [
    (re.compile(r"\bsudo\b"), "sudo not available in sandbox"),
    (re.compile(r"\bapt(-get)?\s+install\b"), "cannot install packages (no network, dependencies are pre-installed)"),
    (re.compile(r"\bnpm\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\byarn\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bpnpm\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bpip\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bgo\s+mod\s+download\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bsystemctl\b"), "systemctl not available"),
    (re.compile(r"curl\s+[^|]*\|\s*(bash|sh)\b"), "piping curl to shell not allowed"),
    (re.compile(r"wget\s+[^|]*\|\s*(bash|sh)\b"), "piping wget to shell not allowed"),
]


# ---------------------------------------------------------------------------
# Input model
# ---------------------------------------------------------------------------


class BashInput(BaseModel):
    command: str = Field(
        ...,
        description="Shell command to execute. Run in a sandboxed bash -c context.",
    )
    timeout: int = Field(
        BASH_DEFAULT_TIMEOUT,
        description=(
            f"Timeout in seconds. Default {BASH_DEFAULT_TIMEOUT}, "
            f"max {BASH_MAX_TIMEOUT}. Process group is SIGKILLed on timeout."
        ),
        ge=1,
        le=BASH_MAX_TIMEOUT,
    )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _intent_denylist_check(command: str) -> Optional[str]:
    """Return a reason string if the command matches the denylist,
    else None. Purely a UX filter -- bwrap is the security layer."""
    for pattern, reason in _INTENT_DENYLIST:
        if pattern.search(command):
            return reason
    return None


_KNOWN_TOOL_LABELS = {
    "mvn": "Running maven",
    "gradle": "Running gradle",
    "npm": "Running npm",
    "yarn": "Running yarn",
    "pnpm": "Running pnpm",
    "pytest": "Running tests",
    "jest": "Running tests",
    "cargo": "Running cargo",
    "make": "Running make",
    "ctest": "Running ctest",
    "python": "Running python",
    "node": "Running node",
    "ruff": "Running ruff",
    "black": "Running black",
    "eslint": "Running eslint",
    "tsc": "Running tsc",
    "gofmt": "Running gofmt",
    "rustfmt": "Running rustfmt",
}


def _summarize_command(command: str) -> str:
    """Produce a short 'Running <thing>' label for ThinkingStarted.

    Recognizes common build/test tools and produces a friendly
    label. Falls back to a truncated raw command otherwise.
    """
    stripped = command.strip()
    if not stripped:
        return "Running empty command"

    tokens = stripped.split()
    first = tokens[0]

    # go build, go test, go vet
    if first == "go" and len(tokens) >= 2:
        return f"Running go {tokens[1]}"

    if first in _KNOWN_TOOL_LABELS:
        return _KNOWN_TOOL_LABELS[first]

    # Fallback: truncate raw
    trimmed = stripped
    if len(trimmed) > 60:
        trimmed = trimmed[:57] + "..."
    return f"Running {trimmed}"


def _build_bwrap_args(workspace: Path, command: str) -> List[str]:
    """Construct the full bwrap argv for a sandboxed bash -c invocation.

    The resulting list is passed to asyncio.create_subprocess_exec.
    Layout:
      - unshare ALL namespaces (including network, pid, ipc, mount, etc.)
      - bind /usr, /lib, /lib64, /bin, /sbin, /etc/ssl read-only
      - bind the workspace directory read-write (the ONLY rw bind)
      - chdir to workspace
      - proc, dev, and /tmp tmpfs
      - environment whitelist only
      - die-with-parent so the sandbox exits if ai-worker crashes

    DO NOT add --share-net. --share-net is a toggle that re-enables
    network after --unshare-all; writing --share-net=false is
    invalid bwrap syntax. Omitting --share-net entirely is the
    correct way to keep network off.
    """
    ws_abs = str(workspace.resolve())
    return [
        "bwrap",
        "--unshare-all",
        "--die-with-parent",
        "--ro-bind", "/usr", "/usr",
        "--ro-bind", "/lib", "/lib",
        "--ro-bind", "/lib64", "/lib64",
        "--ro-bind", "/bin", "/bin",
        "--ro-bind", "/sbin", "/sbin",
        "--ro-bind", "/etc/ssl", "/etc/ssl",
        "--proc", "/proc",
        "--dev", "/dev",
        "--tmpfs", "/tmp",
        "--bind", ws_abs, ws_abs,
        "--chdir", ws_abs,
        "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin",
        "--setenv", "HOME", "/tmp",
        "--setenv", "LANG", "C.UTF-8",
        "--setenv", "LC_ALL", "C.UTF-8",
        # Language-specific cache vars. Safe to always set -- unused
        # by non-Go/Python processes.
        "--setenv", "GOCACHE", "/tmp/gocache",
        "--setenv", "GOPATH", "/tmp/gopath",
        "--",
        "bash",
        "-c",
        command,
    ]


async def _run_in_bwrap(
    command: str,
    workspace: Path,
    timeout: int,
) -> Tuple[int, bytes]:
    """Execute `command` inside bubblewrap with the workspace mounted.

    Returns (exit_code, combined_stdout_stderr_bytes). On timeout,
    returns (-1, b"[killed by timeout]") and SIGKILLs the whole
    process group so children don't leak.
    """
    if shutil.which("bwrap") is None:
        raise FileNotFoundError(
            "bwrap (bubblewrap) not found on PATH. Install it via "
            "'apt install bubblewrap' or rebuild the ai-worker container."
        )

    args = _build_bwrap_args(workspace, command)

    # Filter environment to a whitelist. bwrap's --setenv handles
    # the inside-sandbox env; we also need to make sure the bwrap
    # process itself doesn't inherit sensitive vars from the parent
    # (e.g. GITHUB_TOKEN, FORGE_SECRETS_MASTER_KEY) because those
    # could leak via bwrap's own handling before the unshare.
    safe_parent_env = {
        "PATH": os.environ.get("PATH", "/usr/local/bin:/usr/bin:/bin"),
        "HOME": "/tmp",
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
    }

    # Start in a new process group (setsid) so we can SIGKILL the
    # whole tree on timeout. On non-POSIX this is a no-op.
    preexec_fn = os.setsid if os.name == "posix" else None

    process = await asyncio.create_subprocess_exec(
        *args,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        env=safe_parent_env,
        preexec_fn=preexec_fn,
    )

    try:
        stdout, _ = await asyncio.wait_for(
            process.communicate(), timeout=timeout
        )
    except asyncio.TimeoutError:
        # Kill the whole process group; children don't leak.
        try:
            if os.name == "posix":
                pgid = os.getpgid(process.pid)
                os.killpg(pgid, signal.SIGKILL)
            else:
                process.kill()
        except (ProcessLookupError, PermissionError):
            pass
        # Drain whatever the subprocess produced before death.
        try:
            await asyncio.wait_for(process.wait(), timeout=5)
        except asyncio.TimeoutError:
            pass
        return -1, b"[killed by timeout]"

    return process.returncode or 0, stdout


def _format_bash_output(command: str, exit_code: int, output_bytes: bytes) -> str:
    """Produce the agent-visible bash output:

        $ <command>
        exit code: <N>

        <stdout/stderr up to 100 KB>
        ... [output truncated, M more bytes] (if capped)
    """
    text = output_bytes.decode("utf-8", errors="replace")
    truncated_note = ""
    raw_len = len(text)
    if raw_len > BASH_OUTPUT_CAP_BYTES:
        text = text[:BASH_OUTPUT_CAP_BYTES]
        truncated_note = (
            f"\n... [output truncated, "
            f"{raw_len - BASH_OUTPUT_CAP_BYTES} more bytes]"
        )

    return f"$ {command}\nexit code: {exit_code}\n\n{text}{truncated_note}"


# ---------------------------------------------------------------------------
# Tool class
# ---------------------------------------------------------------------------


class BashTool(BaseTool):
    name = "bash"
    description = (
        "Execute a shell command in the workspace directory. Use this "
        "for build, test, lint, and git inspection commands. The "
        "sandbox has NO network access -- you cannot install new "
        "dependencies (they're pre-installed). Stay inside the "
        "workspace directory. Default timeout 120s, max 600s."
    )
    input_model = BashInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        # bash can always mutate -- we don't try to introspect the command.
        return False

    async def execute(self, arguments: BashInput, context: ToolExecutionContext):
        # Layer 2: denylist front filter (not security -- UX)
        reason = _intent_denylist_check(arguments.command)
        if reason:
            # Observability -- how often does the agent hit the denylist
            # with which commands? Helps tune the denylist over time.
            logger.info(
                "agent.bash_denylist_hit",
                extra={
                    "event": "agent.bash_denylist_hit",
                    "reason": reason,
                    "command_prefix": arguments.command[:60],
                },
            )
            yield ToolResult(
                is_error=True,
                output=(
                    f"Command rejected: {reason}\n\n"
                    f"$ {arguments.command}"
                ),
            )
            return

        # Layer 1: bwrap sandbox
        label = _summarize_command(arguments.command)
        yield ThinkingStarted(label=label)

        # Run inside try/finally so ThinkingStopped always fires,
        # then yield the ToolResult AFTER ThinkingStopped to satisfy
        # the "ToolResult is last" contract.
        error_result: ToolResult | None = None
        exit_code = 0
        output = b""
        try:
            try:
                exit_code, output = await _run_in_bwrap(
                    command=arguments.command,
                    workspace=self._workspace_root,
                    timeout=arguments.timeout,
                )
            except FileNotFoundError as e:
                # bwrap missing -- clear error, one code path.
                error_result = ToolResult(
                    is_error=True,
                    output=(
                        f"bash sandbox unavailable: {e}\n\n"
                        "The ai-worker container installs bubblewrap in "
                        "the Phase 0 Dockerfile update; if you're running "
                        "outside the container, install bubblewrap and retry."
                    ),
                )
            except OSError as e:
                error_result = ToolResult(
                    is_error=True,
                    output=f"Failed to start bash subprocess: {e}",
                )
        finally:
            yield ThinkingStopped()

        # ToolResult is always the last item yielded.
        if error_result is not None:
            yield error_result
        else:
            yield ToolResult(
                output=_format_bash_output(arguments.command, exit_code, output),
                is_error=(exit_code != 0),
            )


# ---------------------------------------------------------------------------
# Registration helper (bash + set_phase -- the two "exec" tools)
# ---------------------------------------------------------------------------


def register_exec_tools(registry, workspace_root: Path) -> None:
    """Register BashTool and SetPhaseTool against the given ToolRegistry.

    Phase 5's _create_engine calls this alongside register_file_tools
    and register_context_tools. The two "exec" tools (bash + set_phase)
    are registered together for convenience -- SetPhaseTool is a
    meta/signal tool but sits in the same category for wiring purposes.
    """
    # Import at call time to avoid a circular import if phase_tool ever
    # grows an import back to bash_tool.
    from .phase_tool import SetPhaseTool

    registry.register(BashTool(workspace_root))
    registry.register(SetPhaseTool())
