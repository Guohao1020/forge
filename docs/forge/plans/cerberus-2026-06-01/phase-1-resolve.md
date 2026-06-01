## Phase 1 — 解析逻辑（forge/checks，TDD）

**Goal:** `server/internal/forge/checks.go`：把 workspace + project checks 加法合并成
`[]Check{Name, Command}`。
**Depends-on:** Phase 0（`db.ForgeCheck`）　**Unblocks:** Phase 2
**Completion gate:** `go test ./internal/forge/...` 全绿。

> 设计：纯函数 `resolveChecks(ws, proj []db.ForgeCheck) []Check`（加法，非覆盖——与 F1 不同）；
> `ResolveChecks(ctx, q, wsID, projID)` 薄包装。复用 F1 的 `standards.Querier` 风格独立接口。

---

### Task 1.1: 类型 + 纯函数 + Resolve（先写测试）

**Files:**
- Create: `server/internal/forge/checks.go`
- Test: `server/internal/forge/checks_test.go`

- [ ] **Step 1: 写失败测试**

`server/internal/forge/checks_test.go`：
```go
package forge

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func chk(name, cmd string) db.ForgeCheck {
	return db.ForgeCheck{Name: name, Command: cmd, Enabled: true}
}

func TestResolveChecks_Additive(t *testing.T) {
	ws := []db.ForgeCheck{chk("build", "go build ./..."), chk("lint", "ruff check")}
	proj := []db.ForgeCheck{chk("test", "go test ./...")}
	got := resolveChecks(ws, proj)
	if len(got) != 3 {
		t.Fatalf("expected workspace+project additive = 3 checks; got %d", len(got))
	}
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if !names["build"] || !names["lint"] || !names["test"] {
		t.Fatalf("missing checks; got %+v", got)
	}
}

type fakeCheckQ struct{ ws, proj []db.ForgeCheck }

func (f fakeCheckQ) ListWorkspaceChecks(_ context.Context, _ pgtype.UUID) ([]db.ForgeCheck, error) {
	return f.ws, nil
}
func (f fakeCheckQ) ListProjectChecks(_ context.Context, _ pgtype.UUID) ([]db.ForgeCheck, error) {
	return f.proj, nil
}

var _ CheckQuerier = fakeCheckQ{}

func TestResolveChecks_NoProject(t *testing.T) {
	q := fakeCheckQ{ws: []db.ForgeCheck{chk("build", "go build ./...")}}
	got, err := ResolveChecks(context.Background(), q, pgtype.UUID{Valid: true}, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Command != "go build ./..." {
		t.Fatalf("no-project resolve should yield workspace checks only; got %+v", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/ 2>&1 | tail -8"`
Expected: 编译失败（`resolveChecks` / `CheckQuerier` / `ResolveChecks` undefined）。

- [ ] **Step 3: 写实现**

`server/internal/forge/checks.go`：
```go
package forge

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Check is a resolved verification check the daemon runs in the task workdir.
type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// resolveChecks merges workspace + project checks additively (both run).
func resolveChecks(ws, proj []db.ForgeCheck) []Check {
	out := make([]Check, 0, len(ws)+len(proj))
	for _, c := range ws {
		out = append(out, Check{Name: c.Name, Command: c.Command})
	}
	for _, c := range proj {
		out = append(out, Check{Name: c.Name, Command: c.Command})
	}
	return out
}

// CheckQuerier is the subset of generated db methods ResolveChecks needs.
// *db.Queries satisfies it.
type CheckQuerier interface {
	ListWorkspaceChecks(ctx context.Context, workspaceID pgtype.UUID) ([]db.ForgeCheck, error)
	ListProjectChecks(ctx context.Context, projectID pgtype.UUID) ([]db.ForgeCheck, error)
}

// ResolveChecks loads workspace + project checks for a task and merges them.
// projectID may be the zero UUID (no project) — then only workspace checks apply.
func ResolveChecks(ctx context.Context, q CheckQuerier, workspaceID, projectID pgtype.UUID) ([]Check, error) {
	ws, err := q.ListWorkspaceChecks(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace checks: %w", err)
	}
	var proj []db.ForgeCheck
	if projectID.Valid {
		proj, err = q.ListProjectChecks(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("list project checks: %w", err)
		}
	}
	return resolveChecks(ws, proj), nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/ -run Checks -v 2>&1 | tail -12"`
Expected: 2 个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add server/internal/forge/checks.go server/internal/forge/checks_test.go
git commit -m "feat(forge): ResolveChecks — additive workspace+project merge"
```

---

## Phase 1 完成检查
- [ ] `resolveChecks` 加法合并 + `ResolveChecks` 包装，2 单测绿
- [ ] `*db.Queries` 满足 `CheckQuerier`
