# Forge API Reference

> Auto-generated from router.go — 102 commits, 2026-04-05

## Authentication

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| POST | `/api/auth/login` | No | — | Login with username/password, returns JWT |
| POST | `/api/auth/logout` | Yes | — | Invalidate current token |
| PUT | `/api/auth/password` | Yes | — | Change own password |
| GET | `/api/auth/me` | Yes | — | Get current user profile + roles |

## GitHub OAuth

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/auth/github/authorize` | Yes | — | Get GitHub OAuth URL |
| GET | `/api/auth/github/callback` | Yes | — | Handle OAuth callback |
| GET | `/api/auth/github/status` | Yes | — | Check GitHub connection |
| DELETE | `/api/auth/github/disconnect` | Yes | — | Disconnect GitHub |
| GET | `/api/github/repos` | Yes | — | List user's GitHub repos |

## Projects

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| POST | `/api/projects/import` | Yes | DEVELOPER+ | Import repos from GitHub |
| POST | `/api/projects` | Yes | DEVELOPER+ | Create new project |
| GET | `/api/projects` | Yes | Any | List all projects |
| GET | `/api/projects/:id` | Yes | Any | Get project detail |
| PUT | `/api/projects/:id` | Yes | PROJECT_ADMIN+ | Update project |
| DELETE | `/api/projects/:id` | Yes | PROJECT_ADMIN+ | Archive project |
| POST | `/api/projects/:id/star` | Yes | Any | Star project |
| DELETE | `/api/projects/:id/star` | Yes | Any | Unstar project |
| POST | `/api/projects/:id/sync` | Yes | Any | Sync to GitHub |
| POST | `/api/projects/:id/detect` | Yes | Any | Detect tech stack |

## Code Browsing

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/projects/:id/code/tree` | Yes | Any | File tree |
| GET | `/api/projects/:id/code/file` | Yes | Any | File content |
| GET | `/api/projects/:id/code/branches` | Yes | Any | Branch list |
| GET | `/api/projects/:id/code/prs` | Yes | Any | PR list |
| GET | `/api/projects/:id/code/prs/:prNumber` | Yes | Any | PR detail + diff |

## Tasks

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| POST | `/api/projects/:id/tasks` | Yes | Any | Create task |
| GET | `/api/projects/:id/tasks` | Yes | Any | List tasks |
| GET | `/api/projects/:id/tasks/:taskId` | Yes | Any | Get task detail + steps |
| GET | `/api/projects/:id/tasks/:taskId/nodes` | Yes | Any | Get DAG nodes |
| POST | `/api/projects/:id/tasks/:taskId/cancel` | Yes | Any | Cancel task |

## Conversation (AI Analysis)

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| POST | `/api/projects/:id/tasks/:taskId/messages` | Yes | Any | Send message (triggers AI) |
| GET | `/api/projects/:id/tasks/:taskId/messages` | Yes | Any | Get conversation history |
| POST | `/api/projects/:id/tasks/:taskId/analyze` | Yes | Any | Trigger AI analysis |
| POST | `/api/projects/:id/tasks/:taskId/confirm` | Yes | Any | Confirm requirements → plan |
| POST | `/api/projects/:id/tasks/:taskId/approve-plan` | Yes | Any | Approve plan → execute |

## Streaming (SSE)

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/stream/tasks/:taskId` | Token | — | SSE stream (task progress + code tokens + analyze tokens) |

## Versions

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| POST | `/api/projects/:id/versions` | Yes | Any | Create version |
| GET | `/api/projects/:id/versions` | Yes | Any | List versions |
| GET | `/api/projects/:id/versions/:vid` | Yes | Any | Version detail + tasks |
| PUT | `/api/projects/:id/versions/:vid` | Yes | Any | Update version |
| POST | `/api/projects/:id/versions/:vid/release` | Yes | Any | Release version (git tag) |

## Profiles (AI Memory)

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/projects/:id/profiles` | Yes | Any | List profile dimensions |
| GET | `/api/projects/:id/profiles/:key` | Yes | Any | Get profile by key |
| PUT | `/api/projects/:id/profiles/:key` | Yes | Any | Save profile (from AI Worker) |
| POST | `/api/projects/:id/profiles/scan` | Yes | Any | Trigger profile scan |

## Tests & Artifacts

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/projects/:id/tasks/:taskId/test-results` | Yes | Any | List test results |
| POST | `/api/projects/:id/tasks/:taskId/test-results` | Yes | Any | Save test result |
| GET | `/api/projects/:id/artifacts` | Yes | Any | List artifacts |
| GET | `/api/projects/:id/artifacts/:artifactId` | Yes | Any | Get artifact detail |

## Preview Environments

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/projects/:id/previews` | Yes | Any | List active previews |
| GET | `/api/projects/:id/tasks/:taskId/preview` | Yes | Any | Get task preview |
| POST | `/api/projects/:id/tasks/:taskId/preview` | Yes | Any | Create preview |
| DELETE | `/api/projects/:id/previews/:previewId` | Yes | Any | Destroy preview |

## Deploy & Pipeline

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/projects/:id/environments` | Yes | Any | List environments |
| GET | `/api/projects/:id/environments/:envId` | Yes | Any | Environment detail |
| GET | `/api/projects/:id/environments/:envId/deploys` | Yes | Any | Deploy history |
| POST | `/api/projects/:id/environments/:envId/deploy` | Yes | Any | Trigger deployment |

## Specs Center

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/specs/standards` | Yes | Any | List coding standards |
| GET | `/api/specs/standards/:id` | Yes | Any | Get standard |
| POST | `/api/specs/standards` | Yes | Any | Create standard |
| PUT | `/api/specs/standards/:id` | Yes | Any | Update standard |
| DELETE | `/api/specs/standards/:id` | Yes | Any | Delete standard |
| GET | `/api/specs/prompts` | Yes | Any | List prompt templates |
| POST | `/api/specs/prompts` | Yes | Any | Create template |
| PUT | `/api/specs/prompts/:id` | Yes | Any | Update template |
| DELETE | `/api/specs/prompts/:id` | Yes | Any | Delete template |
| GET | `/api/specs/rules` | Yes | Any | List review rules |
| POST | `/api/specs/rules` | Yes | Any | Create rule |
| PUT | `/api/specs/rules/:id` | Yes | Any | Update rule |
| DELETE | `/api/specs/rules/:id` | Yes | Any | Toggle rule |
| GET | `/api/specs/scaffolds` | Yes | Any | List scaffolds |
| GET | `/api/specs/scaffolds/:id` | Yes | Any | Get scaffold |
| GET | `/api/specs/effective/:projectId` | Yes | Any | Get resolved specs |

## Admin (Phase 3)

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/admin/users` | Yes | PLATFORM_ADMIN | List all users |
| POST | `/api/admin/users` | Yes | PLATFORM_ADMIN | Create user |
| PUT | `/api/admin/users/:userId/role` | Yes | PLATFORM_ADMIN | Change user role |
| GET | `/api/admin/costs` | Yes | PLATFORM_ADMIN | Monthly cost summary |
| GET | `/api/admin/budget` | Yes | PLATFORM_ADMIN | Budget status |
| GET | `/api/projects/:id/costs` | Yes | PROJECT_ADMIN+ | Project cost breakdown |

## Entropy Management (Code Quality)

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/projects/:id/entropy/latest` | Yes | Any | Latest scan result |
| GET | `/api/projects/:id/entropy/scans` | Yes | Any | Scan history |
| GET | `/api/projects/:id/entropy/trends` | Yes | Any | Quality trend data |
| GET | `/api/projects/:id/entropy/config` | Yes | Any | Scan configuration |
| PUT | `/api/projects/:id/entropy/config` | Yes | PROJECT_ADMIN+ | Update scan config |
| POST | `/api/projects/:id/entropy/scan` | Yes | Any | Trigger manual scan |

## System (No Auth)

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/health` | No | — | Health check |
| GET | `/metrics` | No | — | Prometheus metrics |
| GET | `/api/admin/metrics` | No | — | JSON metrics snapshot |

**Total: ~80 endpoints across 16 resource groups**

## Middleware Stack

All requests pass through these middleware layers in order:
1. **Recovery** — panic recovery
2. **RequestID** — X-Request-ID header injection
3. **CORS** — Cross-origin resource sharing
4. **AccessLog** — Structured JSON access logs (for Loki)
5. **Timeout** — 30s request deadline (excludes SSE)
6. **Metrics** — Prometheus counters (requests, errors, latency)
7. **JWTAuth** — Token validation (protected routes only)
8. **RateLimit** — Token bucket (60 burst, 10/sec per user/IP)
