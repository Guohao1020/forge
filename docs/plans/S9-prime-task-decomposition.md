# S9' -- Task Decomposition Enhanced Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the PlannerAgent to output `touched_files` per task node (predicted file paths each sub-task will create or modify), use project context tools (query_api_catalog, query_module_graph, read_project_file) to produce accurate predictions, and build a React Flow DAG visualization on the frontend with conflict indicators for file overlaps.

**Architecture:** Enhance PlannerAgent prompt to include `touched_files` output per task node. PlannerAgent uses context tools from ContextBuilder (project profiles) to determine accurate file paths. Frontend adds a React Flow-based DAG visualization where nodes represent tasks, edges represent depends_on relationships, and nodes with file overlaps show warning icons. Node click opens a side panel with task details and touched_files list.

**Tech Stack:** Python 3.12 (PlannerAgent), Go 1.22 + pgx (task_nodes enhancement), Next.js + React Flow + shadcn/ui

**Dependencies:** S9 (base task decomposition with task_nodes table, already complete), S16 (project profiles for context tools)

**Duration:** 2 days

---

## File Structure

### Python AI Worker

```
ai-worker/src/
+-- agents/planner.py              # MODIFY: add touched_files to output schema + context tool usage
+-- activities/plan.py             # MODIFY: pass context tools data to planner
```

### Go Backend

```
forge-core/
+-- internal/module/task/
|   +-- model.go                   # MODIFY: add TouchedFiles field to TaskNode
|   +-- repository.go              # MODIFY: update CreateNodes to store touched_files
|   +-- handler.go                 # MODIFY: add GET conflicts endpoint
+-- internal/router/router.go     # MODIFY: register new route
```

### Frontend

```
forge-portal/
+-- components/tasks/
|   +-- dag-visualization.tsx      # NEW: React Flow DAG visualization
|   +-- dag-node.tsx               # NEW: custom node component for DAG
|   +-- dag-side-panel.tsx         # NEW: side panel for node details
|   +-- plan-output-card.tsx       # MODIFY: integrate DAG visualization
+-- lib/tasks.ts                   # MODIFY: add conflict detection types
+-- package.json                   # MODIFY: add @xyflow/react dependency
```

---

## Day 1: PlannerAgent Enhancement + Backend

### Task 1: PlannerAgent -- touched_files Output + Context Tools

**Files:**
- Modify: `ai-worker/src/agents/planner.py`
- Modify: `ai-worker/src/activities/plan.py`

**IMPORTANT**: Read `ai-worker/src/agents/planner.py`, `ai-worker/src/activities/plan.py`, `ai-worker/src/context/builder.py` first.

- [ ] **Step 1: Enhance PlannerAgent prompt**

In `ai-worker/src/agents/planner.py`, update `PLANNER_SYSTEM_PROMPT` to include `touched_files`:

```python
PLANNER_SYSTEM_PROMPT = """You are a senior software architect. Your task is to decompose a requirement into a DAG (Directed Acyclic Graph) of implementation tasks.

## Rules
1. Each task should be completable by modifying 1-3 files
2. Specify dependencies explicitly -- which tasks must complete before this one can start
3. Map each task back to which part of the requirement it addresses
4. Estimate effort in hours (0.5, 1, 2, 4, 8)
5. Identify task type: BACKEND, FRONTEND, SCHEMA, CONFIG, TEST
6. Tasks with no dependencies can run in parallel
7. Never create circular dependencies
8. For each task, list the EXACT file paths that will be created or modified (touched_files)
9. Use the project's existing file structure and conventions to determine paths
10. If the project uses Go modules, use the actual module path (e.g., internal/module/user/service.go)
11. If creating new files, follow the project's naming conventions

## Project Analysis
When project profiles are available:
- Use api_catalog to avoid duplicate API endpoints
- Use db_schema to reference existing tables correctly
- Use module_graph to follow existing module boundaries
- Use architecture patterns to match existing code style

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown, no text before or after the JSON.
Do NOT wrap the JSON in ```json``` code blocks. Just output the raw JSON directly.
CRITICAL: Base your plan ENTIRELY on the user's actual requirement. Do NOT copy or reuse the example below.

{"title": "<title>", "tasks": [{"order": 1, "title": "<task>", "description": "<what to implement>", "type": "SCHEMA|BACKEND|FRONTEND|CONFIG|TEST", "files": ["<file paths>"], "touched_files": {"create": ["<new file paths>"], "modify": ["<existing file paths>"]}, "depends_on": [], "estimate_hours": 0.5, "requirement_ref": "<requirement reference>"}], "risk_level": "LOW|MEDIUM|HIGH", "risk_factors": [], "total_estimate_hours": 0, "parallel_tracks": 1}
"""
```

Key change: each task now has `touched_files: { create: string[], modify: string[] }` distinguishing new files from modifications.

- [ ] **Step 2: Update plan activity to pass profile context**

In `ai-worker/src/activities/plan.py`, enhance user prompt construction to include relevant profile data:

```python
# In plan_task_activity, after building context:
if ctx.project_profiles:
    # Inject API catalog summary for planner to avoid duplicates
    api_catalog = ctx.project_profiles.get("api_catalog", {})
    if api_catalog:
        endpoints = api_catalog.get("endpoints", [])
        if endpoints:
            api_summary = "\n".join(
                f"  {e.get('method', '?')} {e.get('path', '?')} -> {e.get('handler', '?')}"
                for e in endpoints[:30]  # limit to 30 for token budget
            )
            user_prompt += f"\n## Existing API Endpoints (DO NOT duplicate)\n{api_summary}\n"

    # Inject module graph for file path prediction
    module_graph = ctx.project_profiles.get("module_graph", {})
    if module_graph:
        modules = module_graph.get("modules", [])
        if modules:
            mod_summary = "\n".join(
                f"  {m.get('name', '?')}: {m.get('path', '?')} -> depends_on {m.get('depends_on', [])}"
                for m in modules[:20]
            )
            user_prompt += f"\n## Project Module Structure\n{mod_summary}\n"

    # Inject DB schema for SCHEMA-type tasks
    db_schema = ctx.project_profiles.get("db_schema", {})
    if db_schema:
        tables = db_schema.get("tables", [])
        if tables:
            table_names = [t.get("name", "?") for t in tables[:30]]
            user_prompt += f"\n## Existing DB Tables\n{', '.join(table_names)}\n"
```

- [ ] **Step 3: Update PlanOutput to capture touched_files**

In `ai-worker/src/activities/plan.py`, the PlanOutput already captures tasks as `List[Dict]`. The `touched_files` field will be included automatically in the task dict. No structural change needed to PlanOutput, but add validation:

```python
# Validate touched_files in each task
for task in result.structured.get("tasks", []):
    if "touched_files" not in task:
        # Fallback: convert files[] to touched_files.modify
        task["touched_files"] = {"create": [], "modify": task.get("files", [])}
```

- [ ] **Step 4: Verify imports**

```bash
cd ai-worker && python -c "from src.activities.plan import plan_task_activity; print('OK')"
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/agents/planner.py ai-worker/src/activities/plan.py
git commit -m "feat(s9'): enhance PlannerAgent with touched_files output and profile context injection"
```

---

### Task 2: Go Backend -- TaskNode touched_files Storage

**Files:**
- Modify: `forge-core/internal/module/task/model.go`
- Modify: `forge-core/internal/module/task/repository.go`
- Modify: `forge-core/internal/module/task/handler.go`
- Modify: `forge-core/internal/router/router.go`

**IMPORTANT**: Read `forge-core/internal/module/task/model.go`, `repository.go`, `handler.go` first.

- [ ] **Step 1: Add TouchedFiles to TaskNode model**

In `model.go`, the existing `TaskNode` struct has a `Files` field (`json.RawMessage`). Add a `TouchedFiles` field alongside it:

```go
type TaskNode struct {
    // ... existing fields ...
    Files          json.RawMessage `json:"files"`
    TouchedFiles   json.RawMessage `json:"touchedFiles"`   // NEW: {"create": [...], "modify": [...]}
    // ... rest of fields ...
}

// FileConflict represents overlapping files between task nodes within a task or version
type FileConflict struct {
    FilePath    string  `json:"filePath"`
    NodeOrderA  int     `json:"nodeOrderA"`
    NodeOrderB  int     `json:"nodeOrderB"`
    NodeTitleA  string  `json:"nodeTitleA"`
    NodeTitleB  string  `json:"nodeTitleB"`
}
```

- [ ] **Step 2: Update repository CreateNodes**

In `repository.go`, update the `CreateNodes` INSERT to include `touched_files`:

```sql
INSERT INTO engine.task_nodes (task_id, node_order, title, description, node_type, depends_on, files, touched_files, estimate_hours, requirement_ref)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
```

Note: This requires adding a `touched_files JSONB NOT NULL DEFAULT '{}'` column to `engine.task_nodes`. Add an ALTER TABLE in the CreateNodes function as a safe migration pattern, or add a new migration file:

```sql
-- In a new migration or in-line with CreateNodes
ALTER TABLE engine.task_nodes ADD COLUMN IF NOT EXISTS touched_files JSONB NOT NULL DEFAULT '{}';
```

- [ ] **Step 3: Add conflict detection handler**

Add an endpoint to detect file conflicts within a task's node set:

```go
// GET /api/projects/:id/tasks/:taskId/conflicts -- detect file overlaps between nodes
func (h *Handler) ListTaskConflicts(c *gin.Context) {
    taskID, _ := strconv.ParseInt(c.Param("taskId"), 10, 64)
    nodes, err := h.service.GetTaskNodes(c.Request.Context(), taskID)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to get task nodes")
        return
    }
    conflicts := detectNodeConflicts(nodes)
    response.OK(c, gin.H{"conflicts": conflicts})
}
```

Conflict detection logic (in-memory, no extra query):
```go
func detectNodeConflicts(nodes []TaskNode) []FileConflict {
    // Build map: filePath -> []nodeOrder
    fileMap := make(map[string][]int)
    nodeMap := make(map[int]string) // order -> title
    for _, n := range nodes {
        nodeMap[n.NodeOrder] = n.Title
        var tf struct {
            Create []string `json:"create"`
            Modify []string `json:"modify"`
        }
        _ = json.Unmarshal(n.TouchedFiles, &tf)
        allFiles := append(tf.Create, tf.Modify...)
        for _, f := range allFiles {
            fileMap[f] = append(fileMap[f], n.NodeOrder)
        }
    }
    // Find overlaps
    var conflicts []FileConflict
    for path, orders := range fileMap {
        if len(orders) > 1 {
            for i := 0; i < len(orders)-1; i++ {
                for j := i + 1; j < len(orders); j++ {
                    conflicts = append(conflicts, FileConflict{
                        FilePath:   path,
                        NodeOrderA: orders[i],
                        NodeOrderB: orders[j],
                        NodeTitleA: nodeMap[orders[i]],
                        NodeTitleB: nodeMap[orders[j]],
                    })
                }
            }
        }
    }
    return conflicts
}
```

- [ ] **Step 4: Register route**

In `router.go`, add:
```go
protected.GET("/projects/:id/tasks/:taskId/conflicts", deps.TaskHandler.ListTaskConflicts)
```

- [ ] **Step 5: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/
git commit -m "feat(s9'): add touched_files to TaskNode and file conflict detection API"
```

---

## Day 2: Frontend DAG Visualization

### Task 3: Install React Flow + Create DAG Components

**Files:**
- Modify: `forge-portal/package.json` (add @xyflow/react)
- Create: `forge-portal/components/tasks/dag-node.tsx`
- Create: `forge-portal/components/tasks/dag-side-panel.tsx`
- Create: `forge-portal/components/tasks/dag-visualization.tsx`
- Modify: `forge-portal/components/tasks/plan-output-card.tsx`
- Modify: `forge-portal/lib/tasks.ts`

- [ ] **Step 1: Install React Flow**

```bash
cd forge-portal && npm install @xyflow/react
```

- [ ] **Step 2: Update lib/tasks.ts types**

Add types for touched_files and conflicts:

```typescript
export interface TouchedFiles {
  create: string[];
  modify: string[];
}

export interface TaskNode {
  id: number;
  taskId: number;
  nodeOrder: number;
  title: string;
  description?: string;
  nodeType: string;
  status: string;
  dependsOn: number[];
  files: string[];
  touchedFiles: TouchedFiles;
  estimateHours?: number;
  requirementRef?: string;
}

export interface NodeConflict {
  filePath: string;
  nodeOrderA: number;
  nodeOrderB: number;
  nodeTitleA: string;
  nodeTitleB: string;
}

export async function getTaskConflicts(projectId: number, taskId: number): Promise<NodeConflict[]> {
  const res = await api.get<{ conflicts: NodeConflict[] }>(
    `/projects/${projectId}/tasks/${taskId}/conflicts`
  );
  return res.conflicts || [];
}
```

- [ ] **Step 3: Create custom DAG node component**

`forge-portal/components/tasks/dag-node.tsx`:

Custom React Flow node with:
- Node body: title + type badge + estimate hours
- Color by node_type:
  - BACKEND: `border-blue-500 bg-blue-500/10`
  - FRONTEND: `border-green-500 bg-green-500/10`
  - SCHEMA: `border-orange-500 bg-orange-500/10`
  - TEST: `border-purple-500 bg-purple-500/10`
  - CONFIG: `border-gray-500 bg-gray-500/10`
- Status indicator: small dot (PENDING=gray, READY=blue, RUNNING=purple pulse, COMPLETED=green, SKIPPED=gray strikethrough)
- Conflict warning: if node has file overlaps, show `AlertTriangle` icon (amber) at top-right corner
- Estimate hours shown as small badge: "2h"
- Source/target handles for edges

- [ ] **Step 4: Create DAG side panel**

`forge-portal/components/tasks/dag-side-panel.tsx`:

Slide-in panel (right side, 360px wide) showing details of clicked node:
- Node title and description
- Type badge + status badge
- Estimate hours
- Requirement reference
- `touched_files` section:
  - "New files" list (create) with `FilePlus` icon
  - "Modified files" list (modify) with `FileEdit` icon
- Conflicts section (if any): list of conflicting file paths with the other node's title
- Dependencies: list of depends_on nodes by title
- Close button

- [ ] **Step 5: Create DAG visualization component**

`forge-portal/components/tasks/dag-visualization.tsx`:

Main component that:
1. Takes `TaskNode[]` and `NodeConflict[]` as props
2. Converts task nodes to React Flow nodes/edges:
   - Each TaskNode becomes a node at position calculated by a simple layered layout:
     - X: based on longest dependency chain depth (layer)
     - Y: distributed evenly within each layer
   - Each `depends_on` relationship becomes an edge (animated for RUNNING status)
3. Layout algorithm (simple, no dagre needed):
   ```
   Layer 0: nodes with depends_on = []
   Layer 1: nodes depending only on Layer 0
   Layer N: nodes depending on Layer N-1
   ```
   - Node width: 240px, gap: 60px horizontal, 80px vertical
4. Bottom stats bar: "Total: X hours | Parallel tracks: Y | Nodes: Z"
5. Controls: zoom in/out, fit view, minimap
6. Click node -> open DagSidePanel

- [ ] **Step 6: Integrate into PlanOutputCard**

Read `forge-portal/components/tasks/plan-output-card.tsx` and modify:

- If plan output has tasks with `touched_files` fields, render `DagVisualization`
- If tasks have `depends_on` but no `touched_files`, render `DagVisualization` with legacy format (touched_files = {create: [], modify: files})
- If no depends_on at all (legacy), keep existing simple list
- Add toggle: "List View" / "DAG View" tabs
- Default to DAG view when data supports it

- [ ] **Step 7: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 8: Commit**

```bash
git add forge-portal/
git commit -m "feat(s9'): add React Flow DAG visualization with conflict indicators and side panel"
```

---

### Task 4: Build Verification + End-to-End Testing

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: Rebuild AI Worker**

```bash
docker compose -f docker-compose.dev.yml up -d --build ai-worker
```

- [ ] **Step 4: End-to-end verification**

1. Restart Go backend (new binary)
2. Create new task -> AI analyzes -> confirm
3. PLAN step completes:
   - DB task_nodes contain `touched_files` with create/modify arrays
   - API `GET /projects/:id/tasks/:taskId/nodes` returns touched_files
   - API `GET /projects/:id/tasks/:taskId/conflicts` returns any overlaps
4. Frontend:
   - PlanOutputCard shows React Flow DAG
   - Nodes colored by type (BACKEND=blue, FRONTEND=green, etc.)
   - Edges show dependency flow
   - Click node -> side panel shows touched_files
   - Conflict nodes show warning icon
   - Bottom bar shows total hours and parallel tracks
5. Toggle between DAG view and list view works

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s9'): complete enhanced task decomposition with DAG + touched_files + conflict pre-marking"
```

---

## Acceptance Criteria

- [ ] PlannerAgent outputs `touched_files: { create: [...], modify: [...] }` per task node
- [ ] PlannerAgent uses project profile data (api_catalog, module_graph, db_schema) for accurate file prediction
- [ ] task_nodes table stores touched_files as JSONB
- [ ] Conflict detection API identifies file overlaps between nodes
- [ ] React Flow DAG visualizes task dependency graph
- [ ] Node colors match type: BACKEND=blue, FRONTEND=green, SCHEMA=orange, TEST=purple
- [ ] Nodes with file overlaps show warning icon
- [ ] Click node opens side panel with details + touched_files + conflicts
- [ ] Bottom stats: total hours, parallel tracks, node count
- [ ] DAG/List view toggle works
- [ ] `go build` + `npm run build` + ai-worker rebuild pass
