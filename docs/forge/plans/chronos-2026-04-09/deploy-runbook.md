# chronos Deploy Runbook

> **When:** after all Phase 0-7 code has landed on `feat/agent-variant-b-single-agent` and CI is green
> **Who:** Harvey (or whoever is shepherding the deploy)
> **Rollback:** `git reset --hard <pre-deploy-sha>` + `docker-compose restart`
> **Estimated time:** 15-30 minutes for the deploy, +5 minutes for verification

---

## Pre-deploy checklist

- [ ] All Phase 0-7 completion checks are green (revisit each phase file's "Phase N completion check" section)
- [ ] `git status` in the repo is clean
- [ ] Capture the current deployed SHA for rollback:
  ```bash
  PRE_DEPLOY_SHA=$(git rev-parse HEAD)
  echo "$PRE_DEPLOY_SHA" > /tmp/chronos-rollback-sha
  ```
- [ ] Confirm `FORGE_SECRETS_MASTER_KEY` is set in the forge-core deployment environment (Phase 0 Task 0.6 introduced this env var -- missing it makes workspace module fall back to legacy mode with a loud warning)
- [ ] Confirm `docker-compose.dev.yml` (or prod equivalent) has the ai-worker container configured with the Phase 0 Dockerfile changes (bubblewrap + ripgrep)
- [ ] Local `docker compose build forge-ai-worker` succeeds and the built image has `bwrap` + `rg` on PATH:
  ```bash
  docker compose -f docker-compose.dev.yml run --rm forge-ai-worker \
    bash -c 'bwrap --version && rg --version'
  ```

## Step 1 -- Database migrations + image rebuild

1. **Apply migration 025 (workspaces table):**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -f - < forge-core/migrations/025_workspaces.sql
   ```
   **Verify:**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "\d engine.workspaces"
   ```
   Expected: 10 columns, status CHECK constraint (`pending|ready|error`).

2. **Apply migration 026 (project_deploy_keys table):**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -f - < forge-core/migrations/026_project_deploy_keys.sql
   ```
   **Verify:**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "\d engine.project_deploy_keys"
   ```

3. **Cleanup stale agent_messages rows** (spec 2.6 "no backward compatibility"):
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "DELETE FROM engine.agent_messages WHERE event_type IN ('fix_loop_started', 'fix_loop_completed');"
   ```
   Or for a full wipe (if the team prefers a clean slate):
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "TRUNCATE engine.agent_messages;"
   ```

4. **Rebuild ai-worker image:**
   ```bash
   docker compose -f docker-compose.dev.yml build forge-ai-worker
   ```

5. **Verify bubblewrap and ripgrep are in the built image:**
   ```bash
   docker compose -f docker-compose.dev.yml run --rm forge-ai-worker \
     bash -c 'bwrap --version && rg --version'
   ```
   Expected: both binaries print version strings. If either fails, debug the Dockerfile before continuing.

6. **Build forge-core binary:**
   ```bash
   cd forge-core && go build ./cmd/forge-core
   ```
   Expected: clean build, no errors. Binary at `forge-core/forge-core`.

## Step 2 -- Deploy new code and smoke-test

1. **Bring up the new stack:**
   ```bash
   docker compose -f docker-compose.dev.yml up -d
   ```
   Wait ~10 seconds for containers to settle.

2. **Confirm ai-worker is responsive:**
   ```bash
   curl -sf http://localhost:8090/health | jq
   ```
   Expected: `{"status":"ok","sessions":0,"version":"1.0.0"}` or similar.

3. **Confirm forge-core is responsive:**
   ```bash
   curl -sf http://localhost:8080/health | jq
   ```
   Expected: some ok response.

4. **Smoke-test the workspace prep endpoint** (Phase 5 Task 5.7):
   ```bash
   curl -sf -X POST http://localhost:8090/api/workspace/prep \
     -H "Content-Type: application/json" \
     -d '{"tenant_id":1,"project_id":1,"workspace_path":"nonexistent"}' | jq
   ```
   Expected: `{"status":"error","error":"workspace directory does not exist: ..."}`. This confirms the endpoint is wired and responds in the expected shape.

5. **Full E2E smoke** (optional but recommended):
   ```bash
   cd ai-worker && FORGE_E2E_ENABLED=1 python -m pytest tests/e2e/test_variant_b_smoke.py -v
   ```
   Expected: test passes in ~2 minutes, costs ~$0.20 via real LLM call. If the test fails:
   - Read the stderr carefully -- it prints which shape assertion failed
   - Check the ai-worker container logs: `docker compose logs forge-ai-worker --tail 200`
   - Check the Redis stream: `docker compose exec redis redis-cli XLEN agent:stream:e2e-smoke-1`

6. **Manual smoke via the browser:**
   - Open `http://localhost:3000` (or wherever forge-portal is served)
   - Navigate to a project with a real GitHub repo configured
   - Click into the agent page
   - Send a small message: "What files are in the src/ directory?"
   - Expected UI behavior:
     - Step ribbon shows 7 phases in "pending" state initially
     - SSE stream arrives; at least one tool card renders (probably `list_directory` or `glob`)
     - If the agent calls `set_phase`, the ribbon updates (but no extra tool card for set_phase -- that's the `hideCard` flag working)
     - At turn end, a SummaryCard shows files_created / files_modified / duration / tokens

## Step 3 -- Delete legacy code

Only after Step 2 smoke test passes.

1. **Delete the pair_pipeline files** (Phase 0 Task 0.3 already deleted the core files; this is a final sweep):
   ```bash
   grep -rn "pair_pipeline\|PairPipeline\|FixLoopStarted\|FixLoopCompleted" ai-worker/ forge-core/ forge-portal/ 2>/dev/null
   ```
   Expected: zero matches in code (may appear in comments or git history references, which is fine).

2. **Delete the build-card frontend component** (Phase 6 Task 6.1 already deleted it; final sweep):
   ```bash
   ls forge-portal/components/agent/build-card.tsx 2>/dev/null && echo "STILL EXISTS -- delete it" || echo "ok, deleted"
   ```

3. **Commit any final cleanup** if the sweeps found anything:
   ```bash
   git add -A && git commit -m "chore: final pair_pipeline cleanup post-chronos deploy"
   ```

## Post-deploy verification checklist

Use these to confirm the deploy is healthy. If any fails, consider rolling back.

- [ ] `curl -sf http://localhost:8090/health | jq .status` returns `"ok"`
- [ ] `curl -sf http://localhost:8080/health | jq .status` returns `"ok"`
- [ ] `docker compose logs forge-ai-worker --tail 50 | grep -i error` returns nothing
- [ ] `docker compose logs forge-core --tail 50 | grep -i error` returns nothing
- [ ] A test agent message (via UI or API) produces events in Redis:
  ```bash
  docker compose exec redis redis-cli KEYS "agent:stream:*"
  ```
  Expected: at least one stream key after a test message.
- [ ] The Redis stream events have the expected shape:
  ```bash
  docker compose exec redis redis-cli XRANGE agent:stream:<session_id> - +
  ```
  Expected events: `text_delta`, `tool_started`, `tool_completed`, `phase_changed` (if agent called set_phase), `session_complete`. NO `fix_loop_started` / `fix_loop_completed` should appear.
- [ ] Tool cards in the UI render with the new formatters -- bash cards show command + exit code, file tool cards show path + status
- [ ] `set_phase` calls don't render a tool card (hideCard working)
- [ ] Step ribbon updates as the agent calls `set_phase`
- [ ] Observability: `docker compose logs forge-ai-worker 2>&1 | grep -c "agent.tool_call"` returns > 0 after a real agent run
- [ ] Observability: `docker compose logs forge-core 2>&1 | grep -c "workspace.ensure_ready"` returns > 0 after a workspace setup

## Rollback procedure

If the deploy goes wrong:

1. **Restore the previous code:**
   ```bash
   PRE_DEPLOY_SHA=$(cat /tmp/chronos-rollback-sha)
   git reset --hard "$PRE_DEPLOY_SHA"
   ```

2. **Rebuild and restart:**
   ```bash
   docker compose -f docker-compose.dev.yml build forge-ai-worker
   cd forge-core && go build ./cmd/forge-core
   docker compose -f docker-compose.dev.yml up -d
   ```

3. **Database migrations are additive** -- they don't need to be rolled back. `engine.workspaces` and `engine.project_deploy_keys` just stay present but unused if the old code is restored. This is the intended design (spec rollback section).

4. **Report the failure:** note which step failed, what the error was, and attach the last 200 lines of both ai-worker and forge-core logs to an incident document so the next retry isn't blind.

## Known deferred items

These are NOT in chronos's scope -- if the post-deploy verification exposes them, don't panic:

- **Profile data is empty.** Phase 5 Task 5.5 registers context tools with `profiles={}` -- profile scan pipeline integration is a follow-up. The `query_api_catalog` etc. tools will return "no data" until profile scan is wired. The agent can still work fine using file tools + bash.
- **Cost USD is 0.0 in SessionComplete.** Phase 5 Task 5.3 defers cost tracking -- UsageSnapshot doesn't carry per-turn cost. Future: add `total_cost_usd` field or compute in api_server.
- **No dashboards or alerts.** Phase 7 Task 7.2 landed structured log points but not Grafana dashboards. Post-deploy canary in the runbook uses `docker compose logs | grep` -- good enough for the first week.
- **No distributed tracing / OTel.** Not in chronos scope.

## Success criteria

The deploy is considered successful when:

1. All steps in this runbook completed without errors
2. The post-deploy verification checklist is fully green
3. At least one real user message (from Harvey or a test user) completes end-to-end and produces the expected Variant B UI behavior
4. 24 hours have passed without a rollback

Once #4 is hit, chronos is considered shipped. Update the project memory (Task 7.4) and close out the plan.
