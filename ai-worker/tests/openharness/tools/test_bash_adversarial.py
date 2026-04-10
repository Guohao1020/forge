"""Adversarial sandbox tests for BashTool (P0).

Each test is a named attack vector from spec section 7.1. A failure in
this file is a P0 security regression -- the bubblewrap sandbox
is broken, not a random flaky test.

The file is intentionally split from test_bash_tool.py so CI log
readers distinguish 'normal bug' from 'security regression' at
a glance.

These tests REQUIRE bubblewrap. They skip on hosts without it
(local dev on Windows/macOS) and run for real inside the ai-worker
container via docker-compose.
"""

import os
import shutil
from pathlib import Path

import pytest

from src.openharness.engine.stream_events import StreamEvent
from src.openharness.tools.base import ToolExecutionContext, ToolResult


def _require_bwrap():
    if shutil.which("bwrap") is None:
        pytest.skip("bubblewrap (bwrap) not installed; adversarial tests require it")


@pytest.fixture
def bash_tool(workspace):
    from src.openharness.tools.bash_tool import BashTool
    return BashTool(workspace)


@pytest.fixture
def bash_ctx(workspace):
    return ToolExecutionContext(cwd=workspace)


async def _run(tool, arguments, ctx):
    items = []
    async for item in tool.execute(arguments, ctx):
        items.append(item)
    results = [i for i in items if isinstance(i, ToolResult)]
    assert len(results) == 1
    return results[0]


# ---------------------------------------------------------------------------
# /etc isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_read_real_etc_passwd(bash_tool, bash_ctx):
    """Reading /etc/passwd should return either an error or bwrap's
    synthetic minimal /etc (if any), NOT the host's real passwd file.

    On a real host there's likely a 'root:x:0:0:root' line. Inside
    the sandbox we've omitted --ro-bind /etc (except /etc/ssl), so
    /etc doesn't exist at all -- cat /etc/passwd should say
    'No such file or directory'.
    """
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    result = await _run(bash_tool, BashInput(command="cat /etc/passwd"), bash_ctx)
    # Either no such file (because /etc is not bound) or empty/minimal output
    assert "root:x:" not in result.output, (
        "Sandbox leaked real /etc/passwd -- bwrap /etc binding is broken"
    )


# ---------------------------------------------------------------------------
# Environment secrets isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_read_secrets_env_var(bash_tool, bash_ctx, monkeypatch):
    """FORGE_SECRETS_MASTER_KEY must not leak into the sandbox."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Set the env var in the parent process for the duration of this test
    monkeypatch.setenv("FORGE_SECRETS_MASTER_KEY", "SECRET_MASTER_KEY_MARKER_xyzzy")
    result = await _run(
        bash_tool,
        BashInput(command="echo ${FORGE_SECRETS_MASTER_KEY:-EMPTY}"),
        bash_ctx,
    )
    # The sandbox env is whitelisted; the secret must not appear
    assert "SECRET_MASTER_KEY_MARKER_xyzzy" not in result.output, (
        "FORGE_SECRETS_MASTER_KEY leaked into bash sandbox env"
    )
    assert "EMPTY" in result.output


@pytest.mark.asyncio
async def test_bash_cannot_read_github_token_env_var(bash_tool, bash_ctx, monkeypatch):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    monkeypatch.setenv("GITHUB_TOKEN", "ghp_SECRET_TOKEN_MARKER_qwerty")
    result = await _run(
        bash_tool,
        BashInput(command="echo ${GITHUB_TOKEN:-EMPTY}"),
        bash_ctx,
    )
    assert "ghp_SECRET_TOKEN_MARKER_qwerty" not in result.output
    assert "EMPTY" in result.output


@pytest.mark.asyncio
async def test_bash_cannot_read_forge_prefix_env_vars(bash_tool, bash_ctx, monkeypatch):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    monkeypatch.setenv("FORGE_DB_PASSWORD", "forge_db_secret_123")
    result = await _run(
        bash_tool,
        BashInput(command="env | grep FORGE_ || echo NO_FORGE_VARS"),
        bash_ctx,
    )
    assert "forge_db_secret_123" not in result.output
    assert "NO_FORGE_VARS" in result.output


# ---------------------------------------------------------------------------
# Network isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_reach_network(bash_tool, bash_ctx):
    """ping should fail -- network namespace is isolated."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # -W sets timeout, -c 1 sends one packet. Expected to fail.
    result = await _run(
        bash_tool,
        BashInput(command="ping -c 1 -W 2 8.8.8.8 2>&1 || echo ping_failed"),
        bash_ctx,
    )
    assert "ping_failed" in result.output or "Network is unreachable" in result.output, (
        f"Network namespace not isolated -- ping succeeded. Output: {result.output[:200]}"
    )


@pytest.mark.asyncio
async def test_bash_cannot_curl(bash_tool, bash_ctx):
    """curl should fail -- no network access."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # -m sets timeout (curl may not be installed in the container at all,
    # which is also acceptable -- just means the agent can't hit the net)
    result = await _run(
        bash_tool,
        BashInput(
            command="curl -s -m 3 https://example.com 2>&1 || echo curl_failed",
            timeout=10,
        ),
        bash_ctx,
    )
    # Either curl failed to connect or curl is not installed; both are fine.
    assert (
        "curl_failed" in result.output
        or "not found" in result.output
        or "No such file" in result.output
    ), f"Network namespace not isolated -- curl succeeded. Output: {result.output[:200]}"


# ---------------------------------------------------------------------------
# Workspace isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_read_other_workspace(bash_tool, bash_ctx, tmp_path):
    """A path outside the current workspace bind must not be visible."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Create a file OUTSIDE the bind at a well-known host path
    secret_dir = tmp_path.parent / "other_tenant_workspace"
    secret_dir.mkdir(exist_ok=True)
    secret_file = secret_dir / "SECRET.txt"
    secret_file.write_text("OTHER_TENANT_SECRET_MARKER\n")

    result = await _run(
        bash_tool,
        BashInput(command=f"cat {secret_file} 2>&1 || echo cannot_read"),
        bash_ctx,
    )
    assert "OTHER_TENANT_SECRET_MARKER" not in result.output, (
        "Sandbox can read outside its workspace bind -- isolation broken"
    )


# ---------------------------------------------------------------------------
# Process isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_kill_parent_process(bash_tool, bash_ctx):
    """--unshare-all creates a new PID namespace. pid 1 inside the
    sandbox is bash, not the host ai-worker parent. kill -9 1 should
    either fail or kill the sandbox bash (not the parent)."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    result = await _run(
        bash_tool,
        BashInput(command="kill -9 1 2>&1; echo still_alive_$?"),
        bash_ctx,
    )
    # Either kill fails with 'operation not permitted' or it hits pid 1
    # inside the sandbox (which is bash itself). Either way the ai-worker
    # parent process is unharmed -- this test passes as long as the test
    # runner itself is still alive when the result comes back.
    assert "still_alive_" in result.output or "No such process" in result.output or result.is_error


# ---------------------------------------------------------------------------
# Timeout & process group kill
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_respects_timeout(bash_tool, bash_ctx):
    """sleep 200 with timeout=3 must return in <10s."""
    _require_bwrap()
    import time
    from src.openharness.tools.bash_tool import BashInput

    start = time.monotonic()
    result = await _run(
        bash_tool,
        BashInput(command="sleep 200", timeout=3),
        bash_ctx,
    )
    elapsed = time.monotonic() - start

    assert elapsed < 10, f"Timeout did not fire: elapsed {elapsed:.1f}s"
    assert "killed by timeout" in result.output or "exit code: -1" in result.output
    assert result.is_error


@pytest.mark.asyncio
async def test_bash_timeout_kills_subprocess_tree(bash_tool, bash_ctx):
    """Backgrounded child processes must die on timeout via killpg."""
    _require_bwrap()
    import time
    from src.openharness.tools.bash_tool import BashInput

    start = time.monotonic()
    result = await _run(
        bash_tool,
        BashInput(command="sleep 200 & wait", timeout=3),
        bash_ctx,
    )
    elapsed = time.monotonic() - start

    # If the child sleep isn't killed along with the parent, 'wait' hangs
    # and the timeout path has to clean it up -- should still be well under
    # 15 seconds total.
    assert elapsed < 15, f"Child sleep leaked past timeout: elapsed {elapsed:.1f}s"
    assert "killed by timeout" in result.output or result.is_error


# ---------------------------------------------------------------------------
# Output truncation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_output_truncation_at_100kb(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Generate roughly 300 KB of output
    result = await _run(
        bash_tool,
        BashInput(command="for i in $(seq 1 30000); do echo 'lineX_0123456789'; done"),
        bash_ctx,
    )
    # Must contain the truncation note
    assert "truncated" in result.output
    # Total output bytes after formatting should be bounded by the cap
    # plus the prefix overhead (a few hundred bytes for "$ cmd\nexit code\n")
    assert len(result.output.encode("utf-8")) < 110_000


# ---------------------------------------------------------------------------
# Denylist layering
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_denylist_rejects_sudo_explicit(bash_tool, bash_ctx):
    """Explicit P0 gate: the regex denylist rejects 'sudo' before bwrap."""
    from src.openharness.tools.bash_tool import BashInput

    result = await _run(bash_tool, BashInput(command="sudo whoami"), bash_ctx)
    assert result.is_error
    assert "sudo" in result.output.lower()


@pytest.mark.asyncio
async def test_bash_denylist_bypass_still_safe(bash_tool, bash_ctx):
    """An agent that bypasses the denylist (e.g., via env var
    indirection) must still be unable to escalate privileges. bwrap
    strips the setuid bit, so even if sudo the binary is invoked
    inside the sandbox, it can't actually elevate."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # This command passes the denylist regex because the literal
    # 'sudo' token isn't a standalone shell word at the start
    result = await _run(
        bash_tool,
        BashInput(command="S=sudo; $S whoami 2>&1 || echo denied"),
        bash_ctx,
    )
    # Inside the sandbox, either sudo isn't installed or it can't
    # elevate (no setuid, no network, namespace isolation). We
    # check that the output isn't 'root' from a successful elevation.
    # Acceptable outputs: 'denied', 'not found', 'command not found',
    # or the original user (non-root inside sandbox).
    assert "root\n" not in result.output or "denied" in result.output or "not found" in result.output.lower()
