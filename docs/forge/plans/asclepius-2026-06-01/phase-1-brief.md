## Phase 1 — brief auto_fix 分支（forgeentropy，TDD）

**Goal:** `ComposeBrief` 的 `BriefInput` 增 `AutoFix bool`;开启时插"修复指令段";`ResolveBrief` 透传 `scan.AutoFix`。
**Depends-on:** Phase 0（`ForgeEntropyScan.AutoFix` 字段）　**Unblocks:** Phase 4
**Completion gate:** `ComposeBrief` auto_fix 单测绿(开启含修复段 / 关闭逐字等于 F4);`go build ./...` 绿。

> 现有 `server/internal/forgeentropy/brief.go`（`ComposeBrief`/`BriefInput`/`FindingRef`）+ `resolve.go`（`ResolveBrief`）来自 F4。

---

### Task 1.1: ComposeBrief AutoFix 分支（TDD）

**Files:**
- Modify: `server/internal/forgeentropy/brief.go`
- Modify: `server/internal/forgeentropy/brief_test.go`

- [ ] **Step 1: 写失败测试**

在 `server/internal/forgeentropy/brief_test.go` 末尾追加：
```go
func TestComposeBrief_AutoFixOn(t *testing.T) {
	out := ComposeBrief(BriefInput{ScanName: "weekly", AutoFix: true})
	for _, want := range []string{
		"Fixing (this scan has auto-fix enabled)",
		"gh pr create",
		"Closes",
		"only file issues",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("auto-fix brief missing %q\n---\n%s", want, out)
		}
	}
}

func TestComposeBrief_AutoFixOff(t *testing.T) {
	out := ComposeBrief(BriefInput{ScanName: "weekly", AutoFix: false})
	if strings.Contains(out, "Fixing (this scan has auto-fix enabled)") || strings.Contains(out, "gh pr create") {
		t.Fatalf("auto-fix OFF brief must not contain fixing section\n---\n%s", out)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgeentropy/... 2>&1 | tail -8"`
Expected: 编译失败（`BriefInput` 无 `AutoFix` 字段）。

- [ ] **Step 3: 实现**

`server/internal/forgeentropy/brief.go`:`BriefInput` struct 加字段(放在 `OpenFindings` 之后):
```go
	OpenFindings  []FindingRef
	AutoFix       bool // when true, the brief instructs the agent to fix + open a PR
```

在 `ComposeBrief` 内,**`"\n## How to report\n"` 那一行之前**插入:
```go
	if in.AutoFix {
		b.WriteString("\n## Fixing (this scan has auto-fix enabled)\n")
		b.WriteString("For findings you can fix SAFELY and with high confidence:\n")
		b.WriteString("- make the change, commit to a new branch, and open a PR with `gh pr create`\n")
		b.WriteString("- put `Closes <the identifier of the issue you are working on>` in the PR body\n")
		b.WriteString("- report the PR URL in your task output\n")
		b.WriteString("For anything risky, ambiguous, or large: do NOT fix — file an issue instead (as below).\n")
		b.WriteString("If you lack git push / GitHub access here, skip fixing and only file issues.\n")
	}
```

- [ ] **Step 4: 运行确认通过(含 F4 既有测试)**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgeentropy/... 2>&1 | tail -8"`
Expected: 4 个测试全 PASS（F4 的 AllSections/OmitsEmptySections + 新 AutoFixOn/AutoFixOff）。
（`TestComposeBrief_OmitsEmptySections` 用默认 `AutoFix:false`,故修复段不出现——仍绿。）

- [ ] **Step 5: Commit**

```bash
git add server/internal/forgeentropy/brief.go server/internal/forgeentropy/brief_test.go
git commit -m "feat(forge): ComposeBrief auto_fix branch (fix + open PR instructions)"
```

---

### Task 1.2: ResolveBrief 透传 AutoFix

**Files:**
- Modify: `server/internal/forgeentropy/resolve.go`

- [ ] **Step 1: 透传**

在 `ResolveBrief` 内,`in := BriefInput{ScanName: scan.Name, CustomFocus: scan.CustomFocus}` 这一行改为:
```go
	in := BriefInput{ScanName: scan.Name, CustomFocus: scan.CustomFocus, AutoFix: scan.AutoFix}
```
（`scan` 是 `db.ForgeEntropyScan`,Phase 0 已给它生成 `AutoFix bool` 字段。）

- [ ] **Step 2: 编译 + 测试**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... && go test ./internal/forgeentropy/... 2>&1 | tail -5"`
Expected: build 通过;测试 PASS。

- [ ] **Step 3: Commit**

```bash
git add server/internal/forgeentropy/resolve.go
git commit -m "feat(forge): ResolveBrief passes scan.AutoFix into brief"
```

---

## Phase 1 完成检查
- [ ] `ComposeBrief` AutoFix on/off 两单测绿,F4 既有两测仍绿
- [ ] `ResolveBrief` 透传 `scan.AutoFix`，编译通过
