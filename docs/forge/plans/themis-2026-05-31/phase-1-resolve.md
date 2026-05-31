## Phase 1 — 解析逻辑（forge/standards，TDD）

**Goal:** `server/internal/forge/standards` 包：把 workspace + project standards 按
(category,name) 覆盖、按 project profile 过滤、拆成 core（instructions）+ detail（skill）。

**Depends-on:** Phase 0（sqlc 类型 `db.ForgeStandard`）　**Unblocks:** Phase 2
**Completion gate:** `go test ./internal/forge/standards/...` 全绿。

> 设计：纯函数 `resolveStandards(ws, proj []db.ForgeStandard, projTags []string) Resolved`
> 便于单测；`Resolve(ctx, q, wsID, projID)` 薄包装走 DB。`Resolved{Core, Detail string}`。

---

### Task 1.1: 类型 + 纯解析函数（先写测试）

**Files:**
- Create: `server/internal/forge/standards/resolve.go`
- Test: `server/internal/forge/standards/resolve_test.go`

- [ ] **Step 1: 写失败测试**

`server/internal/forge/standards/resolve_test.go`：
```go
package standards

import (
	"strings"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func std(category, name string, tags []string, core, detail string) db.ForgeStandard {
	return db.ForgeStandard{Category: category, Name: name, ProfileTags: tags, CoreContent: core, DetailContent: detail, Enabled: true}
}

func TestResolve_ProjectOverridesWorkspace(t *testing.T) {
	ws := []db.ForgeStandard{std("naming", "go", nil, "ws-core", "ws-detail")}
	proj := []db.ForgeStandard{std("naming", "go", nil, "proj-core", "proj-detail")}
	got := resolveStandards(ws, proj, nil)
	if !strings.Contains(got.Core, "proj-core") || strings.Contains(got.Core, "ws-core") {
		t.Fatalf("project core should override workspace; got %q", got.Core)
	}
}

func TestResolve_ProfileFilter(t *testing.T) {
	ws := []db.ForgeStandard{
		std("lang", "go", []string{"go"}, "go-core", "go-detail"),
		std("lang", "java", []string{"java"}, "java-core", "java-detail"),
		std("general", "naming", nil, "naming-core", "naming-detail"), // empty tags = always
	}
	got := resolveStandards(ws, nil, []string{"go"})
	if !strings.Contains(got.Core, "go-core") || !strings.Contains(got.Core, "naming-core") {
		t.Fatalf("go + empty-tag standards should apply; got %q", got.Core)
	}
	if strings.Contains(got.Core, "java-core") {
		t.Fatalf("java standard should be filtered out for go project; got %q", got.Core)
	}
}

func TestResolve_CoreDetailSplit(t *testing.T) {
	ws := []db.ForgeStandard{std("api", "rest", nil, "CORE-RULES", "DETAILED-GUIDANCE")}
	got := resolveStandards(ws, nil, nil)
	if !strings.Contains(got.Core, "CORE-RULES") {
		t.Fatalf("core missing core_content; got %q", got.Core)
	}
	if !strings.Contains(got.Detail, "DETAILED-GUIDANCE") || !strings.Contains(got.Detail, "name: forge-standards") {
		t.Fatalf("detail skill must contain detail_content + frontmatter; got %q", got.Detail)
	}
}

func TestResolve_EmptyDowngrade(t *testing.T) {
	got := resolveStandards(nil, nil, nil)
	if got.Core != "" || got.Detail != "" {
		t.Fatalf("empty standards must yield empty Resolved; got %+v", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd <forge-repo>/server && go test ./internal/forge/standards/... 2>&1 | tail -8`
Expected: 编译失败 / FAIL（`resolveStandards` undefined）。

- [ ] **Step 3: 写实现**

`server/internal/forge/standards/resolve.go`：
```go
// Package standards implements Forge's spec-center: resolving categorized,
// scoped, profile-filtered coding standards into a two-layer payload —
// Core (mandatory, appended to agent instructions) and Detail (compiled into
// an on-demand forge-standards skill). Forge-owned; isolated from Multica core.
package standards

import (
	"fmt"
	"sort"
	"strings"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Resolved is the two-layer injection payload.
type Resolved struct {
	Core   string // markdown appended to agent instructions (mandatory, always-on)
	Detail string // SKILL.md content for the forge-standards skill; "" if none
}

// SkillName is the fixed name of the auto-generated standards skill.
const SkillName = "forge-standards"

// resolveStandards is the pure core: override workspace standards with project
// standards by (category,name), filter by profile tags, then split into
// Core/Detail. No I/O — fully unit-testable.
func resolveStandards(ws, proj []db.ForgeStandard, projTags []string) Resolved {
	type key struct{ cat, name string }
	merged := map[key]db.ForgeStandard{}
	for _, s := range ws {
		merged[key{s.Category, s.Name}] = s
	}
	for _, s := range proj { // project overrides workspace
		merged[key{s.Category, s.Name}] = s
	}

	tagSet := map[string]bool{}
	for _, t := range projTags {
		tagSet[t] = true
	}
	applies := func(s db.ForgeStandard) bool {
		if len(s.ProfileTags) == 0 {
			return true // empty = applies to all
		}
		for _, t := range s.ProfileTags {
			if tagSet[t] {
				return true
			}
		}
		return false
	}

	var kept []db.ForgeStandard
	for _, s := range merged {
		if applies(s) {
			kept = append(kept, s)
		}
	}
	// Deterministic order: category, then name.
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Category != kept[j].Category {
			return kept[i].Category < kept[j].Category
		}
		return kept[i].Name < kept[j].Name
	})

	var core, detail strings.Builder
	for _, s := range kept {
		if c := strings.TrimSpace(s.CoreContent); c != "" {
			fmt.Fprintf(&core, "### [%s] %s\n%s\n\n", s.Category, s.Name, c)
		}
		if d := strings.TrimSpace(s.DetailContent); d != "" {
			fmt.Fprintf(&detail, "## [%s] %s\n%s\n\n", s.Category, s.Name, d)
		}
	}

	res := Resolved{Core: strings.TrimSpace(core.String())}
	if detail.Len() > 0 {
		res.Detail = fmt.Sprintf("---\nname: %s\ndescription: Project coding standards resolved by Forge.\n---\n\n# Coding Standards\n\n%s",
			SkillName, strings.TrimSpace(detail.String()))
	}
	return res
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd <forge-repo>/server && go test ./internal/forge/standards/... -v 2>&1 | tail -15`
Expected: 4 个测试全 PASS。

- [ ] **Step 5: Commit**

```bash
git add server/internal/forge/standards/resolve.go server/internal/forge/standards/resolve_test.go
git commit -m "feat(forge): standards resolve — override + profile filter + core/detail split"
```

---

### Task 1.2: DB 包装 Resolve()

**Files:**
- Modify: `server/internal/forge/standards/resolve.go`（追加 Querier 接口 + Resolve）

- [ ] **Step 1: 追加接口与包装函数**

把文件顶部 import 块补上 `"context"` 和 `"github.com/jackc/pgx/v5/pgtype"`，然后在末尾追加：
```go
// Querier is the subset of generated db methods Resolve needs.
// *db.Queries (sqlc-generated) satisfies this interface.
type Querier interface {
	ListWorkspaceStandards(ctx context.Context, workspaceID pgtype.UUID) ([]db.ForgeStandard, error)
	ListProjectStandards(ctx context.Context, projectID pgtype.UUID) ([]db.ForgeStandard, error)
	GetForgeProjectProfile(ctx context.Context, projectID pgtype.UUID) (db.ForgeProjectProfile, error)
}
```

```go
// Resolve loads standards for (workspaceID, projectID) and returns the two-layer
// payload. projectID may be the zero UUID (no project) — then only workspace
// standards apply and no profile filter is performed.
func Resolve(ctx context.Context, q Querier, workspaceID, projectID pgtype.UUID) (Resolved, error) {
	ws, err := q.ListWorkspaceStandards(ctx, workspaceID)
	if err != nil {
		return Resolved{}, fmt.Errorf("list workspace standards: %w", err)
	}
	var proj []db.ForgeStandard
	var tags []string
	if projectID.Valid {
		proj, err = q.ListProjectStandards(ctx, projectID)
		if err != nil {
			return Resolved{}, fmt.Errorf("list project standards: %w", err)
		}
		if prof, perr := q.GetForgeProjectProfile(ctx, projectID); perr == nil {
			tags = prof.Tags
		} // no profile row → tags nil → only empty-tag standards apply
	}
	return resolveStandards(ws, proj, tags), nil
}
```

- [ ] **Step 2: 编译验证**

Run: `cd <forge-repo>/server && go build ./internal/forge/... 2>&1 | tail -5 && go test ./internal/forge/standards/... 2>&1 | tail -3`
Expected: 编译通过；纯函数测试仍全绿。

- [ ] **Step 3: Commit**

```bash
git add server/internal/forge/standards/resolve.go
git commit -m "feat(forge): Resolve() DB wrapper over generated queries"
```

---

## Phase 1 完成检查
- [ ] `resolveStandards` 纯函数 4 个单测全绿（覆盖/过滤/拆分/降级）
- [ ] `Resolve()` 包装编译通过，`db.Queries` 满足 `Querier`
