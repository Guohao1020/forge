## Phase 3 — API + autopilot 代管 + UI

**Goal:** `/api/forge/entropy-scans` CRUD，handler 侧代管 backing Autopilot（POST 建 / PATCH 改 / DELETE 删
同步）；前端 core 接线 + `packages/views/forge-entropy/` + web 路由。
**Depends-on:** Phase 0　**Unblocks:** Phase 4
**Completion gate:** `go build ./...` 通过；三包 typecheck 绿。

> 后端镜像 F1/F2/F3 的 forge handler 与路由；前端镜像 F2 `forge-checks` 的 list+editor 结构、
> reviewer 下拉镜像 F3。autopilot 代管用既有 `CreateAutopilot`/`CreateAutopilotTrigger`/`UpdateAutopilot`/
> `UpdateAutopilotTrigger`/`ListAutopilotTriggers`/`DeleteAutopilot` + handler-local `computeNextRun`。

---

### Task 3.1: 后端 handler + 路由

**Files:**
- Create: `server/internal/handler/forge_entropy.go`
- Modify: `server/cmd/server/router.go`（forge 路由块）

- [ ] **Step 1: handler**

`server/internal/handler/forge_entropy.go`：
```go
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F4: entropy scan config. Forge owns the scan definition and manages a
// backing Autopilot (schedule trigger = cron, execution_mode = create_issue,
// assignee = scanner agent). The dispatch hook composes the scanner's brief.

type ForgeEntropyScanBody struct {
	ProjectID        string `json:"project_id,omitempty"`
	Name             string `json:"name"`
	ScannerAgentID   string `json:"scanner_agent_id"`
	CustomFocus      string `json:"custom_focus"`
	IncludeStandards bool   `json:"include_standards"`
	IncludeChecks    bool   `json:"include_checks"`
	CronExpression   string `json:"cron_expression"`
	Timezone         string `json:"timezone"`
	Enabled          bool   `json:"enabled"`
}

type ForgeEntropyScanResponse struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id,omitempty"`
	Name             string `json:"name"`
	ScannerAgentID   string `json:"scanner_agent_id"`
	CustomFocus      string `json:"custom_focus"`
	IncludeStandards bool   `json:"include_standards"`
	IncludeChecks    bool   `json:"include_checks"`
	CronExpression   string `json:"cron_expression"`
	Timezone         string `json:"timezone"`
	Enabled          bool   `json:"enabled"`
	AutopilotID      string `json:"autopilot_id,omitempty"`
}

func entropyScanToResponse(s db.ForgeEntropyScan) ForgeEntropyScanResponse {
	out := ForgeEntropyScanResponse{
		ID: uuidToString(s.ID), Name: s.Name, ScannerAgentID: uuidToString(s.ScannerAgentID),
		CustomFocus: s.CustomFocus, IncludeStandards: s.IncludeStandards, IncludeChecks: s.IncludeChecks,
		CronExpression: s.CronExpression, Timezone: s.Timezone, Enabled: s.Enabled,
	}
	if s.ProjectID.Valid {
		out.ProjectID = uuidToString(s.ProjectID)
	}
	if s.AutopilotID.Valid {
		out.AutopilotID = uuidToString(s.AutopilotID)
	}
	return out
}

func (h *Handler) ListForgeEntropyScans(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	var projParam pgtype.UUID
	if pid := r.URL.Query().Get("project_id"); pid != "" {
		p, ok := parseUUIDOrBadRequest(w, pid, "project_id")
		if !ok {
			return
		}
		projParam = p
	}
	scans, err := h.Queries.ListEntropyScans(r.Context(), db.ListEntropyScansParams{
		WorkspaceID: parseUUID(wsID), ProjectID: projParam,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list entropy scans")
		return
	}
	out := make([]ForgeEntropyScanResponse, 0, len(scans))
	for _, s := range scans {
		out = append(out, entropyScanToResponse(s))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateForgeEntropyScan(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeEntropyScanBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.CronExpression == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expression are required")
		return
	}
	scannerID, ok := parseUUIDOrBadRequest(w, req.ScannerAgentID, "scanner_agent_id")
	if !ok {
		return
	}
	var projParam pgtype.UUID
	if req.ProjectID != "" {
		p, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
		if !ok {
			return
		}
		projParam = p
	}
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if err := service.ValidateTimezone(tz); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nextRun, err := computeNextRun(req.CronExpression, tz)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 1. backing autopilot first — a failure here leaves no orphan scan.
	status := "active"
	if !req.Enabled {
		status = "paused"
	}
	autopilot, err := h.Queries.CreateAutopilot(r.Context(), db.CreateAutopilotParams{
		WorkspaceID:   parseUUID(wsID),
		Title:         "Entropy scan: " + req.Name,
		AssigneeType:  "agent",
		AssigneeID:    scannerID,
		Status:        status,
		ExecutionMode: "create_issue",
		CreatedByType: "member",
		CreatedByID:   parseUUID(userID),
		ProjectID:     projParam,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backing autopilot")
		return
	}
	if _, err := h.Queries.CreateAutopilotTrigger(r.Context(), db.CreateAutopilotTriggerParams{
		AutopilotID:    autopilot.ID,
		Kind:           "schedule",
		Enabled:        true,
		CronExpression: pgtype.Text{String: req.CronExpression, Valid: true},
		Timezone:       pgtype.Text{String: tz, Valid: true},
		NextRunAt:      pgtype.Timestamptz{Time: nextRun, Valid: true},
	}); err != nil {
		_ = h.Queries.DeleteAutopilot(r.Context(), autopilot.ID)
		writeError(w, http.StatusInternalServerError, "failed to create schedule trigger")
		return
	}

	// 2. scan row.
	scan, err := h.Queries.CreateEntropyScan(r.Context(), db.CreateEntropyScanParams{
		WorkspaceID:      parseUUID(wsID),
		ProjectID:        projParam,
		Name:             req.Name,
		ScannerAgentID:   scannerID,
		CustomFocus:      req.CustomFocus,
		IncludeStandards: req.IncludeStandards,
		IncludeChecks:    req.IncludeChecks,
		CronExpression:   req.CronExpression,
		Timezone:         tz,
		Enabled:          req.Enabled,
		CreatedBy:        parseUUID(userID),
	})
	if err != nil {
		_ = h.Queries.DeleteAutopilot(r.Context(), autopilot.ID)
		writeError(w, http.StatusInternalServerError, "failed to create entropy scan")
		return
	}
	// 3. link scan → autopilot.
	if err := h.Queries.SetEntropyScanAutopilot(r.Context(), db.SetEntropyScanAutopilotParams{
		ID: scan.ID, AutopilotID: autopilot.ID,
	}); err != nil {
		_ = h.Queries.DeleteAutopilot(r.Context(), autopilot.ID)
		_, _ = h.Queries.DeleteEntropyScan(r.Context(), db.DeleteEntropyScanParams{ID: scan.ID, WorkspaceID: parseUUID(wsID)})
		writeError(w, http.StatusInternalServerError, "failed to link autopilot")
		return
	}
	scan.AutopilotID = autopilot.ID
	writeJSON(w, http.StatusCreated, entropyScanToResponse(scan))
}

func (h *Handler) UpdateForgeEntropyScan(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	scanID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "id")
	if !ok {
		return
	}
	var req ForgeEntropyScanBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.CronExpression == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expression are required")
		return
	}
	scannerID, ok := parseUUIDOrBadRequest(w, req.ScannerAgentID, "scanner_agent_id")
	if !ok {
		return
	}
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if err := service.ValidateTimezone(tz); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nextRun, err := computeNextRun(req.CronExpression, tz)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scan, err := h.Queries.UpdateEntropyScan(r.Context(), db.UpdateEntropyScanParams{
		ID: scanID, WorkspaceID: parseUUID(wsID),
		Name: req.Name, ScannerAgentID: scannerID, CustomFocus: req.CustomFocus,
		IncludeStandards: req.IncludeStandards, IncludeChecks: req.IncludeChecks,
		CronExpression: req.CronExpression, Timezone: tz, Enabled: req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "entropy scan not found")
		return
	}
	// Sync the backing autopilot best-effort — never fail the scan update on it.
	if scan.AutopilotID.Valid {
		h.syncEntropyAutopilot(r.Context(), scan, nextRun)
	}
	writeJSON(w, http.StatusOK, entropyScanToResponse(scan))
}

func (h *Handler) DeleteForgeEntropyScan(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	scanID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "id")
	if !ok {
		return
	}
	autopilotID, err := h.Queries.DeleteEntropyScan(r.Context(), db.DeleteEntropyScanParams{
		ID: scanID, WorkspaceID: parseUUID(wsID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "entropy scan not found")
		return
	}
	if autopilotID.Valid {
		if err := h.Queries.DeleteAutopilot(r.Context(), autopilotID); err != nil {
			slog.Warn("forge entropy: delete backing autopilot failed", "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// syncEntropyAutopilot mirrors scan fields onto the backing autopilot + its
// schedule trigger. Best-effort; logs and continues on any error.
func (h *Handler) syncEntropyAutopilot(ctx context.Context, scan db.ForgeEntropyScan, nextRun time.Time) {
	status := "active"
	if !scan.Enabled {
		status = "paused"
	}
	if _, err := h.Queries.UpdateAutopilot(ctx, db.UpdateAutopilotParams{
		ID:           scan.AutopilotID,
		Title:        pgtype.Text{String: "Entropy scan: " + scan.Name, Valid: true},
		AssigneeType: pgtype.Text{String: "agent", Valid: true},
		AssigneeID:   scan.ScannerAgentID,
		Status:       pgtype.Text{String: status, Valid: true},
		ProjectID:    scan.ProjectID,
	}); err != nil {
		slog.Warn("forge entropy: sync autopilot failed", "error", err)
	}
	triggers, err := h.Queries.ListAutopilotTriggers(ctx, scan.AutopilotID)
	if err != nil {
		slog.Warn("forge entropy: list triggers failed", "error", err)
		return
	}
	for _, t := range triggers {
		if t.Kind != "schedule" {
			continue
		}
		if _, err := h.Queries.UpdateAutopilotTrigger(ctx, db.UpdateAutopilotTriggerParams{
			ID:             t.ID,
			Enabled:        pgtype.Bool{Bool: true, Valid: true},
			CronExpression: pgtype.Text{String: scan.CronExpression, Valid: true},
			Timezone:       pgtype.Text{String: scan.Timezone, Valid: true},
			NextRunAt:      pgtype.Timestamptz{Time: nextRun, Valid: true},
		}); err != nil {
			slog.Warn("forge entropy: sync trigger failed", "error", err)
		}
	}
}
```

> `UpdateAutopilot` / `UpdateAutopilotTrigger` 是 COALESCE 式（已核：null=保留），故只传要改的字段，
> `Description`/`ExecutionMode` 不传即保留；`IssueTitleTemplate`/`project_id`/`next_run_at` 为直接 set。

- [ ] **Step 2: 路由**

`server/cmd/server/router.go`，在 `/api/forge/review-config` 块之后加：
```go
			// Forge F4: entropy scan config (periodic whole-repo quality scan).
			r.Route("/api/forge/entropy-scans", func(r chi.Router) {
				r.Get("/", h.ListForgeEntropyScans)
				r.Post("/", h.CreateForgeEntropyScan)
				r.Route("/{id}", func(r chi.Router) {
					r.Patch("/", h.UpdateForgeEntropyScan)
					r.Delete("/", h.DeleteForgeEntropyScan)
				})
			})
```

- [ ] **Step 3: 编译**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -10 && echo OK"`
Expected: 打印 `OK`。（若报 `chi` 包名/路径不符，照 `internal/handler/autopilot.go` 的 chi import 修正。）

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/forge_entropy.go server/cmd/server/router.go
git commit -m "feat(forge): entropy-scans CRUD API + backing autopilot management"
```

---

### Task 3.2: 前端 core 接线 + view + web 路由

**Files:**
- Create: `packages/core/types/forge-entropy.ts`
- Modify: `packages/core/types/index.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/workspace/queries.ts`
- Create: `packages/views/forge-entropy/forge-entropy-page.tsx`
- Create: `packages/views/forge-entropy/index.ts`
- Modify: `packages/views/package.json`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/forge-entropy/page.tsx`

- [ ] **Step 1: 类型**

`packages/core/types/forge-entropy.ts`：
```ts
// Forge F4: entropy scan config — periodic whole-repo quality scan for a scope.

export interface ForgeEntropyScan {
  id: string;
  project_id?: string;
  name: string;
  scanner_agent_id: string;
  custom_focus: string;
  include_standards: boolean;
  include_checks: boolean;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
  autopilot_id?: string;
}

export interface ForgeEntropyScanInput {
  project_id?: string;
  name: string;
  scanner_agent_id: string;
  custom_focus: string;
  include_standards: boolean;
  include_checks: boolean;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
}
```

`packages/core/types/index.ts`，在 `export type { ForgeReviewConfig } from "./forge-review";` 之后加：
```ts
export type { ForgeEntropyScan, ForgeEntropyScanInput } from "./forge-entropy";
```

- [ ] **Step 2: client 方法**

`packages/core/api/client.ts`：import 块加 `ForgeEntropyScan, ForgeEntropyScanInput`（在 `ForgeReviewConfig` 旁）。
在 `putForgeReviewConfig` 方法之后加：
```ts
  // Forge F4: entropy scans (periodic quality scan config)
  async listForgeEntropyScans(): Promise<ForgeEntropyScan[]> {
    return this.fetch("/api/forge/entropy-scans");
  }

  async createForgeEntropyScan(
    data: ForgeEntropyScanInput,
  ): Promise<ForgeEntropyScan> {
    return this.fetch("/api/forge/entropy-scans", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateForgeEntropyScan(
    id: string,
    data: ForgeEntropyScanInput,
  ): Promise<ForgeEntropyScan> {
    return this.fetch(`/api/forge/entropy-scans/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  async deleteForgeEntropyScan(id: string): Promise<void> {
    await this.fetch(`/api/forge/entropy-scans/${id}`, { method: "DELETE" });
  }
```

- [ ] **Step 3: queries**

`packages/core/workspace/queries.ts`，在 `forgeReviewConfig` key 之后加：
```ts
  forgeEntropyScans: (wsId: string) =>
    ["workspaces", wsId, "forge-entropy-scans"] as const,
```
在 `forgeReviewConfigOptions` 之后加：
```ts
export function forgeEntropyScanListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeEntropyScans(wsId),
    queryFn: () => api.listForgeEntropyScans(),
  });
}
```

- [ ] **Step 4: view**

`packages/views/forge-entropy/forge-entropy-page.tsx`：
```tsx
"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  agentListOptions,
  forgeEntropyScanListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import type { ForgeEntropyScan, ForgeEntropyScanInput } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Label } from "@multica/ui/components/ui/label";

const EMPTY: ForgeEntropyScanInput = {
  name: "",
  scanner_agent_id: "",
  custom_focus: "",
  include_standards: true,
  include_checks: true,
  cron_expression: "0 9 * * 1",
  timezone: "UTC",
  enabled: true,
};

export function ForgeEntropyPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data: scans = [], isLoading } = useQuery(forgeEntropyScanListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<ForgeEntropyScanInput>(EMPTY);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const startCreate = () => {
    setEditingId(null);
    setForm(EMPTY);
    setError("");
  };

  const startEdit = (s: ForgeEntropyScan) => {
    setEditingId(s.id);
    setForm({
      project_id: s.project_id,
      name: s.name,
      scanner_agent_id: s.scanner_agent_id,
      custom_focus: s.custom_focus,
      include_standards: s.include_standards,
      include_checks: s.include_checks,
      cron_expression: s.cron_expression,
      timezone: s.timezone,
      enabled: s.enabled,
    });
    setError("");
  };

  const invalidate = () =>
    qc.invalidateQueries({ queryKey: workspaceKeys.forgeEntropyScans(wsId) });

  const submit = async () => {
    if (!form.name.trim() || !form.cron_expression.trim()) {
      setError("name and cron are required");
      return;
    }
    if (!form.scanner_agent_id) {
      setError("pick a scanner agent");
      return;
    }
    setSaving(true);
    setError("");
    try {
      if (editingId) {
        await api.updateForgeEntropyScan(editingId, form);
      } else {
        await api.createForgeEntropyScan(form);
      }
      await invalidate();
      startCreate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save scan");
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    await api.deleteForgeEntropyScan(id);
    await invalidate();
    if (editingId === id) startCreate();
  };

  const sorted = useMemo(
    () => [...scans].sort((a, b) => a.name.localeCompare(b.name)),
    [scans],
  );
  const activeAgents = useMemo(
    () => agents.filter((a) => !a.archived_at),
    [agents],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Entropy scans</h1>
          <p className="text-xs text-muted-foreground">
            A scanner agent periodically surveys the whole repo for quality drift
            and files advisory issues. The brief is composed from this project's
            standards (F1) + checks (F2) + your focus areas.
          </p>
        </div>
        <Button size="sm" onClick={startCreate}>
          New scan
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_1.2fr] min-h-0 flex-1">
        {/* List */}
        <div className="min-h-0 overflow-y-auto rounded-md border">
          {isLoading ? (
            <p className="p-4 text-sm text-muted-foreground">Loading…</p>
          ) : sorted.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">No scans yet.</p>
          ) : (
            <ul className="divide-y">
              {sorted.map((s) => (
                <li
                  key={s.id}
                  className="flex items-center justify-between gap-2 px-3 py-2 hover:bg-accent/40"
                >
                  <button
                    type="button"
                    className="min-w-0 flex-1 text-left"
                    onClick={() => startEdit(s)}
                  >
                    <div className="truncate text-sm font-medium">{s.name}</div>
                    <div className="truncate font-mono text-xs text-muted-foreground">
                      {s.cron_expression} · {s.project_id ? "project" : "workspace"}
                      {s.enabled ? "" : " · disabled"}
                    </div>
                  </button>
                  <Button variant="ghost" size="sm" onClick={() => remove(s.id)}>
                    Delete
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Editor */}
        <div className="min-h-0 overflow-y-auto rounded-md border p-4">
          <h2 className="mb-3 text-sm font-medium">
            {editingId ? "Edit scan" : "New scan"}
          </h2>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="weekly entropy scan"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Scanner agent</Label>
              <select
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                value={form.scanner_agent_id}
                onChange={(e) =>
                  setForm({ ...form, scanner_agent_id: e.target.value })
                }
              >
                <option value="">— select an agent —</option>
                {activeAgents.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Cron</Label>
                <Input
                  className="font-mono"
                  value={form.cron_expression}
                  onChange={(e) =>
                    setForm({ ...form, cron_expression: e.target.value })
                  }
                  placeholder="0 9 * * 1"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Timezone</Label>
                <Input
                  value={form.timezone}
                  onChange={(e) => setForm({ ...form, timezone: e.target.value })}
                  placeholder="UTC"
                />
              </div>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.include_standards}
                onChange={(e) =>
                  setForm({ ...form, include_standards: e.target.checked })
                }
              />
              Include project standards (F1) in the brief
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.include_checks}
                onChange={(e) =>
                  setForm({ ...form, include_checks: e.target.checked })
                }
              />
              Include verification checks (F2) in the brief
            </label>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Additional focus areas
              </Label>
              <Textarea
                rows={3}
                value={form.custom_focus}
                onChange={(e) =>
                  setForm({ ...form, custom_focus: e.target.value })
                }
                placeholder="dead code, TODO/FIXME accumulation, doc staleness…"
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              />
              Enabled
            </label>
            {error ? <p className="text-xs text-destructive">{error}</p> : null}
            <div className="flex justify-end gap-2">
              {editingId ? (
                <Button variant="ghost" size="sm" onClick={startCreate}>
                  Cancel
                </Button>
              ) : null}
              <Button size="sm" onClick={submit} disabled={saving}>
                {editingId ? "Save" : "Create"}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
```

`packages/views/forge-entropy/index.ts`：
```ts
export { ForgeEntropyPage } from "./forge-entropy-page";
```

- [ ] **Step 5: views package export + web 路由**

`packages/views/package.json`，在 `"./forge-review": "./forge-review/index.ts",` 之后加：
```json
    "./forge-entropy": "./forge-entropy/index.ts",
```

`apps/web/app/[workspaceSlug]/(dashboard)/forge-entropy/page.tsx`：
```tsx
export { ForgeEntropyPage as default } from "@multica/views/forge-entropy";
```

- [ ] **Step 6: typecheck**

Run (Windows PowerShell)：
```
cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8
```
Expected: 三包 Done。

- [ ] **Step 7: Commit**

```bash
git add packages/core/types/forge-entropy.ts packages/core/types/index.ts packages/core/api/client.ts packages/core/workspace/queries.ts packages/views/forge-entropy/ packages/views/package.json "apps/web/app/[workspaceSlug]/(dashboard)/forge-entropy/"
git commit -m "feat(forge): F4 entropy-scans UI + core wiring"
```

---

## Phase 3 完成检查
- [ ] `/api/forge/entropy-scans` CRUD + 代管 backing autopilot（建/改/删同步），`go build ./...` 绿
- [ ] 前端 core 接线 + view + web 路由，三包 typecheck 绿
