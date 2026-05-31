# chronos · Phase 0 — Infrastructure & Plumbing

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 0 of 7 · **Tasks:** 6 · **Depends on:** nothing · **Unblocks:** everything
> **Spec reference:** [Design spec §2.8, §3, §4.1, §4.4](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Land all the "clear the decks" changes — container base image updates, Python deps, dead code deletion, DB migrations, secrets service — so subsequent phases have a clean floor to build on.

**Completion gate:** `docker compose -f docker-compose.dev.yml build forge-ai-worker` succeeds with bwrap and ripgrep installed; forge-core builds cleanly without `pair_pipeline`/`build_verify_hook`/`ci_autofix_hook`; `engine.workspaces` and `engine.project_deploy_keys` tables exist; `secrets.Encrypt`/`Decrypt` unit tests pass.

**Downstream impact:** every later phase either imports the secrets service, inserts into the new tables, runs inside the updated container, or depends on the dead-code deletion having landed. Phase 0 is the floor.

---

### Task 0.1: Add bubblewrap + ripgrep to ai-worker container

**Files:**
- Modify: `ai-worker/Dockerfile`

**Context:** BashTool will run shell commands inside bwrap (a Linux namespace sandbox — think "chroot but safer, no network, isolated from host"). The base image also needs ripgrep because GrepTool shells out to `rg` (no Python fallback, one code path).

- [ ] **Step 1: Read the current Dockerfile**

Run: `cat ai-worker/Dockerfile`
Expected: see `apt-get install -y --no-install-recommends ca-certificates` on line 10-12, Python 3.12-slim base.

- [ ] **Step 2: Add bubblewrap + ripgrep to apt install list**

Edit `ai-worker/Dockerfile` lines 10-12. Replace:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
```

With:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    bubblewrap \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*
```

- [ ] **Step 3: Verify the image builds**

Run: `docker build -t forge-ai-worker:variant-b-test ai-worker/`
Expected: build succeeds, no "unable to locate package" errors for bubblewrap or ripgrep.

- [ ] **Step 4: Verify the installed binaries work inside the image**

Run:
```bash
docker run --rm --cap-add SYS_ADMIN forge-ai-worker:variant-b-test \
  bash -lc 'bwrap --version && rg --version'
```
Expected: both `bubblewrap 0.x.x` and `ripgrep x.y.z` print with version numbers. `SYS_ADMIN` is required because bwrap uses namespace syscalls.

**Note:** if `bwrap --version` errors with "Failed to create new namespace" even under `--cap-add SYS_ADMIN`, the host kernel may not have user namespaces enabled. Check with `sysctl kernel.unprivileged_userns_clone` on the host. For Docker Desktop on Linux this is usually fine; on macOS/Windows the check happens inside the Linux VM which supports it.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/Dockerfile
git commit -m "feat(ai-worker): add bubblewrap + ripgrep to container base image

Required by the upcoming BashTool (bwrap sandbox) and GrepTool
(ripgrep backend). This unblocks Phase 4 of the single-agent refactor."
```

---

### Task 0.2: Add `pathspec` to Python requirements

**Files:**
- Modify: `ai-worker/requirements.txt`

**Context:** GlobTool uses the `pathspec` library for gitignore-style pattern matching rather than handwritten fnmatch. This is the "don't reinvent the glob" rule — Git already solved this and `pathspec` is the canonical Python port.

- [ ] **Step 1: Add pathspec pinned to 0.12+**

Edit `ai-worker/requirements.txt`. After the `# Config` section (pydantic lines), add:

```
# Path matching (GlobTool — gitignore-style patterns)
pathspec>=0.12
```

- [ ] **Step 2: Verify pip install picks it up**

Run:
```bash
cd ai-worker
python -m venv /tmp/forge-req-check-venv
/tmp/forge-req-check-venv/bin/pip install -r requirements.txt
/tmp/forge-req-check-venv/bin/python -c "import pathspec; print(pathspec.__version__)"
rm -rf /tmp/forge-req-check-venv
```
Expected: pathspec installs, import works, version >= 0.12.

(If you're on Windows host doing plan-mode dev, skip the venv check — the install will be verified when Task 0.1 rebuilds the container in Step 3.)

- [ ] **Step 3: Commit**

```bash
git add ai-worker/requirements.txt
git commit -m "feat(ai-worker): add pathspec dep for GlobTool

pathspec>=0.12 is the canonical Python impl of gitignore-style pattern
matching. Used by GlobTool in Phase 3."
```

---

### Task 0.3: Delete pair_pipeline and dead hook files

**Files:**
- Delete: `ai-worker/src/openharness/engine/pair_pipeline.py`
- Delete: `ai-worker/src/openharness/hooks/builtin/build_verify_hook.py`
- Delete: `ai-worker/src/openharness/hooks/builtin/ci_autofix_hook.py`
- Delete: `ai-worker/tests/test_pair_pipeline.py`
- Delete: `ai-worker/tests/test_build_verify.py`
- Delete: `ai-worker/tests/test_ci_autofix.py`
- Delete: `ai-worker/tests/e2e/test_pair_pipeline_real_llm.py`
- Modify: `ai-worker/src/openharness/hooks/builtin/__init__.py` (remove BuildVerifyHook export)

**Context:** A2 architecture eliminates the outer Coder→BuildVerify→Reviewer loop. The BuildVerifyHook and CiAutofixHook existed solely to serve that loop — they have no other consumers once pair_pipeline is gone. Verified via grep before this task was written (see reconnaissance in this plan's predecessor session).

The spec's original file-delete list missed these two hook files; this task catches them. Deleting them now (rather than Phase 5 when we rewrite `_create_engine`) has one advantage: any forgotten `from ... import BuildVerifyHook` anywhere in the codebase fails loudly at Phase 0 build time, not in the middle of a bigger refactor.

- [ ] **Step 1: Sanity-check no other code imports these**

Run:
```bash
grep -rn "pair_pipeline\|PairPipeline\|build_verify_hook\|BuildVerifyHook\|ci_autofix_hook\|CiAutofixHook" \
  ai-worker/src/ forge-core/ forge-portal/ \
  --include="*.py" --include="*.go" --include="*.ts" --include="*.tsx"
```
Expected: matches only in `ai-worker/src/api_server.py` (a single comment referring to BuildVerifyHook in the pair_pipeline-routing block, which we will rewrite in Phase 5), `pair_pipeline.py` itself, and the hook files themselves. If anything else shows up, stop and investigate.

- [ ] **Step 2: Delete the source files**

```bash
rm ai-worker/src/openharness/engine/pair_pipeline.py
rm ai-worker/src/openharness/hooks/builtin/build_verify_hook.py
rm ai-worker/src/openharness/hooks/builtin/ci_autofix_hook.py
```

- [ ] **Step 3: Update `hooks/builtin/__init__.py`**

The current file is a single line:
```python
from .build_verify_hook import BuildVerifyHook
```

Replace it with an empty comment so the file still exists as a package marker:

```python
"""Builtin hooks. Intentionally empty after the A2 refactor —
pair_pipeline's BuildVerifyHook and CiAutofixHook were removed along with
pair_pipeline itself. New hooks go here if they're needed."""
```

- [ ] **Step 4: Delete the test files**

```bash
rm ai-worker/tests/test_pair_pipeline.py
rm ai-worker/tests/test_build_verify.py
rm ai-worker/tests/test_ci_autofix.py
rm ai-worker/tests/e2e/test_pair_pipeline_real_llm.py
```

Also clean compiled bytecode that may linger:
```bash
find ai-worker -name "pair_pipeline*.pyc" -delete 2>/dev/null
find ai-worker -name "build_verify*.pyc" -delete 2>/dev/null
find ai-worker -name "ci_autofix*.pyc" -delete 2>/dev/null
```

- [ ] **Step 5: Verify api_server.py still imports cleanly**

`api_server.py` currently has a guarded try/except import of `run_pair_pipeline` and `PairPipelineConfig`. After this deletion that try block will fail its import at module load time and fall through to the except branch setting them to `None`. That's fine — it was already designed to degrade. The actual fix happens in Phase 5 when we rewrite `_create_engine` and `_route_and_stream`. For now, verify the file still parses:

```bash
python -c "import ast; ast.parse(open('ai-worker/src/api_server.py').read()); print('OK')"
```
Expected: `OK`.

- [ ] **Step 6: Run remaining ai-worker tests to confirm the delete didn't break unrelated tests**

Run inside the container (the one we'll rebuild in Task 0.1):
```bash
docker run --rm -v "$(pwd)/ai-worker:/app" forge-ai-worker:variant-b-test \
  bash -lc 'cd /app && python -m pytest tests/ -x --ignore=tests/e2e -q 2>&1 | tail -30'
```
Expected: collection succeeds (no ImportError on deleted modules), tests that don't depend on pair_pipeline pass or fail with their pre-existing statuses. If a test fails with `ModuleNotFoundError: No module named 'src.openharness.engine.pair_pipeline'`, that test also needs deletion — add it to the `rm` list above and restart this step.

- [ ] **Step 7: Commit**

```bash
git add -u ai-worker/
git commit -m "feat(ai-worker): delete pair_pipeline and dead hooks (A2 refactor)

Single-agent architecture eliminates the outer Coder→BuildVerify→Reviewer
loop. BuildVerifyHook and CiAutofixHook existed only for pair_pipeline —
no other consumers. Deleting now so any stray imports fail loudly at
Phase 0 rather than mid-refactor.

- pair_pipeline.py, test_pair_pipeline.py, e2e test
- build_verify_hook.py, test_build_verify.py
- ci_autofix_hook.py, test_ci_autofix.py
- hooks/builtin/__init__.py emptied to package marker

api_server.py still has a guarded pair_pipeline import block; it
degrades cleanly to None on import failure. Phase 5 will rewrite
_route_and_stream and _create_engine to drop the guard entirely."
```

---

### Task 0.4: Migration 025 — `engine.workspaces` table

**Files:**
- Create: `forge-core/migrations/025_workspaces.sql`

**Context:** The workspace state machine needs a persisted row per (tenant, project). Three states: `pending` | `ready` | `error`. UNIQUE(tenant_id, project_id) gives us the natural PK for the row, and PG advisory locks will serialize concurrent `EnsureReady` calls.

- [ ] **Step 1: Check the migration numbering**

Run: `ls forge-core/migrations/ | tail -5`
Expected: `024_agent_sessions.sql` is the latest. The next number is 025. If anything has shifted, use the next available number and update the filename below.

- [ ] **Step 2: Write the migration**

Create `forge-core/migrations/025_workspaces.sql`:

```sql
-- Workspace state machine for the single-agent Variant B architecture.
--
-- One row per (tenant, project). Lifecycle:
--
--   no row → INSERT status='pending' (with pg_advisory_xact_lock)
--   pending → clone + deps prep ok → UPDATE status='ready'
--   pending → clone or prep fails  → UPDATE status='error', last_error
--   error   → next EnsureReady call wipes dir, re-enters 'pending'
--   ready   → subsequent calls may fetch + reset --hard (row stays 'ready')
--
-- Advisory lock protocol:
--   SELECT pg_advisory_xact_lock(hashtext('workspace:' || tenant || ':' || project))
--   within the transaction that reads/writes this row.

CREATE TABLE IF NOT EXISTS engine.workspaces (
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

CREATE INDEX IF NOT EXISTS idx_workspaces_tenant_project
    ON engine.workspaces(tenant_id, project_id);

CREATE INDEX IF NOT EXISTS idx_workspaces_status_updated
    ON engine.workspaces(status, updated_at DESC);

COMMENT ON TABLE engine.workspaces IS
    'One row per (tenant, project). Owns the clone lifecycle for the single-agent Variant B architecture. Guarded by pg_advisory_xact_lock on hashtext(workspace:tenant:project).';
```

- [ ] **Step 3: Apply the migration locally**

Run:
```bash
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U forge -d forge -f - < forge-core/migrations/025_workspaces.sql
```
Expected: `CREATE TABLE`, `CREATE INDEX`, `CREATE INDEX`, `COMMENT` all succeed.

If the postgres container is not running: `docker compose -f docker-compose.dev.yml up -d postgres` first.

- [ ] **Step 4: Verify table shape**

Run:
```bash
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U forge -d forge -c "\d engine.workspaces"
```
Expected: table shows 10 columns, status CHECK constraint, unique index on (tenant_id, project_id), secondary index on (status, updated_at desc).

- [ ] **Step 5: Verify idempotency (re-running the migration is safe)**

Run the same `psql -f` command a second time. Expected: `NOTICE: relation "workspaces" already exists, skipping` (or similar) — no error.

- [ ] **Step 6: Commit**

```bash
git add forge-core/migrations/025_workspaces.sql
git commit -m "feat(forge-core): migration 025 engine.workspaces

Three-state row (pending|ready|error) per (tenant, project) for the
Variant B workspace manager. Protected by pg_advisory_xact_lock at
the DAO layer — see Phase 1 for Go implementation."
```

---

### Task 0.5: Migration 026 — `engine.project_deploy_keys` table

**Files:**
- Create: `forge-core/migrations/026_project_deploy_keys.sql`

**Context:** Project-level SSH deploy keys, one keypair per project. Public key uploaded to GitHub via the deploy-key API, private key encrypted with AES-GCM and stored in `private_key_enc`. The encryption layer (§secrets) is Task 0.6 next; this table is where those ciphertexts live.

- [ ] **Step 1: Write the migration**

Create `forge-core/migrations/026_project_deploy_keys.sql`:

```sql
-- Per-project SSH deploy keys for git clone auth in the single-agent
-- architecture. One keypair per project. Public key is uploaded to
-- GitHub via POST /repos/{owner}/{repo}/keys (read_only=false for
-- forward compat with future git push from agent). Private key is
-- AES-GCM encrypted with a key derived from FORGE_SECRETS_MASTER_KEY.
--
-- Storage format of private_key_enc:
--   nonce(12 bytes) || ciphertext || tag(16 bytes)
-- See forge-core/internal/secrets/crypto.go for Encrypt/Decrypt.
--
-- github_key_id is the ID returned by GitHub's deploy-key API, used
-- for future rotation (delete old → upload new).

CREATE TABLE IF NOT EXISTS engine.project_deploy_keys (
    project_id       BIGINT PRIMARY KEY,
    tenant_id        BIGINT NOT NULL,
    public_key       TEXT NOT NULL,
    private_key_enc  BYTEA NOT NULL,
    key_type         TEXT NOT NULL DEFAULT 'ed25519',
    github_key_id    BIGINT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_deploy_keys_tenant
    ON engine.project_deploy_keys(tenant_id);

COMMENT ON TABLE engine.project_deploy_keys IS
    'Per-project SSH deploy keys. Private key is AES-GCM encrypted with key derived from FORGE_SECRETS_MASTER_KEY via HKDF.';

COMMENT ON COLUMN engine.project_deploy_keys.private_key_enc IS
    'Storage format: nonce(12) || ciphertext || tag(16). See forge-core/internal/secrets/crypto.go.';
```

- [ ] **Step 2: Apply the migration**

```bash
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U forge -d forge -f - < forge-core/migrations/026_project_deploy_keys.sql
```
Expected: `CREATE TABLE`, `CREATE INDEX`, `COMMENT`, `COMMENT` — all succeed.

- [ ] **Step 3: Verify table shape**

```bash
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U forge -d forge -c "\d engine.project_deploy_keys"
```
Expected: 7 columns, `project_id` is the primary key (not a serial), `private_key_enc` is `bytea` not `text`.

- [ ] **Step 4: Test idempotency**

Run the `psql -f` command a second time. Expected: no error.

- [ ] **Step 5: Commit**

```bash
git add forge-core/migrations/026_project_deploy_keys.sql
git commit -m "feat(forge-core): migration 026 engine.project_deploy_keys

Per-project ed25519 deploy keys for SSH-based git auth. private_key_enc
is bytea holding AES-GCM(nonce || ciphertext || tag). Secrets service
(Task 0.6) owns the encryption."
```

---

### Task 0.6: Secrets service — AES-GCM Encrypt/Decrypt

**Files:**
- Create: `forge-core/internal/secrets/crypto.go`
- Create: `forge-core/internal/secrets/crypto_test.go`

**Context:** A single internal service `secrets.Encrypt(plaintext) / Decrypt(ciphertext)` that owns symmetric crypto for the whole forge-core process. Phase 1's deploy key module uses it; any future secret (API keys in project settings, webhook secrets) can reuse it. The encryption key is derived via HKDF from `FORGE_SECRETS_MASTER_KEY` env var (a base64-encoded 32-byte master key). When we eventually swap to Vault/KMS, only this file changes.

The storage format is the same as the migration comment: `nonce(12) || ciphertext || tag(16)`. This is what `crypto/cipher.AEAD.Seal()` naturally produces when we prepend the nonce — no custom framing.

**Adversarial test requirements (mandatory):**
- Encryption is non-deterministic (two calls to `Encrypt(same_plaintext)` produce different ciphertexts because nonces are random)
- Ciphertext does not contain the plaintext as a substring
- Decrypting a tampered ciphertext returns an error (GCM auth tag catches it)
- Decrypting with the wrong master key returns an error

- [ ] **Step 1: Write the failing tests**

Create `forge-core/internal/secrets/crypto_test.go`:

```go
package secrets

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// testMasterKey is a fixed 32-byte key for deterministic testing.
// Production uses FORGE_SECRETS_MASTER_KEY env var.
var testMasterKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, 32))

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(testMasterKey)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	svc := newTestService(t)
	plaintext := []byte("hello world")
	ct, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := svc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("roundtrip mismatch: want %q got %q", plaintext, pt)
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	// Two encryptions of the same plaintext must produce different
	// ciphertexts because nonces are random. This is an essential
	// property: fixed nonces with the same key is a GCM break.
	svc := newTestService(t)
	plaintext := []byte("same plaintext every time")
	ct1, _ := svc.Encrypt(plaintext)
	ct2, _ := svc.Encrypt(plaintext)
	if bytes.Equal(ct1, ct2) {
		t.Fatal("ciphertexts are identical — nonce is not random, GCM is broken")
	}
}

func TestCiphertextDoesNotContainPlaintext(t *testing.T) {
	svc := newTestService(t)
	plaintext := []byte("FORGE_UNIQUE_MARKER_STRING_12345")
	ct, _ := svc.Encrypt(plaintext)
	if bytes.Contains(ct, plaintext) {
		t.Fatal("ciphertext contains plaintext as substring — not encrypted")
	}
	if strings.Contains(string(ct), "FORGE_UNIQUE_MARKER") {
		t.Fatal("ciphertext leaks plaintext partially")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	svc := newTestService(t)
	ct, _ := svc.Encrypt([]byte("secret data"))
	// Flip one bit in the ciphertext body (not nonce)
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[15] ^= 0x01
	_, err := svc.Decrypt(tampered)
	if err == nil {
		t.Fatal("tampered ciphertext decrypted without error — GCM auth failed")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	svc1 := newTestService(t)
	// Different master key
	otherKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xCD}, 32))
	svc2, err := NewService(otherKey)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ct, _ := svc1.Encrypt([]byte("written with svc1"))
	_, err = svc2.Decrypt(ct)
	if err == nil {
		t.Fatal("decryption with wrong key succeeded — should fail auth check")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	// Encrypting empty bytes should still work and produce a valid
	// ciphertext (12-byte nonce + 0-byte body + 16-byte tag = 28 bytes).
	svc := newTestService(t)
	ct, err := svc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt(empty): %v", err)
	}
	if len(ct) != 12+16 {
		t.Fatalf("want 28-byte ciphertext for empty plaintext, got %d", len(ct))
	}
	pt, err := svc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt(empty ct): %v", err)
	}
	if len(pt) != 0 {
		t.Fatalf("want empty plaintext, got %d bytes", len(pt))
	}
}

func TestNewServiceRejectsShortKey(t *testing.T) {
	// Master key must be exactly 32 bytes after base64 decode.
	short := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err := NewService(short)
	if err == nil {
		t.Fatal("NewService accepted a short master key")
	}
}

func TestNewServiceRejectsBadBase64(t *testing.T) {
	_, err := NewService("not-valid-base64!!!")
	if err == nil {
		t.Fatal("NewService accepted invalid base64")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail with "package doesn't exist"**

Run: `cd forge-core && go test ./internal/secrets/...`
Expected: compilation error — `package secrets` does not exist yet. This is the intended TDD failure.

- [ ] **Step 3: Implement the secrets service**

Create `forge-core/internal/secrets/crypto.go`:

```go
// Package secrets provides symmetric crypto for the forge-core process.
//
// A single Service instance owns an HKDF-derived AES-256-GCM key
// material keyed from FORGE_SECRETS_MASTER_KEY (a base64-encoded
// 32-byte master key). Callers use Encrypt/Decrypt to seal arbitrary
// byte slices — deploy key private keys today, potentially more
// secret kinds later.
//
// Storage format (what Encrypt returns and Decrypt consumes):
//
//	nonce(12) || ciphertext || tag(16)
//
// This is exactly what cipher.AEAD.Seal produces when we prepend the
// nonce before the output and Open consumes when we pass nonce as the
// first arg. No custom framing.
//
// When the master key source changes (Vault, KMS, etc.), only
// NewService needs to change — everything else calls Encrypt/Decrypt
// through the same interface.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	masterKeyBytes = 32 // AES-256
	nonceBytes     = 12 // GCM standard nonce size
	tagBytes       = 16 // GCM tag
	hkdfSalt       = "forge-secrets-v1"
	hkdfInfo       = "forge-deploy-key-encryption"
)

// Service is the single entry point for symmetric encryption in forge-core.
type Service struct {
	aead cipher.AEAD
}

// NewService constructs a Service from a base64-encoded 32-byte master
// key. The derived key is held in-process via aes.NewCipher / GCM.
//
// Errors on:
//   - base64 decode failure
//   - master key length != 32 bytes after decoding
//   - HKDF or AES initialization failure (unreachable in practice)
func NewService(masterKeyB64 string) (*Service, error) {
	master, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("secrets: master key is not valid base64: %w", err)
	}
	if len(master) != masterKeyBytes {
		return nil, fmt.Errorf("secrets: master key must decode to %d bytes, got %d",
			masterKeyBytes, len(master))
	}

	// Derive a separate subkey for encryption rather than using the
	// master directly. This is defense-in-depth: rotating the info
	// tag lets us introduce a new derivation domain without changing
	// the master key.
	derivedKey := make([]byte, 32)
	kdf := hkdf.New(func() hashNew { return sha256New() }, master, []byte(hkdfSalt), []byte(hkdfInfo))
	if _, err := io.ReadFull(kdf, derivedKey); err != nil {
		return nil, fmt.Errorf("secrets: HKDF expand failed: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: cipher.NewGCM: %w", err)
	}

	return &Service{aead: aead}, nil
}

// Encrypt seals plaintext with AES-256-GCM using a fresh random 12-byte
// nonce. The returned blob has the nonce prepended so Decrypt can
// recover it without a separate parameter. The output is
// len(plaintext) + 28 bytes (12 nonce + 16 tag).
func (s *Service) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secrets: rand.Read: %w", err)
	}
	// Seal appends the output to the first arg. We pass `nonce` so the
	// result is literally nonce || ciphertext || tag, which is the
	// storage format we committed to in the migration comments.
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens a blob produced by Encrypt. Returns an error on any of:
//   - blob too short to contain nonce + tag
//   - GCM auth tag mismatch (wrong key, tampered blob, wrong nonce)
func (s *Service) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) < nonceBytes+tagBytes {
		return nil, errors.New("secrets: ciphertext too short")
	}
	nonce := blob[:nonceBytes]
	ct := blob[nonceBytes:]
	pt, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt: %w", err)
	}
	return pt, nil
}
```

- [ ] **Step 4: Resolve the hash import stub**

The sketch above used placeholder `hashNew` / `sha256New`. Replace them with the real import. Edit `crypto.go`:

Replace the import block:
```go
import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)
```

With:
```go
import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/hkdf"
)
```

And replace the HKDF call:
```go
kdf := hkdf.New(func() hashNew { return sha256New() }, master, []byte(hkdfSalt), []byte(hkdfInfo))
```

With:
```go
kdf := hkdf.New(func() hash.Hash { return sha256.New() }, master, []byte(hkdfSalt), []byte(hkdfInfo))
```

- [ ] **Step 5: Fetch the HKDF dependency**

Run: `cd forge-core && go get golang.org/x/crypto/hkdf && go mod tidy`
Expected: `go.mod` gets a new indirect dep on `golang.org/x/crypto`; if it was already present, nothing changes.

- [ ] **Step 6: Run tests and verify they all pass**

Run: `cd forge-core && go test ./internal/secrets/... -v`
Expected: all 9 tests pass:
- TestEncryptDecryptRoundtrip
- TestEncryptNonDeterministic
- TestCiphertextDoesNotContainPlaintext
- TestDecryptTamperedCiphertextFails
- TestDecryptWithWrongKeyFails
- TestEncryptEmptyPlaintext
- TestNewServiceRejectsShortKey
- TestNewServiceRejectsBadBase64

If any test fails, fix the implementation and re-run — do not proceed.

- [ ] **Step 7: Run `go vet` on the new package**

Run: `cd forge-core && go vet ./internal/secrets/...`
Expected: no output (clean).

- [ ] **Step 8: Commit**

```bash
git add forge-core/internal/secrets/ forge-core/go.mod forge-core/go.sum
git commit -m "feat(forge-core): add secrets service with AES-GCM Encrypt/Decrypt

Single internal service owning symmetric crypto for the whole forge-core
process. Master key comes from FORGE_SECRETS_MASTER_KEY (base64 32
bytes), HKDF-derived to a per-domain subkey via 'forge-deploy-key-
encryption' info tag. Storage format: nonce(12) || ciphertext || tag(16).

Includes adversarial test suite:
- non-deterministic encryption (random nonces)
- ciphertext does not contain plaintext
- tampered ciphertext rejected by GCM auth
- wrong-key decryption rejected
- short/invalid master key rejected at construction

When we swap to Vault/KMS, only NewService changes."
```

---

### Phase 0 completion check

Before starting Phase 1:

- [ ] `docker build ai-worker/` succeeds and the built image has `bwrap` + `rg` on PATH
- [ ] `ai-worker/requirements.txt` includes `pathspec>=0.12`
- [ ] `ai-worker/src/openharness/engine/pair_pipeline.py` is deleted
- [ ] `ai-worker/src/openharness/hooks/builtin/{build_verify,ci_autofix}_hook.py` are deleted
- [ ] `ai-worker/tests/test_{pair_pipeline,build_verify,ci_autofix}.py` and `e2e/test_pair_pipeline_real_llm.py` are deleted
- [ ] `hooks/builtin/__init__.py` is now an empty package marker
- [ ] `engine.workspaces` and `engine.project_deploy_keys` tables exist in the dev database
- [ ] `go test ./internal/secrets/...` passes all 9 test cases
- [ ] Branch has 6 new commits (one per task)

Phase 0 outputs are leveraged by every later phase:
- Phase 1 imports `internal/secrets` for deploy key encryption
- Phase 1 inserts into `engine.workspaces` and `engine.project_deploy_keys`
- Phases 2-7 run inside the container with bwrap + ripgrep available
- Phase 3 uses `pathspec` for GlobTool
- Phase 5 can delete the guarded pair_pipeline import in api_server.py because the module doesn't exist anymore

---
