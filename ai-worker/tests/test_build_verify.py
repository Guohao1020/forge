from pathlib import Path
from unittest.mock import patch

import pytest
from src.openharness.hooks.builtin.build_verify_hook import (
    BuildVerifyHook,
    _required_markers_for,
)


@pytest.mark.asyncio
async def test_build_success():
    hook = BuildVerifyHook(build_command="echo build_ok", timeout_seconds=10)
    result = await hook.run(code_files={"test.txt": "hello"})
    assert result.success
    assert "build_ok" in result.output


@pytest.mark.asyncio
async def test_build_failure():
    hook = BuildVerifyHook(build_command="exit 1", timeout_seconds=10)
    # Use shell=True equivalent via a command that fails
    # On Windows, "exit 1" alone won't work in subprocess_exec, use python
    hook2 = BuildVerifyHook(
        build_command="python -c raise(SystemExit(1))",
        timeout_seconds=10,
    )
    result = await hook2.run(code_files={"test.txt": "hello"})
    assert not result.success


@pytest.mark.asyncio
async def test_build_command_not_found():
    hook = BuildVerifyHook(
        build_command="__nonexistent_compiler_xyz__ --check",
        timeout_seconds=10,
    )
    result = await hook.run(code_files={"test.txt": "hello"})
    assert not result.success
    assert "not found" in result.output.lower()


@pytest.mark.asyncio
async def test_build_timeout():
    # Write a script that sleeps, then run it
    hook = BuildVerifyHook(
        build_command="python sleeper.py",
        timeout_seconds=1,
    )
    result = await hook.run(code_files={"sleeper.py": "import time\ntime.sleep(30)"})
    assert not result.success
    assert "timed out" in result.output.lower()


@pytest.mark.asyncio
async def test_to_hook_result_success():
    hook = BuildVerifyHook(build_command="echo ok", timeout_seconds=10)
    result = await hook.run(code_files={})
    hr = hook.to_hook_result(result)
    assert hr.success
    assert not hr.blocked


@pytest.mark.asyncio
async def test_to_hook_result_failure():
    hook = BuildVerifyHook(
        build_command="python -c raise(SystemExit(1))",
        timeout_seconds=10,
    )
    result = await hook.run(code_files={})
    hr = hook.to_hook_result(result)
    assert not hr.success
    assert hr.blocked
    assert hr.hook_type == "build_verify"


@pytest.mark.asyncio
async def test_code_files_written():
    """Verify files are actually written to the temp directory."""
    hook = BuildVerifyHook(build_command="python -c print(open('hello.py').read())", timeout_seconds=10)
    result = await hook.run(code_files={"hello.py": "print('world')"})
    assert result.success
    assert "print('world')" in result.output


# ---- Preflight marker check tests (TASK 8) --------------------------------


def test_required_markers_for_go():
    assert _required_markers_for("go build ./...") == ("go.mod",)
    assert _required_markers_for("go test ./...") == ("go.mod",)


def test_required_markers_for_other_tools():
    # Sanity that the marker map covers the main toolchains. Plan only
    # mandates `go`, but the map is generic so detection-driven
    # pipelines for other languages get the same protection.
    assert "pom.xml" in _required_markers_for("mvn compile")
    assert "package.json" in _required_markers_for("npm run build")
    assert "Cargo.toml" in _required_markers_for("cargo build")


def test_required_markers_for_unknown_tool_returns_empty():
    # Unknown commands fall through to "" so the preflight is a no-op
    # and we let the subprocess speak for itself (matches existing
    # echo / python tests).
    assert _required_markers_for("echo hello") == ()
    assert _required_markers_for("custom-script") == ()
    assert _required_markers_for("") == ()


@pytest.mark.asyncio
async def test_preflight_blocks_go_build_without_go_mod(tmp_path):
    """The flagship plan TASK 8 case: `go build` in a directory with
    no go.mod must fail BEFORE invoking the subprocess. This catches
    misconfigured pipelines (wrong language detection, wrong work dir)
    early and surfaces an explicit, actionable error instead of an
    opaque tool message."""
    hook = BuildVerifyHook(build_command="go build ./...", timeout_seconds=10)

    # Sentinel: if the preflight is broken and lets the subprocess run,
    # we want the test to clearly diagnose that. Patch create_subprocess_exec
    # so a leak triggers a recognizable assertion failure rather than a
    # 30-second toolchain hunt.
    with patch(
        "asyncio.create_subprocess_exec",
        side_effect=AssertionError("subprocess MUST NOT run when preflight blocks"),
    ):
        result = await hook.run(
            code_files={"main.go": "package main\nfunc main() {}\n"},
            project_dir=tmp_path,  # empty dir, no go.mod
        )

    assert not result.success
    assert "preflight failed" in result.output.lower()
    assert "go.mod" in result.output


@pytest.mark.asyncio
async def test_preflight_passes_when_go_mod_exists(tmp_path):
    """Preflight allows the subprocess through when go.mod is present
    in the work dir. We don't actually need a working Go toolchain
    here — patching subprocess lets us assert the preflight let it
    through without depending on a real `go build`."""
    (tmp_path / "go.mod").write_text("module example.com/test\n", encoding="utf-8")

    hook = BuildVerifyHook(build_command="go build ./...", timeout_seconds=10)

    # Mock subprocess to return success without actually invoking go.
    class FakeProc:
        returncode = 0

        async def communicate(self):
            return (b"", b"")

    async def fake_create(*args, **kwargs):
        return FakeProc()

    with patch("asyncio.create_subprocess_exec", side_effect=fake_create):
        result = await hook.run(
            code_files={"main.go": "package main\nfunc main() {}\n"},
            project_dir=tmp_path,
        )

    assert result.success, f"expected success, got: {result.output}"


@pytest.mark.asyncio
async def test_preflight_accepts_llm_generated_marker(tmp_path):
    """If the work dir is empty but the LLM emits go.mod alongside the
    code, preflight should accept it — the marker is satisfied by the
    files we are about to write."""
    hook = BuildVerifyHook(build_command="go build ./...", timeout_seconds=10)

    class FakeProc:
        returncode = 0

        async def communicate(self):
            return (b"", b"")

    async def fake_create(*args, **kwargs):
        return FakeProc()

    with patch("asyncio.create_subprocess_exec", side_effect=fake_create):
        result = await hook.run(
            code_files={
                "go.mod": "module example.com/llm\n",
                "main.go": "package main\nfunc main() {}\n",
            },
            project_dir=tmp_path,  # empty dir
        )

    assert result.success


@pytest.mark.asyncio
async def test_preflight_skipped_for_unknown_tool(tmp_path):
    """Unknown tools (echo, custom scripts) bypass the preflight and
    let the subprocess speak. This preserves the legacy behavior of
    the existing tests in this file (echo build_ok etc.)."""
    hook = BuildVerifyHook(build_command="echo hello", timeout_seconds=10)
    result = await hook.run(
        code_files={"x.txt": "y"},
        project_dir=tmp_path,
    )
    assert result.success
    assert "hello" in result.output
