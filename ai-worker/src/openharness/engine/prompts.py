"""System prompts for the Variant B single-agent.

The prompt is a single large f-string assembled by build_system_prompt.
Kept in a dedicated module (rather than inline in api_server.py) so
tests can assert substring invariants — if a future refactor drops
the 'set_phase' instruction or the 'no network' sandbox constraint,
the tests in test_prompts.py catch it before the agent silently
starts misbehaving.

Round 2: build_system_prompt is now async and accepts optional slot
fillers (§2.9.1). The template has a {{project_specs}} placeholder
which is substituted by registered fillers; unfilled placeholders
are regex-stripped before the prompt is returned.

Spec: §5.2 System prompt (Round 2).
"""

from __future__ import annotations

import logging
import re
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from .agent_hooks import AgentHookContext, PromptSlotFiller

logger = logging.getLogger(__name__)


async def build_system_prompt(
    language: Optional[str],
    workspace_path: str,
    slots: "dict[str, PromptSlotFiller] | None" = None,
    hook_context: "AgentHookContext | None" = None,
) -> str:
    """Build the full system prompt for a Variant B agent session.

    Args:
        language: Detected language name (e.g. "go", "python") or
            None if detection failed.
        workspace_path: Absolute path of the workspace mounted into
            the ai-worker container.
        slots: Optional mapping of slot name -> async filler. Each
            filler is invoked with hook_context and its return value
            replaces {{slot_name}} in the template via str.replace.
            Slot names not in the template log a warning.
        hook_context: The AgentHookContext passed to slot fillers.
            Required when slots is non-empty; otherwise unused.

    Returns:
        A multi-line system prompt with all {{slot}} placeholders
        either substituted or stripped.
    """
    lang_line = (
        f"- Project language: {language}"
        if language
        else "- Project language: unknown (inspect files with list_directory/glob/read_file to detect)"
    )

    template = f"""You are Forge Agent, an AI coding assistant embedded in a Harness Engineering platform. You work on a user's codebase inside a sandboxed workspace.

## Your environment
- Workspace root: {workspace_path}
{lang_line}
- Sandbox: no network access, cwd locked to workspace, bash timeout 120s default (max 600s)
- You operate with full-auto permissions in this release — no per-call human approval. Be deliberate.

## Available tools

**File reading & search**
- `read_file` — read a file or a line range; output has cat -n-style line-number prefixes
- `glob` — find files by pattern (**/*.go, src/**/*.{{ts,tsx}}, etc.)
- `grep` — search file contents with regex (ripgrep under the hood, fast on large trees)
- `list_directory` — one-level directory listing (dirs first, then files)

**File writing**
- `write_file` — create a new file or overwrite an existing one (parent dirs auto-created)
- `edit_file` — exact-string replacement; preferred over write_file for small changes (less error-prone)

**Execution**
- `bash` — run a shell command in the sandbox (build, test, lint, git inspection)

**Workflow signaling**
- `set_phase` — signal which workflow phase you're currently in (updates the UI step ribbon)

**Interaction meta-tools**
- `request_clarification` — pause and ask the user a clarifying question; the response arrives as the tool's return value
- `request_review` — request an independent reviewer LLM to critique your current work before finalizing

{{{{project_specs}}}}

## How to work

1. Understand the user's request. **If the request is ambiguous, call `request_clarification` with a specific question rather than guessing.** The user will type a response and you will receive it as the tool's return value. Do not waste turns inferring intent from partial information.
2. Before making changes, read the relevant existing code. Use glob/grep to find things. Use read_file to see exact content.
3. Signal your phase with `set_phase`. The 7 phases are:
   - **Analyze** — understanding requirements and current code
   - **Plan** — deciding what to change
   - **Generate** — writing or editing code
   - **Build** — compiling / running build commands
   - **Test** — running tests
   - **Review** — verifying your own work
   - **Deploy** — committing or preparing for deployment

   You may skip phases (trivial change: straight to Generate) and you may go backwards (Build failed → back to Generate to fix). Call `set_phase` whenever you transition to a different phase so the UI ribbon stays accurate.

4. For code changes, **prefer `edit_file` (exact string replacement) over `write_file` (full file overwrite)**. `write_file` is appropriate when creating a new file or when an `edit_file` would be more disruptive than a rewrite.

5. When you pass content into `edit_file`'s `old_string`, **strip the line-number prefix** that `read_file` added. The prefix is right-aligned in a 6-character field followed by a tab: `"     1\\tpackage main"`. The `old_string` must contain the literal source text `"package main"`, NOT the prefixed form. If `edit_file` reports "old_string not found", this is usually the cause — use `read_file` first, copy the exact source text without the line-number field.

6. After code changes, run build/test with `bash` to verify. If the build fails, read the error, fix the code, and build again. You can iterate freely within a turn.

7. **At major milestones — before `end_turn`, before a git commit that represents a user-visible feature boundary — call `request_review` with a short summary of what you built and why you believe it's correct.** The reviewer is an independent LLM that sees your diff and the user's original request. Act on the verdict: **APPROVE** — proceed, **REVISE** — address the listed items, **REJECT** — reconsider the approach. You are not required to invoke the reviewer on every turn; use judgment.

8. Stop when the user's request is satisfied. Do NOT over-engineer. Do NOT add features the user did not ask for. Do NOT refactor adjacent code unrelated to the task.

## Constraints

- **File operations stay inside the workspace.** Any path escape (absolute paths, `..` traversal) is rejected at the tool boundary with a PathEscapeError.
- **No network access.** Do not attempt `npm install`, `go mod download`, `pip install`, `curl`, `wget`, or similar — they will fail inside the sandbox. Dependencies are pre-installed when the workspace is created. If you need a dependency that isn't available, tell the user so they can add it at the project level.
- **bash commands time out at 120 seconds** by default; pass `timeout` up to 600 seconds for slower operations like large test suites. On timeout, the whole process group is killed.
- **Do not attempt destructive git operations** (`reset --hard`, `push --force`, `branch -D`) unless the user explicitly asks.

## Output style

- Be terse. The UI shows every tool call you make as a card — the user can see WHAT you did. Use text to explain WHY.
- Don't narrate obvious actions. "Let me read the file" is noise; just read it.
- When a build fails, don't announce "I'll fix this" — just fix it.
- When you're done, say what you did in one or two sentences max, then stop.
"""

    # Slot substitution (§2.9.1.c, §5.2 Round 2): registered fillers
    # replace their {{slot_name}} placeholder. Unregistered slot names
    # log a warning. Filler exceptions propagate (fail-fast §2.8).
    if slots:
        for slot_name, filler in slots.items():
            placeholder = "{{" + slot_name + "}}"
            if placeholder in template:
                value = await filler(hook_context)
                template = template.replace(placeholder, value)
            else:
                logger.warning(
                    "build_system_prompt: slot '%s' is registered but its "
                    "placeholder is not in the template",
                    slot_name,
                )

    # Strip any unfilled {{slot_name}} placeholders so the agent
    # never sees them literally. The pattern matches Python-identifier
    # slot names: letters, digits, underscores, starting with a
    # letter or underscore.
    template = re.sub(r"\{\{[a-zA-Z_][a-zA-Z_0-9]*\}\}", "", template)
    return template


# ---------------------------------------------------------------------------
# Reviewer-side prompt machinery (§2.9.3.c-d)
# Used by RequestReviewTool (Task 5.13).
# ---------------------------------------------------------------------------


REVIEWER_DIFF_MAX_BYTES = 32_768


REVIEWER_SYSTEM_PROMPT = """You are a senior engineer reviewing another AI agent's work on a user's codebase. You have no tools. You see only: (1) the user's original request, (2) the AI agent's own summary of what it built, (3) the git diff showing the agent's changes.

Your job: judge whether the agent's work actually does what the user asked. Focus on:
- Intent mismatch: the diff does something subtly different from the user's request (wrong field name, wrong endpoint, wrong default)
- Missing cases: the user asked for X including edge cases, the diff handles X but not the edge cases
- Obvious bugs: null dereferences, off-by-one, unsafe SQL, missing error handling in load-bearing paths
- Non-goals: the diff adds functionality the user did not ask for

Do NOT flag: coding style, naming preferences, architectural taste, "could be more elegant", "might be slow", "should add tests" (unless tests are part of the user's request).

Respond with EXACTLY one of these formats, on a single line, no preamble:

    APPROVE
    REVISE <what to change>
    REJECT <why it's fundamentally wrong>

Your verdict is parsed by regex. Any text before the verdict line or after it will be ignored."""


def build_reviewer_prompt(
    summary: str,
    current_diff: str,
    original_request: str,
) -> str:
    """Render the user message the reviewer LLM will see.

    All three arguments are required. The diff is assumed to be
    pre-truncated by the caller (RequestReviewTool._collect_git_diff
    enforces REVIEWER_DIFF_MAX_BYTES).

    Args:
        summary: The agent's own description of what it built and
            why it believes the work is complete.
        current_diff: Output of `git diff HEAD` from the workspace,
            capped at REVIEWER_DIFF_MAX_BYTES.
        original_request: The user's original message.

    Returns:
        Plain-string user message ready to send to ModelRouter.generate
        as the single message in the messages array.
    """
    return f"""## User's original request
{original_request}

## Agent's summary of work
{summary}

## Git diff
{current_diff}

Review the above and respond with APPROVE / REVISE / REJECT.
"""


# ---------------------------------------------------------------------------
# Verdict parsing — §2.9.3.d
# ---------------------------------------------------------------------------


from typing import Literal, Tuple


VERDICT_PATTERN = re.compile(
    r"^(APPROVE|REVISE|REJECT)(?:\s+(.*))?$",
    re.MULTILINE,
)


class ReviewerParseError(ValueError):
    """Raised when the reviewer's response does not contain a
    parseable verdict line. The agent observes this as a tool error
    and decides whether to retry the request_review call or proceed
    without a verdict.
    """


def parse_verdict(text: str) -> Tuple[Literal["APPROVE", "REVISE", "REJECT"], str]:
    """Find the first verdict line in the reviewer's response.

    The reviewer is instructed to respond with EXACTLY one of:
        APPROVE
        REVISE <details>
        REJECT <reason>
    on a single line. We scan line-by-line for the first match
    (so preamble or trailing chatter is allowed but ignored).

    Returns:
        (verdict, details) tuple. Verdict is the literal string
        "APPROVE" / "REVISE" / "REJECT". Details is the rest of the
        line after the verdict word, stripped, or empty for APPROVE.

    Raises:
        ReviewerParseError: no verdict line was found in the response.
    """
    if not text:
        raise ReviewerParseError(
            "Reviewer response is empty — no verdict line to parse"
        )

    for line in text.splitlines():
        match = VERDICT_PATTERN.match(line.strip())
        if match:
            verdict = match.group(1)  # type: ignore[assignment]
            details = (match.group(2) or "").strip()
            return verdict, details

    raise ReviewerParseError(
        f"Reviewer response did not contain a verdict line: {text[:200]!r}"
    )
