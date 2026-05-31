## Phase 2 — 双层注入钩子（InjectStandards + claim hook）

**Goal:** `forge.InjectStandards()` 把解析结果合进任务的 agent payload（Core→instructions，
Detail→forge-standards skill）；在 daemon claim handler 插一行调用。

**Depends-on:** Phase 1（`standards.Resolve`）　**Unblocks:** Phase 5
**Completion gate:** inject 集成测试断言 instructions 含 Core、skills 含 forge-standards；
claim 钩子编译通过。

> 注意写入路径：`docs/forge/plans/themis-2026-05-31/`（与 index 同目录）。

---

### Task 2.1: InjectStandards（先写测试）

**Files:**
- Create: `server/internal/forge/inject.go`
- Test: `server/internal/forge/inject_test.go`

- [ ] **Step 1: 写失败测试（用 fake Querier，不依赖 DB）**

`server/internal/forge/inject_test.go`：
```go
package forge

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/forge/standards"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type fakeQ struct {
	ws, proj []db.ForgeStandard
	tags     []string
}

func (f fakeQ) ListWorkspaceStandards(_ context.Context, _ pgtype.UUID) ([]db.ForgeStandard, error) {
	return f.ws, nil
}
func (f fakeQ) ListProjectStandards(_ context.Context, _ pgtype.UUID) ([]db.ForgeStandard, error) {
	return f.proj, nil
}
func (f fakeQ) GetForgeProjectProfile(_ context.Context, _ pgtype.UUID) (db.ForgeProjectProfile, error) {
	return db.ForgeProjectProfile{Tags: f.tags}, nil
}

var _ standards.Querier = fakeQ{}

func validUUID() pgtype.UUID { return pgtype.UUID{Valid: true} }

func TestInjectStandards_AppendsCoreAndSkill(t *testing.T) {
	q := fakeQ{ws: []db.ForgeStandard{
		{Category: "api", Name: "rest", CoreContent: "CORE-RULES", DetailContent: "DETAILED", Enabled: true},
	}}
	instr := "You are a code agent."
	var skills []service.AgentSkillData
	InjectStandards(context.Background(), q, &instr, &skills, validUUID(), pgtype.UUID{})

	if !strings.Contains(instr, "CORE-RULES") || !strings.Contains(instr, "You are a code agent.") {
		t.Fatalf("instructions should keep base + append core; got %q", instr)
	}
	if len(skills) != 1 || skills[0].Name != standards.SkillName || !strings.Contains(skills[0].Content, "DETAILED") {
		t.Fatalf("expected one forge-standards skill with detail; got %+v", skills)
	}
}

func TestInjectStandards_EmptyNoop(t *testing.T) {
	q := fakeQ{}
	instr := "base"
	var skills []service.AgentSkillData
	InjectStandards(context.Background(), q, &instr, &skills, validUUID(), pgtype.UUID{})
	if instr != "base" || len(skills) != 0 {
		t.Fatalf("no standards must leave payload unchanged; got instr=%q skills=%d", instr, len(skills))
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd <forge-repo>/server && go test ./internal/forge/ 2>&1 | tail -8`
Expected: 编译失败（`InjectStandards` undefined）。

- [ ] **Step 3: 写实现**

`server/internal/forge/inject.go`：
```go
// Package forge wires Forge's spec-center into Multica's task dispatch.
package forge

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/forge/standards"
	"github.com/multica-ai/multica/server/internal/service"
)

// InjectStandards resolves Forge standards for (workspaceID, projectID) and
// merges them into the task's agent payload: Core → instructions (appended,
// mandatory), Detail → a forge-standards skill (appended). Best-effort: on any
// error it logs and leaves the payload unchanged — never blocks a task.
func InjectStandards(
	ctx context.Context,
	q standards.Querier,
	instructions *string,
	skills *[]service.AgentSkillData,
	workspaceID, projectID pgtype.UUID,
) {
	res, err := standards.Resolve(ctx, q, workspaceID, projectID)
	if err != nil {
		slog.Warn("forge: resolve standards failed", "error", err)
		return
	}
	if res.Core != "" {
		if strings.TrimSpace(*instructions) == "" {
			*instructions = res.Core
		} else {
			*instructions = *instructions + "\n\n## Forge Coding Standards (mandatory)\n\n" + res.Core
		}
	}
	if res.Detail != "" {
		*skills = append(*skills, service.AgentSkillData{
			Name:        standards.SkillName,
			Description: "Project coding standards resolved by Forge.",
			Content:     res.Detail,
		})
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd <forge-repo>/server && go test ./internal/forge/ -v 2>&1 | tail -12`
Expected: 2 个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add server/internal/forge/inject.go server/internal/forge/inject_test.go
git commit -m "feat(forge): InjectStandards merges resolved standards into agent payload"
```

---

### Task 2.2: 接入 daemon claim handler（唯一侵入点）

**Files:**
- Modify: `server/internal/handler/daemon.go`（claim 响应组装处，约 1129-1140 行后）

- [ ] **Step 1: 加 projectID 辅助函数**

在 `daemon.go`（或同包合适处）加：
```go
// taskProjectID derives the project UUID for standards resolution from the
// task's issue. Returns the zero UUID when there is no issue/project.
func (h *Handler) taskProjectID(ctx context.Context, issueID pgtype.UUID) pgtype.UUID {
	if !issueID.Valid {
		return pgtype.UUID{}
	}
	iss, err := h.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return pgtype.UUID{}
	}
	return iss.ProjectID
}
```
> 实现时确认：claim 处的 `task` 变量是否有 `IssueID`/`WorkspaceID` 字段（F0 探查确认
> handler 已用 `task.AgentID`、`task.WorkspaceID`）；`GetIssue` 返回的 issue 是否有
> `ProjectID pgtype.UUID`（migration 034 加的）。若 `GetIssue` 签名不同，按实际调整。

- [ ] **Step 2: 在 resp.Agent 赋值后插一行注入**

在 `ClaimTaskByRuntime` 里 `resp.Agent = &TaskAgentData{...}` 这个赋值块（约 1139 行 `}`）
**之后**插入：
```go
if resp.Agent != nil {
	forge.InjectStandards(
		r.Context(), h.Queries,
		&resp.Agent.Instructions, &resp.Agent.Skills,
		task.WorkspaceID, h.taskProjectID(r.Context(), task.IssueID),
	)
}
```
并在文件 import 块加 `"github.com/multica-ai/multica/server/internal/forge"`。

- [ ] **Step 3: 编译验证**

Run: `cd <forge-repo>/server && go build ./... 2>&1 | tail -8`
Expected: 编译通过。若 `task.WorkspaceID`/`task.IssueID` 字段名不符，按 claim 处实际类型修正。

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/daemon.go
git commit -m "feat(forge): inject standards into agent payload at daemon task claim"
```

---

## Phase 2 完成检查
- [ ] InjectStandards 2 个测试全绿（注入 + 空降级）
- [ ] daemon claim 钩子接入，`go build ./...` 通过
- [ ] 侵入面 = daemon.go 一行调用 + 一个 helper + 一个 import
