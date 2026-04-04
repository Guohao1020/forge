# S16+S17 -- Project Profile RAG Enhancement + Cloud Preview Environment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** (S16) Add pgvector-based semantic search to project profiles so CoderAgent can query relevant profile chunks by meaning rather than loading entire profiles, enable incremental profile updates on git push, and add ER diagram / dependency graph visualization. (S17) Create ephemeral cloud preview environments per task/PR with auto-generated Dockerfile + K8s manifests, unique preview URLs, idle auto-scaling, and automatic cleanup on PR merge/close.

**Architecture:**
- S16: Add pgvector extension to PostgreSQL, store profile data as chunked embeddings in a new `engine.profile_embeddings` table. ContextBuilder queries embeddings with semantic similarity when building AI context (e.g., "user authentication" finds auth-related profile chunks). Git push webhooks trigger incremental profile updates. Frontend adds D3/React Flow visualizations for ER diagrams and module dependency graphs.
- S17: On PR creation or manual trigger, AI detects project type and generates Dockerfile + K8s manifests for a preview namespace (`preview-{taskId}`). Deploy to K8s with Ingress at `{taskId}.preview.forge.example.com`. KEDA or cron job scales to 0 after 30 minutes idle. PR merge/close destroys the preview namespace.

**Tech Stack:** PostgreSQL + pgvector, OpenAI embeddings API, Go 1.22 + pgx, Python (embedding + profile updates), K8s + cert-manager + KEDA, Next.js + React Flow / D3 + shadcn/ui

**Dependencies:** S16 base profile system (already complete), S14 (K8s deployment capabilities), S13 (Docker build)

**Duration:** 4 days (S16: 2 days, S17: 2 days)

---

## File Structure

### S16: Profile RAG

#### Database

```
forge-core/
+-- migrations/
|   +-- 017_profile_embeddings.sql         # NEW: pgvector extension + embeddings table
```

#### Go Backend

```
forge-core/
+-- internal/module/profile/
|   +-- model.go                           # MODIFY: add ProfileEmbedding struct
|   +-- repository.go                      # MODIFY: add embedding CRUD + similarity search
|   +-- service.go                         # MODIFY: add incremental update + semantic search
|   +-- handler.go                         # MODIFY: add search endpoint + webhook handler
+-- internal/router/router.go             # MODIFY: register new routes
```

#### Python AI Worker

```
ai-worker/src/
+-- context/
|   +-- embeddings.py                      # NEW: embedding generation + semantic search client
|   +-- builder.py                         # MODIFY: use semantic search for context building
+-- activities/profile.py                  # MODIFY: add incremental update + embedding generation
```

#### Frontend

```
forge-portal/
+-- components/profile/
|   +-- er-diagram.tsx                     # NEW: ER diagram visualization (React Flow)
|   +-- module-graph.tsx                   # NEW: module dependency graph (React Flow)
+-- app/(dashboard)/projects/[id]/
|   +-- profile/page.tsx                   # MODIFY: add ER diagram + module graph tabs
```

### S17: Preview Environments

#### Go Backend

```
forge-core/
+-- internal/module/preview/
|   +-- model.go                           # MODIFY: add lifecycle fields
|   +-- repository.go                      # MODIFY: add expiry queries
|   +-- service.go                         # MODIFY: add create/destroy/scale logic
|   +-- handler.go                         # MODIFY: add lifecycle endpoints
+-- internal/temporal/
|   +-- activity/preview_activities.go     # NEW: preview env lifecycle activities
|   +-- workflow/preview_workflow.go        # NEW: preview env creation workflow
+-- internal/router/router.go             # MODIFY: register webhook route
```

#### Frontend

```
forge-portal/
+-- components/preview/
|   +-- preview-env-card.tsx               # NEW: preview environment card
|   +-- preview-log-viewer.tsx             # NEW: preview deployment logs
+-- app/(dashboard)/projects/[id]/
|   +-- tasks/[taskId]/preview/page.tsx    # NEW: preview environment for a task
```

---

## S16 Day 1: pgvector + Embedding Generation

### Task 1: Database Migration -- pgvector + Embeddings Table

**Files:**
- Create: `forge-core/migrations/017_profile_embeddings.sql`

- [ ] **Step 1: Create migration**

```sql
-- S16: pgvector for semantic profile search
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS engine.profile_embeddings (
    id              BIGSERIAL PRIMARY KEY,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    profile_key     VARCHAR(50) NOT NULL,     -- which dimension this chunk belongs to
    chunk_index     INT NOT NULL,             -- order within the dimension
    chunk_text      TEXT NOT NULL,            -- the actual text content
    metadata        JSONB NOT NULL DEFAULT '{}',  -- source file, line range, etc.
    embedding       vector(1536) NOT NULL,    -- OpenAI text-embedding-3-small
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profile_embeddings_project ON engine.profile_embeddings(project_id);
CREATE INDEX IF NOT EXISTS idx_profile_embeddings_key ON engine.profile_embeddings(project_id, profile_key);

-- HNSW index for fast approximate nearest neighbor search
CREATE INDEX IF NOT EXISTS idx_profile_embeddings_vector
    ON engine.profile_embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

COMMENT ON TABLE engine.profile_embeddings IS 'Chunked profile data with vector embeddings for semantic search';
```

- [ ] **Step 2: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/017_profile_embeddings.sql
git commit -m "feat(s16): add pgvector extension and profile_embeddings table with HNSW index"
```

---

### Task 2: Embedding Generation + Semantic Search

**Files:**
- Create: `ai-worker/src/context/embeddings.py`
- Modify: `ai-worker/src/activities/profile.py`
- Modify: `ai-worker/src/context/builder.py`

- [ ] **Step 1: Create embeddings module**

`ai-worker/src/context/embeddings.py`:

```python
import httpx
import logging
from typing import List, Dict, Any

logger = logging.getLogger(__name__)

EMBEDDING_MODEL = "text-embedding-3-small"  # 1536 dimensions
CHUNK_SIZE = 500  # characters per chunk (roughly 100-150 tokens)
CHUNK_OVERLAP = 50  # overlap between chunks


class EmbeddingClient:
    def __init__(self, api_key: str, base_url: str = "https://api.openai.com/v1"):
        self._client = httpx.AsyncClient(
            base_url=base_url,
            headers={"Authorization": f"Bearer {api_key}"},
            timeout=30.0,
        )

    async def embed_text(self, text: str) -> List[float]:
        """Generate embedding for a single text."""
        resp = await self._client.post(
            "/embeddings",
            json={"model": EMBEDDING_MODEL, "input": text},
        )
        resp.raise_for_status()
        data = resp.json()
        return data["data"][0]["embedding"]

    async def embed_batch(self, texts: List[str]) -> List[List[float]]:
        """Generate embeddings for a batch of texts (max 2048)."""
        resp = await self._client.post(
            "/embeddings",
            json={"model": EMBEDDING_MODEL, "input": texts[:2048]},
        )
        resp.raise_for_status()
        data = resp.json()
        return [d["embedding"] for d in sorted(data["data"], key=lambda x: x["index"])]

    async def close(self):
        await self._client.aclose()


def chunk_text(text: str, chunk_size: int = CHUNK_SIZE, overlap: int = CHUNK_OVERLAP) -> List[str]:
    """Split text into overlapping chunks."""
    chunks = []
    start = 0
    while start < len(text):
        end = start + chunk_size
        chunk = text[start:end]
        if chunk.strip():
            chunks.append(chunk.strip())
        start = end - overlap
    return chunks


def chunk_profile_dimension(profile_key: str, profile_value: dict) -> List[Dict[str, Any]]:
    """
    Convert a profile dimension into chunks suitable for embedding.
    Each chunk includes context about what it represents.
    """
    chunks = []

    if profile_key == "api_catalog":
        endpoints = profile_value.get("endpoints", [])
        # Group endpoints into chunks of 5
        for i in range(0, len(endpoints), 5):
            batch = endpoints[i:i+5]
            text = f"API Endpoints:\n" + "\n".join(
                f"  {e.get('method', '?')} {e.get('path', '?')} -> {e.get('handler', '?')}"
                for e in batch
            )
            chunks.append({
                "text": text,
                "metadata": {"type": "api_catalog", "start_index": i, "count": len(batch)},
            })

    elif profile_key == "db_schema":
        tables = profile_value.get("tables", [])
        for table in tables:
            text = f"Database Table: {table.get('name', '?')}\n"
            text += f"Columns: {', '.join(c.get('name', '?') + ' ' + c.get('type', '?') for c in table.get('columns', []))}\n"
            rels = table.get("relations", [])
            if rels:
                text += f"Relations: {', '.join(str(r) for r in rels)}\n"
            chunks.append({
                "text": text,
                "metadata": {"type": "db_schema", "table": table.get("name")},
            })

    elif profile_key == "module_graph":
        modules = profile_value.get("modules", [])
        for mod in modules:
            text = f"Module: {mod.get('name', '?')}\n"
            text += f"Path: {mod.get('path', '?')}\n"
            text += f"Dependencies: {', '.join(mod.get('depends_on', []))}\n"
            text += f"Exports: {', '.join(mod.get('exports', []))}\n"
            chunks.append({
                "text": text,
                "metadata": {"type": "module_graph", "module": mod.get("name")},
            })

    elif profile_key == "business_rules":
        rules = profile_value.get("rules", [])
        for rule in rules:
            text = f"Business Rule [{rule.get('domain', '?')}]: {rule.get('rule', '?')}"
            if rule.get("source"):
                text += f" (source: {rule['source']})"
            chunks.append({
                "text": text,
                "metadata": {"type": "business_rules", "domain": rule.get("domain")},
            })

    else:
        # Generic: chunk the JSON as text
        import json
        full_text = json.dumps(profile_value, ensure_ascii=False, indent=2)
        for i, chunk in enumerate(chunk_text(full_text)):
            chunks.append({
                "text": f"{profile_key}: {chunk}",
                "metadata": {"type": profile_key, "chunk_index": i},
            })

    return chunks
```

- [ ] **Step 2: Update profile activity for incremental embedding**

In `ai-worker/src/activities/profile.py`, add embedding generation after profile scan:

```python
# After saving profile to forge-core API, generate embeddings
from src.context.embeddings import EmbeddingClient, chunk_profile_dimension
from src.config import settings

async def generate_profile_embeddings(project_id: int, profile_key: str, profile_value: dict):
    """Generate and store embeddings for a profile dimension."""
    client = EmbeddingClient(api_key=settings.openai_api_key)
    try:
        chunks = chunk_profile_dimension(profile_key, profile_value)
        if not chunks:
            return

        texts = [c["text"] for c in chunks]
        embeddings = await client.embed_batch(texts)

        # Save to forge-core API (or directly to DB)
        for i, (chunk, embedding) in enumerate(zip(chunks, embeddings)):
            await save_embedding(project_id, profile_key, i, chunk["text"], chunk["metadata"], embedding)

        logger.info(f"Generated {len(embeddings)} embeddings for {profile_key}")
    finally:
        await client.close()
```

- [ ] **Step 3: Update ContextBuilder for semantic search**

In `ai-worker/src/context/builder.py`, add a method to query relevant profile chunks:

```python
async def query_relevant_profiles(self, project_id: int, query: str, limit: int = 10) -> List[Dict]:
    """
    Semantic search: find profile chunks most relevant to the query.
    Used when building context for code generation -- instead of loading
    all profiles, load only the relevant chunks.
    """
    # Generate query embedding
    from src.context.embeddings import EmbeddingClient
    from src.config import settings

    client = EmbeddingClient(api_key=settings.openai_api_key)
    try:
        query_embedding = await client.embed_text(query)
    finally:
        await client.close()

    # Search via forge-core API
    resp = await self._client.post(
        f"{settings.forge_api_url}/api/projects/{project_id}/profiles/search",
        json={"embedding": query_embedding, "limit": limit},
    )
    if resp.status_code == 200:
        return resp.json().get("data", {}).get("chunks", [])
    return []
```

In the `build()` method, when `purpose="code-generation"`, use semantic search:

```python
# If building context for code generation and we have a requirement summary,
# use semantic search to find relevant profile chunks
if purpose == "code-generation" and requirement_summary:
    relevant_chunks = await self.query_relevant_profiles(project_id, requirement_summary)
    if relevant_chunks:
        ctx.project_profiles["_relevant"] = relevant_chunks
```

- [ ] **Step 4: Add search endpoint to Go profile handler**

In `forge-core/internal/module/profile/handler.go`:

```go
// POST /api/projects/:id/profiles/search -- semantic search
func (h *Handler) SearchProfiles(c *gin.Context) {
    projectID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
    var req struct {
        Embedding []float64 `json:"embedding" binding:"required"`
        Limit     int       `json:"limit"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid request")
        return
    }
    if req.Limit <= 0 { req.Limit = 10 }

    chunks, err := h.service.SearchEmbeddings(c.Request.Context(), projectID, req.Embedding, req.Limit)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "search failed")
        return
    }
    response.OK(c, gin.H{"chunks": chunks})
}
```

Repository SQL for similarity search:
```sql
SELECT id, profile_key, chunk_index, chunk_text, metadata,
    1 - (embedding <=> $2::vector) AS similarity
FROM engine.profile_embeddings
WHERE project_id = $1
ORDER BY embedding <=> $2::vector
LIMIT $3
```

Register route:
```go
protected.POST("/projects/:id/profiles/search", deps.ProfileHandler.SearchProfiles)
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/ ai-worker/
git commit -m "feat(s16): add pgvector semantic search for profile embeddings with chunking"
```

---

## S16 Day 2: Incremental Updates + Frontend Visualization

### Task 3: Incremental Profile Update + ER Diagram

**Files:**
- Modify: `forge-core/internal/module/profile/handler.go` (webhook)
- Create: `forge-portal/components/profile/er-diagram.tsx`
- Create: `forge-portal/components/profile/module-graph.tsx`
- Modify: `forge-portal/app/(dashboard)/projects/[id]/profile/page.tsx`

- [ ] **Step 1: Add git push webhook handler**

In `forge-core/internal/module/profile/handler.go`:

```go
// POST /api/webhooks/github/push -- triggered by GitHub webhook on push
func (h *Handler) HandleGitPush(c *gin.Context) {
    // Parse GitHub push webhook payload
    var payload struct {
        Ref        string `json:"ref"`
        Repository struct {
            FullName string `json:"full_name"`
            HTMLURL  string `json:"html_url"`
        } `json:"repository"`
        Commits []struct {
            Added    []string `json:"added"`
            Modified []string `json:"modified"`
            Removed  []string `json:"removed"`
        } `json:"commits"`
    }
    if err := c.ShouldBindJSON(&payload); err != nil {
        c.JSON(400, gin.H{"error": "invalid payload"})
        return
    }

    // Find project by repo URL
    project, err := h.projectRepo.FindByRepoURL(c.Request.Context(), payload.Repository.HTMLURL)
    if err != nil || project == nil {
        c.JSON(200, gin.H{"status": "skipped", "reason": "project not found"})
        return
    }

    // Determine which profile dimensions need updating based on changed files
    changedFiles := collectChangedFiles(payload.Commits)
    keysToUpdate := determineProfileKeys(changedFiles)
    // e.g., migration files changed -> update db_schema
    //        handler/router changed -> update api_catalog
    //        go.mod/package.json changed -> update module_graph

    if len(keysToUpdate) > 0 {
        // Trigger incremental scan via Temporal
        h.service.TriggerIncrementalScan(c.Request.Context(), project.ID, project.CreatedBy, keysToUpdate)
    }

    c.JSON(200, gin.H{"status": "ok", "updated_keys": keysToUpdate})
}

func determineProfileKeys(files []string) []string {
    keys := map[string]bool{}
    for _, f := range files {
        switch {
        case strings.Contains(f, "migration") || strings.Contains(f, "schema"):
            keys["db_schema"] = true
        case strings.Contains(f, "handler") || strings.Contains(f, "router") || strings.Contains(f, "controller"):
            keys["api_catalog"] = true
        case strings.HasSuffix(f, "go.mod") || strings.HasSuffix(f, "package.json"):
            keys["module_graph"] = true
        case strings.Contains(f, "service") || strings.Contains(f, "rules"):
            keys["business_rules"] = true
        }
    }
    var result []string
    for k := range keys {
        result = append(result, k)
    }
    return result
}
```

Register webhook (public route, no JWT):
```go
api.POST("/webhooks/github/push", deps.ProfileHandler.HandleGitPush)
```

- [ ] **Step 2: Create ER diagram component**

`forge-portal/components/profile/er-diagram.tsx`:

Uses React Flow to render an ER diagram from `db_schema` profile data:
- Each table is a React Flow node with:
  - Header: table name
  - Body: column list (name, type, PK/FK indicators)
  - Color: purple border for tables with foreign keys
- Edges: foreign key relationships between tables
- Layout: auto-arranged using a simple grid (or dagre if installed)
- Interactive: zoom, pan, click table for detail

Input: `tables: Array<{name, columns: Array<{name, type, primary, nullable}>, relations: Array<{table, column}>}>`

- [ ] **Step 3: Create module dependency graph component**

`forge-portal/components/profile/module-graph.tsx`:

Uses React Flow to render module dependencies:
- Each module is a node with: name, path, exports count
- Edges: depends_on relationships
- Color by module type (inferred from path):
  - auth = blue, api/handler = green, service = purple, model/db = orange
- Interactive: click module shows exports list

Input: `modules: Array<{name, path, depends_on: string[], exports: string[]}>`

- [ ] **Step 4: Update profile page with visualization tabs**

In `forge-portal/app/(dashboard)/projects/[id]/profile/page.tsx`:

Add a tab layout:
```
[Data View]  [ER Diagram]  [Module Graph]
```

- **Data View**: existing profile cards (API catalog, DB schema, etc.)
- **ER Diagram**: renders `ERDiagram` component from `db_schema` profile
- **Module Graph**: renders `ModuleGraph` component from `module_graph` profile

If a dimension has no data, show "No data. Click Scan to generate." message.

- [ ] **Step 5: Verify builds**

```bash
cd forge-core && go build ./cmd/forge-core
cd forge-portal && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/ forge-portal/ ai-worker/
git commit -m "feat(s16): add incremental profile updates via webhook + ER diagram + module graph visualization"
```

---

## S17 Day 1: Preview Environment Creation

### Task 4: Preview Environment Workflow + Activities

**Files:**
- Create: `forge-core/internal/temporal/activity/preview_activities.go`
- Create: `forge-core/internal/temporal/workflow/preview_workflow.go`
- Modify: `forge-core/internal/module/preview/service.go`
- Modify: `forge-core/internal/module/preview/handler.go`

**IMPORTANT**: Read `forge-core/internal/module/preview/` and `pipeline.preview_environments` schema (migration 015).

- [ ] **Step 1: Create preview activities**

`forge-core/internal/temporal/activity/preview_activities.go`:

```go
type PreviewActivities struct {
    db     *pgxpool.Pool
    k8s    *k8s.Client
    docker DockerClient
}

type CreatePreviewInput struct {
    TaskID      int64  `json:"taskId"`
    ProjectID   int64  `json:"projectId"`
    TenantID    int64  `json:"tenantId"`
    BranchName  string `json:"branchName"`
    PRNumber    int    `json:"prNumber"`
    RepoURL     string `json:"repoUrl"`
    ProjectName string `json:"projectName"`
    ProjectType string `json:"projectType"` // go, node, python
}

type CreatePreviewOutput struct {
    PreviewURL string `json:"previewUrl"`
    Namespace  string `json:"namespace"`
    Status     string `json:"status"`
}

// CreatePreview sets up a complete preview environment
func (a *PreviewActivities) CreatePreview(ctx context.Context, input CreatePreviewInput) (*CreatePreviewOutput, error) {
    namespace := fmt.Sprintf("preview-%d", input.TaskID)
    previewHost := fmt.Sprintf("%d.preview.forge.example.com", input.TaskID)

    slog.Info("creating preview environment",
        "task_id", input.TaskID, "namespace", namespace, "host", previewHost)

    // 1. Create K8s namespace
    err := a.k8s.EnsureNamespace(ctx, namespace)
    if err != nil {
        return nil, fmt.Errorf("create namespace: %w", err)
    }

    // 2. Build Docker image (reuse artifact build or build fresh)
    imageTag := fmt.Sprintf("forge-preview/%s:%d", input.ProjectName, input.TaskID)
    err = a.docker.Build(ctx, DockerBuildOpts{
        ContextURL: input.RepoURL,
        Branch:     input.BranchName,
        Tags:       []string{imageTag},
    })
    if err != nil {
        return nil, fmt.Errorf("docker build: %w", err)
    }

    // 3. Generate K8s manifests (simplified for preview)
    manifest := generatePreviewManifest(input.ProjectName, imageTag, namespace, previewHost, 8080)

    // 4. Apply manifests
    err = a.k8s.ApplyManifest(ctx, namespace, manifest)
    if err != nil {
        return nil, fmt.Errorf("apply manifest: %w", err)
    }

    // 5. Wait for deployment ready
    statusCh, _ := a.k8s.WatchRollout(ctx, namespace, input.ProjectName)
    for status := range statusCh {
        if status.Phase == "AVAILABLE" {
            break
        }
        if status.Phase == "FAILED" {
            return nil, fmt.Errorf("preview deployment failed: %s", status.Message)
        }
    }

    return &CreatePreviewOutput{
        PreviewURL: fmt.Sprintf("https://%s", previewHost),
        Namespace:  namespace,
        Status:     "READY",
    }, nil
}

// DestroyPreview tears down a preview environment
func (a *PreviewActivities) DestroyPreview(ctx context.Context, namespace string) error {
    slog.Info("destroying preview environment", "namespace", namespace)
    return a.k8s.DeleteNamespace(ctx, namespace)
}

// ScaleToZero scales preview pods to 0 (idle timeout)
func (a *PreviewActivities) ScaleToZero(ctx context.Context, namespace, deploymentName string) error {
    return a.k8s.ScaleDeployment(ctx, namespace, deploymentName, 0)
}

func generatePreviewManifest(name, image, namespace, host string, port int) string {
    return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: app
        image: %s
        ports:
        - containerPort: %d
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "512Mi"
---
apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
  - port: 80
    targetPort: %d
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - %s
    secretName: %s-tls
  rules:
  - host: %s
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: 80`,
    name, namespace, name, name, image, port,
    name, namespace, name, port,
    name, namespace, host, name, host, name)
}
```

- [ ] **Step 2: Create preview workflow**

`forge-core/internal/temporal/workflow/preview_workflow.go`:

```go
type PreviewInput struct {
    TaskID      int64
    ProjectID   int64
    TenantID    int64
    BranchName  string
    PRNumber    int
    RepoURL     string
    ProjectName string
    ProjectType string
}

func PreviewWorkflow(ctx workflow.Context, input PreviewInput) error {
    localCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 10 * time.Minute,
    })

    // Create preview environment
    var result map[string]interface{}
    err := workflow.ExecuteActivity(localCtx, "CreatePreview", input).Get(ctx, &result)
    if err != nil {
        return err
    }

    // Update preview_environments table
    _ = workflow.ExecuteActivity(localCtx, "UpdatePreviewStatus",
        input.TaskID, result["previewUrl"], result["namespace"], "READY").Get(ctx, nil)

    // Set expiry timer: 30 minutes idle auto-scale-to-zero
    // (In production, this would use KEDA ScaledObject instead)
    selector := workflow.NewSelector(ctx)

    // Timer: 30 minutes
    timerFuture := workflow.NewTimer(ctx, 30*time.Minute)
    selector.AddFuture(timerFuture, func(f workflow.Future) {
        // Scale to 0 pods
        _ = workflow.ExecuteActivity(localCtx, "ScaleToZero",
            result["namespace"], input.ProjectName).Get(ctx, nil)
    })

    // Signal: destroy (from PR merge/close or manual)
    destroyCh := workflow.GetSignalChannel(ctx, "destroy_preview")
    selector.AddReceive(destroyCh, func(ch workflow.ReceiveChannel, more bool) {
        var reason string
        ch.Receive(ctx, &reason)
        // Destroy namespace
        _ = workflow.ExecuteActivity(localCtx, "DestroyPreview",
            result["namespace"]).Get(ctx, nil)
        _ = workflow.ExecuteActivity(localCtx, "UpdatePreviewStatus",
            input.TaskID, "", result["namespace"], "DESTROYED").Get(ctx, nil)
    })

    selector.Select(ctx)
    return nil
}
```

- [ ] **Step 3: Update preview service and handler**

In `service.go`:
```go
func (s *Service) CreatePreview(ctx context.Context, taskID, projectID, tenantID, userID int64) error {
    // Get task info (branch, PR number)
    // Get project info (repo URL, name, type)
    // Start PreviewWorkflow via Temporal
    // Insert preview_environments record with status=CREATING
}

func (s *Service) DestroyPreview(ctx context.Context, previewID int64) error {
    // Get preview record
    // Signal workflow: destroy_preview
    // Update status to DESTROYING
}
```

In `handler.go` -- existing endpoints already registered (migration 015 + router.go).

- [ ] **Step 4: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/
git commit -m "feat(s17): add preview environment creation workflow with auto-scale and cleanup"
```

---

## S17 Day 2: Frontend Preview UI + Lifecycle Management

### Task 5: Frontend -- Preview Environment Cards

**Files:**
- Create: `forge-portal/components/preview/preview-env-card.tsx`
- Create: `forge-portal/components/preview/preview-log-viewer.tsx`
- Modify: `forge-portal/components/tasks/task-workspace.tsx`

- [ ] **Step 1: Create PreviewEnvCard component**

`forge-portal/components/preview/preview-env-card.tsx`:

```
+-------------------------------------------------------------+
| Preview Environment                                           |
| Status: READY                                                |
|                                                               |
| URL: https://42.preview.forge.example.com                    |
| [Open in Browser]  [Copy URL]                                |
|                                                               |
| Namespace: preview-42                                        |
| Created: 15 min ago                                          |
| Expires: Idle 30 min -> auto-scale to 0                      |
|                                                               |
| [View Logs]  [Destroy]                                       |
+-------------------------------------------------------------+
```

States:
- CREATING: spinner + "Setting up preview environment..."
- READY: green dot + URL (clickable) + copy button + destroy button
- ERROR: red dot + error message + retry button
- DESTROYING: spinner + "Tearing down..."
- DESTROYED: gray + "Preview environment removed"

- [ ] **Step 2: Create PreviewLogViewer component**

Reuses log viewer pattern: monospace, auto-scroll, connects to SSE for preview deployment logs.

- [ ] **Step 3: Integrate into task workspace**

In `task-workspace.tsx`, add a preview section below the DEPLOY step:

```tsx
// After all steps, if task has a preview environment:
{preview && (
  <div className="mt-6">
    <PreviewEnvCard
      preview={preview}
      onDestroy={() => destroyPreview(projectId, preview.id)}
    />
  </div>
)}

// Or add a "Create Preview" button if no preview exists yet
{!preview && task.branchName && (
  <Button variant="outline" onClick={() => createPreview(projectId, taskId)}>
    <Globe className="mr-2 h-4 w-4" />
    Create Preview Environment
  </Button>
)}
```

- [ ] **Step 4: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(s17): add preview environment cards with status, URL, and lifecycle controls"
```

---

### Task 6: Build Verification + End-to-End Testing

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: End-to-end verification**

S16:
1. Run profile scan on a project with database migrations
2. Verify embeddings generated in profile_embeddings table
3. Test semantic search: query "user authentication" -> returns auth-related chunks
4. ER diagram tab renders tables with relationships
5. Module graph tab renders dependency arrows
6. Push to GitHub -> webhook triggers incremental update

S17:
1. Complete a task through to code push (branch created)
2. Click "Create Preview Environment"
3. Preview workflow starts: build image -> create namespace -> deploy -> set up ingress
4. Preview card shows CREATING -> READY with URL
5. Open preview URL in browser (requires DNS/ingress setup)
6. After 30 minutes idle -> pods scale to 0
7. Destroy preview -> namespace removed, card shows DESTROYED

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(s16+s17): complete profile RAG enhancement and cloud preview environments"
```

---

## Acceptance Criteria

### S16 -- Profile RAG Enhancement
- [ ] pgvector extension installed, profile_embeddings table created with HNSW index
- [ ] Profile data chunked intelligently per dimension type
- [ ] Embeddings generated using OpenAI text-embedding-3-small (1536 dims)
- [ ] Semantic search: query returns most relevant profile chunks by cosine similarity
- [ ] ContextBuilder uses semantic search for code generation (instead of loading all profiles)
- [ ] Git push webhook triggers incremental profile update
- [ ] ER diagram visualizes db_schema with table relationships (React Flow)
- [ ] Module dependency graph visualizes module_graph (React Flow)
- [ ] Profile page has tabs: Data View / ER Diagram / Module Graph

### S17 -- Cloud Preview Environments
- [ ] AI detects project type and generates Dockerfile + K8s manifests for preview
- [ ] Preview namespace created: `preview-{taskId}`
- [ ] Preview URL: `{taskId}.preview.forge.example.com` with auto SSL (cert-manager)
- [ ] 30-min idle auto-scale to 0 (Temporal timer or KEDA)
- [ ] PR merge/close triggers preview destruction (workflow signal)
- [ ] Frontend: preview card with URL, status, logs, destroy button
- [ ] `go build` + `npm run build` + ai-worker pass
