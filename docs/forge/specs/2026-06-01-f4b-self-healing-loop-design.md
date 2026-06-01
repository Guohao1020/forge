# F4b 自愈闭环 · 设计（asclepius）

> Forge-on-Multica 切片。在 F4 熵扫描之上,让 scanner agent 能安全修的直接修 + 开 PR,
> 复用已就绪的 F2 验证门禁 + F3 AI Review;后端把 agent 开的 PR 录入并 link 到 issue。建议性。

**Status:** Approved（2026-06-01 brainstorming）
**Plan:** `docs/forge/plans/asclepius-2026-06-01/`（待 writing-plans 产出）。

---

## 1. 决策链（brainstorming 2026-06-01）

| # | 决策 | 选择 |
|---|------|------|
| Q1 | 修复粒度/由谁修 | **同一 scanner 扫描时顺手修,开一个 PR**(能修的改+开 PR,修不了的建 issue) |
| Q2 | PR 录入机制 | **后端显式录入**(解析 agent 返回的 PRURL → Forge sidecar link,**不**碰 GitHub-App 耦合的 `github_pull_request`) |
| Q3 | 门禁/评审后处置 | **建议性,人工合并**(无自动合并) |

**核心洞察**(F4b 探明):
1. **F2 门禁 + F3 评审已对熵扫描任务自动触发** —— scanner 任务 issue-bound + 有 workdir,F2
   在 daemon(`result.Status=="completed" && task.IssueID!=""`)、F3 在 service
   (`ShouldEnqueueReview(issueValid, workDir, ctx)`)均命中。**F4b 不写一行 F2/F3 代码。**
2. **PR 是真正的缺口**:Multica 不自动建 PR;agent 可在 execenv 用 `gh` 自开 PR,task result 带回
   `pr_url`,但**后端存进 result JSON 后就不再碰**(dead-end)。
3. **`github_pull_request` 是 GitHub-App 强耦合**(`installation_id BIGINT NOT NULL` + `title`/
   `head_sha`/`pr_created_at` 等 NOT NULL,只能由 webhook/API 填)。故"自包含录入"必须落在 Forge
   自己的 sidecar,而非 `github_pull_request`。

---

## 2. 目标 / 非目标

**目标**
- 给熵扫描加 **per-scan `auto_fix` 开关**:开启则 scanner 能安全修的直接改 + 开 PR + 修不了的建 issue;
  关闭则维持 F4 纯建议行为。
- 后端把 agent 开的 PR 录入 Forge sidecar `forge_fix_pr` 并 link 到该 issue + 发系统评论(自包含,
  不依赖 GitHub App)。
- 复用已就绪的 F2 门禁 + F3 评审(零新增)。
- 建议性:PR 录入后等人工合并。
- Forge 隔离(R2):`forge_` 前缀、最小 Multica 侵入(CompleteTask 一处一行钩子)。

**非目标(本切片不做)**
- auto-merge / 绿灯自动合并。
- 给 F3 评审加 approve/reject 裁决信号(当前 F3 是建议性评论)。
- close-on-merge 推进 issue 的**自包含**实现(走 GitHub App webhook 若装,否则人工)。
- 修复 PR 的专门 UI 面板(issue 上的系统评论即够)。
- 重建/替代 `github_pull_request` 的 GitHub App 镜像(那是 webhook 的地盘;agent PR body 写
  `Closes MUL-N`,若装了 App 则 webhook **额外**自动 enrich+close,是免费 bonus,F4b 不依赖)。

---

## 3. 当前事实(设计依据,不改)

- **F2 门禁**:`server/internal/daemon/daemon.go`(~2219)`if result.Status=="completed" && task.IssueID!=""`
  → `runForgeChecks(workDir)`;任一 check 失败 → task 转 `blocked` + `verification_failed` + 评论。
- **F3 评审**:`server/internal/service/task.go:1091` CompleteTask 内 `s.MaybeEnqueueReview(ctx, task)`;
  `forgereview.ShouldEnqueueReview(issueValid, workDir, ctx)` = issue-bound + 有 workdir + 非 review 任务。
- **scanner 任务**:F4 `dispatchCreateIssue` → `EnqueueTaskForIssue`,issue-bound + claim 时分配 workdir。
  完成时 F2 + F3 自动跑在 fix diff 上。
- **PR dead-end**:`handler/daemon.go:1781` `TaskCompleteRequest{ PRURL string \`json:"pr_url"\` }`;塞进
  `result` 后,`service.CompleteTask(ctx, taskID, result []byte, sessionID, workDir)` 不解析 `pr_url`。
- **系统评论**:`service/task.go:1924` `createAgentComment(ctx, issueID, agentID, content, commentType, parentID)`。
- **brief 合成**:`forgeentropy.ComposeBrief` / `ResolveBrief` 在 `dispatchCreateIssue` 时把 brief 写进
  issue.description。**brief 合成时 issue 尚未建**(无 identifier)→ brief 用通用措辞指示 agent 用
  "它正在做的这个 issue 的 identifier" 写 `Closes`。

---

## 4. 数据模型(迁移 115)

### 4.1 `forge_entropy_scan` 加列
```sql
ALTER TABLE forge_entropy_scan ADD COLUMN auto_fix BOOLEAN NOT NULL DEFAULT FALSE;
```
down:`ALTER TABLE forge_entropy_scan DROP COLUMN auto_fix;`

### 4.2 新 sidecar `forge_fix_pr`
```sql
CREATE TABLE forge_fix_pr (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id     UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    task_id      UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    pr_url       TEXT NOT NULL,
    branch       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_forge_fix_pr_issue ON forge_fix_pr(issue_id);
CREATE UNIQUE INDEX idx_forge_fix_pr_task_url ON forge_fix_pr(task_id, pr_url);
```
唯一索引 `(task_id, pr_url)` 防同一任务重复完成时重复录入(幂等)。

### 4.3 sqlc 查询 `server/pkg/db/queries/forge_fix_pr.sql`
- `CreateFixPR`(INSERT ... ON CONFLICT (task_id, pr_url) DO NOTHING RETURNING *)—— 幂等。
- `ListFixPRsByIssue`(可选,供未来 UI / 可观测)。
- 注:`auto_fix` 列并入 F4 已有的 `CreateEntropyScan`/`UpdateEntropyScan`/`ListEntropyScans` —— 改
  `forge_entropy.sql` 这三个 query 带上 `auto_fix`,重生成。

---

## 5. brief 改造(`forgeentropy`)

`ComposeBrief` 的 `BriefInput` 增 `AutoFix bool`;`ResolveBrief` 把 `scan.AutoFix` 透传。
`AutoFix=true` 时,在 "How to report" 段**之前**插入**修复指令段**(大意):

```
## Fixing (this scan has auto-fix enabled)
For findings you can fix SAFELY and with high confidence:
- make the change, commit to a new branch, and open a PR with `gh pr create`
- put `Closes <the identifier of the issue you are working on>` in the PR body
- report the PR URL in your task output
For anything risky, ambiguous, or large: do NOT fix — file an issue instead (as below).
If you lack git push / GitHub access in this environment, skip fixing and only file issues.
```

`AutoFix=false` 时 brief 与 F4 **逐字一致**(纯建议)。修复指令段为 best-effort 措辞,显式给出
"无凭证则退化为只建 issue" 的 fallback。

---

## 6. PR 桥(后端,填 dead-end)

新文件 `server/internal/service/forge_fix_pr.go`:
```
func (s *TaskService) MaybeRecordFixPR(ctx, task db.AgentTaskQueue, result []byte)
```
- 从 `result` JSON 解析 `pr_url`(最小 struct `{ PrURL string \`json:"pr_url"\` }`)+ `branch_name`(若有)。
- 门禁:`task.IssueID.Valid && prURL != ""`。否则直接返回。
- `CreateFixPR{WorkspaceID, IssueID, TaskID:task.ID, PrURL, Branch}`(幂等 ON CONFLICT DO NOTHING)。
- 成功插入(非冲突)→ `createAgentComment(ctx, task.IssueID, task.AgentID, "🔧 Fix PR opened: <url>", ...)`。
- best-effort:任一步失败记 `slog.Warn`,**绝不阻断 CompleteTask**。

**接线**:`service/task.go` CompleteTask 内,紧挨 `s.MaybeEnqueueReview(ctx, task)` 加一行
`s.MaybeRecordFixPR(ctx, task, result)`。一处一行,延续 F3 钩子风格。

> 该桥**不分熵专属**——任何 issue-bound 任务返回 `pr_url` 都会被录入 link。这是填一个通用 dead-end,
> 比加"是否熵任务"判断更简单、更有价值。Forge 侧逻辑仍在 `forge_` 前缀文件/表内。

---

## 7. F2 / F3 复用(零新增代码)
scanner 修复任务 issue-bound + 有 workdir → F2 门禁(daemon)+ F3 评审(service)**自动**跑在 fix diff 上:
- F2 任一 check 失败 → 任务 `blocked` + `verification_failed` 评论(现有)。
- F3 → reviewer agent 评审该 diff 发建议性评论(现有)。
F4b 不碰 F2/F3 任何代码,只是让 scanner 任务产出可被它们消费的 fix diff。

---

## 8. 处置(建议性)
PR 录入 + link + 评论后**等人工合并**。无自动合并。F2 的 blocked 状态 + F3 的评审评论 = 人决策依据。
PR 真正合并 / issue 关闭走 GitHub App webhook(若装,凭 `Closes MUL-N`)或人工 —— 不在 F4b 自包含范围。

---

## 9. API + UI
- `ForgeEntropyScanBody` / `ForgeEntropyScanResponse` + `ForgeEntropyScan(Input)` TS 类型加 `auto_fix bool`;
  handler 的 Create/Update 透传;`entropyScanToResponse` 带出。
- view `forge-entropy-page.tsx` 加一个 checkbox:「Let the agent fix what it safely can and open a PR」。
- `forge_fix_pr` 只读,无专门 UI(issue 系统评论即可见)。

---

## 10. 错误处理(汇总)
- PR 桥 best-effort:解析/插入/评论失败 → 记日志,不阻断 CompleteTask。
- 幂等:`(task_id, pr_url)` 唯一索引 + ON CONFLICT DO NOTHING,重复完成不重复录入/评论。
- agent 无 GitHub push auth → 开不了 PR → brief 指示退化为只建 issue。
- `auto_fix=false` → brief 退回 F4 纯建议。

---

## 11. 测试 / 验收(凭证现实)

> **双重凭证门**:活体修复需 ① provider 凭证(agent 真跑)② execenv GitHub push auth(agent 开 PR)。
> 但 **brief 合成 + PR 桥 + F2/F3 触发条件全可绕凭证验**(后端逻辑)。

- **纯单测**
  - `ComposeBrief` auto_fix 分支:`AutoFix=true` → 含 "Fixing" / `gh pr create` / `Closes` 段;
    `AutoFix=false` → 不含、且与 F4 输出一致。
  - PR 桥纯逻辑:解析 `pr_url` + 门禁(无 issue / 空 url → 不录入)。
- **绕凭证集成(源码构建栈)**
  1. 建 `auto_fix=true` 的 scan → 断言 `forge_entropy_scan.auto_fix=true` 往返。
  2. **直接给 `CompleteTask` 喂一个带 `pr_url` 的完成请求**(经 daemon 完成端点,或构造任务后调完成)
     → 断言 `forge_fix_pr` 行(issue_id/task_id/pr_url)+ issue 上出现 `🔧 Fix PR opened` 系统评论。
     **全后端,无需 agent 真开 PR。**
  3. 幂等:同一任务重复完成 → `forge_fix_pr` 仍一行、评论不重复。
- **活体(延后)**:auto_fix scanner agent 真扫 + 真修 + `gh` 开 PR → F2 门禁 + F3 评审跑在 diff 上 →
  PR 录入 link。标注双重凭证依赖。

---

## 12. 范围 / 拆分
单切片,体量约等于 F4:一列 `auto_fix` + 一 sidecar `forge_fix_pr` + brief 一个分支 + `forgeentropy`
PR 桥 + CompleteTask 一处钩子 + API/UI 一个开关 + 绕凭证验收。

## 13. 后续
F5 可观测闭环(熵趋势 / 修复率 / PR 合并率)。auto-merge / F3 approve 裁决 / close-on-merge 自包含 后置。
