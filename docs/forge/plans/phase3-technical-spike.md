# Phase 3 Technical Spike — Feasibility & Priority Analysis

> **Date**: 2026-04-05
> **Purpose**: Inform Harvey's decision on Phase 3 priorities
> **Method**: Code-level analysis of dependencies, effort, and impact

---

## Phase 3 Candidate Modules

Based on milestone-plan.md v6.0 and technical-design.md v3.0, Phase 3 includes:

| Module | Estimated Effort | Dependencies | User Impact |
|--------|-----------------|--------------|-------------|
| Constraint Engine | 5-7 days | golangci-lint, eslint installed | HIGH — enforces code quality |
| Entropy Management | 3-5 days | Constraint Engine | MEDIUM — automated cleanup |
| Enterprise Auth (RBAC) | 5-7 days | None | HIGH — multi-user access |
| Cost Control | 2-3 days | None | MEDIUM — budget management |
| IM Bot (DingTalk/Feishu) | 3-5 days | forge-bot service | HIGH — mobile access |
| Observability (Grafana) | 5-7 days | Grafana + Loki + Prometheus | MEDIUM — operations |
| Canary Deployment | 3-5 days | Argo Rollouts + K8s | LOW — advanced deployment |

---

## 1. Constraint Engine (golangci-lint + eslint integration)

### What it does
Run real linters on AI-generated code BEFORE review. Currently ReviewerAgent does AI-based lint (pattern matching in prompt). Real linters catch issues AI misses.

### Current state
- `forge-task-runner` Dockerfile already installs `golangci-lint` (Go) and `node` (for eslint)
- `S11'` plan includes inline lint check after generation
- `ReviewerAgent` has language-specific lint rules in system prompt

### What's needed
```
1. New Temporal Activity: RunLintCheck
   Input: files[], language, project_id
   Process: Write files to temp dir → run golangci-lint/eslint → parse output
   Output: {passed: bool, issues: [{file, line, rule, message, severity}]}

2. Integrate into TaskWorkflow between GENERATE and REVIEW steps:
   GENERATE → LINT_CHECK → REVIEW
   If lint fails: feed issues back to CoderAgent for auto-fix (1 attempt)

3. Frontend: show lint results in step timeline
```

### Effort: 3 days (activity + workflow integration + frontend)
### Risk: LOW — linters are deterministic, no AI uncertainty
### Recommendation: **DO FIRST** — immediate code quality improvement, no infrastructure needed

---

## 2. Enterprise Auth (RBAC)

### What it does
Multiple users with different roles (Admin, Developer, Viewer) access the same Forge instance. Currently: single admin user with full access.

### Current state
- `auth` schema has: tenants, users, roles, user_roles tables (all exist)
- JWT middleware extracts user_id and tenant_id from token
- Role constants defined but not enforced: PLATFORM_ADMIN, ORG_ADMIN, PROJECT_ADMIN, DEVELOPER, VIEWER
- Login works, user creation works

### What's needed
```
1. Role-based middleware: check user's role before allowing actions
   - VIEWER: read-only (list projects, view tasks, browse code)
   - DEVELOPER: create tasks, run AI generation
   - PROJECT_ADMIN: manage project settings, specs, versions
   - PLATFORM_ADMIN: all actions + system settings

2. User management UI: invite users, assign roles
   - /settings/users page (list, invite, role assignment)

3. OAuth2/OIDC login (DingTalk scan, GitHub OAuth already works)
```

### Effort: 5 days (middleware + UI + OAuth)
### Risk: MEDIUM — role enforcement must not break existing single-user flow
### Recommendation: **DO SECOND** — enables team usage, high business value

---

## 3. IM Bot (DingTalk / Feishu)

### What it does
Product managers interact with Forge via DingTalk or Feishu chat instead of Web UI. @forge + natural language → same AI pipeline.

### Current state
- `forge-bot` service planned but not built
- Architecture defined: lightweight Go service receiving webhooks
- No DingTalk/Feishu SDK integrated

### What's needed
```
1. New service: forge-bot/
   - DingTalk webhook receiver (HTTP POST)
   - Message parser: extract @forge mention + requirement text
   - Call forge-core API to create task + send message
   - Format AI response as DingTalk card message
   - Push progress updates via DingTalk outgoing message

2. DingTalk App registration (requires company admin)

3. Message card templates for:
   - Requirement clarification (with option buttons)
   - Plan review summary
   - Task completion notification with PR link
```

### Effort: 5 days (webhook + API integration + card templates)
### Risk: MEDIUM — DingTalk API may have rate limits and format constraints
### Recommendation: **DO THIRD** — high user impact but needs DingTalk admin setup

---

## 4. Cost Control

### What it does
Track and limit AI token usage per tenant, project, and task. Prevent runaway costs from AI loops.

### Current state
- `model_calls` table exists (tracks every LLM call with token counts)
- `tenant_budgets` table exists (per-tenant limits)
- `KillSwitch` L1/L2/L3 designed but not implemented
- AI Worker logs token usage per activity

### What's needed
```
1. Token budget enforcement:
   - Before each LLM call: check remaining budget
   - If exceeded: return error, trigger KillSwitch L1 (pause new tasks)
   - Dashboard: show usage by tenant/project/model

2. Cost reporting API:
   - GET /api/admin/costs?period=month — total tokens, cost estimate
   - GET /api/projects/:id/costs — per-project breakdown

3. Frontend: cost dashboard widget on admin settings page
```

### Effort: 3 days (budget check + API + dashboard)
### Risk: LOW — read-heavy, no architectural changes
### Recommendation: **DO ALONGSIDE RBAC** — small effort, prevents cost surprises

---

## 5. Observability (Grafana + Loki + Prometheus)

### What it does
Monitor Forge platform health: API latency, AI response times, error rates, Temporal workflow metrics.

### Current state
- forge-core uses structured JSON logging (slog)
- No metrics endpoint
- No log aggregation
- Temporal has built-in metrics (exposed via Prometheus)

### What's needed
```
1. Prometheus metrics endpoint: /metrics
   - HTTP request duration histogram
   - AI activity duration histogram
   - Task status counter
   - Active SSE connections gauge

2. Grafana dashboards (3):
   - Platform health (request rate, error rate, latency p50/p95/p99)
   - AI performance (model latency, token usage, fallback rate)
   - Task pipeline (tasks/day, completion rate, avg time per stage)

3. Loki log aggregation (structured JSON → Loki → Grafana Explore)

4. docker-compose addition: grafana + loki + prometheus services
```

### Effort: 5 days (metrics + dashboards + docker)
### Risk: LOW — additive, doesn't change existing code behavior
### Recommendation: **DEFER** — valuable for operations but not user-facing

---

## 6. Entropy Management

### What it does
Periodically scan projects for code quality degradation: naming violations, dead code, missing docs, low coverage. Auto-create fix PRs.

### Current state
- Concept designed in technical-design.md §2.6
- No implementation
- Depends on Constraint Engine being implemented first

### What's needed
```
1. Scheduled Temporal workflow: EntropyScanWorkflow
   - Runs weekly per project (configurable)
   - Scans: naming conventions, dead code, missing error handling
   - Uses AI to generate fix suggestions
   - Creates auto-fix branch + PR if issues found

2. Quality trends tracking:
   - Store scan results in project_profiles (quality_trends dimension)
   - Frontend: trend charts on quality dashboard

3. Configuration: which rules to enforce, auto-fix vs report-only
```

### Effort: 5 days (workflow + scanning + auto-fix)
### Risk: MEDIUM — auto-fix PRs must not introduce regressions
### Recommendation: **DEFER** — needs Constraint Engine first, lower priority

---

## 7. Canary Deployment (Argo Rollouts)

### What it does
Deploy AI-generated code to production with gradual traffic shifting: 5% → 25% → 50% → 100%, with automatic rollback on error spike.

### Current state
- K8s deployment code exists (GenerateK8sManifests activity)
- No Argo Rollouts integration
- Requires real K8s cluster (ACK)

### What's needed
```
1. Install Argo Rollouts in K8s cluster
2. Generate Rollout resources instead of Deployment
3. Prometheus analysis: check error rate at each step
4. Auto-rollback on threshold breach
```

### Effort: 3 days (after K8s cluster is available)
### Risk: HIGH — production deployment, must be thoroughly tested
### Recommendation: **DEFER** — needs K8s cluster + production traffic

---

## Recommended Phase 3 Execution Order

```
Week 1: Constraint Engine (3 days) + Cost Control (2 days)
         → Real linting on generated code + budget protection

Week 2: Enterprise Auth — RBAC (5 days)
         → Multi-user team access

Week 3: IM Bot — DingTalk (5 days)
         → Mobile/IM access for product managers

Week 4: Observability (5 days)
         → Platform monitoring dashboards

Later:  Entropy Management (after Constraint Engine stabilizes)
        Canary Deployment (after K8s cluster is production-ready)
```

### Why this order
1. **Constraint Engine** is highest ROI — immediate code quality improvement, zero infrastructure dependency
2. **RBAC** unlocks team usage — currently the platform is single-user which limits adoption
3. **IM Bot** is highest user-impact — PMs prefer DingTalk over opening a browser
4. **Observability** is operations-essential but can wait until there's actual traffic to monitor
