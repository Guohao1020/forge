# S14 -- K8s Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy AI-generated artifacts to Kubernetes environments with AI-generated manifests (Deployment, Service, Ingress, ConfigMap, HPA), rolling update strategy, real-time deployment status tracking via SSE, health checks, and one-click rollback to previous versions. Environment management UI shows pod status and resource usage.

**Architecture:** After BUILD step produces a Docker image, AI generates K8s manifests based on project type and target environment. Deployment uses `kubectl apply` via client-go. Status tracking watches the Deployment rollout via the K8s watch API and streams events to frontend via SSE. Rollback creates a new deploy record pointing to the previous artifact's manifest. Pipeline.deploy_records (migration 014) and pipeline.environments (migration 008) tables already exist.

**Tech Stack:** Go 1.22 + client-go (K8s SDK), K8s (k3s dev / ACK prod), Next.js + SSE + shadcn/ui

**Dependencies:** S13 (artifact management), pipeline.deploy_records + pipeline.environments tables (migrations 008, 014 exist)

**Duration:** 3 days

---

## File Structure

### Go Backend

```
forge-core/
+-- internal/temporal/
|   +-- activity/deploy_activities.go      # NEW: K8s deploy + rollback + health check activities
|   +-- workflow/task_workflow.go           # MODIFY: add real DEPLOY step
+-- internal/module/pipeline/
|   +-- model.go                           # MODIFY: add DeployRecord fields + manifest types
|   +-- repository.go                      # MODIFY: add deploy history + rollback queries
|   +-- service.go                         # MODIFY: add deploy logic + rollback + status tracking
|   +-- handler.go                         # MODIFY: add rollback + pod status + deploy logs endpoints
+-- internal/k8s/
|   +-- manifest.go                        # NEW: K8s manifest generation (AI-assisted)
|   +-- client.go                          # MODIFY: add ApplyManifest, WatchRollout, GetPodStatus methods
+-- internal/router/router.go             # MODIFY: register new routes
```

### Python AI Worker

```
ai-worker/src/
+-- agents/manifest_generator.py           # NEW: AI agent for K8s manifest generation
+-- activities/manifest.py                 # NEW: manifest generation activity
```

### Frontend

```
forge-portal/
+-- app/(dashboard)/projects/[id]/
|   +-- environments/page.tsx              # NEW: environment management page (replaces basic env cards)
|   +-- environments/[envId]/page.tsx      # NEW: environment detail with deploy history
+-- components/deploy/
|   +-- deploy-status-tracker.tsx          # NEW: real-time rollout progress
|   +-- rollback-dialog.tsx                # NEW: rollback confirmation
|   +-- pod-status-grid.tsx                # NEW: pod status visualization
|   +-- deploy-log-viewer.tsx              # NEW: kubectl output viewer
+-- components/project-sidebar.tsx         # MODIFY: update "Environments" nav
+-- lib/deploy.ts                          # NEW: deployment API client
```

---

## Day 1: K8s Manifest Generation + Apply

### Task 1: AI Manifest Generator

**Files:**
- Create: `ai-worker/src/agents/manifest_generator.py`
- Create: `ai-worker/src/activities/manifest.py`
- Modify: `ai-worker/src/worker.py`

- [ ] **Step 1: Create ManifestGeneratorAgent**

`ai-worker/src/agents/manifest_generator.py`:

```python
MANIFEST_SYSTEM_PROMPT = """You are a Kubernetes infrastructure expert. Generate K8s resource manifests for deploying an application.

## Rules
1. Generate ALL necessary resources: Deployment, Service, Ingress, ConfigMap
2. Add HPA (Horizontal Pod Autoscaler) for production environments
3. Environment-specific configuration:
   - DEV: 1 replica, relaxed resource limits (100m-500m CPU, 128Mi-512Mi memory), no HPA
   - STAGING: 2 replicas, production-like limits (250m-1 CPU, 256Mi-1Gi memory), HPA 2-5
   - PROD: 3 replicas, strict limits (500m-2 CPU, 512Mi-2Gi memory), HPA 3-10
4. Rolling update strategy: maxSurge=1, maxUnavailable=0 (zero-downtime)
5. Health checks: readinessProbe (HTTP GET /health) + livenessProbe (HTTP GET /health)
6. Labels: app={name}, version={version}, managed-by=forge
7. Namespace: {project}-{env_type}

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object.
{"manifests": [{"kind": "Deployment", "name": "...", "yaml": "apiVersion: apps/v1\\nkind: Deployment\\n..."}, {"kind": "Service", "yaml": "..."}, {"kind": "Ingress", "yaml": "..."}, {"kind": "ConfigMap", "yaml": "..."}, {"kind": "HorizontalPodAutoscaler", "yaml": "..."}], "namespace": "...", "summary": "..."}
"""

class ManifestGeneratorAgent(BaseAgent):
    purpose = Purpose.GENERATE

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = MANIFEST_SYSTEM_PROMPT
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
```

- [ ] **Step 2: Create manifest generation activity**

`ai-worker/src/activities/manifest.py`:

```python
@dataclass
class ManifestInput:
    task_id: int
    project_id: int
    project_name: str
    image_url: str
    version: str
    env_type: str  # DEV, STAGING, PROD
    port: int      # application port (default 8080)
    env_vars: Dict[str, str]  # environment variables

@dataclass
class ManifestOutput:
    manifests: List[Dict[str, str]]  # [{kind, name, yaml}]
    namespace: str
    combined_yaml: str  # all manifests joined with ---
    summary: str
    tokens_used: int = 0

@activity.defn(name="generate_k8s_manifests")
async def generate_k8s_manifests_activity(input: ManifestInput) -> ManifestOutput:
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-generation")

        user_prompt = f"""Generate Kubernetes manifests for deploying this application:

- Project: {input.project_name}
- Image: {input.image_url}
- Version: {input.version}
- Environment: {input.env_type}
- Application Port: {input.port}
- Environment Variables: {json.dumps(input.env_vars)}

Generate Deployment, Service, Ingress, ConfigMap, and HPA (if STAGING/PROD)."""

        router = ModelRouter()
        agent = ManifestGeneratorAgent(router)
        result = await agent.run(user_prompt, ctx)

        manifests = result.structured.get("manifests", [])
        namespace = result.structured.get("namespace", f"{input.project_name}-{input.env_type.lower()}")

        # Combine all YAML into one document
        combined = "\n---\n".join(m.get("yaml", "") for m in manifests)

        return ManifestOutput(
            manifests=manifests,
            namespace=namespace,
            combined_yaml=combined,
            summary=result.structured.get("summary", ""),
            tokens_used=result.tokens_used,
        )
    finally:
        await builder.close()
```

- [ ] **Step 3: Register activity**

In `worker.py`: add `generate_k8s_manifests_activity`.

- [ ] **Step 4: Verify**

```bash
cd ai-worker && python -c "from src.activities.manifest import generate_k8s_manifests_activity; print('OK')"
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/
git commit -m "feat(s14): add ManifestGeneratorAgent and K8s manifest generation activity"
```

---

### Task 2: Go Backend -- Deploy Activities + K8s Client

**Files:**
- Create: `forge-core/internal/temporal/activity/deploy_activities.go`
- Create: `forge-core/internal/k8s/manifest.go`
- Modify: `forge-core/internal/k8s/client.go`

**IMPORTANT**: Read `forge-core/internal/k8s/client.go` first (already has mock/real K8s client pattern).

- [ ] **Step 1: Add manifest helper**

`forge-core/internal/k8s/manifest.go`:

```go
package k8s

import (
    "fmt"
    "strings"
)

// GenerateFallbackManifest produces a basic Deployment+Service if AI generation fails
func GenerateFallbackManifest(projectName, imageURL, namespace, envType string, port int) string {
    replicas := 1
    cpuRequest, memRequest := "100m", "128Mi"
    cpuLimit, memLimit := "500m", "512Mi"

    switch envType {
    case "STAGING":
        replicas = 2
        cpuRequest, memRequest = "250m", "256Mi"
        cpuLimit, memLimit = "1", "1Gi"
    case "PROD":
        replicas = 3
        cpuRequest, memRequest = "500m", "512Mi"
        cpuLimit, memLimit = "2", "2Gi"
    }

    return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    managed-by: forge
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
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
            cpu: "%s"
            memory: "%s"
          limits:
            cpu: "%s"
            memory: "%s"
        readinessProbe:
          httpGet:
            path: /health
            port: %d
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: %d
          initialDelaySeconds: 15
          periodSeconds: 20
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
  type: ClusterIP`,
    projectName, namespace, projectName, replicas, projectName, projectName,
    imageURL, port, cpuRequest, memRequest, cpuLimit, memLimit, port, port,
    projectName, namespace, projectName, port)
}
```

- [ ] **Step 2: Add deploy methods to K8s client**

In `forge-core/internal/k8s/client.go`, add:

```go
// ApplyManifest applies a YAML manifest to the cluster
func (c *Client) ApplyManifest(ctx context.Context, namespace, yamlManifest string) error

// WatchRollout watches a Deployment rollout and returns status updates via channel
func (c *Client) WatchRollout(ctx context.Context, namespace, deploymentName string) (<-chan RolloutStatus, error)

// GetPodStatus returns pod statuses for a deployment
func (c *Client) GetPodStatus(ctx context.Context, namespace, appLabel string) ([]PodStatus, error)

// RollbackDeployment rolls back to a previous revision
func (c *Client) RollbackDeployment(ctx context.Context, namespace, deploymentName string, revision int) error

type RolloutStatus struct {
    Phase            string    // PROGRESSING, AVAILABLE, FAILED
    ReadyReplicas    int
    DesiredReplicas  int
    UpdatedReplicas  int
    Message          string
    Timestamp        time.Time
}

type PodStatus struct {
    Name      string    `json:"name"`
    Phase     string    `json:"phase"`     // Running, Pending, Failed, Succeeded
    Ready     bool      `json:"ready"`
    Restarts  int       `json:"restarts"`
    Age       string    `json:"age"`
    CPU       string    `json:"cpu"`       // from metrics API
    Memory    string    `json:"memory"`    // from metrics API
}
```

- [ ] **Step 3: Create deploy activities**

`forge-core/internal/temporal/activity/deploy_activities.go`:

```go
type DeployActivities struct {
    db  *pgxpool.Pool
    k8s *k8s.Client
}

type DeployInput struct {
    TaskID       int64  `json:"taskId"`
    ProjectID    int64  `json:"projectId"`
    TenantID     int64  `json:"tenantId"`
    EnvID        int64  `json:"envId"`
    ArtifactID   int64  `json:"artifactId"`
    ImageURL     string `json:"imageUrl"`
    Version      string `json:"version"`
    Manifest     string `json:"manifest"`     // combined YAML
    Namespace    string `json:"namespace"`
    DeployedBy   int64  `json:"deployedBy"`
}

// DeployToK8s applies manifest and watches rollout
func (a *DeployActivities) DeployToK8s(ctx context.Context, input DeployInput) error {
    slog.Info("deploying to K8s", "task_id", input.TaskID, "namespace", input.Namespace)

    // 1. Create namespace if not exists
    _ = a.k8s.EnsureNamespace(ctx, input.Namespace)

    // 2. Apply manifest
    err := a.k8s.ApplyManifest(ctx, input.Namespace, input.Manifest)
    if err != nil {
        return fmt.Errorf("kubectl apply failed: %w", err)
    }

    // 3. Watch rollout (with timeout)
    statusCh, err := a.k8s.WatchRollout(ctx, input.Namespace, extractDeploymentName(input.Manifest))
    if err != nil {
        return fmt.Errorf("watch rollout failed: %w", err)
    }

    // 4. Stream status updates to Redis (for SSE)
    for status := range statusCh {
        publishDeployStatus(a.db, input.TaskID, status)
        if status.Phase == "AVAILABLE" {
            break
        }
        if status.Phase == "FAILED" {
            return fmt.Errorf("deployment failed: %s", status.Message)
        }
    }

    // 5. Health check: verify pods are ready
    pods, _ := a.k8s.GetPodStatus(ctx, input.Namespace, extractAppLabel(input.Manifest))
    for _, pod := range pods {
        if !pod.Ready {
            return fmt.Errorf("pod %s not ready after deployment", pod.Name)
        }
    }

    slog.Info("deployment successful", "task_id", input.TaskID, "namespace", input.Namespace)
    return nil
}

// RollbackDeploy rolls back to a previous deploy record
func (a *DeployActivities) RollbackDeploy(ctx context.Context, envID, previousDeployID int64) error {
    // 1. Get the previous deploy record's manifest
    // 2. Apply it
    // 3. Watch rollout
    // 4. Update deploy_records status
    return nil
}
```

- [ ] **Step 4: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/
git commit -m "feat(s14): add K8s deploy activities with manifest apply, rollout watch, and health checks"
```

---

## Day 2: Pipeline Module Enhancement + Rollback

### Task 3: Pipeline Module -- Deploy Records + Rollback API

**Files:**
- Modify: `forge-core/internal/module/pipeline/model.go`
- Modify: `forge-core/internal/module/pipeline/repository.go`
- Modify: `forge-core/internal/module/pipeline/service.go`
- Modify: `forge-core/internal/module/pipeline/handler.go`
- Modify: `forge-core/internal/router/router.go`

**IMPORTANT**: Read all pipeline module files first.

- [ ] **Step 1: Enhance model with rollback types**

```go
type DeployRecord struct {
    ID            int64      `json:"id"`
    TenantID      int64      `json:"tenantId"`
    ProjectID     int64      `json:"projectId"`
    EnvironmentID int64      `json:"environmentId"`
    ArtifactID    *int64     `json:"artifactId,omitempty"`
    Version       string     `json:"version"`
    Status        string     `json:"status"`
    DeployedBy    int64      `json:"deployedBy"`
    StartedAt     time.Time  `json:"startedAt"`
    CompletedAt   *time.Time `json:"completedAt,omitempty"`
    K8sManifest   *string    `json:"k8sManifest,omitempty"`
    ErrorMessage  *string    `json:"errorMessage,omitempty"`
    CreatedAt     time.Time  `json:"createdAt"`
}

type RollbackRequest struct {
    DeployRecordID int64 `json:"deployRecordId" binding:"required"`
    Reason         string `json:"reason"`
}

type DeployStatusResponse struct {
    Deploy  DeployRecord `json:"deploy"`
    Pods    []PodStatus  `json:"pods"`
    Events  []string     `json:"events"`
}
```

- [ ] **Step 2: Add rollback handler**

```go
// POST /api/projects/:id/environments/:envId/rollback -- rollback to previous version
func (h *Handler) RollbackDeploy(c *gin.Context)

// GET /api/projects/:id/environments/:envId/pods -- get pod statuses
func (h *Handler) GetPodStatus(c *gin.Context)

// GET /api/projects/:id/environments/:envId/deploy-logs -- SSE stream deploy logs
func (h *Handler) StreamDeployLogs(c *gin.Context)
```

- [ ] **Step 3: Register routes**

```go
protected.POST("/projects/:id/environments/:envId/rollback", deps.PipelineHandler.RollbackDeploy)
protected.GET("/projects/:id/environments/:envId/pods", deps.PipelineHandler.GetPodStatus)
protected.GET("/projects/:id/environments/:envId/deploy-logs", deps.PipelineHandler.StreamDeployLogs)
```

- [ ] **Step 4: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/
git commit -m "feat(s14): add rollback, pod status, and deploy log streaming API"
```

---

## Day 3: Frontend -- Environment Management + Deploy UI

### Task 4: Frontend -- Environment + Deploy Pages

**Files:**
- Create: `forge-portal/lib/deploy.ts`
- Create: `forge-portal/components/deploy/deploy-status-tracker.tsx`
- Create: `forge-portal/components/deploy/rollback-dialog.tsx`
- Create: `forge-portal/components/deploy/pod-status-grid.tsx`
- Create: `forge-portal/components/deploy/deploy-log-viewer.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/environments/page.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/environments/[envId]/page.tsx`
- Modify: `forge-portal/components/project-sidebar.tsx`

- [ ] **Step 1: Create API client lib/deploy.ts**

```typescript
export interface DeployRecord { ... }
export interface PodStatus { ... }

export async function listDeployRecords(projectId: number, envId: number): Promise<DeployRecord[]>
export async function triggerDeploy(projectId: number, envId: number, artifactId: number): Promise<DeployRecord>
export async function rollbackDeploy(projectId: number, envId: number, deployId: number, reason: string): Promise<void>
export async function getPodStatus(projectId: number, envId: number): Promise<PodStatus[]>
```

- [ ] **Step 2: Create DeployStatusTracker component**

Real-time deployment progress:
```
Deploying v0.1.5-a3b...
[====>                  ] 2/3 pods ready

Events:
14:23:01  Deployment scaled up
14:23:05  Pod myapp-abc123 Pulling image
14:23:12  Pod myapp-abc123 Started
14:23:15  Pod myapp-def456 Pulling image
```

Connects to SSE endpoint for real-time updates. Shows progress bar (readyReplicas / desiredReplicas).

- [ ] **Step 3: Create RollbackDialog component**

shadcn AlertDialog:
- Shows current version and target rollback version
- Reason textarea (optional)
- Confirm button
- Warning: "This will replace the current deployment with version v0.1.4"

- [ ] **Step 4: Create PodStatusGrid component**

Grid of pod cards:
```
+--- myapp-abc123 ---+  +--- myapp-def456 ---+  +--- myapp-ghi789 ---+
| Running | Ready    |  | Running | Ready    |  | Pending | Not Ready|
| CPU: 120m          |  | CPU: 95m           |  | CPU: --            |
| Memory: 256Mi      |  | Memory: 210Mi      |  | Memory: --         |
| Restarts: 0        |  | Restarts: 1        |  | Restarts: 0        |
| Age: 2h            |  | Age: 2h            |  | Age: 30s           |
+--------------------+  +--------------------+  +--------------------+
```

- [ ] **Step 5: Create DeployLogViewer component**

Similar to TestLogViewer: monospace, auto-scroll, color-coded kubectl output.

- [ ] **Step 6: Create environment management page**

`environments/page.tsx`:

```
+-------------------------------------------------------------+
| Environments                                                  |
+-------------------------------------------------------------+
| +--- Development ------------------------------------------+ |
| | Status: ACTIVE  |  Version: v0.1.5  |  3/3 pods ready   | |
| | Last deploy: 2h ago by Harvey                            | |
| | [View Details]  [Deploy Latest]  [Rollback]              | |
| +----------------------------------------------------------+ |
|                                                               |
| +--- Staging -----------------------------------------------+ |
| | Status: INACTIVE  |  No deployments yet                  | |
| | [Deploy]                                                  | |
| +----------------------------------------------------------+ |
|                                                               |
| +--- Production --------------------------------------------+ |
| | Status: INACTIVE  |  No deployments yet                  | |
| | [Deploy]                                                  | |
| +----------------------------------------------------------+ |
+-------------------------------------------------------------+
```

- [ ] **Step 7: Create environment detail page**

`environments/[envId]/page.tsx`:

Shows: pod status grid, deploy history table, deploy logs, rollback button, resource usage summary.

- [ ] **Step 8: Update ProjectSidebar**

Ensure "Environments" link points to the new page:
```tsx
{ icon: Server, label: "Environments", href: `/projects/${projectId}/environments` },
```

- [ ] **Step 9: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 10: Commit**

```bash
git add forge-portal/
git commit -m "feat(s14): add environment management UI with deploy tracker, pod status, and rollback"
```

---

### Task 5: Build Verification + End-to-End Testing

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: End-to-end verification**

1. Complete a task through BUILD step (artifact created)
2. Navigate to Environments page
3. Click "Deploy" on Development environment
4. DeployStatusTracker shows real-time progress
5. Pods appear in PodStatusGrid as they start
6. Deploy completes -> environment status = ACTIVE
7. Deploy history shows the record
8. Click "Rollback" -> confirm dialog -> previous version deployed
9. Deploy logs show kubectl output

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(s14): complete K8s deployment with manifest generation, rollout, and rollback"
```

---

## Acceptance Criteria

- [ ] AI generates K8s manifests (Deployment, Service, Ingress, ConfigMap, HPA)
- [ ] Environment-specific config: DEV (relaxed), STAGING (prod-like), PROD (strict)
- [ ] Rolling update: maxSurge=1, maxUnavailable=0 (zero-downtime)
- [ ] Health checks: readinessProbe + livenessProbe on /health
- [ ] Deployment via client-go kubectl apply
- [ ] Real-time rollout progress via SSE
- [ ] Pod status display with CPU/memory usage
- [ ] One-click rollback to previous version
- [ ] Deploy logs captured and displayed
- [ ] Environment management page with all 3 environments
- [ ] `go build` + `npm run build` pass
