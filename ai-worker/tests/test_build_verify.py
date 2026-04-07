import pytest
from src.openharness.hooks.builtin.build_verify_hook import BuildVerifyHook


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
