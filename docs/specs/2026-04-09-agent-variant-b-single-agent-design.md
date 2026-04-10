# Agent Variant B · Single-Agent Design

> Design spec — 2026-04-09
>
> **Topic:** Rebuild Forge's AI agent pipeline around a single tool-using agent
> that can actually drive the Variant B mockup, replacing the current
> pair_pipeline workaround that uses regex-extracted code blocks.
>
> **Scope:** Option **A** — only the agent interaction layer. Adjacent work
> (Harness Engineering context layer, permission UI, multi-platform
> artifacts, etc.) is explicitly out of scope and deferred.
>
> **Engineering standard:** Silicon-valley grade infra. No compromises,
> no hardcoded special cases, no regex-as-security-boundary, one code path.

---

## 1. Problem

### 1.1 What the user sees today

The Forge Portal ships the full Variant B frontend shell
(`forge-portal/components/agent/`): chat panel, tool-execution cards,
step ribbon, code panel, build card, summary card, status bar. The
SSE event vocabulary to drive those components is already defined
(`ai-worker/src/openharness/engine/stream_events.py`): `text_delta`,
`tool_started`, `tool_completed`, `thinking_started/stopped`,
`fix_loop_started/completed`, `session_complete`.

The frontend is waiting for a real agent to feed it.

### 1.2 What the backend actually does

The backend does not drive Variant B. It drives a degraded approximation:

1. `ai-worker/src/openharness/engine/query.py` contains a real multi-turn
   agent loop with streaming API, `stop_reason=tool_use` detection,
   tool execution, hooks, and permission checks — all correct.
2. `ai-worker/src/openharness/engine/pair_pipeline.py` does **not** use that
   loop. It wires a fixed Coder → BuildVerify → Reviewer sequence and
   passes a single prompt to a `QueryEngine` whose `ToolRegistry` is
   **empty**. The LLM responds with fenced code blocks in a single text
   message, and `_extract_code_files()` uses a regex to pull those blocks
   out. `BuildVerifyHook` runs `mvn`/`go build` against a snapshot of
   the extracted files. If the build fails, the pipeline re-prompts with
   the error and repeats up to `max_cycles`.
3. `ai-worker/src/openharness/tools/` contains only `context_tools.py` —
   five read-only profile-query tools plus `read_project_file`. There is
   no `write_file`, no `edit_file`, no `bash`/`execute_command`, no
   `glob`, no `grep`. The agent has no hands.

The pair_pipeline is a workaround born from that fact. Once the agent
gets real write/execute tools, it disappears.

### 1.3 What Variant B actually requires

Variant B's mockup (`~/.gstack/projects/voc-shulex-forge/designs/agent-terminal-shotgun-20260406/variant-B-dense.html`)
depicts a Cursor-style interaction:

- One AI speaker ("Forge") — not two
- Sequential tool cards: `read_file` → `read_file` → `write_file` ×4 →
  `edit_file` → `execute_command mvn compile` → (error) → `edit_file` ×2
  → `execute_command mvn compile` (running)
- A step ribbon that reflects the agent's current phase
- A code panel on the right with the file currently under edit
- A summary card when the turn finishes

No part of this is compatible with "single-prompt, regex-extract, retry-
loop". Variant B requires a real tool-use loop with real tools.

### 1.4 Why this matters now

Every feature downstream of the agent (permission UI, version management,
constraint engine integration, cost observability, IM bot reuse) assumes
the agent can actually drive the UI the frontend already exists for.
Building any of those on top of the current hollow pipeline is building
on sand. This spec fixes the spine.

---

## 2. Decisions

Six clarification rounds produced this decision chain. Every decision
here was explicitly confirmed by the user during brainstorming; the
rationale is preserved so future reviewers can audit the trade-offs
rather than re-litigate them.

### 2.1 Scope — Option A

Only the agent interaction layer. No adjacent work.

**Rationale:** The current pipeline is hollow at the core. Expanding
scope to include context layer improvements, permission UI, or platform
integrations before the core is real would be building on sand.

### 2.2 Architecture — Option A2: kill pair_pipeline

`pair_pipeline.py` is deleted. `run_pair_pipeline`, `PairPipelineConfig`,
`CycleResult`, `PairPipelineResult`, `ReviewDecision`, `_build_fix_prompt`,
`_build_review_prompt`, `_parse_review_decision`, `_extract_code_files`,
`_count_compile_errors` are all removed. The routing fork in
`_route_and_stream` that decides "pair vs. legacy QueryEngine" is removed;
all requests go through `QueryEngine`.

**Rationale:**
1. The Variant B mockup is a single-speaker view (all messages are
   "Forge", one `msg-model` tag). The pair/Coder/Reviewer distinction was
   invented for pair_pipeline, not required by Variant B.
2. pair_pipeline exists *because* there are no write/execute tools. Once
   tools exist, the outer cycle has nothing to do — the agent can iterate
   build/fix on its own inside the agent loop.
3. Coder-self-reviews-via-running-tests is a stronger form of review than
   Reviewer-reads-code. Claude Code is single-agent and outperforms our
   current two-agent regex version dramatically.
4. If independent reviewer perspective is needed later, it can be added
   as a `request_review` meta-tool (option A3 from brainstorming) without
   disturbing the A2 architecture.

### 2.3 Tool surface — T2 (pragmatic set)

Seven workspace-operating tools plus one meta-tool:

| Tool | Purpose | Read-only |
|---|---|---|
| `read_file` | Read file contents with optional line range | ✓ |
| `write_file` | Create or overwrite a file | ✗ |
| `edit_file` | Exact-string replacement (Claude Code contract) | ✗ |
| `glob` | Find files by glob pattern | ✓ |
| `grep` | Search file contents by regex (ripgrep) | ✓ |
| `list_directory` | One-level directory listing | ✓ |
| `bash` | Execute shell command in sandbox | ✗ |
| `set_phase` | Meta-tool: signal current phase to UI ribbon | ✓ |

All file-operating tools operate directly on the workspace directory
(not through a forge-core HTTP indirection). The existing
`context_tools.py` tools (`query_api_catalog` etc.) remain registered
alongside these, migrated to the new `BaseTool` signature — see §4.

**Not included and why:**
- `apply_patch` / `multi_edit`: optimization, not capability. LLMs
  struggle with unified-diff format. Use `edit_file` N times.
- `run_tests`: special-casing `bash pytest`. `bash` is enough; the
  frontend can pattern-match "a bash command whose first token is a
  known test runner" for richer card rendering later.
- `git_diff`: can be expressed as `bash git diff`.
- `web_fetch`: out of scope (prompt-injection risk, no network in
  sandbox).

### 2.4 Permission mode — P1 now, P3 slot reserved

This release runs in `PermissionMode.FULL_AUTO` (current default). No
user-facing approval UI. All tools execute without confirmation. Workspace
safety comes from git rollback, not from per-call approval.

The data model, event vocabulary, and `PermissionChecker` API shall leave
room for a future mode **P3** — "auto-run in workspace, ask for
destructive/out-of-workspace/sensitive bash" — to be added without
architectural change. Specifically:

- `PermissionChecker.evaluate()` already returns a `Decision` that can
  carry a "needs_confirmation" state alongside `allowed`.
- The SSE event stream reserves the event type names
  `tool_permission_requested`, `tool_permission_granted`,
  `tool_permission_denied` — implementation deferred, but the names
  shall not be reused for anything else.
- No bidirectional RPC is introduced in this release. When P3 lands, it
  will introduce a Redis pub/sub or WebSocket channel for approval
  round-trips.

**Rationale:** P2/P3 require the agent to pause mid-execution waiting
for a user response, which means rewriting `_run_and_publish` from
fire-and-forget SSE into a bidirectional protocol. That is a separate
large piece of work and does not belong in the same release as the tool
surface rebuild.

### 2.5 Step ribbon — dynamic phases via `set_phase` tool

The step ribbon's seven phase labels (`Analyze`, `Plan`, `Generate`,
`Build`, `Test`, `Review`, `Deploy`) remain as a fixed enum because they
are the user-facing mental model the mockup established. However:

- "Current phase" is no longer derived from a fixed workflow position.
  It comes from a `PhaseChanged` event emitted by the `SetPhaseTool`.
- The agent chooses when to transition phases by calling `set_phase`.
  It may skip phases (trivial change goes straight to Generate) and it
  may go backwards (Build failed → return to Generate to fix).
- The ribbon supports three states per cell: `upcoming` (grey), `active`
  (highlight + pulse), `visited` (faded checkmark).
- Initial state (before any `phase_changed` event): all cells `upcoming`,
  no highlight.

**Rationale:** A fixed 7-step ribbon lies about what the agent is doing.
A dynamic ribbon is honest. Sniffing the phase from the agent's text
output would be a heuristic with hallucination risk; a tool call is an
explicit, verifiable signal.

### 2.6 Other Variant B component decisions

| # | Component | Decision | Rationale |
|---|---|---|---|
| 1 | Step Ribbon | Dynamic via `set_phase` | See §2.5 |
| 2 | Code Panel | Shell only, read-only preview | Full diff rendering is independent large work, defer |
| 3 | Build Card | **Delete** | In A2 "build" is just `bash mvn`, unified tool card |
| 4 | Summary Card | Keep, `end_turn` triggers | SessionComplete data already computed; near-zero cost to keep |
| 5 | Fix Loop Banner | **Delete events; frontend visual detection** | Events were pair_pipeline-only concepts; see §5.3 |
| 6 | Thinking Indicator | Repurpose to "bash tool executing" | Valuable during 30s `mvn` run; useless during API first-token |

### 2.7 Workspace lifecycle — W1 long-lived per project

- One workspace per (tenant, project), shared across sessions.
- Lazy-created when the user sends the first message to an agent for
  that project (not when opening the project page).
- SSH deploy key authentication (project-level), not HTTPS + token.
- On new session start (not mid-session), workspace is
  `git fetch origin && git reset --hard origin/<default_branch>` to
  prevent it from rotting into an unmergeable mess. In-session changes
  stay put across multiple messages in the same session.
- Clone failure → agent emits `ErrorEvent`, session halts.
- Tenant isolation via path: `workspaces/{tenant_id}/{project_id}/`.
- Disk reclamation: not implemented in this release. Manual cleanup.

**Rationale:** Chosen over per-session clone (slow) and per-turn reset
(user's prior work disappears mid-conversation). The "reset on new
session, keep during session" rule is a deliberate compromise: it feels
continuous *while the user is working* and keeps the workspace honest
*between sessions*.

### 2.8 Engineering standard — Silicon Valley grade

The user's explicit standard for this infra phase is "no compromises, no
debt". This rules out three specific anti-patterns that drafts of this
design originally contained:

1. **No `if tool_name == "bash"` hardcoded special cases in the agent
   loop.** If a tool needs to emit events during execution, the tool
   abstraction must support it (§4.1).
2. **No regex denylist as security boundary.** Shell command denylists
   can be trivially bypassed with `${IFS}`, base64, nested quoting, etc.
   Real isolation requires process/namespace-level sandboxing (§4.6).
3. **No parallel code paths** ("Linux one way, Windows another" /
   "dev one way, prod another" / "rg present one way, Python fallback
   another"). One code path (§3, §6, §7).

Specific manifestations:
- `BaseTool` gets refactored from `async def execute(...) -> ToolResult`
  to `AsyncIterator[StreamEvent | ToolResult]`.
- `BashTool` uses `bubblewrap` for isolation, not regex.
- Windows dev mode requires running `forge-ai-worker` through
  docker-compose. No "run ai-worker directly on Windows host and skip
  sandbox" escape hatch.
- `grep` requires `ripgrep` in the container; there is no Python
  fallback path.
- File paths are validated at the type level via a `WorkspacePath` class,
  not by a helper that each tool author must remember to call.
- Security-sensitive code (sandbox, path resolution, deploy key crypto)
  gets an explicit adversarial test suite as a P0 gate.

### 2.9 Round 2 strategic additions (post-CEO-review)

This subsection was added after the chronos Round 1 plan completed and
triggered an autoplan CEO review (2026-04-09, `[subagent-only]` mode —
Codex CLI unavailable). The reviewer produced 5 substantive findings
Harvey accepted. The original §2.1–§2.8 decisions remain in force;
this section adds what Round 1 **missed** and Round 2 must include.

Full CEO review reasoning is preserved at
`~/.claude/projects/D--shulex-work-forge/memory/chronos-ceo-review-2026-04-09.md`.

#### 2.9.1 Verification hooks in the agent loop (Critical 1)

**Problem:** Forge's target users are PMs/ops who cannot read code. For
them, "agent output compiles" ≠ "agent output is correct". Round 1
chronos rebuilt the builder but left no structural check for intent
correctness, meaning chronos alone ships a product that is
undifferentiated from Cursor/Claude Code and silently defers Forge's
core value proposition.

**Decision:** chronos does NOT solve the verification problem (that's
another project). But chronos **must leave verification hooks in the
agent loop** so the next project can plug in without modifying
`query.py`.

##### 2.9.1.a Registry model and class names

The existing `ai-worker/src/openharness/hooks/` package holds a
**subprocess** hook executor (`HookRegistry`, `HookExecutor`) that runs
external shell commands on `PRE_TOOL_USE` / `POST_TOOL_USE` / etc.
events. §2.9.1 is introducing a **parallel, in-process Python-callable
hook system**. The two do not replace each other and must not share a
class name.

- Existing subprocess hooks: `openharness.hooks.HookRegistry` +
  `HookExecutor`. Unchanged in Round 2. Still wired into
  `_execute_tool_call` around PRE_TOOL_USE / POST_TOOL_USE as today.
- New in-process agent hooks (Round 2): live in a new module
  `ai-worker/src/openharness/engine/agent_hooks.py` with class name
  **`AgentHookRegistry`** (distinct from `HookRegistry`).
  `AgentHookRegistry` holds four fields:
  - `pre_turn: list[PreTurnHook]`
  - `pre_tool_call: list[PreToolCallHook]`
  - `post_turn: list[PostTurnHook]`
  - `system_prompt_slots: dict[str, PromptSlotFiller]`

##### 2.9.1.b Callable signatures (pinned)

All four hook signatures are `async` callables. No sync hooks —
agent loop is `async` end-to-end.

```python
# agent_hooks.py
from typing import Awaitable, Callable, Protocol

# Input: the message list the model is about to see, plus context.
# Output: the message list the model actually sees (may be mutated copy).
# Hook may also mutate the system prompt via ctx.system_prompt_buffer.
class PreTurnHook(Protocol):
    async def __call__(
        self,
        ctx: "AgentHookContext",
        messages: list[Message],
    ) -> list[Message]: ...

# Input: tool name, parsed arguments, context.
# Output: either the (possibly-mutated) arguments to execute, or a
# PreToolCallBlock(reason) to short-circuit execution with a
# ToolResult(is_error=True, output=reason). Raising is a bug.
class PreToolCallHook(Protocol):
    async def __call__(
        self,
        ctx: "AgentHookContext",
        tool_name: str,
        arguments: BaseModel,
    ) -> "BaseModel | PreToolCallBlock": ...

# Input: the turn's final assistant message (after stop_reason=end_turn).
# Output: None. Hook may record metrics, trigger follow-up events, etc.
class PostTurnHook(Protocol):
    async def __call__(
        self,
        ctx: "AgentHookContext",
        final_message: Message,
    ) -> None: ...

# Input: current project/session metadata.
# Output: a string to substitute for the slot placeholder.
class PromptSlotFiller(Protocol):
    async def __call__(self, ctx: "AgentHookContext") -> str: ...

@dataclass(frozen=True)
class PreToolCallBlock:
    reason: str
```

`AgentHookContext` carries `project_id`, `session_id`, `workspace_dir`,
and a mutable `system_prompt_buffer: list[str]` that `pre_turn` hooks
can append to. It is constructed once per session at `_create_engine`
time and passed into every hook call.

##### 2.9.1.c Invocation order and failure mode (pinned)

- **Order within a registry:** list order (registration order). No
  priority field, no topological sort. If downstream projects need
  ordering, they register in the right order. Silicon Valley standard
  §2.8 — simplest mechanism, one code path.
- **Failure mode:** any hook raising an exception halts the turn via
  `ErrorEvent(message="agent hook {name} failed: {exc}", recoverable=False)`
  and logs with `logger.exception`. This is the same "fail fast" stance
  as §4.12's `_create_engine` "no AsyncMock fallback". The empty
  default registries Round 2 ships never raise, so this failure mode
  only bites downstream projects with buggy hooks — loudly, as §2.8
  requires.
- **`pre_tool_call` block semantics:** if a hook returns
  `PreToolCallBlock(reason)`, `_execute_tool_call` emits
  `ToolExecutionStarted` normally, then emits
  `ToolExecutionCompleted(..., is_error=True, output=reason)`, and the
  agent loop continues. The tool is **not** executed. No silent skip.
- **`system_prompt_slots` substitution:** `build_system_prompt`
  renders a template with `{{slot_name}}` placeholders. For each slot
  in `registry.system_prompt_slots`, the registered filler is invoked
  and its return value replaces `{{slot_name}}` via plain
  `str.replace`. Slot names not registered are left untouched (with a
  `logger.warning`). No Jinja, no f-strings at render time.
- **`pre_tool_call` applies uniformly** including to
  `RequestClarificationTool` and `RequestReviewTool`. A constraint hook
  that blocks review is Harvey's problem to debug, not a special case
  in chronos.

##### 2.9.1.d Wiring in `_create_engine`

Round 2 modifies the `_create_engine` code block in §4.12 so it
constructs an empty `AgentHookRegistry` alongside the existing
`HookRegistry`. Both are passed into `QueryEngine`. See §4.12 for the
updated code. Downstream projects populate `AgentHookRegistry` via a
module-level factory (e.g., `build_agent_hooks(project_id)`); how that
factory is wired is out of scope for chronos and in scope for the
follow-up project.

##### 2.9.1.e What Round 2 ships

- `agent_hooks.py` module with `AgentHookRegistry`, `AgentHookContext`,
  `PreToolCallBlock`, and the four Protocol signatures above
- `build_system_prompt` accepts an optional
  `slots: dict[str, PromptSlotFiller]` argument and substitutes
  `{{slot_name}}` placeholders
- `_create_engine` constructs an **empty** `AgentHookRegistry` and
  passes it into `QueryEngine`
- `QueryEngine.submit_message` invokes `registry.pre_turn` before
  sending the message list, `registry.pre_tool_call` inside
  `_execute_tool_call` between permission check and tool execution,
  and `registry.post_turn` after `stop_reason=end_turn`
- Contract tests verify all three extension points: a trivial
  `pre_turn` hook that appends `"foo"` to the system prompt, a
  `pre_tool_call` hook that blocks a named tool, and a `post_turn`
  hook that increments a counter. Each test asserts the observable
  behavior end-to-end.

**Scope cut:** chronos Round 2 does NOT ship any real hook
implementations. The extension points are present and tested
(trivial hooks in the contract test) but no spec/constraint/entropy
hooks are provided. Those are follow-up projects. The subprocess
`HookRegistry` is unchanged — that system continues to handle command
hooks as today.

#### 2.9.2 `request_clarification` meta-tool + bidirectional SSE (Critical 2)

**Problem:** PM/ops requests are inherently ambiguous ("add a login
page", "make the dashboard faster"). Round 1 chronos has no way for
the agent to pause mid-turn and ask the human a clarifying question.
The system prompt instructs the agent to "ask for clarification" in
text, but text-only clarification means the agent ends the turn and
hopes the user starts a new session with the answer — losing all
in-progress state.

**Decision:** add a `request_clarification(question: str) -> str`
meta-tool. When the agent calls it:

1. The tool yields a `ClarificationRequested(question, tool_use_id)`
   stream event
2. The tool awaits a `ClarificationResponse` on the session's Redis
   return channel (pause point — see §2.9.2.c)
3. The frontend renders the question as a special system message with
   an input field
4. The user's response travels back through a new **bidirectional SSE
   return channel** (Redis pub/sub, one channel per session)
5. The waiting tool's future is resolved with the user's response
6. The tool yields a `ToolResult(output=<user_response>)` (single
   terminal `ToolResult`, preserving §4.1 contract)
7. The agent continues its turn with the answer

This requires a new **Phase 5a — Bidirectional RPC** in the Round 2
plan because spec §2.4 explicitly said "P2/P3 bidirectional RPC is an
independent large project, out of chronos scope." Harvey decided on
2026-04-09 that eating the bidirectional cost inside chronos is worth
it because `request_clarification` without it is a fake feature (the
agent can't actually wait for a response without blocking the whole
worker thread, and without bidirectional communication there's no
transport for the response anyway).

##### 2.9.2.a Transport choice — Redis pub/sub (not Streams)

Harvey chose Redis pub/sub over Redis Streams on 2026-04-09 despite
pub/sub being lossy if the subscriber is late. Rationale:

- The subscriber (ai-worker's per-session return-channel listener) is
  started **before** the tool yields `ClarificationRequested` and
  **before** the client is told to ask the question. There is no
  "subscriber late" window — if the subscriber isn't up, the tool
  fails fast via §2.9.2.f and the session halts.
- Pub/sub has a simpler lifecycle (no trim, no consumer group, no
  acks) which matches the one-shot "question → single response → done"
  pattern. Streams would require consumer groups to support
  multi-pod ai-worker deployments, and chronos is single-pod.
- Message loss on disconnect is handled by the future timeout
  (§2.9.2.f) which halts the session cleanly — same semantics as any
  network-partition failure mode.

If chronos ever scales to multi-pod ai-worker, the subscriber lives
inside the pod that holds the session's `QueryEngine` instance. A
later project (sticky session routing at the forge-core layer) can
address multi-pod; it's out of chronos scope.

##### 2.9.2.b Channel schema (pinned)

**Channel name:** `agent:return:{session_id}`. One channel per session.
Created when the session starts (see §2.9.2.c subscriber lifecycle),
destroyed when the session ends.

**Message shape** (JSON, UTF-8, published as a single Redis pub/sub
message):

```json
{
  "type": "clarification_response",
  "session_id": "sess_01HX...",
  "tool_use_id": "toolu_01HY...",
  "response": "user's text answer"
}
```

Fields:
- `type` (string, required) — only `"clarification_response"` in
  Round 2. Extensible for future return-channel message types (e.g.
  P3 permission approvals).
- `session_id` (string, required) — must match the subscriber's
  session. Used for defense-in-depth; forge-core's
  `POST /api/sessions/{id}/clarify` already validates the path
  matches, but the subscriber verifies too.
- `tool_use_id` (string, required) — the Anthropic tool-use ID that
  this response corresponds to. Threaded through from
  `ClarificationRequested(tool_use_id)`. Without this, two
  clarifications in the same turn (e.g. "what language? what test
  framework?") would race.
- `response` (string, required) — the user's answer. May be empty
  string (user hit enter with nothing). Max 4 KiB enforced at the
  forge-core endpoint.

**Rejected on receipt** (subscriber discards the message):
- Wrong `session_id` — logs warning, discards.
- Wrong `type` — logs warning, discards.
- `tool_use_id` not in the pending futures map — logs warning,
  discards (stale response after timeout, or replay).
- Malformed JSON — logs warning, discards.

None of these cause session halt — a malicious or buggy publisher
should not be able to crash the agent. Only a legitimate timeout
(§2.9.2.f) halts.

##### 2.9.2.c Subscriber lifecycle (pinned)

**Where the subscriber lives:** ai-worker, inside a new module
`ai-worker/src/openharness/engine/return_channel.py`. One subscriber
per session, owned by the session's `QueryEngine` instance, not a
global singleton.

**Startup:** `QueryEngine.__init__` (or the existing session-cache
path in `api_server.py::_get_or_create_engine`) awaits
`return_channel = await ReturnChannel.open(session_id, redis_client)`,
which:
1. `await redis.pubsub()` — creates a pub/sub client.
2. `await pubsub.subscribe(f"agent:return:{session_id}")`.
3. Spawns a `asyncio.Task` that calls `pubsub.listen()` in a loop and
   dispatches messages to the `ClarificationCoordinator` (see
   §2.9.2.d).
4. Returns the `ReturnChannel` instance.

**Shutdown:** `QueryEngine.close()` (new method, called on LRU
eviction or DELETE `/api/sessions/{id}`):
1. Cancels the listener task.
2. `await pubsub.unsubscribe(f"agent:return:{session_id}")`.
3. `await pubsub.close()`.
4. Cancels any still-pending clarification futures with
   `CancelledError` (the tools waiting on them yield
   `ToolResult(is_error=True, output="session cancelled")`, and the
   agent loop halts via `ErrorEvent`).

**Connection pooling:** one `redis.asyncio.Redis` client at the
process level (already exists in `api_server.py::_get_redis`), shared
across sessions. Each session creates its own `pubsub()` handle (which
multiplexes on the shared connection). No per-session TCP connection.

**Failure during listener loop:** if `pubsub.listen()` raises (Redis
dropped, network partition), the listener task logs with
`logger.exception` and cancels all pending futures with a
`ReturnChannelError`. Tools yield `ToolResult(is_error=True, "return
channel lost")`. The session halts via §2.9.2.f.

##### 2.9.2.d Pause/resume state machine (pinned)

The pause point lives in `RequestClarificationTool`, not in
`_execute_tool_call`. §4.1's tool contract is preserved — the tool is
an `AsyncIterator[StreamEvent | ToolResult]` that may do arbitrary
async work (including awaiting an external future) between yields,
as long as it yields exactly one terminal `ToolResult` on the happy
path, or raises a `SessionHaltError` subclass to halt the session
(see §4.1 updated contract).

First, the `SessionHaltError` base class (lives in
`ai-worker/src/openharness/engine/agent_hooks.py` alongside
`ClarificationCoordinator`):

```python
class SessionHaltError(Exception):
    """Base class for errors that halt the session rather than being
    translated to ToolResult(is_error=True). See §4.1 BaseTool contract
    and §2.9.2.f timeout policy."""

class ClarificationTimeout(SessionHaltError):
    def __init__(self, tool_use_id: str, timeout_seconds: float):
        super().__init__(
            f"clarification timeout after {timeout_seconds}s "
            f"(tool_use_id={tool_use_id})"
        )
        self.tool_use_id = tool_use_id
        self.timeout_seconds = timeout_seconds

class ReturnChannelError(SessionHaltError):
    """Raised when the Redis return channel is lost mid-wait."""
```

A new object `ClarificationCoordinator` lives on the `QueryEngine`
instance (one per session). It holds:

```python
class ClarificationCoordinator:
    def __init__(self):
        self._pending: dict[str, asyncio.Future[str]] = {}
        # key: tool_use_id -> future that resolves to the user's response

    async def wait_for(self, tool_use_id: str, timeout: float) -> str:
        fut = asyncio.get_running_loop().create_future()
        self._pending[tool_use_id] = fut
        try:
            return await asyncio.wait_for(fut, timeout=timeout)
        finally:
            self._pending.pop(tool_use_id, None)

    def deliver(self, tool_use_id: str, response: str) -> None:
        fut = self._pending.get(tool_use_id)
        if fut is None:
            logger.warning("clarification response for unknown tool_use_id: %s", tool_use_id)
            return
        if fut.done():
            logger.warning("clarification response arrived after completion: %s", tool_use_id)
            return
        fut.set_result(response)

    def cancel_all(self) -> None:
        for fut in list(self._pending.values()):
            if not fut.done():
                fut.cancel()
        self._pending.clear()
```

`ToolExecutionContext` gains a `clarification_coordinator:
ClarificationCoordinator | None` field (optional for tools that don't
use it) and a `tool_use_id: str | None` field (also optional —
populated by `_execute_tool_call` before invoking a tool that needs
it). `RequestClarificationTool.execute`:

```python
async def execute(self, arguments, context):
    tool_use_id = context.tool_use_id  # populated by _execute_tool_call
    yield ClarificationRequested(question=arguments.question, tool_use_id=tool_use_id)
    try:
        response = await context.clarification_coordinator.wait_for(
            tool_use_id, timeout=CLARIFICATION_TIMEOUT_SECONDS,
        )
    except asyncio.TimeoutError:
        # Fail-fast: halt the session per §2.9.2.f
        raise ClarificationTimeout(tool_use_id, CLARIFICATION_TIMEOUT_SECONDS)
    except asyncio.CancelledError:
        # Session is being torn down; propagate
        raise
    yield ToolResult(output=response, is_error=False)
```

**Why the future lives on `ClarificationCoordinator` and not
`ToolExecutionContext`:** the subscriber task (owned by
`ReturnChannel`) needs to deliver the response without holding a
reference to the tool's execution context (which is per-call and
disposable). The coordinator is per-session and stable.

**Why this doesn't block the worker thread:** `uvicorn` runs on
`asyncio`. The tool is inside an async generator that yields control
back to the event loop via `await`. Other sessions' requests are
serviced by the event loop while this session's future is pending. No
thread is blocked. This is the one-sentence "how" Harvey asked for.

**Contract with §4.1:** `test_base_tool_contract.py`'s
`test_tool_yields_exactly_one_tool_result` still applies to
`RequestClarificationTool` — the tool yields exactly one
`ClarificationRequested` (StreamEvent) followed by exactly one
`ToolResult`. The test does not need to know that an external
future resolved between the two yields; from the contract test's
perspective it's just a normal async tool that happens to be slow.
§7.3 adds a **contract-test fixture** that auto-delivers a canned
response when it sees `ClarificationRequested` — the test verifies
the tool yields the delivered response as its `ToolResult.output`.

##### 2.9.2.e Event vocabulary: `ClarificationRequested` vs
`ToolExecutionStarted/Completed`

Both pairs of events fire. The tool-execution lifecycle events remain
authoritative for the step ribbon and tool-card UI; the clarification
event is an additional mid-execution stream event that the frontend
renders as an inline input component. Ordering inside a single tool
call:

```
ToolExecutionStarted(tool_use_id, tool_name="request_clarification", tool_input={"question": "..."})
ClarificationRequested(question, tool_use_id)
  [pause — future awaited]
  [user types answer, POST /api/sessions/{id}/clarify publishes to channel]
  [subscriber receives, coordinator delivers, future resolves]
ToolExecutionCompleted(tool_use_id, tool_name="request_clarification", output=<response>, is_error=False)
```

The frontend uses `ClarificationRequested` to render the input box
*under* the tool card. On submit, it POSTs to
`/api/sessions/{id}/clarify`, then waits for
`ToolExecutionCompleted` with matching `tool_use_id` to know the
round-trip finished. No new event beyond `ClarificationRequested` is
required — the Completed event already carries `tool_use_id`.

`ClarificationResponse` is a **channel message**, not a stream event.
It exists only on Redis, never on the SSE outbound stream. The
frontend never sees it directly; the user's typed response travels
via HTTP POST to forge-core, which publishes the channel message.

##### 2.9.2.f Timeout behavior — session halts (no fallback)

**Decision:** On clarification timeout, the entire session halts via
`ErrorEvent`. The tool does NOT return `is_error=True` and let the
agent continue.

Rationale (all three are load-bearing — this section is the policy
call, not a preference):

1. **§2.8 "no fallbacks" / "one code path".** A silent timeout
   returning `ToolResult(is_error=True, output="no user response
   within N seconds")` is a second code path in the agent loop —
   "clarification timed out" vs "user responded". Both need distinct
   frontend handling, distinct tests, distinct agent retry logic.
   §2.8 explicitly bans this.
2. **§2.7 precedent.** Clone failure → `ErrorEvent`, session halts.
   Bash timeout → `ErrorEvent`, session halts. Clarification timeout
   is the same failure class: the user promised to be interactive
   and wasn't. Halt is the established response.
3. **Product correctness.** If the agent continues on timeout, it's
   guessing at defaults against a request the user explicitly said
   was ambiguous. PMs/ops cannot review the result. That's the
   exact anti-pattern §2.9.1's verification hooks are trying to
   prevent. Halting forces the user to start a new session with
   context, which is the correct product behavior.

**Implementation:**

- `ClarificationTimeout(SessionHaltError)` is a new exception type in
  `ai-worker/src/openharness/engine/agent_hooks.py`, a subclass of
  `SessionHaltError` (see §2.9.2.d and §4.1 BaseTool contract).
- `RequestClarificationTool.execute` catches `asyncio.TimeoutError`
  and raises `ClarificationTimeout(tool_use_id, timeout_seconds)`.
- `_execute_tool_call` (§4.1, Round 2 code block) catches
  `SessionHaltError` and yields `ErrorEvent(recoverable=False)`
  followed by a terminal `ToolResultBlock(tool_use_id=...,
  content="session halted: clarification timeout", is_error=True)`.
- `run_agent_loop`'s outer tool-execution block picks up the terminal
  `ToolResultBlock` and emits `ToolExecutionCompleted(tool_use_id,
  tool_name, output=..., is_error=True)` before the `ErrorEvent`
  closes the SSE stream. This means the frontend tool card
  transitions out of the "running" state before the session banner
  appears — unwind is clean.
- Default timeout: **10 minutes** (`CLARIFICATION_TIMEOUT_SECONDS =
  600`). Configurable via env `FORGE_CLARIFICATION_TIMEOUT_SECONDS`.
  5 minutes in the original proposal was too short for "user walked
  away from keyboard"; 10 minutes is the middle ground. This default
  is the only configuration surface for Round 2.
- Frontend renders the `ErrorEvent` as a terminal error banner. The
  clarification input becomes disabled with a "session expired" hint.

**What the frontend does on timeout:** the `ErrorEvent` closes the
SSE stream. The UI shows a red banner "Session ended: clarification
timeout after 10 minutes. Start a new session to continue." No
recovery UI in Round 2.

##### 2.9.2.g `POST /api/sessions/{id}/clarify` endpoint contract

Lives in **forge-core** (`forge-core/internal/module/agent/handler.go`).
forge-core publishes to Redis; ai-worker subscribes. This keeps
ai-worker's surface to the frontend read-only (forward SSE only).

**Request:**
```
POST /api/sessions/{id}/clarify
Content-Type: application/json
Authorization: Bearer <tenant-scoped JWT>

{
  "tool_use_id": "toolu_01HY...",
  "response": "use TypeScript"
}
```

**Validation:**
- JWT must match a user with access to the session's tenant. Uses the
  same auth middleware as other `/api/sessions/*` routes.
- Session must exist and belong to the caller's tenant. Look up via
  the existing session DAO.
- `tool_use_id` must be non-empty and ≤ 128 chars (Anthropic format
  cap).
- `response` must be a string, ≤ 4 KiB. Empty string is allowed.

**Responses:**
- `204 No Content` — published successfully.
- `400 Bad Request` — invalid body shape or size limit exceeded.
- `401 Unauthorized` — JWT missing or invalid.
- `403 Forbidden` — tenant mismatch.
- `404 Not Found` — session ID not in DAO.
- `409 Conflict` — session is not currently awaiting clarification.
  (forge-core checks a session state field; see below.)
- `410 Gone` — session is already completed or halted.

**How forge-core knows if a clarification is pending:** forge-core
does NOT subscribe to Redis. Instead, it tracks a flag
`session.awaiting_clarification` in the session DAO. This flag is set
by ai-worker when it receives a `ClarificationRequested` event in its
own SSE consumer loop (which already exists for
`engine.agent_messages` persistence — see §5.8 LRU cache discussion).
Actually — ai-worker does not own the session DAO. Revision:

**Revised:** forge-core publishes **without checking** whether a
clarification is pending. The subscriber on the ai-worker side handles
the mismatch (rejects on `tool_use_id` not found, logs warning, no
effect). forge-core returns `204` if the Redis `PUBLISH` succeeds.
This is simpler and matches the §2.8 "one code path" rule — no shared
state between forge-core and ai-worker about pending clarifications.

The `409 Conflict` case is removed; forge-core cannot distinguish it
from `204` without polling ai-worker state. The UI uses the forward
SSE stream as the source of truth (only enables the input after
seeing `ClarificationRequested`, disables it after seeing
`ToolExecutionCompleted` with matching `tool_use_id`).

Updated response set: `204`, `400`, `401`, `403`, `404`, `410`. The
`410` case is when the session DAO row is in a terminal state
(completed/halted); forge-core still checks this before publishing
to avoid spamming Redis for dead sessions.

##### 2.9.2.h Concurrency

Multiple sessions may concurrently await clarification. Each has its
own `ClarificationCoordinator` (per-session), its own
`ReturnChannel` subscriber, and its own entry in the Redis pub/sub
namespace. No shared state. Stress test target: 10 concurrent
sessions, each in a clarification wait, each receiving its response
within timeout. Verified in Phase 5a integration tests.

A single session may issue multiple clarifications in one turn —
e.g. `request_clarification("what language?")` then
`request_clarification("what test framework?")`. Both are separate
tool calls with distinct `tool_use_id`s. The coordinator's
`_pending` dict keys on `tool_use_id` so the two don't collide.
However: the agent executes tool calls sequentially in a turn, so
at any given instant only one future is pending per session. Parallel
tool calls within a turn are not in chronos scope (§4.1 shows
sequential tool execution).

**Scope:**
- New Phase 5a (~9 tasks, ~2000 lines) delivers:
  - `return_channel.py` module with `ReturnChannel` class and
    `agent:return:{session_id}` channel schema
  - `agent_hooks.py` `ClarificationCoordinator` class (lives in the
    same module as the agent hooks so the imports stay clean)
  - `ClarificationRequested(question, tool_use_id)` stream event
  - `ClarificationResponse` JSON message shape (dataclass +
    validation helpers in `return_channel.py`)
  - Pause/resume state machine via `ClarificationCoordinator` +
    `asyncio.wait_for` timeout (10 minutes default), halting the
    session on timeout per §2.9.2.f
  - Per-session return-channel subscriber lifecycle in
    `QueryEngine.__init__` / `close()`
  - forge-core `POST /api/sessions/{id}/clarify` endpoint
  - Concurrent-session integration test (10 sessions)
- Phase 4 stream events add `ClarificationRequested(question,
  tool_use_id)`
- Phase 5 adds `RequestClarificationTool` to the agent's tool
  registry via a new `register_interaction_tools` helper
- Phase 5 adds `tool_use_id` to `ToolExecutionContext`
- Phase 5 adds `ClarificationCoordinator` instantiation in
  `_create_engine`
- Phase 6 frontend adds a clarification input component that renders
  below a `ClarificationRequested` event, submits via the new
  `/api/sessions/{id}/clarify` endpoint, and disables input when the
  agent is not waiting
- Phase 1a `/api/workspace/prep` client remains as planned (the
  bidirectional channel is session-scoped, not workspace-scoped)

**Non-goals for Round 2:** the full P3 permission mode (per-call user
approval for writes/bash) is STILL out of scope. Round 2 only adds
bidirectional communication for `request_clarification` — the
infrastructure is reusable for P3 later, but P3 itself is not
implemented. Permission mode stays at `FULL_AUTO`.

#### 2.9.3 `request_review` meta-tool (High 3)

**Problem:** Round 1 rejected A3 (optional reviewer) with the reasoning
"Claude Code is single-agent and outperforms pair_pipeline's
regex-based two-agent." That reasoning compared against a broken
baseline. Claude Code's single-agent design is correct **for engineers
who review their own diffs**. Forge's users are PMs/ops who cannot. An
independent reviewer LLM invocation before `end_turn` catches a class
of "plausible-looking but wrong" errors that self-review-via-tests
misses.

**Decision:** add a `request_review(summary: str) -> str` meta-tool
that the agent can invoke voluntarily.

##### 2.9.3.a Tool behavior (pinned)

`RequestReviewTool` subclasses `SimpleTool` (not `BaseTool`) — it does
not yield mid-execution stream events, it just performs an async LLM
call and returns the verdict. Implementation outline:

```python
class RequestReviewInput(BaseModel):
    summary: str  # what the agent wants reviewed; agent's own summary

class RequestReviewTool(SimpleTool):
    name = "request_review"
    description = "Request an independent reviewer LLM to critique your current work before finalizing."
    input_model = RequestReviewInput

    def __init__(self, model_router: ModelRouter, workspace_dir: Path):
        self._router = model_router
        self._workspace = workspace_dir

    async def _execute_simple(self, arguments, context):
        diff = await self._collect_git_diff()
        prompt = build_reviewer_prompt(
            summary=arguments.summary,
            current_diff=diff,
            original_request=context.original_user_request,
        )
        try:
            response_text = await self._router.generate(
                purpose=Purpose.REVIEW,
                system_prompt=REVIEWER_SYSTEM_PROMPT,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1024,
            )
        except ModelRouterError as exc:
            return ToolResult(
                output=f"reviewer unavailable: {exc}",
                is_error=True,
            )
        return ToolResult(output=response_text, is_error=False)
```

##### 2.9.3.b `Purpose.REVIEW` — internal to the tool only

**Resolving the §4.12 / §8 contradiction:** §4.12 and §8 both say the
`Purpose.REVIEW` branch in `_create_engine` is deleted. **Those
deletions stand.** Round 2 does not reintroduce a `Purpose.REVIEW`
branch to `_create_engine`'s system-prompt selection path. The agent
loop is still single-purpose at the engine level.

Where `Purpose.REVIEW` lives in Round 2:

- The `Purpose` enum in `ai-worker/src/openharness/model_router.py`
  still defines the value (it was not deleted from the enum, only
  from `_create_engine`'s switch).
- `RequestReviewTool.__init__` receives a `ModelRouter` instance
  directly (not a `ModelRouterAdapter` bound to `Purpose.GENERATE`).
- Inside `_execute_simple`, the tool calls
  `await self._router.generate(purpose=Purpose.REVIEW, ...)` — the
  purpose is selected per-call at the `ModelRouter` layer, where it
  controls model selection and routing (e.g. cheaper model for
  reviews, different temperature). This is the existing
  `ModelRouter.generate(purpose, ...)` contract — nothing new.
- The tool is wired in `_create_engine` via
  `register_interaction_tools(tool_registry, model_router,
  workspace_dir)` which constructs both `RequestClarificationTool`
  and `RequestReviewTool` with the shared `ModelRouter` instance.

**Public API stays single-purpose:** `_create_engine` still takes
only a workspace path and a `RunRequest`; it does not accept a
`Purpose` parameter. `QueryEngine` still has one system prompt. The
reviewer is a tool invocation, not an engine mode.

**§8 update:** §8 Step 3 line "Remove all remaining `fix_loop_*`
references, `Purpose.REVIEW` branch in `_create_engine`" stays as
written. The `Purpose.REVIEW` **enum value** is not removed (the tool
still uses it internally); only the `_create_engine` branch is
removed. Clarified in §8 Step 3.

##### 2.9.3.c Reviewer prompt template (pinned)

`build_reviewer_prompt` lives in `ai-worker/src/openharness/engine/
prompts.py` alongside `build_system_prompt`. Signature:

```python
def build_reviewer_prompt(
    summary: str,
    current_diff: str,
    original_request: str,
) -> str:
    """Render the reviewer prompt. All three arguments are required.

    summary:
        The agent's own description of what it built and why it
        believes the work is complete. Passed through from
        RequestReviewInput.summary.

    current_diff:
        Output of `git diff HEAD` run inside the workspace, capped
        at REVIEWER_DIFF_MAX_BYTES (default 32 KiB). If the diff
        exceeds the cap, it is truncated with a "<diff truncated
        at 32KiB>" marker at the end.

    original_request:
        The user's original message that kicked off the session.
        Pulled from QueryEngine._messages[0].content.
    """
```

The output of `build_reviewer_prompt` is a plain string (the user
message the reviewer LLM will see). The reviewer system prompt is a
separate constant `REVIEWER_SYSTEM_PROMPT` in the same file.

**System prompt (pinned):**

```
You are a senior engineer reviewing another AI agent's work on a
user's codebase. You have no tools. You see only: (1) the user's
original request, (2) the AI agent's own summary of what it built,
(3) the git diff showing the agent's changes.

Your job: judge whether the agent's work actually does what the user
asked. Focus on:
- Intent mismatch: the diff does something subtly different from the
  user's request (wrong field name, wrong endpoint, wrong default)
- Missing cases: the user asked for X including edge cases, the diff
  handles X but not the edge cases
- Obvious bugs: null dereferences, off-by-one, unsafe SQL, missing
  error handling in load-bearing paths
- Non-goals: the diff adds functionality the user did not ask for

Do NOT flag: coding style, naming preferences, architectural taste,
"could be more elegant", "might be slow", "should add tests" (unless
tests are part of the user's request).

Respond with EXACTLY one of these formats, on a single line, no
preamble:

    APPROVE
    REVISE <what to change>
    REJECT <why it's fundamentally wrong>

Your verdict is parsed by regex. Any text before the verdict line
or after it will be ignored.
```

**User message template** (what `build_reviewer_prompt` returns):

```
## User's original request
{original_request}

## Agent's summary of work
{summary}

## Git diff
{current_diff}

Review the above and respond with APPROVE / REVISE / REJECT.
```

##### 2.9.3.d Verdict parsing

The tool parses the reviewer's response with a simple regex:

```python
VERDICT_PATTERN = re.compile(
    r"^(APPROVE|REVISE|REJECT)(?:\s+(.*))?$",
    re.MULTILINE,
)

def parse_verdict(text: str) -> tuple[Literal["APPROVE", "REVISE", "REJECT"], str]:
    for line in text.splitlines():
        match = VERDICT_PATTERN.match(line.strip())
        if match:
            verdict = match.group(1)
            details = (match.group(2) or "").strip()
            return verdict, details
    raise ReviewerParseError(
        f"Reviewer response did not contain a verdict line: {text[:200]!r}"
    )
```

If parsing fails, the tool returns `ToolResult(is_error=True,
output=f"reviewer output could not be parsed: {exc}")`. No retry —
the agent sees the error and decides whether to retry the
`request_review` call or continue without a verdict.

##### 2.9.3.e Git diff collection

`RequestReviewTool._collect_git_diff` runs `git diff HEAD` inside the
workspace **directly via `asyncio.create_subprocess_exec`**, NOT
through `BashTool`/bwrap. Rationale:

- The reviewer needs read-only access to the working tree state.
  `git diff HEAD` is deterministic and safe — no user-controlled
  input, no shell expansion.
- Running it through `BashTool` would emit spurious
  `ThinkingStarted`/`ThinkingStopped` events and a tool card for an
  internal operation the user does not care about.
- bwrap's network/filesystem restrictions do not apply — `git diff`
  reads the workspace which is already accessible.

**Exemption from §4.6 bwrap policy:** `git diff HEAD` is one of
exactly two subprocess calls in Round 2 that bypass bwrap (the other
is `git` commands run by the workspace manager — §3.5). Both are
hardcoded, parameter-less, read-only git invocations. No other
subprocess may bypass bwrap without a spec amendment. The
`_collect_git_diff` method's argv is literally
`["git", "diff", "HEAD"]` with no parameters — no shell, no user
input, no format strings.

Output capped at `REVIEWER_DIFF_MAX_BYTES = 32_768`. If exceeded,
truncated and marked. `cwd` is the session's workspace directory.

##### 2.9.3.f Model selection for `Purpose.REVIEW`

The `ModelRouter` picks the model based on `Purpose`. Round 2 does
not hardcode which model serves `Purpose.REVIEW`; that's the router's
existing responsibility. The only Round 2 requirement is that the
router has a registered model for `Purpose.REVIEW` — verified at
`ModelRouter` construction time with a fail-fast assertion. If no
model is registered, `_create_engine` fails and the agent does not
start.

**Rationale for fail-fast:** §2.8 no-fallbacks. If the reviewer model
isn't configured, we do not silently disable `request_review` — the
agent would think it has a tool that doesn't work.

##### 2.9.3.g Tests

- Mock the `ModelRouter.generate` call so tests don't pay LLM cost.
- Test: `build_reviewer_prompt` substitutes all three inputs
  correctly (substring invariants, same style as existing
  `build_system_prompt` tests).
- Test: `parse_verdict` on `"APPROVE"`, `"REVISE foo bar"`,
  `"REJECT not at all"`, `"some text\nAPPROVE\nmore text"`, and
  unparseable input.
- Test: `_collect_git_diff` runs in a real git workspace fixture,
  returns the expected diff.
- Test: timeout on `git diff` (> 30s) raises and the tool returns
  `is_error=True`.
- Test: `RequestReviewTool.execute` on a mocked router returns the
  verdict text unchanged as `ToolResult.output`.
- Contract test: auto-covered by the 12-tool (now 14-tool) contract
  suite; the tool's `input_model` has `summary` which the contract
  test's `_make_valid_arguments` must support. Update the contract
  fixture accordingly.

**System prompt update (pinned):** §5.2's "How to work" section gains
a new bullet instructing the agent when to call `request_review`:

> 7. At major milestones — before `end_turn`, before a git commit that
>    represents a user-visible feature boundary — call
>    `request_review` with a short summary of what you built and why
>    you believe it's correct. The reviewer is an independent LLM that
>    sees your diff and the user's original request. Act on the
>    verdict: APPROVE → proceed, REVISE → address the listed items,
>    REJECT → reconsider the approach. You are not required to invoke
>    the reviewer on every turn; use judgment.

**Explicit trade-off acknowledged:** this partially walks back Round 1
Q2 A2's rejection of pair_pipeline's 2-agent model. The difference:
Round 1's A2 killed the *mandatory* 2-agent outer loop.
`request_review` is an *optional* meta-tool. The agent is still a
single agent; the reviewer is a LLM call the agent makes, not a
second permanent agent instance.

**Scope:** Phase 5 adds `RequestReviewTool` alongside
`RequestClarificationTool`. Uses the existing `ModelRouter` with a
different `Purpose.REVIEW` (the enum value stays, only the
`_create_engine` branch is deleted). Reviewer prompt lives in
`prompts.py` as `build_reviewer_prompt(summary, current_diff,
original_request)` plus the `REVIEWER_SYSTEM_PROMPT` constant.

#### 2.9.4 Phase 1 split: 1a (minimal) + 1b (deploy keys) (High 4)

**Problem:** Round 1 Phase 1 was 13 tasks including full ed25519
deploy key lifecycle + GitHub upload API + AES-GCM encryption + key
rotation interfaces. For a solo dev MVP this is gold-plating. The
safety gain over HTTPS+token (using the existing `injectToken` helper
that's already in `manager.go`) is marginal for the first production
version, and the 2-week time cost is real.

**Decision:** split Phase 1 into:

- **Phase 1a — Workspace Minimal** (~6-7 tasks): `StateRepo` state
  machine + `EnsureReady` core loop + `RealGitRunner` via HTTPS+token
  (retains the existing `injectToken` pattern temporarily) + prep RPC
  client + caller migration (both activity files + agent service) +
  main.go wiring. Unblocks Phase 5 agent operation.
- **Phase 1b — Deploy Keys** (~5-6 tasks): `DeployKeyRepo` + ed25519
  generation + GitHub deploy-key upload API + migration of git auth
  from HTTPS+token to SSH + key rotation interface. Can run **in
  parallel with or after Phase 5** because nothing in Phase 5/6/7
  depends on the auth mechanism (they all use `EnsureReady` which
  abstracts over it).

Phase 1a completion unblocks Phase 5 immediately. Phase 1b delivery
before Phase 7 production deploy is the ideal sequence, but if 1b
slips, the MVP ships on HTTPS+token and 1b becomes a follow-up
release.

**Structural impact:** `phase-1-workspace.md` is split into two files.
The existing 13-task structure carries over mostly intact — tasks
1.1-1.6 go to 1a (state + ensure + git + prep + lookup + main.go),
tasks 1.2-1.4 (deploy keys + GitHub upload + SSH git wrapper) move to
1b. Phase 1a's `RealGitRunner` uses a temporary HTTPS+token path that
Phase 1b replaces.

##### 2.9.4.a Tagged sections in §3 (authoritative mapping)

The existing §3 (workspace manager layer) was written assuming a
single Phase 1 that migrated to SSH deploy keys wholesale. With the
1a/1b split, several §3 subsections apply only to Phase 1b. The table
below is authoritative for the Round 2 plan writer:

| §3 subsection | Phase 1a scope | Phase 1b scope |
|---|---|---|
| §3.1 Module responsibility | All of it | (no change) |
| §3.2 Current-state baseline / extension plan | State DAO + ensure state machine. `RealGitRunner` implemented with HTTPS+token (temporary `injectToken` path retained). | Replace `RealGitRunner` to use SSH deploy keys. Delete `injectToken`. |
| §3.3 New files inside `workspace/` | `state.go`, `ensure.go`, `prep.go`, `state_test.go`, `ensure_test.go`. `git.go` exists but uses HTTPS+token. | `keys.go` (ed25519 gen, AES-GCM, GitHub upload), `keys_test.go`. `git.go` rewritten to use SSH. |
| §3.4 Shared workspace volume | All of it — volume mount + path config | (no change) |
| §3.5 Auth migration: token → deploy key | **Deferred to Phase 1b in full.** Phase 1a keeps `injectToken(repoURL, token)` exactly as it is today in `manager.go`. No SSH URL conversion. `ProjectLookup` in Phase 1a returns `(httpsURL, token, defaultBranch)` — same shape as today. See §2.9.4.b for the transitional interface. | Implements the entire §3.5 migration: delete `injectToken`, add `gitCommand()` helper, convert HTTPS URLs to SSH via `toSSHURL(httpsURL)`. `ProjectLookup` becomes `(sshURL, defaultBranch)` — Phase 1b is a **breaking change** to the interface signature documented and planned in §2.9.4.b. |
| §3.6 Data model — `engine.workspaces` | Phase 1a creates this table | (no change) |
| §3.6 Data model — `engine.project_deploy_keys` | **Phase 1a does NOT create this table.** It stays unused. | Phase 1b creates the `engine.project_deploy_keys` table migration and all code that reads/writes it. |
| §3.7 `EnsureReady` state machine | Phase 1a implements the full state machine using HTTPS+token auth inside `RealGitRunner` | (no change — state machine is auth-independent; only `RealGitRunner`'s internals swap) |
| §3.8 SSH deploy key lifecycle | **Deferred to Phase 1b entirely.** | All of it. |
| §3.9 Dependency pre-install | All of it — independent of auth mechanism | (no change) |
| §3.10 Path plumbing to ai-worker | All of it | (no change) |
| §3.11 Concurrency semantics | All of it | (no change) |
| §3.12 Failure-mode matrix row: "Deploy key upload to GitHub fails" | N/A (deploy keys don't exist yet) | Row becomes active. |
| §3.12 Failure-mode matrix row: "GitHub PAT revoked" | **Phase 1a addition** — if `injectToken`'s token is rejected by GitHub, clone fails with `last_error="github_auth_failed"`. Row is deleted again in Phase 1b when PAT usage ends. | Row deleted. |
| §Appendix B Files touched | `keys.go`, `keys_test.go`, the `project_deploy_keys` migration, the `EnsureClone`→SSH deletion of `injectToken`, and the `manager.go` SSH rewrite are all **Phase 1b**. Everything else in Appendix B under workspace is **Phase 1a**. | — |

##### 2.9.4.b `ProjectLookup` interface — the breaking change

Phase 1a and Phase 1b use **two different `ProjectLookup` signatures**
because Phase 1a needs a token and Phase 1b doesn't. The plan must
either (a) version the interface, (b) use an adapter, or (c) do a
hard cutover in Phase 1b.

**Decision:** (c) hard cutover. Phase 1a introduces `ProjectLookup` as:

```go
type ProjectLookup interface {
    LookupProject(ctx context.Context, tenantID, projectID int64) (ProjectInfo, error)
}

type ProjectInfo struct {
    RepoURL       string  // HTTPS URL in 1a, SSH URL in 1b
    AccessToken   string  // populated in 1a, empty in 1b
    DefaultBranch string
}
```

Phase 1b rewrites `ProjectInfo` to drop `AccessToken` and renames
`RepoURL` to `SSHURL`. All callers migrate in the same Phase 1b
commit. This is a narrow surface (one interface, one struct, ≤5
callers), so a hard cutover is cheaper than versioning.

The Phase 1b rewrite is a breaking change **inside the monorepo** —
no external clients consume `ProjectLookup`. Hard cutover is safe.

##### 2.9.4.c Phase 1a temporary path retained in one place

Phase 1a's `RealGitRunner` retains `injectToken` — but only inside
`workspace.git.go` (Phase 1a version). No other Phase 1a file
imports `injectToken`. Phase 1b's first task is to delete the
Phase 1a `git.go` wholesale and replace it with the SSH version.
This keeps the "temporary code" surface area to one file.

**Out-of-scope for Round 2:** key rotation implementation. Deploy keys
are generated once and reused; rotation is a follow-up project.

##### 2.9.4.d Phase 1b gating for public deployment

Phase 1b is **not optional** for public deployment. The §3.8 security
rationale ("deploy keys live in forge-core for prompt-injection
containment") still holds. If Phase 1b is deferred indefinitely, the
MVP can ship for solo-dev / internal testing but public deployment is
blocked until Phase 1b lands. Round 2's `index.md` must call this
out explicitly.

#### 2.9.5 Execution / documentation consequences

The four decisions above have cascading effects on both the Round 1
plan files **and** this spec itself. All edits are bundled into
Round 2.

##### 2.9.5.a Spec self-edits (this document)

§2.9 is a non-terminal amendment. Several earlier sections must be
updated in lockstep to avoid contradictions:

| Section | Round 2 edit |
|---|---|
| §3.5 Auth migration | Tagged "Phase 1b entirely" per §2.9.4.a. Add a reference to §2.9.4.a at the top of §3.5 so readers know 1a defers the migration. |
| §3.6 Data model | `engine.project_deploy_keys` table migration tagged "Phase 1b". |
| §3.8 SSH deploy key lifecycle | Tagged "Phase 1b entirely". |
| §3.12 Failure-mode matrix | Add row "GitHub PAT revoked (Phase 1a only)". Tag row "Deploy key upload fails" as Phase 1b. |
| §4.12 `_create_engine` | Update code block to construct empty `AgentHookRegistry` and `ClarificationCoordinator`, and to call `register_interaction_tools` for the two new meta-tools. Note that the `Purpose.REVIEW` branch is still removed; the `Purpose.REVIEW` enum value is retained for `RequestReviewTool` internal use. |
| §5.1 `run_agent_loop` changes | Note that the loop now invokes `registry.pre_turn` / `registry.post_turn` around each turn, and `registry.pre_tool_call` inside `_execute_tool_call`. |
| §5.2 System prompt | Add slot-substitution mechanism (`{{slot_name}}` replacement). Add the "at major milestones, call request_review" bullet from §2.9.3.g. |
| §5.3 Event vocabulary | No longer "final" — add `ClarificationRequested(question, tool_use_id)` and `ErrorEvent` note for clarification timeout. Rename section from "final" to "Event vocabulary". |
| §7.1 Adversarial tests | Add 3 new rows: clarification timeout halts session; clarification response with wrong session_id rejected; clarification response with unknown tool_use_id rejected. |
| §7.2 Unit tests per tool | Add `RequestClarificationTool`, `RequestReviewTool` rows. |
| §7.3 Contract tests | Add a test fixture for `RequestClarificationTool` — auto-delivers a canned response when the tool yields `ClarificationRequested`. The single-`ToolResult` contract still holds; the test just needs a delivery mechanism. Document the fixture pattern for the plan writer. |
| §7.4 Agent loop integration tests | Add 2 new scenarios: clarification round-trip (happy path with injected delivery) and clarification timeout (session halts). Add 1 scenario: `request_review` tool with mocked `ModelRouter.generate`. Add 1 scenario: `AgentHookRegistry.pre_tool_call` blocks a tool. |
| §7.6 E2E smoke test | Rewrite assertions to cover a clarification round-trip: the fixture project's instructions include an ambiguity that forces the agent to call `request_clarification`; a test harness publishes to the return channel on seeing `ClarificationRequested`; assertions check that the agent received the injected response and continued. Fixture setup is new, not "minor". |
| §7.8 Observability | Add `agent.clarification_requested`, `agent.clarification_responded`, `agent.clarification_timeout`, `agent.review_requested`, `agent.review_verdict` log points. Document the structured fields (session_id, tool_use_id, latency_ms). |
| §8 Step 3 | Keep the line "`Purpose.REVIEW` branch in `_create_engine` removed". Add a parenthetical "(the `Purpose.REVIEW` enum value is retained for `RequestReviewTool` internal use — see §2.9.3.b)". |
| Appendix B Files touched | Add new files: `ai-worker/src/openharness/engine/agent_hooks.py`, `ai-worker/src/openharness/engine/return_channel.py`, `ai-worker/src/openharness/tools/interaction_tools.py`, `forge-core/internal/module/agent/clarify_handler.go`, corresponding tests. Tag deploy-key files as Phase 1b per §2.9.4.a. |

##### 2.9.5.b Plan file deltas (`docs/plans/chronos-2026-04-09/`)

| Round 1 file | Round 2 change |
|---|---|
| `index.md` | Rewritten — 9 phases, new dependency graph, Round 2 status note → replaced with "delivered" when Round 2 plan writing completes |
| `phase-1-workspace.md` | **Delete.** Split into `phase-1a-workspace-minimal.md` + `phase-1b-deploy-keys.md` |
| `phase-4-bash-events.md` | Add `ClarificationRequested(question, tool_use_id)` event + serialization (Task 4.9) |
| new `phase-5a-bidirectional-rpc.md` | ~9 tasks, ~2000 lines: `return_channel.py` module, `ClarificationCoordinator`, forge-core `/api/sessions/{id}/clarify` endpoint, integration tests |
| `phase-5-agent-loop.md` | Add tasks for `AgentHookRegistry` wiring (5.8), hook contract tests (5.9), `RequestClarificationTool` (5.10), `register_interaction_tools` helper (5.11), `RequestReviewTool` (5.12), `build_reviewer_prompt` (5.13), system prompt update (5.14). ~+800 lines |
| `phase-6-frontend.md` | Add clarification input component (Task 6.10) + clarification state machine integration (Task 6.11). ~+400 lines |
| `phase-7-deploy.md` | Update smoke test to include a clarification round-trip assertion per §7.6 rewrite |

Round 2 plan size estimate: ~21,000 lines across 11 files (9 phases +
`index.md` + `deploy-runbook.md` + `retro.md`), ~76 tasks total.

Round 2 must re-run the spec review loop (spec-document-reviewer
subagent) on this updated spec before plan rewriting starts. After
plan rewriting completes, Round 2 must re-run autoplan CEO/Design/Eng/
DX reviews before declaring the plan final.

---

## 3. Workspace manager layer

**Extending the existing `forge-core/internal/workspace/` package** — not
creating a parallel module.

### 3.1 Module responsibility

Own the physical code artifact for each project: clone, pull, deploy
key lifecycle, dependency pre-install coordination, reset-on-new-session.
Does **not** own agent session state, conversation history, or tool
execution — those stay in `agent/`.

### 3.2 Current-state baseline and the extension plan

A `workspace.Manager` already exists at `forge-core/internal/workspace/manager.go`
and is wired into `main.go:122`, consumed by `project.NewService` (line
126), the async Worker (line 154), and `agent.NewService` (line 245).
It provides:

- `NewManager(root)` — defaults to `/data/forge/workspaces`
- `ProjectDir(tenantID, projectID) → tenant-{N}/project-{N}/repo`
- `TaskDir(tenantID, projectID, taskID) → tenant-{N}/project-{N}/tasks/task-{N}`
- `EnsureClone(ctx, tenantID, projectID, repoURL, token, defaultBranch) → dir, error` — HTTPS + token auth via `injectToken()`
- `CreateWorktree(ctx, ..., branchName) → dir, error` — git worktree per task
- `WriteFiles(taskDir, []FileToWrite) → error`
- `CleanupTask(ctx, ...) → error`

The existing `EnsureClone` is a straightforward "clone if missing, pull
if present" with no state table, no error state, no deploy-key logic —
it uses HTTPS + GitHub token injected into the URL. The existing path
convention `tenant-{N}/project-{N}/repo` is the one the other code paths
(project service, worker) already assume, and we keep it. The spec's
original `{tenant_id}/{project_id}/` was a wrong guess by the spec author
and is corrected here.

**Extension, not replacement:**
- Keep `ProjectDir`, `TaskDir`, `CreateWorktree`, `WriteFiles`,
  `CleanupTask`, `FileToWrite` as-is. They are used by other modules
  (project service, async worker) that are out of scope for this spec.
- **Replace** `EnsureClone` with a new `EnsureReady` method that owns
  the state machine, deploy-key lifecycle, and dependency pre-install.
  The old signature `EnsureClone(..., repoURL, token, defaultBranch)`
  is deleted; callers are migrated to `EnsureReady(ctx, tenantID, projectID)`
  which reads repoURL and defaultBranch from the project record via an
  injected `ProjectLookup` interface, and obtains auth via deploy keys
  instead of tokens.
- **Replace** the top-level `injectToken()` helper — HTTPS+token auth
  is removed entirely in favor of SSH deploy keys. Every call site is
  audited and migrated (see the caller migration below).

**Caller migration (verified against repo HEAD on 2026-04-09):**

`EnsureClone` is called from exactly two sites:

- `forge-core/internal/temporal/activity/build_activities.go:96` —
  the build activity clones the project repo before running build
  logic. Migrates to `EnsureReady(ctx, tenantID, projectID)`. The
  signature loses `repoURL`, `token`, and `defaultBranch` because
  workspace resolves those internally via `ProjectLookup`.
- `forge-core/internal/temporal/activity/devops_activities.go:134` —
  the devops activity clones before deploy logic. Same migration.

Both activity files are in the Worker runtime (`worker.NewWorker(...)`
at `main.go:154`), which already has a `workspaceMgr` injected. The
worker itself doesn't need wiring changes; the two activity files
update their call sites and delete their `token`/`repoURL` fields from
the activity inputs.

`project.NewService` **does not call `EnsureClone`**. It takes a
`WorkspaceProvider` interface (defined at
`forge-core/internal/module/project/service.go:34`) that only exposes
`ProjectDir(tenantID, projectID) string` for local file browsing. The
project module has no dependency on the clone lifecycle. No migration
needed — project registration has always been workspace-free, which
aligns with §2.7's "lazy-created when the user sends the first
message".

`agent.NewService` currently takes `workspaceMgr *workspace.Manager`
(per `main.go:245`) but does not yet call `EnsureClone` either — the
wiring landed in commit 280b88f but the service still passes through
to ai-worker without triggering clone. This spec adds the
`EnsureReady` call at the start of each session in
`forge-core/internal/module/agent/service.go`.

After migration:
- `EnsureClone` is deleted from `manager.go`.
- `injectToken` is deleted from `manager.go`.
- All three callsites named above are the only ones that need code
  changes. Compile-time breakage from deleting `EnsureClone` will
  surface any missed migration.

### 3.3 New files inside `forge-core/internal/workspace/`

| File | Status | Responsibility |
|---|---|---|
| `manager.go` | Modified | `EnsureClone` replaced by `EnsureReady`, `injectToken` removed, keep `ProjectDir`/`TaskDir`/`CreateWorktree`/`WriteFiles`/`CleanupTask` |
| `manager_test.go` | Modified | Remove `EnsureClone` tests, add `EnsureReady` tests |
| `state.go` | **New** | `Workspace`, `DeployKey`, `WorkspaceStatus` structs, `engine.workspaces` + `engine.project_deploy_keys` DAO |
| `ensure.go` | **New** | `EnsureReady(ctx, tenantID, projectID) (*Workspace, error)` state machine |
| `git.go` | **New** | Thin wrapper over `os/exec` for git ssh-aware commands |
| `keys.go` | **New** | ed25519 generation, AES-GCM encrypt/decrypt, GitHub deploy key upload |
| `prep.go` | **New** | HTTP client that POSTs to ai-worker's `/api/workspace/prep` |
| `state_test.go`, `ensure_test.go`, `keys_test.go` | **New** | Unit + integration tests |

**Not using go-git:** go-git's HTTPS and SSH support is good enough for
reads but has known edge cases around auth agent forwarding, symlink
handling, and large repos. Keeping the existing pattern of shelling out
to system `git` via `os/exec` (as `manager.go` already does) is worth
the small syscall overhead.

### 3.4 Shared workspace volume (forge-core host ↔ ai-worker container)

This is a cross-process detail the spec must nail down: forge-core and
ai-worker need **read/write access to the same workspace tree** from
different namespaces.

**Current deployment layout** (from `docker-compose.dev.yml:60-66`):

- `forge-ai-worker` container has:
  ```
  environment:
    - FORGE_WORKSPACE_ROOT=/data/forge/workspaces
  volumes:
    - ${FORGE_WORKSPACE_ROOT_HOST:-./workspaces}:/data/forge/workspaces
  ```
- `forge-core` runs on the **host** (not in the compose network —
  ai-worker reaches it via `host.docker.internal:8080`), and reads
  `FORGE_WORKSPACE_ROOT` from its own env (defaults to `./workspaces`
  relative to the working directory at startup).

So:

- forge-core sees paths like `./workspaces/tenant-1/project-25/repo`
  (host-relative)
- ai-worker sees paths like `/data/forge/workspaces/tenant-1/project-25/repo`
  (container-absolute)
- Both point to the **same files on disk**, via the bind mount. Any
  file forge-core writes (e.g., `git clone` output) is immediately
  visible inside ai-worker, and vice versa.

**Protocol for the `workspace_path` RPC field:**

`RunRequest.workspace_path` is always a **relative path** fragment,
unambiguous across namespaces. It looks like:

```
tenant-1/project-25/repo
```

- forge-core's agent service computes this by calling
  `filepath.Rel(cfg.WorkspaceRoot, workspace.ProjectDir(t, p))`.
- ai-worker resolves it by joining with its own `FORGE_WORKSPACE_ROOT`.
- Both sides agree on the layout scheme (`tenant-{N}/project-{N}/repo`).

This is the Stream 4c protocol already implemented in
`ai-worker/src/api_server.py:_route_and_stream`:

```python
ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
resolved = os.path.join(ws_root, req.workspace_path)
if os.path.isdir(resolved):
    ...
```

No changes to that resolution logic. The spec's §5.7 and §3.10
references to `workspace_path` use this relative-fragment contract.

**Production deployment note:** If forge-core ever moves into a
container in the same compose network, both containers bind the same
host volume at `/data/forge/workspaces` and the scheme still works
without code change — the relative fragment is the constant.

### 3.5 Auth migration: token → deploy key

> **Round 2 phasing note:** the migration in this section is deferred
> to **Phase 1b entirely**. Phase 1a retains the existing
> `injectToken(repoURL, token)` path unchanged. `ProjectLookup` in
> Phase 1a returns `(httpsURL, token, defaultBranch)`; Phase 1b
> rewrites it to `(sshURL, defaultBranch)` as a breaking change in
> the same commit that deletes `injectToken`. See §2.9.4.a for the
> full 1a/1b task mapping and §2.9.4.b for the `ProjectLookup`
> interface cutover.

Currently `manager.EnsureClone` uses `injectToken(repoURL, token)` to
stuff a GitHub PAT into the HTTPS URL. In **Phase 1b** this is
replaced wholesale:

- `injectToken` is **deleted**.
- All git invocations go through a new `gitCommand()` helper in `git.go`
  that sets `GIT_SSH_COMMAND` to point at the project's deploy-key
  tempfile (see §3.8) and uses the SSH URL form
  `git@github.com:{owner}/{repo}.git`.
- The `ProjectLookup` interface, injected into the workspace Manager
  at construction time, returns `(sshURL, defaultBranch)` for a given
  `(tenantID, projectID)`.
- Existing stored `repoURL` values in the `project` table are HTTPS
  URLs (`https://github.com/{owner}/{repo}.git`); the workspace package
  converts them to SSH form via a one-line helper
  `toSSHURL(httpsURL)`. No database migration needed.
- If a project row has a URL that can't be converted (non-github, weird
  scheme), `EnsureReady` returns `error` status with
  `last_error="repo_url_unsupported"`. Out of scope to support arbitrary
  git hosts this release — GitHub only.

### 3.6 Data model

Migrations added as new files in `forge-core/migrations/` (or the
project's convention for goose/Flyway):

```sql
-- engine.workspaces
CREATE TABLE engine.workspaces (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL,
    host_path       TEXT NOT NULL,
    container_path  TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'error')),
    last_synced_at  TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, project_id)
);

CREATE INDEX idx_workspaces_tenant_project ON engine.workspaces(tenant_id, project_id);

-- engine.project_deploy_keys — Phase 1b ONLY (see §2.9.4.a)
-- Phase 1a does not create this table. Phase 1b's first migration
-- adds it as part of the deploy-key rollout.
CREATE TABLE engine.project_deploy_keys (
    project_id      BIGINT PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    public_key      TEXT NOT NULL,
    private_key_enc BYTEA NOT NULL,  -- AES-GCM: nonce || ciphertext || tag
    key_type        TEXT NOT NULL DEFAULT 'ed25519',
    github_key_id   BIGINT,          -- GitHub deploy key ID (nullable for non-GitHub)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.7 `EnsureReady` state machine

Three persisted states only: `pending` | `ready` | `error`. Resync is
an **in-memory transition**, not a fourth row state.

```
         ┌──────────┐
         │ no record│
         └────┬─────┘
              │ first call: INSERT status='pending'
              │             (+ PG advisory lock on (tenant_id, project_id))
              ▼
         ┌──────────┐      clone | prep fails
         │ pending  │ ─────────────────┐
         └────┬─────┘                  │
              │ clone ok               │
              │ deps preinstall ok     │
              ▼                        ▼
         ┌──────────┐               ┌──────────┐
         │  ready   │               │  error   │
         └────┬─────┘               └────┬─────┘
              │                          │ next call wipes dir,
              │ next call on new         │ re-runs clone + prep
              │ session:                 │ from 'pending' transition
              │   1. fetch               │
              │   2. reset --hard        │
              │   3. row stays 'ready'   │
              │      (update updated_at) │
              │                          │
              └────────────┐             │
                           │             │
                           ▼             ▼
                     ┌──────────┐  (row state may go
                     │  ready   │   back to 'pending'
                     │ (fresh   │   during re-clone if
                     │  commit) │   wipe is needed)
                     └──────────┘
```

**Row state semantics:**

- `pending`: actively being created or re-created. Other callers must
  wait on the PG advisory lock.
- `ready`: workspace directory exists, has git metadata, deps prepared.
  Further `EnsureReady` calls may do fetch+reset without changing row
  state.
- `error`: last attempt failed. Next `EnsureReady` call transitions
  row back to `pending`, wipes directory, re-runs clone + prep. Session
  that sees `error` surfacing (first call to fail) emits `ErrorEvent`
  and halts; next session's call may recover.

**Advisory lock protocol:**

```sql
SELECT pg_advisory_xact_lock(hashtext('workspace:' || $tenant_id || ':' || $project_id));
-- then read/insert/update engine.workspaces row
-- lock auto-released at transaction end
```

Using `pg_advisory_xact_lock` rather than `pg_try_*` because we want
concurrent callers to block, not fail.

- **`pending` row is persisted** so concurrent callers observe it.
- **PG advisory lock** on `(tenant_id, project_id)` serializes concurrent
  `EnsureReady` calls: the second caller blocks until the first
  transitions out of `pending`, then sees `ready` and returns.
- **`error` state is not a dead end.** The next call retries from
  scratch (delete workspace directory, re-clone). This is the "Agent
  emits ErrorEvent, halt this session" semantics: the halt is scoped to
  the current session, not to the project.
- **New-session resync** is driven by the Go agent service, which
  decides "is this a new session?" and passes a flag. The workspace
  service does not know about sessions.

### 3.8 SSH deploy key lifecycle

> **Round 2 phasing note:** this entire subsection is **Phase 1b
> only**. Phase 1a does not implement any deploy-key lifecycle. See
> §2.9.4.a for the task mapping.

**Generation (first EnsureReady call for a project with no key row):**

1. Generate ed25519 keypair in Go using `crypto/ed25519` + OpenSSH
   format via `golang.org/x/crypto/ssh`.
2. Public key comment: `forge-deploy-{tenant}-{project}-{epoch}`.
3. Call existing GitHub adapter (under `forge-core/internal/module/adapter/github`)
   to upload public key via `POST /repos/{owner}/{repo}/keys`,
   `read_only=false` (forward-compatible with future push), store
   returned `github_key_id`.
4. Encrypt private key with AES-GCM:
   - Derive key: `HKDF(SHA256, FORGE_SECRETS_MASTER_KEY, salt="forge-deploy-key-v1", info="")`
   - Generate random 12-byte nonce
   - Pack storage format: `nonce(12) || ciphertext || tag(16)`
5. Insert row into `project_deploy_keys`.

**Usage (each clone/fetch/reset):**

1. Decrypt private key into memory.
2. Write to tempfile at `/tmp/forge-key-{random}.pem`, mode 0600.
3. Run git command with environment:
   ```
   GIT_SSH_COMMAND="ssh -i /tmp/forge-key-{random}.pem \
                        -o StrictHostKeyChecking=accept-new \
                        -o IdentitiesOnly=yes \
                        -o UserKnownHostsFile=/tmp/forge-known-hosts-{tenant}"
   ```
4. `defer os.Remove(tempfile)` — tempfile is deleted on all paths,
   including panic.
5. Decrypted plaintext is zeroed in memory (Go `runtime.GC()` can't
   guarantee this, but `crypto/subtle.ConstantTimeCompare`-style hygiene
   applies for the tempfile cleanup).

**Why not `StrictHostKeyChecking=no`:** MITM resistance. On first
connect, we accept the host key and record it; on any subsequent
divergence the connection is rejected. The known_hosts file is
per-tenant so one tenant's MITM doesn't poison another's.

**Why keys live in forge-core, not ai-worker:** Prompt-injection
containment. If the agent in ai-worker is successfully manipulated to
do something malicious, it cannot exfiltrate a deploy key it never had
access to. All git operations run in forge-core's address space.

**Master key source:** `FORGE_SECRETS_MASTER_KEY` environment variable,
32 bytes base64-encoded. An internal `secrets.Encrypt(plaintext) / Decrypt(ciphertext)`
service is the single consumer, so future replacement with Vault/KMS
touches only that service.

**Key rotation:** Not implemented in this release. The data model
supports it (update `private_key_enc`, call GitHub API to delete old
`github_key_id` and upload new one, update row). Manual operational
procedure for now.

### 3.9 Dependency pre-install

Because the `bash` sandbox blocks network (§4.8), `npm install` / `go
mod download` / `mvn dependency:go-offline` cannot run from agent bash
calls. Dependencies must be pre-installed at workspace create time.

**Flow:**

1. Workspace service completes clone.
2. Workspace service calls ai-worker's `POST /api/workspace/prep` over
   HTTP with `{tenant_id, project_id, workspace_path}`.
3. ai-worker's handler detects language via existing
   `detect_language(project_dir)` machinery from
   `ai-worker/src/openharness/skills/languages/`.
4. For the detected language, runs the profile's declared prep command
   (`go mod download`, `mvn dependency:go-offline -B`, `npm ci`, etc.)
   in the **ai-worker container** (which has network) in the workspace
   directory.
5. Returns success/failure.
6. Workspace service updates `workspaces.status` accordingly.

**Why ai-worker runs prep, not forge-core:** The ai-worker base image
has the language toolchains installed (go, maven, npm, python, etc.);
forge-core is a thin Go binary. Installing toolchains into forge-core
violates "one container, one responsibility".

**Profile missing:** If `detect_language` returns None (unknown language
or unknown project layout), prep is skipped and status goes directly to
`ready`. Any build failures due to missing deps are surfaced through
agent bash output, and the user can decide to add a language profile.
This is a **known soft failure** — the agent degrades gracefully rather
than blocking on unknown projects.

### 3.10 Path plumbing to ai-worker

`RunRequest.workspace_path` already exists in `ai-worker/src/api_server.py`
as a relative path resolved against `FORGE_WORKSPACE_ROOT` env var on the
ai-worker side (the Stream 4c protocol described in §3.4). This stays.
Changes:

- `workspace_path` becomes **required**, not optional. Any request
  without it is a 400.
- The "is pair_pipeline or legacy" branching in `_route_and_stream` is
  removed. All requests flow through `QueryEngine` with the workspace
  path populated.
- Agent service (`forge-core/internal/module/agent/service.go`) calls
  `workspaceMgr.EnsureReady(ctx, tenantID, projectID)` synchronously
  before submitting the RunRequest to ai-worker. If EnsureReady fails,
  the agent session fails with an `ErrorEvent` and the RunRequest is
  never sent.
- The relative fragment sent on the wire is computed as
  `filepath.Rel(cfg.WorkspaceRoot, workspace.ProjectDir(tenantID, projectID))`,
  which evaluates to `tenant-{N}/project-{N}/repo`.
- In ai-worker's `_route_and_stream`, the `is_dir()` check on the
  resolved workspace path is **defensive-only**. It should never fire
  in normal operation because forge-core always calls EnsureReady
  first. If it does fire, it's an operational bug (mount not set up,
  race condition between forge-core commit and ai-worker read), so
  it returns 500 and logs loudly — ai-worker does **not** try to
  create the workspace itself.

### 3.11 Concurrency semantics

- **Two concurrent EnsureReady for same project:** Serialized by PG
  advisory lock. Second caller observes `ready` after first finishes
  and returns immediately.
- **Two concurrent sessions on same workspace:** Shared directory. Both
  sessions see each other's changes. This is known unsafe for
  cross-session file conflicts, but solo-dev usage doesn't hit it in
  practice, and version management is explicitly out of scope (deferred
  to SH-3a/3b/4 in the Harness Engineering roadmap).
- **EnsureReady during active session:** The new-session resync
  (fetch + reset hard) does not fire mid-session. Only on session
  creation. This is enforced in the Go agent service, not in the
  workspace service.

### 3.12 Failure-mode matrix

| Failure point | Phase | Status | last_error | Behavior |
|---|---|---|---|---|
| GitHub PAT revoked / expired | **1a only** | `error` | `clone failed: github_auth_failed` | Halt; human must refresh the PAT in forge-core project settings. Row is deleted in Phase 1b when PAT usage ends. |
| Deploy key upload to GitHub fails (4xx/5xx) | **1b only** | `error` | `deploy_key_upload failed: <code>` | Session halts, agent emits ErrorEvent |
| Clone fails — auth | 1a + 1b | `error` | `clone failed: authentication` | Halt; human must check credentials (PAT in 1a, deploy key permissions in 1b) |
| Clone fails — network | 1a + 1b | `error` | `clone failed: network` | Halt; next call retries automatically |
| Clone fails — unknown | 1a + 1b | `error` | `clone failed: <stderr>` | Halt |
| Dependency prep fails | 1a + 1b | `ready` | (warning logged) | **Does not halt.** Agent will see build errors if deps are needed. |
| `git reset --hard` on resync fails | 1a + 1b | fall back to wipe + re-clone | — | Transparent recovery |
| Workspace directory missing (manual cleanup) | 1a + 1b | treated as "no record" | — | Re-clones from scratch |
| Disk full | not handled this release | — | — | Future: disk-check hook pre-clone |

---

## 4. Tool layer

New package structure in ai-worker:

```
ai-worker/src/openharness/tools/
├── base.py              (refactored)
├── context_tools.py     (migrated to new BaseTool signature)
├── workspace_path.py    (new: WorkspacePath type)
├── file_tools.py        (new: ReadFileTool, WriteFileTool, EditFileTool,
│                         GlobTool, GrepTool, ListDirectoryTool)
├── bash_tool.py         (new: BashTool with bwrap)
├── phase_tool.py        (new: SetPhaseTool)
└── __init__.py
```

### 4.1 `BaseTool` refactor

Current signature:

```python
class BaseTool(ABC):
    @abstractmethod
    async def execute(
        self, arguments: BaseModel, context: ToolExecutionContext
    ) -> ToolResult: ...
```

New signature:

```python
class BaseTool(ABC):
    name: ClassVar[str]
    description: ClassVar[str]
    input_model: ClassVar[type[BaseModel]]

    @abstractmethod
    def execute(
        self, arguments: BaseModel, context: ToolExecutionContext,
    ) -> AsyncIterator[StreamEvent | ToolResult]:
        """Yield zero or more StreamEvents during execution, then yield
        exactly one ToolResult as the final value.

        Must not raise unhandled exceptions EXCEPT for the session-halt
        class defined in `openharness.engine.agent_hooks`:

            class SessionHaltError(Exception):
                '''Base class for errors that should halt the session
                rather than be translated to ToolResult(is_error=True).
                Subclasses: ClarificationTimeout, ReturnChannelError.'''

        All other error conditions MUST be returned as
        `ToolResult(is_error=True, output=...)`. The agent loop
        catches `SessionHaltError` in `_execute_tool_call` and
        translates it into `ErrorEvent(recoverable=False)` plus a
        terminal `ToolResultBlock(is_error=True)` so the UI can
        unwind cleanly and the session halts. See §2.9.2.f.
        """
        ...

    def is_read_only(self, arguments: BaseModel) -> bool:
        return False
```

A convenience subclass for single-shot tools:

```python
class SimpleTool(BaseTool):
    """Adapter for tools that don't need to yield StreamEvents.
    Subclass this if your tool just returns a ToolResult."""

    @abstractmethod
    async def _execute_simple(
        self, arguments: BaseModel, context: ToolExecutionContext,
    ) -> ToolResult: ...

    async def execute(self, arguments, context):
        yield await self._execute_simple(arguments, context)
```

- `ReadFileTool`, `WriteFileTool`, `EditFileTool`, `GlobTool`, `GrepTool`,
  `ListDirectoryTool`: subclass `SimpleTool`.
- `BashTool`: subclasses `BaseTool` directly (needs mid-execution
  `ThinkingStarted`/`ThinkingStopped` events, see §4.5).
- `SetPhaseTool`: subclasses `BaseTool` directly (emits `PhaseChanged`
  as a typed event, see §4.11).
- All five `context_tools.py` tools: migrated to `SimpleTool`.

**Agent loop impact** (`query.py`):

```python
async def _execute_tool_call(
    context, tool_name, tool_use_id, tool_input,
) -> AsyncIterator[StreamEvent | ToolResultBlock]:
    tool_result: ToolResult | None = None
    try:
        async for item in tool.execute(parsed, exec_ctx):
            if isinstance(item, StreamEvent):
                yield item
            elif isinstance(item, ToolResult):
                if tool_result is not None:
                    raise RuntimeError(
                        f"tool {tool.name} yielded multiple ToolResults"
                    )
                tool_result = item
            else:
                raise TypeError(
                    f"tool {tool.name} yielded unexpected: {type(item).__name__}"
                )
    except SessionHaltError as halt:
        # Session-halt exceptions (ClarificationTimeout, ReturnChannelError
        # — see §2.9.2.f) get translated to ErrorEvent + terminal error
        # ToolResultBlock so the UI can unwind the tool card cleanly.
        yield ErrorEvent(
            message=f"{type(halt).__name__}: {halt}",
            recoverable=False,
        )
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"session halted: {halt}",
            is_error=True,
        )
        return

    if tool_result is None:
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Tool {tool_name} did not yield a ToolResult",
            is_error=True,
        )
        return

    yield ToolResultBlock(
        tool_use_id=tool_use_id,
        content=tool_result.output,
        is_error=tool_result.is_error,
    )
```

The surrounding `run_agent_loop` block also emits
`ToolExecutionCompleted(tool_use_id, tool_name, output=...,
is_error=True)` after the halt path's `ToolResultBlock` so the frontend
tool card transitions out of the running state before the `ErrorEvent`
closes the stream. This is handled by the existing "tool_results[-1]"
lookup in `run_agent_loop` — no changes needed there.

`run_agent_loop`'s tool execution block:

```python
tool_results: List[ToolResultBlock] = []
for tu in tool_uses:
    yield ToolExecutionStarted(
        tool_use_id=tu.id,
        tool_name=tu.name,
        tool_input=tu.input,
    )
    async for item in _execute_tool_call(context, tu.name, tu.id, tu.input):
        if isinstance(item, ToolResultBlock):
            tool_results.append(item)
        else:
            yield item  # passthrough mid-execution events
    yield ToolExecutionCompleted(
        tool_use_id=tu.id,
        tool_name=tu.name,
        output=tool_results[-1].content,
        is_error=tool_results[-1].is_error,
    )
```

`tool_use_id` is threaded into both events so consumers (SessionCollector,
frontend) can correlate started/completed pairs without relying on
positional ordering. See §5.3 for the updated event vocabulary table.

No `if tool_name == "bash"` anywhere. Every tool is treated identically.

### 4.2 `WorkspacePath` type

```python
# workspace_path.py
from pathlib import Path
from pydantic import BaseModel

class PathEscapeError(ValueError): ...

class WorkspacePath:
    """A path guaranteed to be inside a workspace sandbox.

    Never construct directly — use WorkspacePath.resolve(workspace_root, user_path).
    """

    def __init__(self, workspace_root: Path, relative: Path) -> None:
        self.workspace_root = workspace_root
        self.relative = relative

    @classmethod
    def resolve(cls, workspace_root: Path, user_path: str) -> "WorkspacePath":
        if not user_path:
            raise PathEscapeError("empty path")
        p = Path(user_path)
        if p.is_absolute():
            raise PathEscapeError(f"absolute path not allowed: {user_path}")
        # Resolve without touching the filesystem — we don't want symlink
        # resolution (symlinks inside the workspace pointing out are still
        # out; the absolute path check at the end catches them).
        resolved = (workspace_root / p).resolve()
        try:
            relative = resolved.relative_to(workspace_root.resolve())
        except ValueError:
            raise PathEscapeError(
                f"path escapes workspace: {user_path}"
            )
        if any(part == ".." for part in relative.parts):
            raise PathEscapeError(f"path contains '..': {user_path}")
        return cls(workspace_root, relative)

    @property
    def absolute(self) -> Path:
        return self.workspace_root / self.relative
```

Tool input schemas consume this in Pydantic validators so
`_execute_tool_call`'s input validation step catches path escapes
before `execute()` is even called:

```python
class ReadFileInput(BaseModel):
    path: str
    start_line: int | None = None
    limit: int | None = None

    def as_workspace_path(self, root: Path) -> WorkspacePath:
        return WorkspacePath.resolve(root, self.path)
```

(Path is kept as `str` on the model because `WorkspacePath` needs the
workspace root to construct, which isn't available at model-validation
time. The tool's `_execute_simple` calls `.as_workspace_path(context.cwd)`
early and lets `PathEscapeError` bubble up as a `ToolResult(is_error=True)`.)

### 4.3 File tools

**`ReadFileTool`**

```python
name = "read_file"
description = (
    "Read a file from the project workspace. Returns the file contents "
    "as text with line numbers. Use start_line and limit to read a "
    "portion of a large file."
)

class ReadFileInput(BaseModel):
    path: str = Field(..., description="File path relative to workspace root")
    start_line: int | None = Field(None, description="1-indexed first line", ge=1)
    limit: int | None = Field(None, description="Max lines to read", ge=1)
```

- Reject binary files (null byte in first 8KB).
- Default output cap: 2000 lines or 200 KB, whichever hits first.
- Output format: `cat -n`-style with line numbers, so the agent can tell
  `edit_file` exactly which lines to change.
- On truncation, append `\n... [truncated, showing N of M lines]`.
- `is_read_only(args) = True`.

**`WriteFileTool`**

```python
name = "write_file"
description = (
    "Create a new file or overwrite an existing file. Parent directories "
    "are created automatically. For small modifications to existing files, "
    "prefer edit_file."
)

class WriteFileInput(BaseModel):
    path: str = Field(...)
    content: str = Field(...)
```

- `mkdir -p` parent directories.
- Overwrites existing files silently — the agent has access to
  `read_file` first if it cares.
- Returns `"Wrote N lines (M bytes) to <path>"`.
- `is_read_only = False`.

**`EditFileTool`**

```python
name = "edit_file"
description = (
    "Replace an exact string in an existing file. The old_string must "
    "appear exactly once in the file unless replace_all is True. This is "
    "the preferred way to modify code — it's less error-prone than "
    "rewriting entire files."
)

class EditFileInput(BaseModel):
    path: str
    old_string: str
    new_string: str
    replace_all: bool = False
```

- Claude Code exact contract.
- File must exist (this is edit, not create).
- `old_string` not found → `ToolResult(is_error=True, output="old_string not found in <path>. Use read_file first to see exact content.")`.
- `old_string` found N>1 times without `replace_all` → `ToolResult(is_error=True, output="old_string appears N times in <path>. Add more surrounding context to make it unique or set replace_all=true.")`.
- Success → `"Replaced in <path> (+X -Y lines)"` where X/Y are line
  count deltas.
- `is_read_only = False`.

**`GlobTool`**

```python
name = "glob"
description = (
    "Find files matching a glob pattern. Returns paths sorted by "
    "modification time (most recently modified first)."
)

class GlobInput(BaseModel):
    pattern: str = Field(..., description="Glob pattern like '**/*.go' or 'src/**/*.{ts,tsx}'")
    path: str | None = Field(None, description="Subdirectory to search from")
```

- Uses the `pathspec` library (gitignore-style matching) rather than
  `fnmatch` or handwritten matching.
- Result cap: 200 matches. Append `... (N more matches truncated)`.
- Ignore list (hardcoded for the first release):
  `.git/`, `node_modules/`, `.venv/`, `venv/`, `__pycache__/`,
  `dist/`, `build/`, `target/`, `.next/`, `.gradle/`, `.cache/`.
- `is_read_only = True`.

**`GrepTool`**

```python
name = "grep"
description = (
    "Search file contents using regex. Returns matching lines in "
    "'path:line:content' format."
)

class GrepInput(BaseModel):
    pattern: str
    path: str | None = None
    file_glob: str | None = Field(None, description="Optional glob to limit which files are searched")
    case_insensitive: bool = False
```

- Shells out to `rg` (ripgrep) — required, no Python fallback.
  Container base image adds `ripgrep` to its `apt install` list.
- Output cap: 500 result lines or 200 KB. Truncation note.
- Same ignore list as `glob`.
- `is_read_only = True`.

**`ListDirectoryTool`**

```python
name = "list_directory"
description = (
    "List the contents of a directory (one level deep). For recursive "
    "exploration use glob instead."
)

class ListDirectoryInput(BaseModel):
    path: str = "."
```

- One level only.
- Output format: prefixed with type marker — `dir/ foo.go bar.md` —
  sorted alphabetically with directories first.
- Cap: 500 entries.
- Same ignore list.
- `is_read_only = True`.

### 4.4 `BashTool`

```python
name = "bash"
description = (
    "Execute a shell command in the workspace directory. Use this for "
    "build, test, lint, and git inspection commands. The sandbox has NO "
    "network access — you cannot install new dependencies. Stay inside "
    "the workspace directory. Long commands are capped at 600 seconds."
)

class BashInput(BaseModel):
    command: str = Field(..., description="Shell command to execute")
    timeout: int = Field(120, description="Timeout in seconds, default 120, max 600", ge=1, le=600)
```

This is the highest-risk tool and gets the most engineering attention.

### 4.5 `BashTool` execution flow

```python
async def execute(self, arguments: BashInput, context: ToolExecutionContext):
    # Layer 2 (cheap front filter): denylist hint
    blocked_reason = _intent_denylist_check(arguments.command)
    if blocked_reason:
        yield ToolResult(
            is_error=True,
            output=f"Command rejected: {blocked_reason}",
        )
        return

    label = _summarize_command(arguments.command)  # "Running go build" etc.
    yield ThinkingStarted(label=label)
    try:
        exit_code, output = await _run_in_bwrap(
            command=arguments.command,
            workspace=context.cwd,
            timeout=arguments.timeout,
        )
    finally:
        yield ThinkingStopped()

    yield ToolResult(
        output=_format_bash_output(arguments.command, exit_code, output),
        is_error=(exit_code != 0),
    )
```

### 4.6 `BashTool` isolation (bubblewrap)

The sandbox invocation:

```
bwrap \
  --unshare-all \
  --die-with-parent \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --ro-bind /bin /bin \
  --ro-bind /sbin /sbin \
  --ro-bind /etc/ssl /etc/ssl \
  --proc /proc \
  --dev /dev \
  --tmpfs /tmp \
  --bind {workspace_abs} {workspace_abs} \
  --chdir {workspace_abs} \
  --setenv PATH /usr/local/bin:/usr/bin:/bin \
  --setenv HOME /tmp \
  --setenv LANG C.UTF-8 \
  --setenv GOCACHE /tmp/gocache \
  --setenv GOPATH /tmp/gopath \
  -- bash -c {command}
```

- `--unshare-all`: unshares **all** namespaces including the network
  namespace. This is what gives the sandbox no network access —
  the network namespace is isolated, so the sandbox sees only its own
  empty `lo` interface and no routes.
- **Do NOT add `--share-net`.** `--share-net` is a bare toggle (no
  argument) that *re-enables* the network namespace after
  `--unshare-all`. It is **not** a "disable network" flag. Writing
  `--share-net=false` would either be interpreted as "re-enable
  network" (catastrophic silent failure) or rejected as an unknown
  option, depending on bwrap version. The correct way to keep network
  off is to simply omit `--share-net` entirely after `--unshare-all`.
  An adversarial test (§7.1) explicitly verifies
  `ping -c 1 -W 1 8.8.8.8` fails from inside the sandbox to catch
  any regression here.
- `--die-with-parent`: if ai-worker crashes, the sandbox dies with it.
- `/usr`, `/lib`, `/lib64`, `/bin`, `/sbin` read-only bound so standard
  binaries work.
- Workspace is the **only** read-write bind.
- `/etc` is **not bound** — intentionally, so `cat /etc/passwd` reads a
  synthetic minimal file (bwrap provides one) rather than the real one.
- `/etc/ssl` is bound read-only so TLS cert verification works for any
  tool that might (even though net is off).
- `/tmp` is a tmpfs — ephemeral, isolated.
- Env whitelist only: `PATH`, `HOME` (pointed at /tmp), `LANG`,
  language-specific cache vars. **Not passed:** `GITHUB_TOKEN`,
  `FORGE_*`, DB passwords, `FORGE_SECRETS_MASTER_KEY`, anything in the
  parent process environment that could leak.

Language-specific env additions are configurable per workspace based on
the detected language (e.g., `JAVA_HOME`, `MAVEN_OPTS` for Java
workspaces). Default set is just PATH/HOME/LANG.

### 4.7 `BashTool` process management

```python
async def _run_in_bwrap(command, workspace, timeout):
    process = await asyncio.create_subprocess_exec(
        *_build_bwrap_args(workspace, command),
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        preexec_fn=os.setsid,  # new process group
    )
    try:
        stdout, _ = await asyncio.wait_for(
            process.communicate(),
            timeout=timeout,
        )
    except asyncio.TimeoutError:
        try:
            os.killpg(os.getpgid(process.pid), signal.SIGKILL)
        except ProcessLookupError:
            pass
        await process.wait()
        return -1, b"[killed by timeout]"

    return process.returncode, stdout

def _format_bash_output(command, exit_code, output):
    text = output.decode("utf-8", errors="replace")
    if len(text) > 100_000:
        text = text[:100_000] + f"\n... [output truncated, {len(text) - 100_000} more bytes]"
    return f"$ {command}\nexit code: {exit_code}\n\n{text}"
```

- Child process in its own process group via `os.setsid`.
- On timeout, `os.killpg(SIGKILL)` kills the whole group — no orphaned
  children.
- Output cap: 100 KB (combined stdout+stderr). Truncation note appended.
- Return format includes the command, exit code, and output — the agent
  needs all three in its conversation history.

### 4.8 `BashTool` network policy

Sandboxed `bash` has **no network**. This is a known capability tradeoff:

- ✅ **Works:** `go build`, `go test`, `go vet`, `mvn compile`,
  `mvn test -o` (offline), `npm run build`, `npm test`, `pytest`,
  `pylint`, `ruff`, `gofmt`, `git status`, `git diff`, `git log`.
- ❌ **Does not work:** `go mod download`, `npm install`, `pip install`,
  `mvn dependency:resolve` without `-o`, `curl`, `wget`, `git fetch`,
  `git push`, `ssh`.

Dependencies are pre-installed at workspace-create time by the
forge-core workspace service calling ai-worker's `/api/workspace/prep`
endpoint (§3.9), which runs language-specific commands *outside* the
sandbox.

This is an explicit scope cut. "Install a new dependency mid-agent-task"
is deferred to future work, where options include:
- Elevated `sandbox_prep` tool the agent can request
- `--share-net` toggle with explicit user approval via P3 permission mode
- Temporary unsandboxed prep step followed by re-entering sandbox

Agent's system prompt explicitly tells it "no network" so it doesn't try
and get confused.

### 4.9 Intent denylist (Layer 2, non-security)

```python
_INTENT_DENYLIST = [
    (re.compile(r"\bsudo\b"), "sudo not available in sandbox"),
    (re.compile(r"\bapt(-get)?\s+install\b"), "cannot install packages (no network)"),
    (re.compile(r"\bnpm\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bpip\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bsystemctl\b"), "systemctl not available"),
    (re.compile(r"curl\s+.*\|\s*(bash|sh)"), "piping curl to shell not allowed"),
    (re.compile(r"wget\s+.*\|\s*(bash|sh)"), "piping wget to shell not allowed"),
]

def _intent_denylist_check(command: str) -> str | None:
    for pattern, reason in _INTENT_DENYLIST:
        if pattern.search(command):
            return reason
    return None
```

**This is explicitly not a security boundary.** It is a fast user-facing
error message. bubblewrap is the actual security boundary. The denylist
catches the 80% of "agent is about to do something that will fail"
cases and gives the agent a clean error message instead of a confusing
sandbox error 30 seconds later.

Adversarial tests (§7.1) explicitly verify this layering: denylist
bypass attempts succeed at the denylist layer but still fail at bwrap.

### 4.10 Command summarization

```python
def _summarize_command(command: str) -> str:
    """Return a friendly label for ThinkingStarted."""
    first = command.strip().split()[0] if command.strip() else ""
    known = {
        "go": "Running go",
        "mvn": "Running maven",
        "gradle": "Running gradle",
        "npm": "Running npm",
        "pytest": "Running tests",
        "jest": "Running tests",
        "cargo": "Running cargo",
        "make": "Running make",
    }
    # Try sub-command recognition
    if first == "go" and " " in command.strip():
        second = command.strip().split()[1]
        return f"Running go {second}"
    if first == "mvn":
        return "Running maven"
    if first in known:
        return known[first]
    # Fallback: truncated raw command
    trimmed = command.strip()
    if len(trimmed) > 60:
        trimmed = trimmed[:57] + "..."
    return f"Running {trimmed}"
```

Small quality-of-life helper for the thinking indicator label. Tested
in unit tests for each branch.

### 4.11 `SetPhaseTool`

`SetPhaseTool` extends `BaseTool` directly (not `SimpleTool`) so it can
yield `PhaseChanged` as an explicit typed event rather than leaking
phase semantics into the frontend's generic tool-event handling.

```python
Phase = Literal["Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"]

class SetPhaseInput(BaseModel):
    phase: Phase

class SetPhaseTool(BaseTool):
    name = "set_phase"
    description = (
        "Signal which phase you're currently in. The UI step ribbon "
        "will highlight that phase. Available phases: Analyze "
        "(understanding requirements and code), Plan (deciding "
        "changes), Generate (writing code), Build (compiling), Test "
        "(running tests), Review (verifying own work), Deploy "
        "(committing or preparing deployment). Call this when you "
        "start a new phase. You can go backwards (e.g., Build -> "
        "Generate to fix a compile error)."
    )
    input_model = SetPhaseInput

    async def execute(self, arguments, context):
        yield PhaseChanged(phase=arguments.phase)
        yield ToolResult(output=f"Phase set to {arguments.phase}")

    def is_read_only(self, arguments):
        return True
```

**Why not `SimpleTool`:** a `SimpleTool` subclass can only yield a
`ToolResult`, not `StreamEvent`s. Making the agent loop derive
`PhaseChanged` from `ToolExecutionStarted(tool_name="set_phase")` would
spread phase semantics across two files (tool + loop dispatch) and
violate the silicon-valley rule against hardcoded special cases in the
loop. Spending 10 lines to extend `BaseTool` directly keeps the tool
self-contained, symmetric with `BashTool`, and free of hidden coupling.

### 4.12 Tool registry construction

`_create_engine` in `api_server.py` (Round 2, with §2.9.1 hook
registries and §2.9.2 clarification coordinator wired in):

```python
async def _create_engine(
    req: RunRequest,
    workspace_dir: Path,
    redis_client: Redis,
) -> QueryEngine:
    tool_registry = ToolRegistry()

    # T2 file/exec tools — all scoped to workspace_dir
    tool_registry.register(ReadFileTool(workspace_dir))
    tool_registry.register(WriteFileTool(workspace_dir))
    tool_registry.register(EditFileTool(workspace_dir))
    tool_registry.register(GlobTool(workspace_dir))
    tool_registry.register(GrepTool(workspace_dir))
    tool_registry.register(ListDirectoryTool(workspace_dir))
    tool_registry.register(BashTool(workspace_dir))

    # Meta tools
    tool_registry.register(SetPhaseTool())

    # Legacy context tools (now using SimpleTool adapter)
    profiles = _load_project_profiles(req.project_id)
    register_context_tools(tool_registry, profiles, req.project_id)

    # Subprocess hooks (existing)
    command_hook_registry = HookRegistry()
    command_hook_executor = HookExecutor(command_hook_registry)

    # In-process agent hooks (new Round 2 — §2.9.1)
    agent_hook_registry = AgentHookRegistry()  # empty by default
    agent_hook_context = AgentHookContext(
        project_id=req.project_id,
        session_id=req.session_id,
        workspace_dir=workspace_dir,
        system_prompt_buffer=[],
    )

    # Clarification coordinator + return channel (new Round 2 — §2.9.2)
    clarification_coordinator = ClarificationCoordinator()
    return_channel = await ReturnChannel.open(
        session_id=req.session_id,
        redis=redis_client,
        coordinator=clarification_coordinator,
    )

    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    # Interaction tools share the same ModelRouter instance (new Round 2)
    try:
        router = ModelRouter()
        # Fail fast if reviewer model not configured (§2.9.3.f)
        router.require_model_for(Purpose.REVIEW)
        api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
    except Exception as e:
        logger.error("ModelRouter unavailable: %s", e)
        raise  # fail fast, no AsyncMock fallback

    register_interaction_tools(
        tool_registry,
        model_router=router,
        workspace_dir=workspace_dir,
    )

    model = req.model or settings.default_model
    system_prompt = req.system_prompt or await build_system_prompt(
        language=detect_language(workspace_dir),
        workspace_path=str(workspace_dir),
        slots=agent_hook_registry.system_prompt_slots,  # empty by default
        hook_context=agent_hook_context,
    )

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        command_hook_executor=command_hook_executor,
        agent_hook_registry=agent_hook_registry,
        agent_hook_context=agent_hook_context,
        clarification_coordinator=clarification_coordinator,
        return_channel=return_channel,
        permission_checker=permission_checker,
        cwd=workspace_dir,
    )
```

**Removed from existing `_create_engine`:** the `purpose` parameter,
the `Purpose.REVIEW` branch of system_prompt selection, the
`AsyncMock` fallback for unavailable ModelRouter (fail fast — if
ModelRouter is down, the agent is down, and we want to know
immediately rather than silently running with a mock).

**Note on `Purpose.REVIEW`:** the `Purpose.REVIEW` **enum value**
is retained for `RequestReviewTool`'s internal use (§2.9.3.b). Only
the `_create_engine` switch branch that selected a different system
prompt for reviewer mode is removed. `router.require_model_for
(Purpose.REVIEW)` fail-fast assertion replaces any runtime check.

---

## 5. Agent loop and event layer

### 5.1 `run_agent_loop` changes

Two blocks change in Round 2:

1. **Tool execution block** — adapts to the new `BaseTool`
   async-iterator signature. Code shown in §4.1.
2. **Agent hook invocation points** (new in Round 2, §2.9.1):
   - Before each turn: `messages = await registry.pre_turn(ctx,
     messages)` for each hook in registry order. If a hook raises, the
     loop yields `ErrorEvent(recoverable=False)` and halts.
   - Inside `_execute_tool_call`, between permission check and tool
     execution: `for hook in registry.pre_tool_call: result = await
     hook(ctx, tool_name, parsed)`; if any hook returns
     `PreToolCallBlock(reason)`, the loop short-circuits with
     `ToolResultBlock(is_error=True, content=reason)` per §2.9.1.c.
   - After each turn completes with `stop_reason=end_turn`: `for hook
     in registry.post_turn: await hook(ctx, final_message)`. Errors
     halt the loop.

   The registry is owned by `QueryEngine` and passed via
   `_create_engine` as an empty default; downstream projects populate
   it via a project-scoped factory.

**Unchanged safety/limits:**
- `max_turns` default 25
- `MaxTurnsExceeded` error event on exhaustion
- `ApiTextDeltaEvent` → `AssistantTextDelta` passthrough
- Messages modified in place, returned to `QueryEngine._messages`

### 5.2 System prompt

`build_system_prompt` (renamed from `_build_system_prompt`, now
public) lives in `ai-worker/src/openharness/engine/prompts.py`. In
Round 2 it accepts optional slot fillers for §2.9.1 `system_prompt_slots`:

```python
async def build_system_prompt(
    language: str | None,
    workspace_path: str,
    slots: dict[str, PromptSlotFiller] | None = None,
    hook_context: AgentHookContext | None = None,
) -> str:
    lang_hint = (
        f"- Project language: {language}"
        if language else
        "- Project language: unknown (inspect files to detect)"
    )

    template = f"""You are Forge Agent, an AI coding assistant embedded in a Harness Engineering platform.
You work on a user's codebase inside a sandboxed workspace.

## Your environment
- Workspace root: {workspace_path}
{lang_hint}
- Sandbox: no network, cwd locked to workspace, bash timeout 120s default

## Available tools
You have tools for reading, writing, and editing files, searching code
(glob, grep), listing directories, running shell commands (bash),
signaling your current phase to the UI (set_phase), asking the user
clarifying questions (request_clarification), and requesting an
independent reviewer (request_review).

{{{{project_specs}}}}

## How to work
1. Understand the user's request. If the request is ambiguous, call
   `request_clarification` with a specific question rather than
   guessing. The user will type a response and you will receive it as
   the tool's return value.
2. Before making changes, read the relevant existing code. Use glob/grep
   to find things. Use read_file to see exact content.
3. Signal phases with set_phase. Phases are: Analyze (understanding),
   Plan (deciding), Generate (writing), Build (compiling), Test (running
   tests), Review (verifying), Deploy (committing). You may skip phases
   and go backwards. Call set_phase whenever you transition.
4. For code changes, prefer edit_file (exact string replacement) over
   write_file (whole-file overwrite). Rewrite whole files only when
   edit_file would be more disruptive.
5. After code changes, run build/test with bash to verify. If it fails,
   read the error, fix, and build again. You can iterate freely.
6. Stop when the user's request is satisfied. Do not over-engineer. Do
   not add features the user did not ask for. Do not refactor adjacent
   code unrelated to the task.
7. At major milestones — before `end_turn`, before a git commit that
   represents a user-visible feature boundary — call `request_review`
   with a short summary of what you built and why you believe it's
   correct. The reviewer is an independent LLM that sees your diff and
   the user's original request. Act on the verdict: APPROVE → proceed,
   REVISE → address the listed items, REJECT → reconsider the approach.
   You are not required to invoke the reviewer on every turn; use
   judgment.

## Constraints
- File operations stay inside the workspace.
- No network access — do not attempt `npm install`, `go mod download`,
  `curl`, `wget`, etc. Dependencies are pre-installed. If you need a
  dependency that isn't available, tell the user.
- bash commands time out at 120s by default, max 600s.
- Do not attempt destructive git operations (reset --hard, push --force,
  branch deletion) unless the user explicitly asks.

## Style
- Be terse. The UI shows every tool call you make — the user can see
  WHAT you did. Use text to explain WHY.
- Don't narrate obvious actions. "Let me read the file" is noise; just
  read it.
- When a build fails, don't announce "I'll fix this" — just fix it.
"""

    # Slot substitution (§2.9.1.c): `{{slot_name}}` → filler output
    if slots:
        for slot_name, filler in slots.items():
            placeholder = f"{{{{{slot_name}}}}}"
            if placeholder in template:
                value = await filler(hook_context) if hook_context else ""
                template = template.replace(placeholder, value)
            else:
                logger.warning("system_prompt_slot '%s' not in template", slot_name)
    # Strip any unfilled `{{slot}}` placeholders so the agent never
    # sees them literally.
    template = re.sub(r"\{\{[a-zA-Z_][a-zA-Z_0-9]*\}\}", "", template)
    return template
```

This is a starting-point skeleton. It will be iterated based on real
agent runs. The prompt lives in source control and has unit tests that
verify:
- the `{language}` and `{workspace_path}` substitutions work (same as
  Round 1)
- an unregistered `{{project_specs}}` placeholder is stripped to
  empty string (no literal `{{project_specs}}` in the final prompt)
- a registered slot filler's return value replaces the placeholder
- a filler that raises propagates the exception (fail-fast per §2.8)
- the "request_review" and "request_clarification" instruction
  bullets are present (substring assertions)

### 5.3 Event vocabulary

> **Round 2 note:** this table was labeled "(final)" in Round 1. §2.9.2
> adds `ClarificationRequested`; the table is no longer final and the
> label is removed. Any future addition requires a spec amendment with
> matching updates to `stream_events.py` and the frontend event
> dispatcher.

| Event | Source | Change |
|---|---|---|
| `AssistantTextDelta` | `ApiTextDeltaEvent` → yield | Unchanged |
| `AssistantTurnComplete` | `ApiMessageCompleteEvent` → yield | Unchanged |
| `ToolExecutionStarted` | `run_agent_loop` before tool call | Adds `tool_use_id: str` field |
| `ToolExecutionCompleted` | `run_agent_loop` after tool call | Adds `tool_use_id: str` field |
| `ThinkingStarted(label)` | `BashTool.execute()` | Repurposed: bash only |
| `ThinkingStopped` | `BashTool.execute()` finally block | Repurposed |
| `PhaseChanged(phase)` | `SetPhaseTool.execute()` | **New** |
| `ClarificationRequested(question, tool_use_id)` | `RequestClarificationTool.execute()` | **New in Round 2** — §2.9.2 |
| `SessionComplete(...)` | `QueryEngine.submit_message` end | Retained, new trigger logic |
| `ErrorEvent(message, recoverable)` | Any exception path | Unchanged. Fires on clarification timeout per §2.9.2.f |
| ~~`FixLoopStarted`~~ | ~~pair_pipeline~~ | **Deleted** |
| ~~`FixLoopCompleted`~~ | ~~pair_pipeline~~ | **Deleted** |

**Note on `ClarificationRequested` vs `ToolExecutionStarted/Completed`:**
per §2.9.2.e, both pairs fire around a `request_clarification` tool
call. The sequence is
`ToolExecutionStarted → ClarificationRequested → [pause] →
ToolExecutionCompleted`. `ClarificationResponse` is a **channel
message**, not a stream event — it lives on Redis and is never
forwarded via SSE.

### 5.4 `PhaseChanged` event

```python
@dataclass(frozen=True)
class PhaseChanged(StreamEvent):
    phase: Literal["Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"]
```

Added to `stream_events.py`. Serialized in `_serialize_event`:

```python
elif isinstance(event, PhaseChanged):
    base["type"] = "phase_changed"
    base["phase"] = event.phase
```

Also update `_serialize_event`'s existing branches for
`ToolExecutionStarted` and `ToolExecutionCompleted` to emit
`tool_use_id`:

```python
elif isinstance(event, ToolExecutionStarted):
    base["type"] = "tool_started"
    base["tool_use_id"] = event.tool_use_id   # new
    base["tool_name"] = event.tool_name
    base["tool_input"] = json.dumps(event.tool_input, default=str)

elif isinstance(event, ToolExecutionCompleted):
    base["type"] = "tool_completed"
    base["tool_use_id"] = event.tool_use_id   # new
    base["tool_name"] = event.tool_name
    base["output"] = event.output[:4000]
    base["is_error"] = str(event.is_error)
```

### 5.5 `FixLoop*` event deletion

- Remove `FixLoopStarted` and `FixLoopCompleted` from
  `stream_events.py`.
- Remove their branches in `_serialize_event`.
- Remove `_role_for_event_type`'s mentions of `fix_loop_*`.
- Frontend visual detection replaces the event (§6.2).

### 5.6 Session complete logic

With the `tool_use_id` field added to both events (§5.3, §5.4), the
SessionCollector correlates Started/Completed pairs by id: on Started
it stashes bash commands in a `dict[tool_use_id, command]`, on
Completed it looks them up to decide whether the last bash call was
build-like and should update `build_status`. No ordering assumptions.

```python
_BUILD_LIKE_FIRST_TOKENS = {
    "go",        # go build, go test, go vet
    "mvn",       # mvn compile, mvn test
    "gradle",    # gradle build
    "gradlew",   # ./gradlew build
    "npm",       # npm run build, npm test
    "pnpm",
    "yarn",
    "pytest",
    "cargo",     # cargo build, cargo test
    "make",      # make build, make test
    "ctest",
    "dotnet",    # dotnet build
    "tsc",       # TypeScript compile
    "javac",
}

def _is_build_like(command: str) -> bool:
    """Return True if the first whitespace-delimited token of the command
    is a known build/test runner. Used to derive SessionComplete's
    build_status from the most recent bash call."""
    command = command.strip()
    if not command:
        return False
    first = command.split()[0]
    if "/" in first:  # ./gradlew, ./bin/tool
        first = first.rsplit("/", 1)[-1]
    return first in _BUILD_LIKE_FIRST_TOKENS


class SessionCollector:
    """Tracks per-turn statistics from tool execution events.

    Correlates Started/Completed pairs via tool_use_id, which both
    events now carry (see §5.3). No ordering assumptions needed.
    """

    def __init__(self) -> None:
        self.files_created = 0
        self.files_modified = 0
        self.tool_call_count = 0
        self.last_build_status: str = "skipped"
        self._pending_bash: dict[str, str] = {}  # tool_use_id → command

    def observe(self, event: StreamEvent) -> None:
        if isinstance(event, ToolExecutionStarted):
            # Only bash needs the command on the Completed side to
            # derive build_status; other tools don't need pre-stashing.
            if event.tool_name == "bash":
                command = event.tool_input.get("command", "")
                self._pending_bash[event.tool_use_id] = command
            return

        if isinstance(event, ToolExecutionCompleted):
            self.tool_call_count += 1
            if event.tool_name == "write_file" and not event.is_error:
                self.files_created += 1
            elif event.tool_name == "edit_file" and not event.is_error:
                self.files_modified += 1
            elif event.tool_name == "bash":
                command = self._pending_bash.pop(event.tool_use_id, "")
                if _is_build_like(command):
                    self.last_build_status = (
                        "passed" if not event.is_error else "failed"
                    )

    def should_emit_summary(self) -> bool:
        return self.tool_call_count > 0
```

Adding `tool_use_id: str` to both `ToolExecutionStarted` and
`ToolExecutionCompleted` is a contained mutation — it touches
`stream_events.py`, the `run_agent_loop` call sites in `query.py`, the
`_serialize_event` function in `api_server.py`, and the frontend SSE
handlers that now carry the id through for ordering. No backward compat
since the frontend reader for `fix_loop_*` events is already being
deleted.

`QueryEngine.submit_message` wires this up:

```python
async def submit_message(self, prompt: str) -> AsyncIterator[StreamEvent]:
    start_ts = time.monotonic()
    collector = SessionCollector()
    prior_usage = self._total_usage

    user_msg = ConversationMessage.from_user_text(prompt)
    self._messages.append(user_msg)

    context = QueryContext(...)

    async for event in run_agent_loop(context, self._messages):
        collector.observe(event)
        if isinstance(event, AssistantTurnComplete):
            self._total_usage = UsageSnapshot(...)
        yield event

    if collector.should_emit_summary():
        turn_usage = self._total_usage - prior_usage
        yield SessionComplete(
            files_created=collector.files_created,
            files_modified=collector.files_modified,
            build_status=collector.last_build_status,
            duration_ms=int((time.monotonic() - start_ts) * 1000),
            tokens_total=turn_usage.total_tokens,
            cost_usd=turn_usage.total_cost_usd,
        )
```

The `should_emit_summary` check implements the "no SummaryCard for
zero-tool-call sessions" decision.

### 5.7 Routing simplification

`_route_and_stream` becomes (Round 2 — `_create_engine` is now async
and requires a `redis_client` argument per §4.12):

```python
async def _route_and_stream(
    req: RunRequest, session_id: str, correlation_id: str,
) -> AsyncIterator[StreamEvent]:
    if not req.workspace_path:
        raise HTTPException(
            status_code=400,
            detail="workspace_path is required",
        )

    ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
    resolved = Path(os.path.join(ws_root, req.workspace_path))

    if not resolved.is_dir():
        raise HTTPException(
            status_code=500,
            detail=f"workspace not ready: {resolved}",
        )

    redis_client = _get_redis()  # process-level singleton (preexisting)

    engine = _sessions.get(session_id)
    if engine is None:
        engine = await _create_engine(
            req,
            workspace_dir=resolved,
            redis_client=redis_client,
        )
        await _sessions.put(session_id, engine)

    async for event in engine.submit_message(req.message):
        yield event
```

### 5.8 Session cache (LRU)

Replace `_sessions: Dict[str, Any]` with an `LRUSessionCache`. Round 2:
eviction must `await engine.close()` to tear down the return channel
subscriber task cleanly, so `put` is now `async`:

```python
class LRUSessionCache:
    def __init__(self, maxsize: int = 100) -> None:
        self._maxsize = maxsize
        self._cache: OrderedDict[str, QueryEngine] = OrderedDict()

    def get(self, session_id: str) -> QueryEngine | None:
        if session_id in self._cache:
            self._cache.move_to_end(session_id)
            return self._cache[session_id]
        return None

    async def put(self, session_id: str, engine: QueryEngine) -> None:
        if session_id in self._cache:
            self._cache.move_to_end(session_id)
        else:
            self._cache[session_id] = engine
            if len(self._cache) > self._maxsize:
                oldest_id, oldest_engine = self._cache.popitem(last=False)
                await oldest_engine.close()  # tears down ReturnChannel subscriber
                logger.info("Evicted LRU session %s", oldest_id)

    async def pop(self, session_id: str) -> QueryEngine | None:
        engine = self._cache.pop(session_id, None)
        if engine is not None:
            await engine.close()
        return engine
```

100 sessions × ~2MB per (messages + local state) = ~200MB ceiling. Fine
for dev; when prod load justifies, swap for Redis-backed store. `close()`
is idempotent per §2.9.2.c — safe to call twice.

---

## 6. Frontend changes

Keep the frontend surface area minimal. This section lists only the
necessary changes.

### 6.1 Deletions

**Files physically removed:**
- `forge-portal/components/agent/build-card.tsx`
- `forge-portal/components/agent/build-card.test.tsx`

**Code removed:**
- In `agent-chat.tsx`: `BuildInfo` type, `ChatMessage.build` field, all
  build-info rendering branches.
- In `agent-chat.tsx`: `AgentRole` loses `"coder"` and `"reviewer"`, becomes
  `"user" | "assistant" | "system" | "summary"`.
- In `agent-chat.tsx`: `fix_loop_started` / `fix_loop_completed` SSE event
  handler cases removed (see §6.2 for replacement).
- In `hydrateFromDurableLog`: fix_loop_* branches removed.
- In `lib/agent.ts` (or equivalent): fix_loop_* type definitions removed.

### 6.2 Fix loop visual detection (replacing event)

Replaces the deleted `fix_loop_*` events with a frontend-side heuristic:

```ts
function detectFixLoopStart(
  messages: ChatMessage[],
  newToolCall: ToolCall,
): "insert_fixing_banner" | null {
  // Pattern: previous tool was bash with is_error, current tool is bash
  // (and in between there was at least one write/edit).
  //
  // Walk back through the current assistant message's tool calls.
  if (newToolCall.name !== "bash") return null;

  const currentMsg = messages[messages.length - 1];
  if (currentMsg?.role !== "assistant") return null;

  const tools = currentMsg.tools ?? [];
  let sawWrite = false;
  for (let i = tools.length - 1; i >= 0; i--) {
    const t = tools[i];
    if (t.name === "write_file" || t.name === "edit_file") {
      sawWrite = true;
    }
    if (t.name === "bash" && t.isError) {
      return sawWrite ? "insert_fixing_banner" : null;
    }
  }

  return null;
}
```

When this returns `"insert_fixing_banner"`, the chat prepends a subtle
system-style message ("Fixing previous error...") before rendering the
new bash tool card. The banner is visually muted — not the bright
orange pair_pipeline banner, but a faded one-line italic.

Unit tested in `agent-chat.test.tsx`.

### 6.3 Component changes

**`step-ribbon.tsx`** — rewrite to support three-state cells:

```tsx
type PhaseState = "upcoming" | "active" | "visited";
type Phase = "Analyze" | "Plan" | "Generate" | "Build" | "Test" | "Review" | "Deploy";

interface StepRibbonProps {
  currentPhase: Phase | null;      // null = no phase yet
  visitedPhases: Set<Phase>;       // all phases the agent has passed through
}
```

`agent-chat.tsx` state additions:

```tsx
const [currentPhase, setCurrentPhase] = useState<Phase | null>(null);
const [visitedPhases, setVisitedPhases] = useState<Set<Phase>>(new Set());

// In SSE event handler:
case "phase_changed": {
  const phase = event.phase as Phase;
  setCurrentPhase(phase);
  setVisitedPhases(prev => {
    const next = new Set(prev);
    if (currentPhase) next.add(currentPhase);
    return next;
  });
  break;
}
```

Supports backwards moves: if the agent goes from Build back to Generate,
the new Generate cell becomes active; Build transitions from active to
visited.

**`tool-formatters.ts`** — add formatters for seven new tools:

```ts
export function formatToolSummary(name: string, input: any, output?: string): ToolSummary {
  switch (name) {
    case "read_file":
      return { icon: "🔍", label: input.path, status: parseLineCount(output) };
    case "write_file":
      return { icon: "✏️", label: input.path, status: "created" };
    case "edit_file":
      return { icon: "✏️", label: input.path, status: parseEditDelta(output) };
    case "glob":
      return { icon: "📁", label: input.pattern, status: parseMatchCount(output) };
    case "grep":
      return { icon: "🔎", label: input.pattern, status: parseResultCount(output) };
    case "list_directory":
      return { icon: "📂", label: input.path ?? ".", status: parseItemCount(output) };
    case "bash":
      return { icon: "▶", label: truncate(input.command, 60), status: parseExitCode(output) };
    case "set_phase":
      return { icon: "→", label: `Phase: ${input.phase}`, status: "", hideCard: true };
    // Legacy context tools keep their formatters
    default:
      return { icon: "🛠", label: name, status: "" };
  }
}
```

`set_phase` has `hideCard: true` — the phase change is shown in the
ribbon, not as a tool card. Otherwise the user would see a noise tool
card every time the agent transitions phases.

**`thinking-indicator.tsx`** — unchanged visually; change rendering
location. Currently rendered at chat bottom; change to render **attached
to the most recent bash tool card** by a small component boundary in
`agent-chat.tsx`.

**`code-panel.tsx`** — degraded to read-only preview:
- Remove syntax highlighting if present
- Remove diff line decorations
- Accept a single `filePath: string | null` prop
- Fetch content from `GET /api/projects/{id}/code/file?path={filePath}`
- Render into a `<pre>` with `white-space: pre; overflow: auto`
- Trigger: clicking a `read_file` / `write_file` / `edit_file` tool card
  opens this file in the code panel. No live-follow.

**`summary-card.tsx`** — unchanged structurally. Continues to listen
for `session_complete` events. The no-tool-call suppression already
happens on the backend (§5.6), so the frontend doesn't need to filter.

### 6.4 Tool card folding — deferred

Mockup shows foldable tool cards. This release does **not** implement
fold/unfold. Every tool card renders its summary line and nothing more.
Longer output is accessed via the code panel or by copying from message
history.

### 6.5 Legacy data compatibility — none

The user decided "no backward compat" for old session data. The
deployment SQL will wipe `engine.agent_messages` rows (or at minimum
all rows with `event_type LIKE 'fix_loop%'` and any pair_pipeline
session rows). Frontend `hydrateFromDurableLog` has its fix_loop_*
branches physically removed; it will throw an error on unknown event
types if any slip through, which is correct — loud failure beats silent
corruption.

---

## 7. Testing strategy

### 7.1 Adversarial tests (P0)

**Bash sandbox adversarial suite** — `tests/openharness/tools/test_bash_adversarial.py`:

| Test | Expected behavior |
|---|---|
| `test_bash_cannot_read_real_etc_passwd` | Reading /etc/passwd returns bwrap's synthetic /etc, not host |
| `test_bash_cannot_read_secrets_env_var` | `echo $FORGE_SECRETS_MASTER_KEY` returns empty string |
| `test_bash_cannot_read_github_token_env_var` | `echo $GITHUB_TOKEN` returns empty string |
| `test_bash_cannot_reach_network` | `ping -c 1 -W 1 8.8.8.8` returns non-zero exit |
| `test_bash_cannot_curl` | `curl https://example.com` fails with network error |
| `test_bash_cannot_read_other_tenant_workspace` | With tenant A's workspace bound, attempting `cat /data/forge/workspaces/tenant-B/project-1/repo/README.md` fails (tenant-B path is not bind-mounted into the sandbox at all) |
| `test_bash_cannot_cd_out_of_workspace_and_write` | Can `cd /tmp` but writes there are lost when sandbox exits |
| `test_bash_cannot_kill_parent_process` | `kill -9 $PPID` has no effect on ai-worker |
| `test_bash_respects_timeout` | `sleep 200` with timeout=5 is killed within 10s |
| `test_bash_timeout_kills_subprocess_tree` | `bash -c 'sleep 200 & wait'` is fully killed on timeout |
| `test_bash_output_truncation` | 200KB output is truncated to 100KB with notice |
| `test_bash_denylist_rejects_sudo` | denylist rejects `sudo foo` before bwrap |
| `test_bash_denylist_bypass_still_safe` | `SUDO=sudo && $SUDO ls` bypasses denylist but bwrap still blocks (no setuid) |

**Path resolution adversarial suite** —
`tests/openharness/tools/test_workspace_path_adversarial.py`:

| Test | Expected |
|---|---|
| `test_reject_absolute_path` | `/etc/passwd` raises PathEscapeError |
| `test_reject_parent_traversal` | `../other` raises PathEscapeError |
| `test_reject_nested_parent_traversal` | `a/b/../../../etc` raises PathEscapeError |
| `test_reject_symlink_pointing_outside` | symlink inside workspace pointing to /etc raises PathEscapeError (after clone) |
| `test_reject_null_byte` | `foo\x00.txt` raises PathEscapeError |
| `test_accept_normal_relative` | `src/main.go` works |
| `test_accept_deep_relative` | `a/b/c/d/e.txt` works |
| `test_accept_workspace_root` | `.` resolves to workspace root |

**Workspace tenant isolation** —
`forge-core/internal/module/workspace/service_test.go`:

| Test | Phase | Expected |
|---|---|---|
| `TestTenantIsolation_ProjectPathsNoOverlap` | 1a | tenant A project 1 and tenant B project 1 have distinct paths |
| `TestDeployKey_PrivateKeyEncryptionRoundtrip` | 1b | encrypt(decrypt(x)) == x |
| `TestDeployKey_CiphertextDoesNotContainPlaintext` | 1b | encrypted blob has no plaintext substring |
| `TestDeployKey_EachKeyHasUniqueNonce` | 1b | two encryptions of same plaintext produce different ciphertexts |
| `TestEnsureReady_ConcurrentCallers_SingleClone` | 1a | two concurrent calls trigger one git clone, not two |

**Bidirectional RPC adversarial suite (Round 2)** —
`tests/openharness/engine/test_return_channel_adversarial.py`:

| Test | Expected behavior |
|---|---|
| `test_clarification_timeout_halts_session` | With 1-second timeout and no reply, tool raises `ClarificationTimeout`, agent loop emits `ErrorEvent(recoverable=False)`, SSE stream terminates |
| `test_clarification_response_wrong_session_id_rejected` | Publishing `{session_id: "other"}` to `agent:return:{id}` is ignored; the waiting future does not resolve |
| `test_clarification_response_unknown_tool_use_id_rejected` | Publishing with `tool_use_id` not in pending map logs warning and no future resolves |
| `test_clarification_response_malformed_json_rejected` | Publishing non-JSON bytes is discarded; listener does not crash |
| `test_clarification_response_wrong_type_rejected` | Publishing `{type: "other_type"}` is discarded |
| `test_clarification_response_oversized_rejected` | Response >4 KiB is rejected at forge-core before Redis publish (tested in Go) |
| `test_concurrent_sessions_isolation` | 10 sessions each awaiting clarification; each receives only its own response, no cross-talk |
| `test_return_channel_disconnect_halts_session` | Redis connection drop during wait cancels the future, tool yields `ToolResult(is_error=True)`, agent halts |
| `test_session_cancellation_cancels_pending_future` | `QueryEngine.close()` cancels pending clarification futures cleanly without leaking the subscriber task |

All of the above must pass before deployment. A single failure is P0.

### 7.2 Unit tests (per tool)

Each T2 tool plus SetPhaseTool gets a standard suite:
- Happy path
- Input validation failure (Pydantic rejection)
- Expected error paths (file not found, invalid regex, etc.)
- Large input truncation
- WorkspacePath escape (for file tools)
- Concurrent invocation is safe (stateless tools)

Migrated context_tools tools get one test confirming they still work
after SimpleTool migration.

**Round 2 additions — interaction tools (§2.9.2, §2.9.3):**

- `RequestClarificationTool`:
  - Happy path: yields `ClarificationRequested(question, tool_use_id)`
    then (after canned delivery from fixture) yields
    `ToolResult(output=response)`
  - Timeout path: raises `ClarificationTimeout` after configured
    timeout; test uses a 100ms override
  - Session cancellation: the tool's future is cancelled cleanly when
    `ClarificationCoordinator.cancel_all` fires
  - Input validation: empty question rejected, >4 KiB question
    rejected (Pydantic)
- `RequestReviewTool`:
  - Happy path with mocked `ModelRouter.generate` returning `"APPROVE"`:
    tool returns `ToolResult(output="APPROVE", is_error=False)`
  - `build_reviewer_prompt` substring invariants (original_request,
    summary, current_diff all appear in the rendered prompt)
  - `parse_verdict` parses `"APPROVE"`, `"REVISE foo"`, `"REJECT bar"`,
    multi-line responses with the verdict line in the middle
  - `parse_verdict` raises `ReviewerParseError` on unparseable input
  - `_collect_git_diff` runs against a real git fixture and returns
    non-empty diff
  - `_collect_git_diff` caps output at `REVIEWER_DIFF_MAX_BYTES` and
    appends truncation marker
  - ModelRouter unavailable: tool returns `ToolResult(is_error=True)`
  - `Purpose.REVIEW` fail-fast assertion at `_create_engine`
    construction time when no model is registered for that purpose

### 7.3 Contract tests (BaseTool)

`tests/openharness/tools/test_base_tool_contract.py`:

```python
@pytest.mark.parametrize("tool_class", ALL_REGISTERED_TOOL_CLASSES)
async def test_tool_yields_exactly_one_tool_result(
    tool_class, workspace, contract_fixtures,
):
    tool = _construct_tool_for_contract(tool_class, workspace, contract_fixtures)
    arguments = _make_valid_arguments(tool)
    context = _make_contract_context(workspace, contract_fixtures)

    # RequestClarificationTool pauses on an external future. The
    # contract-test fixture auto-delivers a canned response when it
    # sees ClarificationRequested — see _make_contract_context below.
    tool_result_count = 0
    items = []
    async for item in tool.execute(arguments, context):
        items.append(item)
        if isinstance(item, ClarificationRequested):
            # Test fixture delivers the response through the
            # coordinator so the tool's future resolves.
            contract_fixtures.clarification_coordinator.deliver(
                item.tool_use_id, "contract-fixture canned response",
            )
        if isinstance(item, ToolResult):
            tool_result_count += 1

    assert tool_result_count == 1, (
        f"{tool_class.__name__} yielded {tool_result_count} ToolResults, expected 1"
    )
    assert isinstance(items[-1], ToolResult), (
        f"{tool_class.__name__} did not yield ToolResult last"
    )
    # All other items must be StreamEvent instances
    for item in items[:-1]:
        assert isinstance(item, StreamEvent), (
            f"{tool_class.__name__} yielded non-StreamEvent non-final: {type(item)}"
        )

@pytest.mark.parametrize("tool_class", ALL_REGISTERED_TOOL_CLASSES)
async def test_tool_does_not_raise_on_invalid_input(
    tool_class, workspace, contract_fixtures,
):
    """Tools must return ToolResult(is_error=True) rather than raising."""
    tool = _construct_tool_for_contract(tool_class, workspace, contract_fixtures)
    context = _make_contract_context(workspace, contract_fixtures)

    # Deliberately invalid arguments that get past Pydantic but fail at execution
    # (e.g., non-existent file for read_file)
    arguments = _make_failing_arguments(tool)

    tool_result = None
    async for item in tool.execute(arguments, context):
        if isinstance(item, ToolResult):
            tool_result = item

    assert tool_result is not None
    assert tool_result.is_error is True
```

These tests run against *every* registered tool class without hardcoding
which tools exist, so new tools automatically get covered.

**Contract fixture pattern (§2.9.2.d, §2.9.3.g):** tools that need
external state get it via a shared `ContractFixtures` dataclass
constructed per-test:

```python
@dataclass
class ContractFixtures:
    clarification_coordinator: ClarificationCoordinator
    model_router: MagicMock  # mocked ModelRouter for RequestReviewTool
    git_workspace: Path      # real git fixture for _collect_git_diff

@pytest.fixture
def contract_fixtures(workspace):
    coordinator = ClarificationCoordinator()
    router = _mock_model_router(default_response="APPROVE")
    return ContractFixtures(
        clarification_coordinator=coordinator,
        model_router=router,
        git_workspace=workspace,
    )

def _construct_tool_for_contract(tool_class, workspace, fixtures):
    # Tools whose constructors need extra dependencies get them from
    # fixtures. This is an enumerated switch, not a reflection hack,
    # per §2.8 "no hardcoded special cases" — the switch IS the
    # registration contract.
    if tool_class is RequestClarificationTool:
        return RequestClarificationTool()  # stateless
    if tool_class is RequestReviewTool:
        return RequestReviewTool(
            model_router=fixtures.model_router,
            workspace_dir=workspace,
        )
    return tool_class(workspace)

def _make_contract_context(workspace, fixtures):
    return ToolExecutionContext(
        cwd=workspace,
        tool_use_id="toolu_contract_test",
        clarification_coordinator=fixtures.clarification_coordinator,
        original_user_request="contract-test user request",
    )
```

**Why this is not a violation of §2.8:** the contract test has to
construct tools with their real dependencies somewhere. A switch on
tool class in one place — the contract fixture constructor — is
clearer than a dependency-injection framework and is contained to
the test file. The production `register_interaction_tools` helper
does the same enumeration in `_create_engine` (§4.12). If a tool
class is added to `ALL_REGISTERED_TOOL_CLASSES` without a fixture
branch here, the test fails loudly at construction time — the
mechanical gate holds.

### 7.4 Agent loop integration tests

`tests/openharness/engine/test_agent_loop_integration.py`, using
a mocked API client:

- Single tool use round trip
- Multiple tool uses in sequence (read → edit → bash)
- Tool that errors (agent observes the error in ToolResultBlock and
  continues)
- max_turns exhaustion
- API client exception during stream
- SessionCollector counting (write_file ×2 + edit_file ×3 + bash
  success → correct counts in SessionComplete)
- Zero tool calls → no SessionComplete emitted

**Round 2 additions:**

- **`test_clarification_roundtrip_happy_path`** — mock API client
  fires a `request_clarification` tool call; a background task
  publishes a canned response to the session's return channel; the
  agent's tool receives the response and continues; assert the
  second API call carries the tool result with the canned response.
- **`test_clarification_timeout_halts_session`** — mock API client
  fires a `request_clarification`; no response is published;
  `CLARIFICATION_TIMEOUT_SECONDS` is overridden to 0.1s; assert the
  loop yields `ErrorEvent(recoverable=False)` and exits.
- **`test_clarification_during_multi_tool_turn`** — agent calls
  `read_file` then `request_clarification` in the same turn; the
  first tool completes synchronously, the second pauses; response
  arrives, second tool completes, agent continues.
- **`test_request_review_happy_path`** — mock `ModelRouter.generate`
  returns `"APPROVE\n"`; agent calls `request_review`; tool result
  is `APPROVE`; agent continues.
- **`test_request_review_parses_verdict_with_details`** — mock router
  returns `"REVISE add null check on line 42"`; tool result preserves
  the full line.
- **`test_pre_turn_hook_mutates_system_prompt`** — register a
  `pre_turn` hook that appends `"<extra>"` to the system prompt
  buffer; run one turn; assert the mock API client saw the modified
  system prompt.
- **`test_pre_tool_call_hook_blocks_tool`** — register a
  `pre_tool_call` hook that returns `PreToolCallBlock("blocked by
  test")` for tool name `"bash"`; agent tries to call bash; assert
  the loop yields `ToolExecutionCompleted(is_error=True,
  output="blocked by test")` without executing the bash tool.
- **`test_post_turn_hook_fires_on_end_turn`** — register a
  `post_turn` hook that increments a counter; run a turn with
  `stop_reason=end_turn`; assert the counter is 1.
- **`test_pre_turn_hook_exception_halts_loop`** — register a
  `pre_turn` hook that raises `RuntimeError`; assert loop emits
  `ErrorEvent(recoverable=False)` with the hook name in the message.
- **`test_system_prompt_slot_substitution`** — register a
  `system_prompt_slots["project_specs"]` filler returning
  `"spec content"`; build system prompt; assert `"spec content"`
  appears in place of the `{{project_specs}}` placeholder.
- **`test_system_prompt_slot_missing_is_stripped`** — register no
  slot fillers; build system prompt; assert no literal
  `{{project_specs}}` appears in the output (the regex cleanup
  stripped it).

### 7.5 Workspace manager integration tests

Go tests in `forge-core/internal/module/workspace/`:

- `TestEnsureReady_FirstCall_ClonesRepo` — mock git command, verify
  clone is invoked
- `TestEnsureReady_ExistingReadyWorkspace_Resyncs` — mock fetch+reset
- `TestEnsureReady_ErrorState_Retries` — row status='error', next call
  wipes and re-clones
- `TestDeployKey_Generated_UploadedToGitHub` — mock GitHub API
- `TestDeployKey_ReusedAcrossCalls` — second EnsureReady does not
  regenerate
- `TestConcurrentEnsureReady_UsesAdvisoryLock` — two goroutines call
  simultaneously, only one clone runs

Mocking strategy: `git.go` and `keys.go` are thin wrappers; tests inject
`CommandRunner` and `GitHubClient` interfaces.

### 7.6 End-to-end smoke test

`tests/e2e/test_variant_b_smoke.py` — runs against a real LLM with
real Redis (docker-compose dev instance). Round 2 rewrites this test
to exercise the clarification round-trip in addition to the original
shape assertions.

```python
@pytest.mark.e2e
@pytest.mark.skipif(not os.getenv("FORGE_E2E_ENABLED"), reason="E2E disabled")
async def test_agent_can_complete_variant_b_workflow_with_clarification(
    redis_client,
):
    # 1. Set up a small fixture Go project in a temp workspace
    workspace = _setup_fixture_go_project()
    session_id = f"test-e2e-{uuid.uuid4().hex[:8]}"

    # 2. Create an agent session with an intentionally ambiguous prompt
    engine = await _create_engine(
        RunRequest(
            session_id=session_id,
            project_id=1,
            workspace_path=str(workspace.relative_to(FORGE_WORKSPACE_ROOT)),
            message=(
                "Add a new HTTP endpoint that returns greeting data. "
                "The exact path, greeting message, and response format "
                "are up to you — ask if you need clarification."
            ),
        ),
        workspace_dir=workspace,
        redis_client=redis_client,
    )

    # 3. Background task: when the agent asks for clarification, respond.
    clarification_responded = asyncio.Event()

    async def respond_to_clarifications():
        pubsub = redis_client.pubsub()
        await pubsub.subscribe(f"agent:clarification_watch:{session_id}")
        # This is a test-only sidecar channel; ai-worker publishes
        # ClarificationRequested events to it via an e2e-only hook.
        async for message in pubsub.listen():
            if message["type"] != "message":
                continue
            event = json.loads(message["data"])
            if event["type"] == "clarification_requested":
                # Respond via the real /api/sessions/{id}/clarify path
                await forge_core_client.post(
                    f"/api/sessions/{session_id}/clarify",
                    json={
                        "tool_use_id": event["tool_use_id"],
                        "response": (
                            "Path /hello, returns JSON {\"greeting\": \"world\"}, "
                            "HTTP 200 status."
                        ),
                    },
                )
                clarification_responded.set()
                break

    responder_task = asyncio.create_task(respond_to_clarifications())

    # 4. Collect events from the agent's forward SSE stream
    events: List[StreamEvent] = []
    async for event in engine.submit_message(
        "Add a new HTTP endpoint that returns greeting data. "
        "The exact path, greeting message, and response format "
        "are up to you — ask if you need clarification."
    ):
        events.append(event)

    responder_task.cancel()

    # 5. Assert the clarification happened
    clarification_events = [
        e for e in events if isinstance(e, ClarificationRequested)
    ]
    assert len(clarification_events) >= 1, (
        "Agent did not call request_clarification despite ambiguous prompt"
    )
    assert clarification_responded.is_set(), (
        "Background responder did not publish to return channel"
    )

    # 6. Assert the tool execution completed (round-trip closed)
    clarify_completions = [
        e for e in events
        if isinstance(e, ToolExecutionCompleted)
        and e.tool_name == "request_clarification"
    ]
    assert len(clarify_completions) >= 1, (
        "request_clarification did not complete (no ToolExecutionCompleted)"
    )
    assert clarify_completions[0].is_error is False, (
        "Clarification round-trip returned an error"
    )
    assert "world" in clarify_completions[0].output or "/hello" in clarify_completions[0].output, (
        "Clarification response did not contain the injected text"
    )

    # 7. Assert the original workflow shape (unchanged from Round 1)
    tool_calls = [e for e in events if isinstance(e, ToolExecutionCompleted)]
    tool_names = [t.tool_name for t in tool_calls]

    assert "set_phase" in tool_names, "Agent did not signal phase"
    assert any(t in tool_names for t in ["read_file", "glob", "grep", "list_directory"]), \
        "Agent did not explore code"
    assert any(t in tool_names for t in ["write_file", "edit_file"]), \
        "Agent did not write code"
    assert "bash" in tool_names, "Agent did not run build"

    session_complete = next(
        (e for e in events if isinstance(e, SessionComplete)), None
    )
    assert session_complete is not None, "No SessionComplete emitted"

    # 8. Verify the workspace actually has a /hello endpoint that compiles
    result = subprocess.run(
        ["go", "build", "./..."],
        cwd=workspace,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"Build failed: {result.stderr}"
```

- Runs in CI only on merge to main (cost ~$0.20 per run — clarification
  adds one extra LLM turn beyond Round 1's ~$0.10).
- Uses a known-stable fixture project (small, ~5 files).
- Does **not** assert specific tool call counts or phases — only the
  presence of the expected shape. Real agents are non-deterministic.
- **Requires a running Redis instance.** The E2E test harness spins
  up the docker-compose dev Redis before the test suite.
- The `agent:clarification_watch:{session_id}` sidecar channel is a
  **test-only construct**: during E2E runs, a hook is registered via
  `AgentHookRegistry` that echoes `ClarificationRequested` events to
  the watch channel. The production deploy has no such hook. This is
  documented in the fixture setup file.

The existing `ai-worker/tests/e2e/test_agent_pair_pipeline_e2e.py`
(commit a9d60e8) is **deleted** since pair_pipeline doesn't exist.

### 7.7 Frontend tests

- `step-ribbon.test.tsx`: add tests for active/visited transitions,
  backwards movement (Build → Generate), initial no-phase state.
- `agent-chat.test.tsx`: add `detectFixLoopStart` tests covering: no
  previous bash, previous bash success, previous bash error + no writes,
  previous bash error + writes + new bash (positive case).
- `tool-execution.test.tsx`: one test per new tool type verifying the
  correct summary line format.
- Delete `build-card.test.tsx`.
- Delete any test in `agent-chat.test.tsx` that asserts BuildCard or
  fix_loop behavior.

### 7.8 Observability

Structured log points, to be emitted from ai-worker and consumed by the
existing Loki/Grafana setup. **No dashboards** in this release — just
the raw data.

Events to log:

```python
logger.info(
    "agent.tool_call",
    extra={
        "session_id": ...,
        "correlation_id": ...,
        "tool_name": ...,
        "tool_input_size_bytes": ...,
        "tool_output_size_bytes": ...,
        "duration_ms": ...,
        "is_error": ...,
    },
)

logger.info(
    "agent.turn_complete",
    extra={
        "session_id": ...,
        "total_turns": ...,
        "total_tool_calls": ...,
        "total_tokens_in": ...,
        "total_tokens_out": ...,
        "total_cost_usd": ...,
        "duration_ms": ...,
    },
)

logger.warning(
    "agent.bash_denylist_hit",
    extra={
        "session_id": ...,
        "command_prefix": command[:60],
        "reason": ...,
    },
)

logger.info(
    "workspace.ensure_ready",
    extra={
        "tenant_id": ...,
        "project_id": ...,
        "result": "cloned" | "resynced" | "unchanged" | "error",
        "duration_ms": ...,
    },
)

# Round 2 additions — bidirectional RPC (§2.9.2)

logger.info(
    "agent.clarification_requested",
    extra={
        "session_id": ...,
        "tool_use_id": ...,
        "question_length": len(question),
    },
)

logger.info(
    "agent.clarification_responded",
    extra={
        "session_id": ...,
        "tool_use_id": ...,
        "response_length": len(response),
        "latency_ms": ...,  # from ClarificationRequested to response delivered
    },
)

logger.warning(
    "agent.clarification_timeout",
    extra={
        "session_id": ...,
        "tool_use_id": ...,
        "timeout_seconds": CLARIFICATION_TIMEOUT_SECONDS,
    },
)

# Round 2 additions — reviewer meta-tool (§2.9.3)

logger.info(
    "agent.review_requested",
    extra={
        "session_id": ...,
        "summary_length": len(summary),
        "diff_bytes": len(current_diff),
    },
)

logger.info(
    "agent.review_verdict",
    extra={
        "session_id": ...,
        "verdict": "APPROVE" | "REVISE" | "REJECT",
        "details_length": len(details),
        "latency_ms": ...,
    },
)
```

---

## 8. Deployment and rollout

Three-step deploy, no blue-green (solo dev, small blast radius):

### Step 1 — schema migrations + image rebuild

1. **Database migration:** apply new migrations creating
   `engine.workspaces` and `engine.project_deploy_keys`.
2. **Data cleanup:** run SQL to delete pair_pipeline and fix_loop rows
   from `engine.agent_messages`. If the column is a free-text
   `event_type`, use:
   ```sql
   DELETE FROM engine.agent_messages WHERE event_type IN (
     'fix_loop_started', 'fix_loop_completed'
   );
   ```
   For full wipe (if user prefers total reset):
   ```sql
   TRUNCATE engine.agent_messages;
   ```
3. **ai-worker image rebuild:** Dockerfile additions:
   ```
   RUN apt-get update && apt-get install -y --no-install-recommends \
       bubblewrap ripgrep && \
       rm -rf /var/lib/apt/lists/*
   ```
   Plus the pre-install runtime deps (language toolchains) already in
   the image.
4. **forge-core binary:** `go build ./cmd/forge-core` with the new
   workspace module.

### Step 2 — deploy new code, smoke test

1. `docker-compose -f docker-compose.dev.yml up -d --build` to bring up
   new containers.
2. Manual smoke test: create a test project, bind a real GitHub repo
   (small Go or Python project), send one agent message
   ("Explain the project structure"). Verify:
   - Workspace is cloned (check `/data/forge/workspaces/{tenant}/{project}`)
   - Agent emits phase_changed events (check Redis stream)
   - Agent uses read_file/glob tools
   - SessionComplete emitted with realistic stats
3. If any smoke step fails: `git reset --hard <pre-deploy-sha>`,
   `docker-compose up -d --build`, investigate.

### Step 3 — delete old code

Only after Step 2 passes:

- Delete `ai-worker/src/openharness/engine/pair_pipeline.py`.
- Delete `forge-portal/components/agent/build-card.tsx` and test.
- Remove all remaining `fix_loop_*` references, the `Purpose.REVIEW`
  branch in `_create_engine` (the **enum value** `Purpose.REVIEW` is
  retained for `RequestReviewTool`'s internal use — see §2.9.3.b —
  only the `_create_engine` switch branch that selected a different
  system prompt is removed), `AgentRole.coder|reviewer` usage.
- Delete `ai-worker/tests/e2e/test_agent_pair_pipeline_e2e.py`.
- Commit as a single "remove legacy pair_pipeline" commit so rollback is
  clean if needed post-deploy.

### Rollback

- `git reset --hard <pre-deploy-sha>`
- `docker-compose -f docker-compose.dev.yml up -d --build`
- Database: the new tables (`workspaces`, `project_deploy_keys`) can
  stay — they're additive. The `agent_messages` cleanup is not
  reversible but doesn't affect rollback (new sessions work either way).

### Post-deploy verification checklist

- [ ] Adversarial tests all passing in CI
- [ ] Smoke test passes on real project
- [ ] Loki logs show `agent.tool_call` events
- [ ] Redis stream receives `phase_changed` events
- [ ] Frontend step ribbon highlights correctly
- [ ] SummaryCard renders with real stats
- [ ] Bash tool cannot exfiltrate `$GITHUB_TOKEN` (manually verified)

---

## 9. Non-goals (explicit)

These are deliberately **not** in this release. Each has been evaluated
and deferred with specific rationale.

| Non-goal | Rationale for deferral |
|---|---|
| Permission UI (P2/P3) | Needs bidirectional RPC, separate large workstream |
| Full diff rendering in code panel | Complex; text-only preview is enough for v1 |
| Independent Reviewer agent (pair revival) | A3 is incremental over A2; add later if needed |
| Git push / PR creation from agent | Needs deploy key `read_only=false` (already set) + approval UI |
| Dependency install mid-agent-task | Sandboxed no-network is simpler; future P3 toggle |
| `apply_patch` / `multi_edit` | Optimization over `edit_file`, not required |
| Disk quota / cleanup | Operational, can be added without code changes |
| Key rotation | Manual ops procedure for now |
| Dashboards / alerting | Raw data in Loki is enough; dashboards are design work |
| Multi-workspace concurrent conflict handling | Version management, separate SH-3a/3b/4 workstream |
| Vault/KMS integration for master key | Env-var is fine for solo dev; interface is swap-ready |
| Web browsing / `web_fetch` tool | Prompt injection risk; scope cut |
| `run_tests` as separate tool | `bash pytest` works; richer UI card is polish |
| Historic session backward compatibility | User chose "no backward compat", wipe and proceed |

---

## 10. Open questions

None at spec-approval time. All clarification questions answered during
brainstorming (see §2). This section is reserved for questions that
surface during implementation.

---

## Appendix A — Decision audit trail

Every decision in §2 was confirmed by the user in the 2026-04-09
brainstorming session. The confirmation chain:

1. **Scope A** — "只做'让 agent 真正能用 Variant B 的交互'这一件事"
2. **Architecture A2** — "砍掉 pair_pipeline 外层，只留一个单 agent"
3. **Tool surface T2** — "read/write/edit/glob/grep/list_directory/bash"
4. **Permission P1→P3** — "本次 FULL_AUTO，P3 预留接口"
5. **Step Ribbon (b)** — "改造成动态阶段标记"
6. **Other components** — recommended combination accepted
7. **Workspace W1** — long-lived per project, lazy create, SSH
   deploy key, reset on new session, no disk reclaim
8. **Silicon Valley standard** — "目前都是基础建设，以最高级别硅谷啊"
9. **BaseTool refactor** — accepted as breaking change
10. **Bubblewrap** — chosen over forge-sandbox container and firecracker
11. **Windows dev = forced container** — no fallback code path
12. **Resync on new session** — `git reset --hard`
13. **Master key source** — `FORGE_SECRETS_MASTER_KEY` env var
14. **Dep prep location** — ai-worker via RPC
15. **FixLoop events** — delete, frontend visual detection
16. **Session cache** — LRU 100
17. **Zero-tool sessions** — suppress SessionComplete
18. **Backward compat** — none
19. **SimpleTool adapter** — accepted as legitimate convenience API
20. **E2E mode** — real LLM
21. **Deploy order** — three-step (deploy → smoke → delete legacy)

**Round 2 additions** (triggered by the autoplan CEO review on
2026-04-09, `[subagent-only]` mode; full review in
`~/.claude/projects/D--shulex-work-forge/memory/chronos-ceo-review-2026-04-09.md`):

22. **Verification hooks** — chronos adds pre_turn / pre_tool_call /
    post_turn hook registries + system_prompt_slots to the agent loop
    as extension points for future spec/constraint/entropy projects.
    chronos itself ships with empty default hooks; real implementations
    are follow-up projects (§2.9.1).
23. **`request_clarification` meta-tool + bidirectional SSE** —
    Critical 2 from the CEO review. Requires a new Phase 5a delivering
    a Redis pub/sub return channel, pause/resume state machine, and
    frontend clarification input. Walks back Q4's original "P2/P3
    bidirectional RPC is an independent project" decision because the
    clarification tool is fake without it (§2.9.2).
24. **`request_review` meta-tool** — partially walks back Q2 A2's
    rejection of the 2-agent model. The agent is still a single agent;
    `request_review` is an optional meta-tool the agent invokes
    voluntarily at milestones. It fires a dedicated reviewer LLM call
    with no tool access and returns APPROVE/REVISE/REJECT (§2.9.3).
25. **Phase 1 split into 1a (minimal) + 1b (deploy keys)** — Round 1
    Phase 1 was 13 tasks; 1a delivers ~6-7 tasks using HTTPS+token
    (retaining the existing `injectToken` helper) and unblocks Phase 5
    immediately. 1b adds ed25519 deploy keys + GitHub upload + SSH
    migration and can run in parallel with Phase 5/6. Saves ~2 weeks of
    wall-clock time on the critical path (§2.9.4).

## Appendix B — Files touched (summary)

Files are tagged with their phase. **1a** = Phase 1a (workspace
minimal), **1b** = Phase 1b (deploy keys), **2-4** = Phase 2/3/4
(tool refactor), **5** = Phase 5 agent loop, **5a** = Phase 5a
bidirectional RPC (Round 2), **6** = frontend, **7** = deploy.

**New files:**

- *Workspace (Phase 1a):*
  - `forge-core/internal/workspace/state.go` — Workspace status DAO **(1a)**
  - `forge-core/internal/workspace/ensure.go` — EnsureReady state machine **(1a)**
  - `forge-core/internal/workspace/git.go` — git command wrapper (HTTPS+token in 1a, rewritten to SSH in 1b) **(1a, rewritten in 1b)**
  - `forge-core/internal/workspace/prep.go` — HTTP client for ai-worker /api/workspace/prep **(1a)**
  - `forge-core/internal/workspace/state_test.go`, `ensure_test.go` **(1a)**
  - `forge-core/migrations/{nnn}_create_workspaces.sql` **(1a)**
- *Deploy keys (Phase 1b):*
  - `forge-core/internal/workspace/keys.go` — ed25519 gen, AES-GCM crypto, GitHub deploy key upload **(1b)**
  - `forge-core/internal/workspace/keys_test.go` **(1b)**
  - `forge-core/migrations/{nnn+1}_create_project_deploy_keys.sql` **(1b)**
- *Tool layer (Phase 2-4):*
  - `ai-worker/src/openharness/tools/workspace_path.py` **(2)**
  - `ai-worker/src/openharness/tools/file_tools.py` — Read/Write/Edit/Glob/Grep/ListDirectory **(3)**
  - `ai-worker/src/openharness/tools/bash_tool.py` — BashTool + bwrap wrapper **(4)**
  - `ai-worker/src/openharness/tools/phase_tool.py` — SetPhaseTool **(4)**
  - `ai-worker/src/openharness/engine/session_collector.py` **(5)**
  - `ai-worker/src/api_server.py` (new endpoint) — `POST /api/workspace/prep` handler **(1a)**
  - `ai-worker/tests/openharness/tools/test_bash_adversarial.py` **(4)**
  - `ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py` **(2)**
  - `ai-worker/tests/openharness/tools/test_base_tool_contract.py` **(2, expanded in 5)**
  - `ai-worker/tests/e2e/test_variant_b_smoke.py` **(7)**
- *Round 2 additions — Interaction tools + hooks + bidirectional RPC:*
  - `ai-worker/src/openharness/engine/agent_hooks.py` — `AgentHookRegistry`, `AgentHookContext`, `PreToolCallBlock`, Protocol signatures, `ClarificationCoordinator`, `ClarificationTimeout` **(5 + 5a)**
  - `ai-worker/src/openharness/engine/return_channel.py` — `ReturnChannel` class, Redis pub/sub subscriber lifecycle, `ClarificationResponse` validation **(5a)**
  - `ai-worker/src/openharness/tools/interaction_tools.py` — `RequestClarificationTool`, `RequestReviewTool`, `register_interaction_tools` helper **(5)**
  - `ai-worker/src/openharness/engine/prompts.py` — extracted `build_system_prompt` + new `build_reviewer_prompt` + `REVIEWER_SYSTEM_PROMPT` + `parse_verdict` **(5)**
  - `ai-worker/tests/openharness/engine/test_return_channel_adversarial.py` — adversarial bidirectional RPC tests **(5a)**
  - `ai-worker/tests/openharness/engine/test_hooks_integration.py` — hook contract tests **(5)**
  - `ai-worker/tests/openharness/engine/test_clarification_roundtrip_integration.py` **(5a)**
  - `ai-worker/tests/openharness/tools/test_interaction_tools.py` — unit tests for RequestClarificationTool, RequestReviewTool **(5)**
  - `forge-core/internal/module/agent/clarify_handler.go` — `POST /api/sessions/{id}/clarify` handler **(5a)**
  - `forge-core/internal/module/agent/clarify_handler_test.go` **(5a)**

**Modified files:**
- `forge-core/internal/workspace/manager.go` — Phase 1a: call `workspace.EnsureReady` via manager; keep `injectToken` temporarily. Phase 1b: delete `injectToken`, route through `gitCommand()`. **(1a then 1b)**
- `forge-core/internal/workspace/manager_test.go` — remove `EnsureClone` tests, add `EnsureReady` **(1a)**
- `forge-core/cmd/forge-core/main.go` — Phase 1a: pass `ProjectLookup` (HTTPS+token shape) into workspace.NewManager. Phase 1b: update `ProjectLookup` to SSH shape, wire deploy-key crypto service. **(1a then 1b)**
- `forge-core/internal/module/agent/service.go` — call `workspace.Manager.EnsureReady` before agent, pass relative workspace_path **(1a)**
- `forge-core/internal/temporal/activity/build_activities.go` — migrate `EnsureClone` call at line 96 to `EnsureReady` **(1a)**
- `forge-core/internal/temporal/activity/devops_activities.go` — migrate `EnsureClone` call at line 134 to `EnsureReady` **(1a)**
- `ai-worker/src/openharness/tools/base.py` — new signature + SimpleTool **(2)**
- `ai-worker/src/openharness/tools/context_tools.py` — migrate to SimpleTool **(2)**
- `ai-worker/src/openharness/engine/query.py` — adapt tool execution to new AsyncIterator contract; add `AgentHookRegistry` invocation points (pre_turn, pre_tool_call, post_turn); thread `clarification_coordinator` into `ToolExecutionContext` **(2, expanded in 5)**
- `ai-worker/src/openharness/engine/query_engine.py` — SessionCollector integration; `__init__` accepts hook registry + coordinator + return channel; new `close()` method for teardown **(5)**
- `ai-worker/src/openharness/engine/stream_events.py` — add PhaseChanged, add `tool_use_id` to ToolExecution* events, delete FixLoop*, add `ClarificationRequested(question, tool_use_id)` **(4, expanded in 5a)**
- `ai-worker/src/api_server.py` — remove pair routing, rewrite `_create_engine` to register T2 tools, LRU session cache, wire `AgentHookRegistry` + `ClarificationCoordinator` + `ReturnChannel`, `_serialize_event` branch for `ClarificationRequested` **(5, 5a)**
- `ai-worker/Dockerfile` — add `apt install bubblewrap ripgrep` **(4)**
- `forge-portal/components/agent/agent-chat.tsx` — new event handling, visual fix-loop detection, remove coder/reviewer roles, add clarification input state machine **(6)**
- `forge-portal/components/agent/step-ribbon.tsx` — dynamic phase tracking **(6)**
- `forge-portal/components/agent/tool-formatters.ts` — new tool formatters **(6)**
- `forge-portal/components/agent/code-panel.tsx` — degrade to read-only preview **(6)**
- `forge-portal/components/agent/thinking-indicator.tsx` — relocate rendering **(6)**
- `forge-portal/components/agent/clarification-input.tsx` — **new** clarification input component **(6)**

**Deleted files:**
- `ai-worker/src/openharness/engine/pair_pipeline.py` **(7 Step 3)**
- `ai-worker/tests/e2e/test_agent_pair_pipeline_e2e.py` **(7 Step 3)**
- `forge-portal/components/agent/build-card.tsx` **(7 Step 3)**
- `forge-portal/components/agent/build-card.test.tsx` **(7 Step 3)**
- `docs/plans/chronos-2026-04-09/phase-1-workspace.md` **(Round 2 plan rewrite, replaced by phase-1a + phase-1b)**

---

*End of spec.*
