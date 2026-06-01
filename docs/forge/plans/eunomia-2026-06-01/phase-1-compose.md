## Phase 1 — 破环子包化 + 合成逻辑（forgeentropy，TDD）

**Goal:** 把 `checks.go` 迁出为 service-free 子包 `internal/forge/checks`（破 `service→forge→service` 环），
再建 `internal/forgeentropy` 包：纯 `ComposeBrief`（TDD）+ 服务端入口 `ResolveBrief`。
**Depends-on:** Phase 0　**Unblocks:** Phase 2
**Completion gate:** `checks` 迁移后唯一调用点改通、`go build ./...` 绿；`ComposeBrief` 单测绿。

> 破环背景见 spec §5：`internal/forge` 的 `inject.go` import 了 `service`，故顶层 `forge` 包 service-coupled；
> 把 `ResolveChecks` 挪到独立子包后，F4 的 `forgeentropy` 可零环导入 F1 `standards` + F2 `checks`。

---

### Task 1.1: `checks.go` → `internal/forge/checks` 子包

**Files:**
- Move: `server/internal/forge/checks.go` → `server/internal/forge/checks/resolve.go`
- Move: `server/internal/forge/checks_test.go` → `server/internal/forge/checks/resolve_test.go`
- Modify: `server/internal/handler/forge_daemon.go`（唯一调用点）

- [ ] **Step 1: 先查全部调用点（确认只有一处）**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && grep -rn 'forge\.Check\b\|forge\.ResolveChecks\|forge\.CheckQuerier' --include=*.go ."`
Expected: 仅 `internal/handler/forge_daemon.go`（`forge.Check`、`forge.ResolveChecks`）。若有其它，一并在 Step 4 改。

- [ ] **Step 2: 移动文件 + 改包名**

```bash
wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server/internal/forge && mkdir -p checks && git mv checks.go checks/resolve.go && git mv checks_test.go checks/resolve_test.go"
```
然后把这两个文件首行 `package forge` 改成 `package checks`：
- `server/internal/forge/checks/resolve.go`：`package forge` → `package checks`
- `server/internal/forge/checks/resolve_test.go`：`package forge` → `package checks`

（`checks.go` 自包含——只 import `context`/`fmt`/`pgtype`/`db`，无父包引用，迁移干净。）

- [ ] **Step 3: 改唯一调用点 `handler/forge_daemon.go`**

把 `forge.Check` → `checks.Check`、`forge.ResolveChecks` → `checks.ResolveChecks`，并把 import
`"github.com/multica-ai/multica/server/internal/forge"` 换成
`"github.com/multica-ai/multica/server/internal/forge/checks"`（若该文件不再引用 `forge.` 下其它符号，
删除旧 import；否则两个都留）。

具体改动：
```go
// import 块：
"github.com/multica-ai/multica/server/internal/forge/checks"

// 类型引用：
Checks []checks.Check `json:"checks"`

// 调用：
cs, err := checks.ResolveChecks(r.Context(), h.Queries, parseUUID(workspaceID), projID)
```

- [ ] **Step 4: 编译 + 跑迁移后的 checks 测试**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8 && go test ./internal/forge/checks/... 2>&1 | tail -5"`
Expected: build 通过；`internal/forge/checks` 测试（原 ResolveChecks 测试随迁）全绿。

- [ ] **Step 5: Commit**

```bash
git add -A server/internal/forge server/internal/handler/forge_daemon.go
git commit -m "refactor(forge): move checks into forge/checks subpackage (break service cycle)"
```

---

### Task 1.2: `forgeentropy` 包 — ComposeBrief（TDD）+ ResolveBrief

**Files:**
- Create: `server/internal/forgeentropy/brief.go`
- Create: `server/internal/forgeentropy/brief_test.go`
- Create: `server/internal/forgeentropy/resolve.go`

- [ ] **Step 1: 写失败测试（ComposeBrief）**

`server/internal/forgeentropy/brief_test.go`：
```go
package forgeentropy

import (
	"strings"
	"testing"
)

func TestComposeBrief_AllSections(t *testing.T) {
	out := ComposeBrief(BriefInput{
		ScanName:      "weekly",
		StandardsText: "Always write tests.",
		ChecksText:    "- lint: `make lint`",
		CustomFocus:   "Check for dead code.",
		OpenFindings:  []FindingRef{{Number: 12, Title: "TODO debt in auth"}},
	})
	for _, want := range []string{
		"# Entropy Scan: weekly",
		"declared standards (F1)",
		"Always write tests.",
		"verification checks (F2)",
		"make lint",
		"Additional focus areas",
		"Check for dead code.",
		"Already-tracked findings",
		"#12 TODO debt in auth",
		"label `forge-entropy`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("brief missing %q\n---\n%s", want, out)
		}
	}
}

func TestComposeBrief_OmitsEmptySections(t *testing.T) {
	out := ComposeBrief(BriefInput{ScanName: "minimal"})
	for _, absent := range []string{
		"declared standards (F1)",
		"verification checks (F2)",
		"Additional focus areas",
		"Already-tracked findings",
	} {
		if strings.Contains(out, absent) {
			t.Fatalf("brief should omit %q when empty\n---\n%s", absent, out)
		}
	}
	if !strings.Contains(out, "WHOLE-REPOSITORY") || !strings.Contains(out, "How to report") {
		t.Fatalf("brief missing always-on sections\n---\n%s", out)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgeentropy/... 2>&1 | tail -8"`
Expected: 编译失败（`ComposeBrief`/`BriefInput`/`FindingRef` 未定义）。

- [ ] **Step 3: 实现 ComposeBrief**

`server/internal/forgeentropy/brief.go`：
```go
// Package forgeentropy composes the scanner agent's brief for a Forge entropy
// scan: F1 standards + F2 checks + custom focus + an open-findings dedup list.
// Service-free (imports db + forge/standards + forge/checks) so the autopilot
// dispatch path can call it without an import cycle.
package forgeentropy

import (
	"fmt"
	"strings"
)

// FindingLabel marks scanner-filed finding issues so the dedup query can find them.
const FindingLabel = "forge-entropy"

// FindingRef is one already-tracked finding for the dedup list.
type FindingRef struct {
	Number int32
	Title  string
}

// BriefInput is the fully-resolved input to ComposeBrief (no I/O).
type BriefInput struct {
	ScanName      string
	StandardsText string // "" = omit section
	ChecksText    string // "" = omit section
	CustomFocus   string // "" = omit section
	OpenFindings  []FindingRef
}

// ComposeBrief builds the scanner agent's Markdown brief. Pure — unit-testable.
func ComposeBrief(in BriefInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Entropy Scan: %s\n\n", in.ScanName)
	b.WriteString("You are performing a periodic, WHOLE-REPOSITORY quality scan (not a diff review).\n")
	b.WriteString("Survey the entire codebase for accumulated quality entropy and FILE issues for findings.\n")
	b.WriteString("This is advisory — do NOT modify code in this task; only survey and report.\n")

	if in.StandardsText != "" {
		b.WriteString("\n## This project's declared standards (F1)\n")
		b.WriteString(in.StandardsText)
		b.WriteString("\n")
	}
	if in.ChecksText != "" {
		b.WriteString("\n## This project's verification checks (F2)\n")
		b.WriteString(in.ChecksText)
		b.WriteString("\n")
	}
	if in.CustomFocus != "" {
		b.WriteString("\n## Additional focus areas\n")
		b.WriteString(in.CustomFocus)
		b.WriteString("\n")
	}
	if len(in.OpenFindings) > 0 {
		b.WriteString("\n## Already-tracked findings — do NOT re-file these\n")
		for _, f := range in.OpenFindings {
			fmt.Fprintf(&b, "- #%d %s\n", f.Number, f.Title)
		}
		b.WriteString("For each item above that still exists, add a short comment confirming it persists.\n")
		b.WriteString("Only create NEW issues for findings NOT already listed.\n")
	}
	b.WriteString("\n## How to report\n")
	b.WriteString("For each NEW finding, create an issue via the `multica` CLI:\n")
	b.WriteString("- clear title, body with problem + location + suggested fix\n")
	fmt.Fprintf(&b, "- apply the label `%s`\n", FindingLabel)
	b.WriteString("When done, post a summary comment on THIS scan issue.\n")
	return b.String()
}
```

- [ ] **Step 4: 运行确认通过**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgeentropy/... 2>&1 | tail -8"`
Expected: 两个测试 PASS。

- [ ] **Step 5: 实现 ResolveBrief（服务端入口，best-effort）**

`server/internal/forgeentropy/resolve.go`：
```go
package forgeentropy

import (
	"context"
	"log/slog"
	"strings"

	"github.com/multica-ai/multica/server/internal/forge/checks"
	"github.com/multica-ai/multica/server/internal/forge/standards"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Querier aggregates the queries ResolveBrief needs. *db.Queries satisfies it.
type Querier interface {
	standards.Querier
	checks.CheckQuerier
	ListOpenEntropyFindings(ctx context.Context, arg db.ListOpenEntropyFindingsParams) ([]db.ListOpenEntropyFindingsRow, error)
}

// ResolveBrief resolves F1 standards + F2 checks + open findings for the scan's
// scope and composes the brief. Best-effort: any resolve/query error degrades
// that section to empty — never blocks dispatch.
func ResolveBrief(ctx context.Context, q Querier, scan db.ForgeEntropyScan) string {
	in := BriefInput{ScanName: scan.Name, CustomFocus: scan.CustomFocus}

	if scan.IncludeStandards {
		if res, err := standards.Resolve(ctx, q, scan.WorkspaceID, scan.ProjectID); err == nil {
			in.StandardsText = strings.TrimSpace(res.Core + "\n\n" + res.Detail)
		} else {
			slog.Warn("forge entropy: resolve standards failed", "error", err)
		}
	}
	if scan.IncludeChecks {
		if cs, err := checks.ResolveChecks(ctx, q, scan.WorkspaceID, scan.ProjectID); err == nil {
			in.ChecksText = formatChecks(cs)
		} else {
			slog.Warn("forge entropy: resolve checks failed", "error", err)
		}
	}
	if fs, err := q.ListOpenEntropyFindings(ctx, db.ListOpenEntropyFindingsParams{
		WorkspaceID: scan.WorkspaceID,
		ProjectID:   scan.ProjectID,
	}); err == nil {
		for _, f := range fs {
			in.OpenFindings = append(in.OpenFindings, FindingRef{Number: f.Number, Title: f.Title})
		}
	} else {
		slog.Warn("forge entropy: list open findings failed", "error", err)
	}
	return ComposeBrief(in)
}

func formatChecks(cs []checks.Check) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString("- ")
		b.WriteString(c.Name)
		b.WriteString(": `")
		b.WriteString(c.Command)
		b.WriteString("`\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
```

> `ResolveBrief` 由 Phase 2 钩子 + Phase 4 集成（手动触发 → 断言 issue.description 含合成段）端到端覆盖；
> 纯逻辑 `ComposeBrief` 由本 phase 单测覆盖（与 F3 `MaybeEnqueueReview` 编译+集成、`ShouldEnqueueReview` 单测同模式）。

- [ ] **Step 6: 编译 + vet + 测试**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... && go vet ./internal/forgeentropy/... && go test ./internal/forgeentropy/... 2>&1 | tail -5"`
Expected: build/vet 干净；测试 PASS。

- [ ] **Step 7: Commit**

```bash
git add server/internal/forgeentropy/
git commit -m "feat(forge): forgeentropy brief composition (F1+F2+custom+dedup)"
```

---

## Phase 1 完成检查
- [ ] `checks` 子包化，唯一调用点改通，`go build ./...` 绿，迁移后 checks 测试绿
- [ ] `ComposeBrief` 两个单测绿（全段 / 省略空段）
- [ ] `ResolveBrief` + `Querier` 接口编译通过（`*db.Queries` 满足）
