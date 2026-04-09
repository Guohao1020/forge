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
   Real isolation requires process/namespace-level sandboxing (§3.6).
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

---

## 3. Workspace manager layer

New Go module: `forge-core/internal/module/workspace/`.

### 3.1 Module responsibility

Own the physical code artifact for each project: clone, pull, deploy
key lifecycle, dependency pre-install coordination, reset-on-new-session.
Does **not** own agent session state, conversation history, or tool
execution — those stay in `agent/`.

### 3.2 Files

| File | Responsibility |
|---|---|
| `model.go` | `Workspace`, `DeployKey`, `WorkspaceStatus` structs |
| `repository.go` | `engine.workspaces` and `engine.project_deploy_keys` DAO |
| `service.go` | `EnsureReady(ctx, projectID) (*Workspace, error)` main entry |
| `git.go` | Thin wrapper over `os/exec` for git commands (not go-git) |
| `keys.go` | ed25519 generation, AES-GCM encrypt/decrypt, GitHub deploy key upload |
| `sandbox_prep.go` | RPC client that asks ai-worker to pre-install deps |
| `service_test.go`, etc. | Unit + integration tests |

**Not using go-git:** go-git's HTTPS and SSH support is good enough for
reads but has known edge cases around auth agent forwarding, symlink
handling, and large repos. The stability and compatibility delta of
shelling out to system `git` is worth the small syscall overhead.

### 3.3 Data model

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

-- engine.project_deploy_keys
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

### 3.4 `EnsureReady` state machine

```
         ┌──────────┐
         │ no record│
         └────┬─────┘
              │ first call: INSERT status='pending'
              │             (ON CONFLICT acquires PG advisory lock)
              ▼
         ┌──────────┐      clone | prep fails
         │ pending  │ ─────────────────┐
         └────┬─────┘                  │
              │ clone ok               │
              │ deps preinstall ok     │
              ▼                        ▼
         ┌──────────┐               ┌──────────┐
         │  ready   │◀──┐           │  error   │
         └────┬─────┘   │           └────┬─────┘
              │         │                │ next call
              │         │                │ retries
              │ next    │ fetch+reset    │ from scratch
              │ call    │ succeeds       │ (wipe dir,
              ▼         │                │ re-clone)
         ┌──────────┐   │                │
         │  resync  │───┘                │
         │ (fetch + │                    │
         │  reset   │                    │
         │  hard)   │                    │
         └──────────┘                    │
                                         │
         (next EnsureReady call) ◀───────┘
```

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

### 3.5 SSH deploy key lifecycle

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

### 3.6 Dependency pre-install

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

### 3.7 Path plumbing to ai-worker

`RunRequest.workspace_path` already exists in `ai-worker/src/api_server.py`
as a relative path resolved against `FORGE_WORKSPACE_ROOT` env var on the
ai-worker side. This stays. Changes:

- `workspace_path` becomes **required**, not optional. Any request
  without it is a 400.
- The "is pair_pipeline or legacy" branching in `_route_and_stream` is
  removed. All requests flow through `QueryEngine` with the workspace
  path populated.
- Agent service (`forge-core/internal/module/agent/service.go`) calls
  `workspaceMgr.EnsureReady(ctx, projectID)` synchronously before
  submitting the RunRequest to ai-worker. If EnsureReady fails, the
  agent session fails with an `ErrorEvent` and the RunRequest is never
  sent.

### 3.8 Concurrency semantics

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

### 3.9 Failure-mode matrix

| Failure point | Status | last_error | Behavior |
|---|---|---|---|
| Deploy key upload to GitHub fails (4xx/5xx) | `error` | `deploy_key_upload failed: <code>` | Session halts, agent emits ErrorEvent |
| Clone fails — auth | `error` | `clone failed: authentication` | Halt; human must check GitHub deploy key permissions |
| Clone fails — network | `error` | `clone failed: network` | Halt; next call retries automatically |
| Clone fails — unknown | `error` | `clone failed: <stderr>` | Halt |
| Dependency prep fails | `ready` | (warning logged) | **Does not halt.** Agent will see build errors if deps are needed. |
| `git reset --hard` on resync fails | fall back to wipe + re-clone | — | Transparent recovery |
| Workspace directory missing (manual cleanup) | treated as "no record" | — | Re-clones from scratch |
| Disk full | not handled this release | — | Future: disk-check hook pre-clone |

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
        exactly one ToolResult as the final value. Must not raise —
        errors should be returned as ToolResult(is_error=True, output=...).
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
  `ListDirectoryTool`, `SetPhaseTool`: subclass `SimpleTool`.
- `BashTool`: subclasses `BaseTool` directly (needs mid-execution
  `ThinkingStarted`/`ThinkingStopped` events).
- All five `context_tools.py` tools: migrated to `SimpleTool`.

**Agent loop impact** (`query.py`):

```python
async def _execute_tool_call(
    context, tool_name, tool_use_id, tool_input,
) -> AsyncIterator[StreamEvent | ToolResultBlock]:
    tool_result: ToolResult | None = None
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

`run_agent_loop`'s tool execution block:

```python
tool_results: List[ToolResultBlock] = []
for tu in tool_uses:
    yield ToolExecutionStarted(tool_name=tu.name, tool_input=tu.input)
    async for item in _execute_tool_call(context, tu.name, tu.id, tu.input):
        if isinstance(item, ToolResultBlock):
            tool_results.append(item)
        else:
            yield item  # passthrough mid-execution events
    yield ToolExecutionCompleted(
        tool_name=tu.name,
        output=tool_results[-1].content,
        is_error=tool_results[-1].is_error,
    )
```

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
  --share-net=false \
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

- `--unshare-all`: new user, mount, PID, UTS, IPC, cgroup namespaces.
- `--share-net=false`: no network. This is intentional (see §4.8).
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
endpoint (§3.6), which runs language-specific commands *outside* the
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

```python
name = "set_phase"
description = (
    "Signal which phase you're currently in. The UI step ribbon will "
    "highlight that phase. Available phases: Analyze (understanding "
    "requirements and code), Plan (deciding changes), Generate (writing "
    "code), Build (compiling), Test (running tests), Review (verifying "
    "own work), Deploy (committing or preparing deployment). Call this "
    "when you start a new phase. You can go backwards (e.g., Build -> "
    "Generate to fix a compile error)."
)

Phase = Literal["Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"]

class SetPhaseInput(BaseModel):
    phase: Phase

class SetPhaseTool(SimpleTool):
    async def _execute_simple(self, arguments, context):
        # The phase_changed event is emitted by query.py's agent loop
        # when it sees this tool's name, since SimpleTool can't yield
        # events. Trade-off: slight coupling, but only one tool.
        # Alternative rejected: make SetPhaseTool subclass BaseTool
        # directly and yield PhaseChanged, for symmetry with BashTool.
        return ToolResult(output=f"Phase set to {arguments.phase}")
```

**Trade-off note:** There's a tension here. `BashTool` yields its own
events because it extends `BaseTool`. `SetPhaseTool` could do the same
for symmetry. It doesn't, because:

- The phase value is already in `tool_input`, the agent loop already
  yields `ToolExecutionStarted(tool_name="set_phase", tool_input={...})`,
  and frontend can derive `phase_changed` from that without a separate
  event type...

...except the frontend then needs to know that `set_phase` is
semantically different from other tools. That's the kind of "hidden
coupling" the silicon-valley standard rejects.

**Final decision:** `SetPhaseTool` extends `BaseTool` directly (not
`SimpleTool`) and yields a `PhaseChanged` event explicitly. Spending
10 lines on symmetry is worth it. The final code:

```python
class SetPhaseTool(BaseTool):
    name = "set_phase"
    description = "..."
    input_model = SetPhaseInput

    async def execute(self, arguments, context):
        yield PhaseChanged(phase=arguments.phase)
        yield ToolResult(output=f"Phase set to {arguments.phase}")

    def is_read_only(self, arguments):
        return True
```

### 4.12 Tool registry construction

`_create_engine` in `api_server.py`:

```python
def _create_engine(req: RunRequest, workspace_dir: Path) -> QueryEngine:
    tool_registry = ToolRegistry()

    # T2 file/exec tools — all scoped to workspace_dir
    tool_registry.register(ReadFileTool(workspace_dir))
    tool_registry.register(WriteFileTool(workspace_dir))
    tool_registry.register(EditFileTool(workspace_dir))
    tool_registry.register(GlobTool(workspace_dir))
    tool_registry.register(GrepTool(workspace_dir))
    tool_registry.register(ListDirectoryTool(workspace_dir))
    tool_registry.register(BashTool(workspace_dir))

    # Meta tool
    tool_registry.register(SetPhaseTool())

    # Legacy context tools (now using SimpleTool adapter)
    profiles = _load_project_profiles(req.project_id)
    register_context_tools(tool_registry, profiles, req.project_id)

    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    model = req.model or settings.default_model
    system_prompt = req.system_prompt or _build_system_prompt(
        language=detect_language(workspace_dir),
        workspace_path=str(workspace_dir),
    )

    try:
        router = ModelRouter()
        api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
    except Exception as e:
        logger.error("ModelRouter unavailable: %s", e)
        raise  # fail fast, no AsyncMock fallback

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=hook_executor,
        permission_checker=permission_checker,
        cwd=workspace_dir,
    )
```

**Removed from existing `_create_engine`:** the `purpose` parameter,
the `Purpose.REVIEW` branch of system_prompt selection, the
`AsyncMock` fallback for unavailable ModelRouter (fail fast — if
ModelRouter is down, the agent is down, and we want to know
immediately rather than silently running with a mock).

---

## 5. Agent loop and event layer

### 5.1 `run_agent_loop` changes

Only the tool execution block changes (adapting to the new `BaseTool`
signature). The overall flow — stream API, detect `tool_use`, execute
tools, pass results back, repeat — is unchanged. Code shown in §4.1.

**Unchanged safety/limits:**
- `max_turns` default 25
- `MaxTurnsExceeded` error event on exhaustion
- `ApiTextDeltaEvent` → `AssistantTextDelta` passthrough
- Messages modified in place, returned to `QueryEngine._messages`

### 5.2 System prompt

`_build_system_prompt` is a new function in `api_server.py` or
`prompt_templates.py`:

```python
def _build_system_prompt(language: str | None, workspace_path: str) -> str:
    lang_hint = (
        f"- Project language: {language}"
        if language else
        "- Project language: unknown (inspect files to detect)"
    )
    return f"""You are Forge Agent, an AI coding assistant embedded in a Harness Engineering platform.
You work on a user's codebase inside a sandboxed workspace.

## Your environment
- Workspace root: {workspace_path}
{lang_hint}
- Sandbox: no network, cwd locked to workspace, bash timeout 120s default

## Available tools
You have tools for reading, writing, and editing files, searching code
(glob, grep), listing directories, running shell commands (bash), and
signaling your current phase to the UI (set_phase).

## How to work
1. Understand the user's request. Ask for clarification if it's ambiguous.
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
```

This is a starting-point skeleton. It will be iterated based on real
agent runs. The prompt lives in source control and has unit tests that
verify the `{language}` and `{workspace_path}` substitutions work.

### 5.3 Event vocabulary (final)

| Event | Source | Change |
|---|---|---|
| `AssistantTextDelta` | `ApiTextDeltaEvent` → yield | Unchanged |
| `AssistantTurnComplete` | `ApiMessageCompleteEvent` → yield | Unchanged |
| `ToolExecutionStarted` | `run_agent_loop` before tool call | Unchanged |
| `ToolExecutionCompleted` | `run_agent_loop` after tool call | Unchanged |
| `ThinkingStarted(label)` | `BashTool.execute()` | Repurposed: bash only |
| `ThinkingStopped` | `BashTool.execute()` finally block | Repurposed |
| `PhaseChanged(phase)` | `SetPhaseTool.execute()` | **New** |
| `SessionComplete(...)` | `QueryEngine.submit_message` end | Retained, new trigger logic |
| `ErrorEvent(message, recoverable)` | Any exception path | Unchanged |
| ~~`FixLoopStarted`~~ | ~~pair_pipeline~~ | **Deleted** |
| ~~`FixLoopCompleted`~~ | ~~pair_pipeline~~ | **Deleted** |

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

### 5.5 `FixLoop*` event deletion

- Remove `FixLoopStarted` and `FixLoopCompleted` from
  `stream_events.py`.
- Remove their branches in `_serialize_event`.
- Remove `_role_for_event_type`'s mentions of `fix_loop_*`.
- Frontend visual detection replaces the event (§6.2).

### 5.6 Session complete logic

```python
class SessionCollector:
    """Tracks per-turn statistics from tool execution events."""

    def __init__(self) -> None:
        self.files_created = 0
        self.files_modified = 0
        self.tool_call_count = 0
        self.last_build_status: str = "skipped"

    def observe(self, event: StreamEvent) -> None:
        if isinstance(event, ToolExecutionCompleted):
            self.tool_call_count += 1
            if event.tool_name == "write_file" and not event.is_error:
                self.files_created += 1
            elif event.tool_name == "edit_file" and not event.is_error:
                self.files_modified += 1
            elif event.tool_name == "bash":
                # Heuristic: if the command looks build-y, update status
                if self._is_build_like(event.tool_input):
                    self.last_build_status = "passed" if not event.is_error else "failed"

    def should_emit_summary(self) -> bool:
        return self.tool_call_count > 0
```

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

`_route_and_stream` becomes:

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

    engine = _sessions.get(session_id)
    if engine is None:
        engine = _create_engine(req, workspace_dir=resolved)
        _sessions.put(session_id, engine)

    async for event in engine.submit_message(req.message):
        yield event
```

### 5.8 Session cache (LRU)

Replace `_sessions: Dict[str, Any]` with an `LRUSessionCache`:

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

    def put(self, session_id: str, engine: QueryEngine) -> None:
        if session_id in self._cache:
            self._cache.move_to_end(session_id)
        else:
            self._cache[session_id] = engine
            if len(self._cache) > self._maxsize:
                oldest_id, oldest_engine = self._cache.popitem(last=False)
                oldest_engine.clear()
                logger.info("Evicted LRU session %s", oldest_id)

    def pop(self, session_id: str) -> QueryEngine | None:
        return self._cache.pop(session_id, None)
```

100 sessions × ~2MB per (messages + local state) = ~200MB ceiling. Fine
for dev; when prod load justifies, swap for Redis-backed store.

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
| `test_bash_cannot_read_other_tenant_workspace` | Attempting to access another tenant's workspace path fails |
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

| Test | Expected |
|---|---|
| `TestTenantIsolation_ProjectPathsNoOverlap` | tenant A project 1 and tenant B project 1 have distinct paths |
| `TestDeployKey_PrivateKeyEncryptionRoundtrip` | encrypt(decrypt(x)) == x |
| `TestDeployKey_CiphertextDoesNotContainPlaintext` | encrypted blob has no plaintext substring |
| `TestDeployKey_EachKeyHasUniqueNonce` | two encryptions of same plaintext produce different ciphertexts |
| `TestEnsureReady_ConcurrentCallers_SingleClone` | two concurrent calls trigger one git clone, not two |

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

### 7.3 Contract tests (BaseTool)

`tests/openharness/tools/test_base_tool_contract.py`:

```python
@pytest.mark.parametrize("tool_class", ALL_REGISTERED_TOOL_CLASSES)
async def test_tool_yields_exactly_one_tool_result(tool_class, workspace):
    tool = tool_class(workspace)
    arguments = _make_valid_arguments(tool)
    context = ToolExecutionContext(cwd=workspace)

    tool_result_count = 0
    items = []
    async for item in tool.execute(arguments, context):
        items.append(item)
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
async def test_tool_does_not_raise_on_invalid_input(tool_class, workspace):
    """Tools must return ToolResult(is_error=True) rather than raising."""
    tool = tool_class(workspace)
    context = ToolExecutionContext(cwd=workspace)

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

`tests/e2e/test_variant_b_smoke.py` — runs against a real LLM:

```python
@pytest.mark.e2e
@pytest.mark.skipif(not os.getenv("FORGE_E2E_ENABLED"), reason="E2E disabled")
async def test_agent_can_complete_variant_b_workflow():
    # 1. Set up a small fixture Go project in a temp workspace
    workspace = _setup_fixture_go_project()

    # 2. Create an agent session and submit a real message
    engine = _create_engine(
        RunRequest(
            session_id="test-e2e",
            project_id=1,
            workspace_path=str(workspace.relative_to(FORGE_WORKSPACE_ROOT)),
            message="Add a GET /hello endpoint that returns 'world' as JSON.",
        ),
        workspace_dir=workspace,
    )

    # 3. Collect events
    events: List[StreamEvent] = []
    async for event in engine.submit_message(
        "Add a GET /hello endpoint that returns 'world' as JSON."
    ):
        events.append(event)

    # 4. Assert the workflow signature
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

    # 5. Verify the workspace actually has a /hello endpoint that compiles
    result = subprocess.run(
        ["go", "build", "./..."],
        cwd=workspace,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"Build failed: {result.stderr}"
```

- Runs in CI only on merge to main (cost ~$0.10 per run).
- Uses a known-stable fixture project (small, ~5 files).
- Does **not** assert specific tool call counts or phases — only the
  presence of the expected shape. Real agents are non-deterministic.

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
- Remove all remaining `fix_loop_*` references, `Purpose.REVIEW` branch
  in `_create_engine`, `AgentRole.coder|reviewer` usage.
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

## Appendix B — Files touched (summary)

**New files:**
- `forge-core/internal/module/workspace/{model,repository,service,git,keys,sandbox_prep}.go`
- `forge-core/migrations/{nnn}_create_workspaces.sql`
- `forge-core/migrations/{nnn+1}_create_project_deploy_keys.sql`
- `ai-worker/src/openharness/tools/workspace_path.py`
- `ai-worker/src/openharness/tools/file_tools.py`
- `ai-worker/src/openharness/tools/bash_tool.py`
- `ai-worker/src/openharness/tools/phase_tool.py`
- `ai-worker/src/openharness/engine/session_collector.py`
- `ai-worker/src/api_server_prep_handler.py` (or route added to existing)
- `ai-worker/tests/openharness/tools/test_bash_adversarial.py`
- `ai-worker/tests/openharness/tools/test_workspace_path_adversarial.py`
- `ai-worker/tests/openharness/tools/test_base_tool_contract.py`
- `ai-worker/tests/e2e/test_variant_b_smoke.py`

**Modified files:**
- `ai-worker/src/openharness/tools/base.py` — new signature + SimpleTool
- `ai-worker/src/openharness/tools/context_tools.py` — migrate to SimpleTool
- `ai-worker/src/openharness/engine/query.py` — adapt tool execution
- `ai-worker/src/openharness/engine/query_engine.py` — SessionCollector integration
- `ai-worker/src/openharness/engine/stream_events.py` — add PhaseChanged, delete FixLoop*
- `ai-worker/src/api_server.py` — remove pair routing, rewrite `_create_engine`, LRU session cache
- `forge-core/internal/module/agent/service.go` — call `workspace.Service.EnsureReady` before agent
- `forge-portal/components/agent/agent-chat.tsx` — new event handling, visual fix-loop detection, remove coder/reviewer roles
- `forge-portal/components/agent/step-ribbon.tsx` — dynamic phase tracking
- `forge-portal/components/agent/tool-formatters.ts` — new tool formatters
- `forge-portal/components/agent/code-panel.tsx` — degrade to read-only preview
- `forge-portal/components/agent/thinking-indicator.tsx` — relocate rendering

**Deleted files:**
- `ai-worker/src/openharness/engine/pair_pipeline.py`
- `ai-worker/tests/e2e/test_agent_pair_pipeline_e2e.py`
- `forge-portal/components/agent/build-card.tsx`
- `forge-portal/components/agent/build-card.test.tsx`

---

*End of spec.*
