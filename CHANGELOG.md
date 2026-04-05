# Changelog

All notable changes to the Forge platform are documented here.

## [0.3.5] — Webhooks + Security + UX Polish

### Added
- **Webhook System**: HMAC-SHA256 signed HTTP notifications for task events. 3 API endpoints + management UI.
- **Security Headers**: nosniff, DENY, XSS, HSTS (conditional), Permissions-Policy on all responses.
- **Request Body Limit**: 10MB max via MaxBytesReader middleware.
- **CORS Hardening**: Configurable origins via CORS_ORIGINS env, preflight caching.
- **Version Header**: X-Forge-Version injected via ldflags at build time.
- **System Info**: GET /api/system/info (version, uptime, runtime).
- **Error Pages**: Custom 404 and 500 error pages with Forge branding.
- **Breadcrumb Navigation**: Auto-generated breadcrumbs on dashboard pages.
- **Loading Bar**: Top-of-page purple progress indicator on route transitions.
- **Reusable UI**: PageLoading, EmptyState, LoadingBar components.
- **Deploy Dialog**: Proper version input replacing mock version string.
- **CI Pipeline**: forge-bot in CI, race detection, version injection in docker.
- **Slow Request Logging**: Frontend logs API calls > 2s in development.

## [0.3.2] — Platform Polish & Search

### Added
- **Global Search**: `GET /api/search?q=keyword` — searches projects + tasks, ILIKE pattern matching, returns typed results with frontend URLs.
- **Search Bar**: Topbar search with Cmd/Ctrl+K shortcut, 300ms debounce, dropdown results with type badges.
- **Project Stats API**: `GET /api/projects/:id/stats` — task counts by status, active versions, quality score.
- **Project Stats Bar**: Task page shows overview cards (total, completed, in-progress, quality score).
- **Admin Dashboard**: `/settings/dashboard` — platform health, AI performance, task pipeline, cost summary.
- **Password Change**: `PUT /api/auth/password` + `/settings/account` page.
- **Rate Limiting**: Token bucket (60 burst, 10/sec) per user/IP on protected routes.
- **Access Logging**: Structured JSON logs for Loki ingestion (method, path, status, latency, user_id).
- **Request Timeout**: 30s context deadline (excludes SSE/streaming), returns 504 on timeout.
- **Graceful Shutdown**: SIGINT/SIGTERM → 10s drain → proper cleanup.
- **Enhanced Health Check**: `/health` checks DB + Redis, returns 503 on degradation.
- **Promtail**: Docker log → Loki pipeline via Docker socket discovery.
- **Entropy Config UI**: Expandable panel for scan schedule, auto-fix toggles.
- **Pagination Utility**: `pkg/pagination` with Parse(), NewResult(), 7 tests.
- **Favicon**: SVG Forge purple anvil icon + OpenGraph meta tags.
- **Keyboard Shortcut**: Cmd/Ctrl+K for search, Escape to close.

## [0.3.1] — Phase 3 Extended Modules

### Added
- **Observability Stack**: Grafana + Prometheus + Loki in docker-compose. 3 pre-built dashboards (Platform Health, AI Performance, Task Pipeline). Enhanced metrics: AI call tracking, task event counters, pipeline stage duration, SSE connection gauge.
- **Entropy Management**: `EntropyScanWorkflow` (6-step Temporal workflow). Code quality scoring (0-100), issue categorization (naming/dead_code/error_handling/complexity/style). 6 API endpoints. Migration 019. Frontend quality section on project settings.
- **forge-bot**: New IM service skeleton — DingTalk webhook receiver, HMAC signature verification, 6 card templates (welcome, clarification, plan summary, task completed, progress, error). forge-core API client. Docker image.
- **User Management**: Create users, assign roles, inline role change dropdown. `/settings/users` page.
- Enhanced Prometheus metrics: per-path breakdown, AI tokens by model, task status counters, SSE active gauge.

## [0.3.0] — Phase 3 Core Modules

### Added
- **Constraint Engine**: `RunLint` Temporal activity runs golangci-lint/eslint/ruff on generated code. Wired into pipeline between GENERATE and REVIEW. Non-blocking (review catches remaining issues).
- **Cost Control**: Token usage tracking from `model_calls` table. Monthly summary, project breakdown, budget status. 3 API endpoints with RBAC.
- **RBAC Middleware**: `RequireRole()` with 5-level hierarchy (VIEWER < DEVELOPER < PROJECT_ADMIN < ORG_ADMIN < PLATFORM_ADMIN). Applied to write/admin routes. Backward compatible.
- **Prometheus Metrics**: `/metrics` endpoint with request count, error count, latency, uptime. `/api/admin/metrics` JSON endpoint. MetricsMiddleware on all routes.
- 8-stage pipeline: PLAN → TEST_WRITING → GENERATE → **LINT** → REVIEW → TEST → DEPLOY

## [0.2.0] — Phase 2 Harness Engineering

### Added

#### Streaming AI Analysis (P0)
- **Real-time thinking display**: LLM tokens stream character-by-character via Redis → Go SSE → browser.
- `router.chat_stream(channel_prefix="analyze")` publishes to `analyze:stream:{taskId}`.
- Go SSE handler subscribes to both `code:stream` and `analyze:stream` channels.
- `useTaskStream` hook exports `analyzeThinking` and `isAnalyzing` state.
- `StreamingThinking` component: pulsing cursor during thinking, collapsible after completion.
- Falls back to synchronous `agent.run()` if streaming fails — no regression.

#### Harness Engineering Foundation (SH-1)
- **ContextCache**: Redis-backed workflow-level context caching (TTL 10min). Reduces API calls from 20/workflow to 4.
- **Parallel context fetch**: `asyncio.gather` for 4 concurrent API calls in `ContextBuilder.build()`.
- **Agent Loop**: `BaseAgent.run()` refactored from single-round LLM call to multi-round tool-use loop (max 5 rounds). Backward compatible when `tools=None`.
- **ModelRouter tools support**: `chat()` accepts `tools` parameter. Anthropic native format + OpenAI-compatible format conversion for DashScope/DeepSeek.

#### Context Tools (SH-2)
- 5 on-demand context query tools: `query_api_catalog`, `query_db_schema`, `query_business_rules`, `query_module_graph`, `read_project_file`.
- `ContextToolExecutor` class for tool execution with keyword filtering and file truncation.
- Profile availability hints in system prompt to prevent wasted tool calls.
- All 4 pipeline agents (Planner, TestWriter, Coder, Reviewer) integrated with appropriate tool subsets.

#### Version Management (SH-3a, SH-3b, SH-4)
- `project_versions` table with semantic versioning, status lifecycle (PLANNING → IN_PROGRESS → TESTING → RELEASED → CANCELLED).
- Version CRUD API: 5 endpoints (create, list, get, update, release).
- `VersionOrchestrator` Temporal workflow: signal-driven coordination, ContinueAsNew every 50 events.
- 3-layer conflict detection: file-level overlap, package-level heuristic, git merge-tree simulation.
- Tasks extended with `version_id`, `conflict_status`, `blocked_by`, `touched_files`.
- Frontend: version list page (progress bars, status badges) and detail page (task list, conflict warnings, release button).

#### Task Decomposition Enhancement (S9')
- `PlannerAgent` outputs `touched_files` (create/modify lists) for conflict detection.
- `DagVisualization` React component: topological sort into levels, colored type badges, dependency arrows.
- `SaveTouchedFiles` Temporal activity persists file predictions to DB.
- DAG/list toggle in plan review card.

#### Project Smart Onboarding (SP-1)
- `DetectProjectType()`: 15+ file signature patterns detecting 6 project types (web_app, mobile_app, desktop_app, backend_api, library, monorepo) with sub-types (nextjs, flutter, tauri, go_api, etc.).
- Auto-derives: branch strategy, deploy target, artifact type, test frameworks, build tools.
- `POST /projects/:id/detect` endpoint for manual re-detection.
- Project settings page shows detected type, frameworks, test frameworks.

#### AI Recommendation System (SP-2)
- `RecommendationCard` component: 2-3 option cards with pros/cons/risk/context-aware reasoning.
- AI-recommended option highlighted with purple border + badge.
- Expandable context factors section explaining recommendation basis.
- Integrated into chat flow — renders when AI metadata contains `recommendations`.

#### Infrastructure
- `forge-task-runner` Docker image: Go 1.26 + Node 20 + Python 3.12 multi-language test runner.
- `entrypoint.sh`: auto-detect framework → clone → install deps → run tests → report results.
- `BuildDockerImage` Temporal activity for artifact management.
- `GenerateK8sManifests` activity: generates Deployment/Service/Ingress/ConfigMap per environment.
- `Rollback` activity for reverting to previous deployment.
- `PreviewLifecycleWorkflow`: auto-create → idle timeout (30min) → destroy.
- `profile_embeddings` table with pgvector HNSW index for semantic search.

#### Developer Experience
- `Makefile` with targets: dev, test, build, docker, deploy, clean.
- `scripts/dev-start.sh`: Start all services with auto token refresh.
- `scripts/dev-stop.sh`: Clean shutdown of all services.
- `scripts/run-tests.sh`: Run all test suites with coverage.
- `scripts/test-api.sh`: 11 API integration tests.
- `docker-compose.prod.yml`: Full stack production deployment (7 services).
- Production Dockerfiles for forge-core (121MB), ai-worker (365MB), forge-portal (302MB).
- `.editorconfig`, `.pre-commit-config.yaml` (10 hooks), `.github/workflows/ci.yml`.
- `STATUS.md`, `CONTRIBUTING.md` for developer onboarding.

### Fixed
- AnalystAgent placeholder text replaced with real Temporal workflow call.
- Profile scan `_select_files_for_dimension` handles both string and dict entries.
- `SavePRInfo` now persists `branch_name`, `files_changed`, `lines_added`, `lines_deleted`.
- `json` import added to LLM client (was causing tools parse error).
- Version API NULL handling for `git_tag` and `created_by` fields.
- Context key names corrected: `"tenantID"` → `"tenant_id"`, `"userID"` → `"user_id"`.
- Go version in all Dockerfiles updated to 1.26 (matching go.mod).
- `FORGE_API_TOKEN` injected from env into K8s Job containers.
- DashScope API key redacted from session documentation.
- Project type detector path normalization for Windows compatibility.

### Changed
- `ContextBuilder.build()`: serial → parallel API fetching (4x faster).
- System prompt: project profiles moved from static injection to on-demand tool queries.
- `docker-compose.dev.yml`: PostgreSQL image upgraded to `pgvector/pgvector:pg16`.
- Python test count: 55 → 103 (+48 new tests).
- Go test count: 25 → 76 (+51 new tests).

## [0.1.0] — Phase 1 Minimum Closed Loop

Initial release with end-to-end pipeline: login → project management → GitHub integration → Temporal tasks → specs center → AI worker → DevOps deployment.
