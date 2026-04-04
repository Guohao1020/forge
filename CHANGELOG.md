# Changelog

All notable changes to the Forge platform are documented here.

## [Unreleased] — Phase 2 Harness Engineering

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
