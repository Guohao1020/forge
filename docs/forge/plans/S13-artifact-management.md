# S13 -- Artifact Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Docker image from AI-generated code, push to a container registry (ACR or GHCR), run vulnerability scanning with Trivy, and manage artifact metadata (versions, sizes, layers, base images, vulnerabilities) in the database with a frontend artifact list and detail UI.

**Architecture:** After tests pass, a Temporal activity builds a Docker image (AI-generated Dockerfile from S11' or auto-detected), pushes it to the configured registry, runs Trivy scan, and stores metadata in `pipeline.artifacts` (migration 013 already exists). The artifact module provides CRUD API and links artifacts to tasks and versions. Frontend shows artifact list with version history, scan results, and download links.

**Tech Stack:** Go 1.22 + Docker SDK + Trivy, K8s Job (for build), ACR/GHCR (registry), Next.js + shadcn/ui

**Dependencies:** S12 (automated testing), S11' (code generation with Dockerfile), pipeline.artifacts table (migration 013 exists)

**Duration:** 2 days

---

## File Structure

### Go Backend

```
forge-core/
+-- internal/temporal/
|   +-- activity/artifact_activities.go   # NEW: Docker build + push + scan activities
|   +-- workflow/task_workflow.go          # MODIFY: add BUILD step after TEST
+-- internal/module/artifact/
|   +-- model.go                          # MODIFY: add vulnerability and layer types
|   +-- repository.go                     # MODIFY: add search and version history queries
|   +-- service.go                        # MODIFY: add build trigger + scan logic
|   +-- handler.go                        # MODIFY: add detail + scan results endpoints
+-- internal/module/task/model.go         # MODIFY: add StepTypeBuild constant
+-- internal/router/router.go             # MODIFY: register new artifact routes
```

### Frontend

```
forge-portal/
+-- app/(dashboard)/projects/[id]/
|   +-- artifacts/page.tsx                # NEW: artifact list page
|   +-- artifacts/[aid]/page.tsx          # NEW: artifact detail page
+-- components/artifacts/
|   +-- vulnerability-badge.tsx           # NEW: severity badge (CRITICAL/HIGH/MEDIUM/LOW)
|   +-- scan-results-card.tsx             # NEW: Trivy scan results display
|   +-- artifact-version-history.tsx      # NEW: version history timeline
+-- components/project-sidebar.tsx        # MODIFY: add "Artifacts" nav item
+-- lib/artifacts.ts                      # NEW: artifact API client + types
```

---

## Day 1: Docker Build + Push + Scan Activities

### Task 1: Artifact Activities -- Build, Push, Scan

**Files:**
- Create: `forge-core/internal/temporal/activity/artifact_activities.go`
- Modify: `forge-core/internal/module/task/model.go`
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`

**IMPORTANT**: Read `forge-core/internal/temporal/activity/task_activities.go` and `forge-core/internal/module/artifact/` files first.

- [ ] **Step 1: Add StepTypeBuild to task model**

In `forge-core/internal/module/task/model.go`:

```go
const (
    // ... existing step types ...
    StepTypeBuild = "BUILD"   // NEW: Docker build + push + scan
)

// Update AllSteps: insert BUILD between TEST and DEPLOY
var AllSteps = []struct {
    Name     string
    StepType string
}{
    {"Require Analysis", StepTypeAnalyze},
    {"Plan Design", StepTypePlan},
    {"Test Design", StepTypeTestWriting},
    {"Code Generation", StepTypeGenerate},
    {"Code Review", StepTypeReview},
    {"Automated Testing", StepTypeTest},
    {"Build Artifact", StepTypeBuild},    // NEW
    {"Deploy Release", StepTypeDeploy},
}
```

- [ ] **Step 2: Create artifact activities**

`forge-core/internal/temporal/activity/artifact_activities.go`:

```go
type ArtifactActivities struct {
    db       *pgxpool.Pool
    docker   DockerClient
    registry RegistryConfig
}

type RegistryConfig struct {
    URL      string // e.g., registry.cn-hangzhou.aliyuncs.com/forge
    Username string
    Password string
    Type     string // ACR, GHCR, DOCKERHUB
}

type BuildInput struct {
    TaskID      int64  `json:"taskId"`
    ProjectID   int64  `json:"projectId"`
    TenantID    int64  `json:"tenantId"`
    RepoURL     string `json:"repoUrl"`
    Branch      string `json:"branch"`
    Dockerfile  string `json:"dockerfile"`   // Dockerfile content (from AI generation)
    ProjectName string `json:"projectName"`
    Version     string `json:"version"`      // semantic version
}

type BuildOutput struct {
    ImageURL      string            `json:"imageUrl"`
    ImageTag      string            `json:"imageTag"`
    ImageDigest   string            `json:"imageDigest"`
    SizeBytes     int64             `json:"sizeBytes"`
    Layers        int               `json:"layers"`
    BaseImage     string            `json:"baseImage"`
    BuildDuration int64             `json:"buildDurationMs"`
    Vulnerabilities []Vulnerability `json:"vulnerabilities"`
    VulnSummary   VulnSummary       `json:"vulnSummary"`
}

type Vulnerability struct {
    ID          string `json:"id"`          // CVE-2024-XXXX
    Severity    string `json:"severity"`    // CRITICAL, HIGH, MEDIUM, LOW
    Package     string `json:"package"`
    Version     string `json:"version"`
    FixedIn     string `json:"fixedIn"`
    Description string `json:"description"`
}

type VulnSummary struct {
    Critical int `json:"critical"`
    High     int `json:"high"`
    Medium   int `json:"medium"`
    Low      int `json:"low"`
}

// BuildAndPush builds a Docker image from the generated code and pushes to registry
func (a *ArtifactActivities) BuildAndPush(ctx context.Context, input BuildInput) (*BuildOutput, error) {
    slog.Info("building Docker image", "task_id", input.TaskID, "version", input.Version)

    // 1. Generate image tag: {project}-v{version}-{short-sha}
    shortSHA := input.Branch[:7] // or commit SHA
    imageTag := fmt.Sprintf("%s/%s:v%s-%s",
        a.registry.URL, input.ProjectName, input.Version, shortSHA)

    // 2. Build image
    //    Option A: K8s Job with kaniko (no Docker daemon needed)
    //    Option B: Docker SDK buildx (requires Docker daemon)
    //    Start with Option B for simplicity, upgrade to kaniko later
    buildResult, err := a.docker.Build(ctx, DockerBuildOpts{
        ContextURL: input.RepoURL,
        Branch:     input.Branch,
        Dockerfile: input.Dockerfile,
        Tags:       []string{imageTag},
        Platform:   "linux/amd64",
    })
    if err != nil {
        return nil, fmt.Errorf("docker build failed: %w", err)
    }

    // 3. Push to registry
    err = a.docker.Push(ctx, imageTag, a.registry.Username, a.registry.Password)
    if err != nil {
        return nil, fmt.Errorf("docker push failed: %w", err)
    }

    // 4. Run Trivy vulnerability scan
    vulns, summary := a.runTrivyScan(ctx, imageTag)

    return &BuildOutput{
        ImageURL:        imageTag,
        ImageTag:        fmt.Sprintf("v%s-%s", input.Version, shortSHA),
        ImageDigest:     buildResult.Digest,
        SizeBytes:       buildResult.SizeBytes,
        Layers:          buildResult.Layers,
        BaseImage:       extractBaseImage(input.Dockerfile),
        BuildDuration:   buildResult.DurationMs,
        Vulnerabilities: vulns,
        VulnSummary:     summary,
    }, nil
}

// runTrivyScan executes Trivy image scan
func (a *ArtifactActivities) runTrivyScan(ctx context.Context, imageTag string) ([]Vulnerability, VulnSummary) {
    // Run: trivy image --format json {imageTag}
    // Parse JSON output into Vulnerability structs
    // If trivy not available, return empty (non-blocking)
    cmd := exec.CommandContext(ctx, "trivy", "image", "--format", "json", "--severity", "CRITICAL,HIGH,MEDIUM,LOW", imageTag)
    output, err := cmd.Output()
    if err != nil {
        slog.Warn("trivy scan failed", "error", err)
        return nil, VulnSummary{}
    }
    // Parse trivy JSON output...
    return parseTrivy(output)
}
```

- [ ] **Step 3: Integrate BUILD step into workflow**

In `task_workflow.go`, after TEST step and before DEPLOY:

```go
// ---- Step: BUILD (Docker build + push + scan) ----
if allTestsPassed {
    err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
        TaskID: input.TaskID, StepType: "BUILD", TaskStatus: "BUILDING", Duration: 0,
    }).Get(ctx, nil)

    // Extract Dockerfile from generate output
    dockerfile := extractDockerfile(generateResult)
    version := generateSemanticVersion(input.ProjectID, input.TaskID)

    buildInput := map[string]interface{}{
        "taskId":      input.TaskID,
        "projectId":   input.ProjectID,
        "tenantId":    input.TenantID,
        "repoUrl":     input.RepoURL,
        "branch":      input.Branch,
        "dockerfile":  dockerfile,
        "projectName": input.ProjectName,
        "version":     version,
    }

    var buildResult map[string]interface{}
    err = workflow.ExecuteActivity(buildCtx, "BuildAndPush", buildInput).Get(ctx, &buildResult)
    if err != nil {
        slog.Warn("build failed, continuing to deploy without artifact", "error", err)
    } else {
        // Save artifact to database
        _ = workflow.ExecuteActivity(localCtx, "SaveArtifact", input.TaskID, buildResult).Get(ctx, nil)
    }
    _ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "BUILD", buildResult).Get(ctx, nil)
}
```

Version generation:
```go
func generateSemanticVersion(projectID, taskID int64) string {
    // Query latest version for project, increment patch
    // Format: {major}.{minor}.{patch}
    // e.g., 1.0.0 -> 1.0.1
    return fmt.Sprintf("0.1.%d", taskID) // Simple for now
}
```

- [ ] **Step 4: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/
git commit -m "feat(s13): add Docker build, push, and Trivy scan activities with workflow integration"
```

---

## Day 2: Artifact Module Enhancement + Frontend

### Task 2: Backend -- Artifact Model + API Enhancement

**Files:**
- Modify: `forge-core/internal/module/artifact/model.go`
- Modify: `forge-core/internal/module/artifact/repository.go`
- Modify: `forge-core/internal/module/artifact/service.go`
- Modify: `forge-core/internal/module/artifact/handler.go`
- Modify: `forge-core/internal/router/router.go`

**IMPORTANT**: Read all files in `forge-core/internal/module/artifact/` first.

- [ ] **Step 1: Enhance artifact model**

In `model.go`, add vulnerability and detail types:

```go
type Artifact struct {
    ID           int64           `json:"id"`
    TenantID     int64           `json:"tenantId"`
    ProjectID    int64           `json:"projectId"`
    TaskID       *int64          `json:"taskId,omitempty"`
    Name         string          `json:"name"`
    Version      string          `json:"version"`
    ArtifactType string          `json:"artifactType"`
    RegistryURL  *string         `json:"registryUrl,omitempty"`
    SizeBytes    *int64          `json:"sizeBytes,omitempty"`
    Checksum     *string         `json:"checksum,omitempty"`
    Metadata     json.RawMessage `json:"metadata"`
    Status       string          `json:"status"`
    CreatedAt    time.Time       `json:"createdAt"`
}

// Extended metadata stored in JSONB
type ArtifactMetadata struct {
    Layers        int             `json:"layers"`
    BaseImage     string          `json:"baseImage"`
    BuildDuration int64           `json:"buildDurationMs"`
    VulnSummary   VulnSummary     `json:"vulnSummary"`
    Vulnerabilities []Vulnerability `json:"vulnerabilities"`
    Digest        string          `json:"digest"`
    Platform      string          `json:"platform"`
}

type VulnSummary struct {
    Critical int `json:"critical"`
    High     int `json:"high"`
    Medium   int `json:"medium"`
    Low      int `json:"low"`
    Total    int `json:"total"`
}

type Vulnerability struct {
    ID          string `json:"id"`
    Severity    string `json:"severity"`
    Package     string `json:"package"`
    Version     string `json:"version"`
    FixedIn     string `json:"fixedIn"`
    Description string `json:"description"`
}

type ArtifactListResponse struct {
    Artifacts []Artifact `json:"artifacts"`
    Total     int64      `json:"total"`
}
```

- [ ] **Step 2: Add version history query**

In `repository.go`:

```go
// ListByProject returns all artifacts for a project ordered by created_at DESC
func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]Artifact, error)

// GetByID returns a single artifact with full metadata
func (r *Repository) GetByID(ctx context.Context, artifactID int64) (*Artifact, error)

// GetVersionHistory returns artifacts for a project grouped by name
func (r *Repository) GetVersionHistory(ctx context.Context, projectID int64, name string) ([]Artifact, error)
```

SQL for version history:
```sql
SELECT * FROM pipeline.artifacts
WHERE project_id = $1 AND name = $2
ORDER BY created_at DESC
LIMIT 20
```

- [ ] **Step 3: Add new handler endpoints**

```go
// GET /api/projects/:id/artifacts/:artifactId/vulnerabilities -- scan results
func (h *Handler) GetVulnerabilities(c *gin.Context)

// GET /api/projects/:id/artifacts/history?name=myapp -- version history
func (h *Handler) GetVersionHistory(c *gin.Context)
```

- [ ] **Step 4: Register routes**

```go
protected.GET("/projects/:id/artifacts/:artifactId/vulnerabilities", deps.ArtifactHandler.GetVulnerabilities)
protected.GET("/projects/:id/artifacts/history", deps.ArtifactHandler.GetVersionHistory)
```

- [ ] **Step 5: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/
git commit -m "feat(s13): enhance artifact module with vulnerability data and version history API"
```

---

### Task 3: Frontend -- Artifact List + Detail Pages

**Files:**
- Create: `forge-portal/lib/artifacts.ts`
- Create: `forge-portal/components/artifacts/vulnerability-badge.tsx`
- Create: `forge-portal/components/artifacts/scan-results-card.tsx`
- Create: `forge-portal/components/artifacts/artifact-version-history.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/artifacts/page.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/artifacts/[aid]/page.tsx`
- Modify: `forge-portal/components/project-sidebar.tsx`

- [ ] **Step 1: Create API client lib/artifacts.ts**

```typescript
import { api } from "./api";

export interface Artifact {
  id: number;
  tenantId: number;
  projectId: number;
  taskId?: number;
  name: string;
  version: string;
  artifactType: string;
  registryUrl?: string;
  sizeBytes?: number;
  checksum?: string;
  metadata: ArtifactMetadata;
  status: string;
  createdAt: string;
}

export interface ArtifactMetadata {
  layers: number;
  baseImage: string;
  buildDurationMs: number;
  vulnSummary: VulnSummary;
  vulnerabilities: Vulnerability[];
  digest: string;
  platform: string;
}

export interface VulnSummary {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total: number;
}

export interface Vulnerability {
  id: string;
  severity: string;
  package: string;
  version: string;
  fixedIn: string;
  description: string;
}

export const SEVERITY_CONFIG: Record<string, { label: string; color: string; bgClass: string }> = {
  CRITICAL: { label: "Critical", color: "text-red-400",    bgClass: "bg-red-500/20" },
  HIGH:     { label: "High",     color: "text-orange-400", bgClass: "bg-orange-500/20" },
  MEDIUM:   { label: "Medium",   color: "text-amber-400",  bgClass: "bg-amber-500/20" },
  LOW:      { label: "Low",      color: "text-blue-400",   bgClass: "bg-blue-500/20" },
};

export async function listArtifacts(projectId: number): Promise<Artifact[]> {
  const res = await api.get<{ artifacts: Artifact[] }>(`/projects/${projectId}/artifacts`);
  return res.artifacts || [];
}

export async function getArtifact(projectId: number, artifactId: number): Promise<Artifact> {
  return api.get(`/projects/${projectId}/artifacts/${artifactId}`);
}

export async function getVersionHistory(projectId: number, name: string): Promise<Artifact[]> {
  const res = await api.get<{ artifacts: Artifact[] }>(
    `/projects/${projectId}/artifacts/history?name=${encodeURIComponent(name)}`
  );
  return res.artifacts || [];
}
```

- [ ] **Step 2: Create VulnerabilityBadge component**

Colored badge per severity: CRITICAL=red, HIGH=orange, MEDIUM=amber, LOW=blue.

- [ ] **Step 3: Create ScanResultsCard component**

Shows vulnerability summary as bar chart (horizontal stacked) + detailed list with each CVE expandable.

- [ ] **Step 4: Create ArtifactVersionHistory component**

Vertical timeline showing artifact versions with size, date, vuln summary per version.

- [ ] **Step 5: Create artifact list page**

```
+-------------------------------------------------------------+
| Artifacts                                                     |
+-------------------------------------------------------------+
| Image          | Version    | Size   | Vulns       | Status  |
|----------------|------------|--------|-------------|---------|
| myapp          | v0.1.5-a3b| 156 MB | 0C 1H 3M   | READY   |
| myapp          | v0.1.4-f2c| 152 MB | 0C 0H 2M   | READY   |
| myapp          | v0.1.3-e1d| 148 MB | 1C 2H 5M   | READY   |
+-------------------------------------------------------------+
```

- [ ] **Step 6: Create artifact detail page**

Shows full metadata: image URL (copyable), digest, layers, base image, build duration, full vulnerability table, version history timeline.

- [ ] **Step 7: Update ProjectSidebar**

Add:
```tsx
{ icon: Package, label: "Artifacts", href: `/projects/${projectId}/artifacts` },
```

- [ ] **Step 8: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 9: Commit**

```bash
git add forge-portal/
git commit -m "feat(s13): add artifact list and detail pages with vulnerability display"
```

---

### Task 4: Build Verification

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: End-to-end verification**

1. Complete a task through GENERATE -> REVIEW -> TEST
2. BUILD step triggers:
   - Docker image built from AI-generated Dockerfile
   - Image pushed to registry (or mock in dev)
   - Trivy scan runs -> vulnerabilities detected
3. Artifact record created in pipeline.artifacts
4. Frontend:
   - Artifact list shows image name, version, size, vulnerability summary
   - Detail page shows full scan results, build metadata
   - Version history timeline shows previous builds
5. Artifact linked to task (via task_id FK)

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(s13): complete artifact management with Docker build, Trivy scan, and UI"
```

---

## Acceptance Criteria

- [ ] AI generates Dockerfile (multi-stage, language-specific) if not present
- [ ] Docker image built in K8s Job or Docker (dev fallback)
- [ ] Image pushed to configured registry (ACR/GHCR/DockerHub)
- [ ] Trivy vulnerability scan runs on built image
- [ ] Artifact metadata stored: image URL, version, size, layers, base image, vulns
- [ ] Semantic version generated: {project}-v{version}.{patch}-{short-sha}
- [ ] Frontend artifact list with version, size, vulnerability summary
- [ ] Frontend artifact detail with full scan results and version history
- [ ] BUILD step integrates into workflow after TEST
- [ ] Build failure non-blocking for deployment (warns but continues)
- [ ] `go build` + `npm run build` pass
