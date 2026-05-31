## Phase 5 — Decommission & Final Acceptance

**Goal:** 退役旧架构运行态、更新项目记忆、逐条勾 F0 DoD 完成验收。

**Depends-on:** Phase 2（验收闭环跑通）、Phase 4（身份层完成）
**Unblocks:** F1（下一切片）
**Completion gate:** 旧栈不再运行；记忆更新到位；spec §2 全部 DoD 勾完并有证据；最终 push。

---

### Task 5.1: 退役旧架构运行态

> 新 main 是纯 Multica 树，已无旧服务代码（在 archive 分支）。本 task 确保本机上
> 旧的运行态（容器/进程）停掉，且端口不与 Multica 冲突。

- [ ] **Step 1: 停掉旧 forge 的 dev 基础设施**

旧 `docker-compose.dev.yml` 已随旧代码进 `archive/forge-legacy`，新 main 不再有它。
本步只需停掉本机上可能仍在跑的旧容器：

Run:
```bash
docker ps --format '{{.Names}}\t{{.Ports}}' | grep -iE 'temporal|forge|code-server' || echo "no legacy containers running"
```
若有输出，逐个停掉：
```bash
docker stop <legacy-container-name>
```
Expected: 残留的 temporal/forge/code-server 容器全部停止；确认 5432/8080 仅被 Multica 栈使用。

- [ ] **Step 2: 确认新 main 无旧服务引用**

Run:
```bash
cd <forge-repo-path>
git ls-files | grep -E 'ai-worker|forge-core|constraint-worker|devops-worker|forge-bot|forge-portal|temporal' || echo "clean — no legacy code in new main"
```
Expected: 输出 `clean` —— 新 main 不含旧服务代码与 Temporal。

---

### Task 5.2: 更新项目记忆

**Files:**
- Modify: `C:\Users\86157\.claude\projects\D--shulex-work-forge\memory\MEMORY.md`
- Create: `…\memory\forge-on-multica-pivot.md`
- Modify: `…\memory\git-remotes.md`、`…\memory\agent-architecture-a2.md`

- [ ] **Step 1: 记录 Multica-base pivot**

新建 `forge-on-multica-pivot.md`（type: project）：2026-05-30 起 Forge 以 Multica 为底座
（fork/rebase），CLI 驱动模型取代自建 ai-worker loop + Temporal；Harness 概念映射到 Multica
原语（规范→Skills、熵管理→Autopilot、验证门禁→完成 hook、Review→Squad）。链接 [[git-remotes]]、
[[agent-architecture-a2]]。

- [ ] **Step 2: 更新既有记忆**

- `git-remotes.md`：补 `upstream`=multica-ai/multica、`archive/forge-legacy` 分支。
- `agent-architecture-a2.md`：补一行 —— A2 自建 loop 已于 2026-05-30 被 Multica CLI 驱动取代。
- `MEMORY.md`：加一行索引指向 `forge-on-multica-pivot.md`。

- [ ] **Step 3: 无需提交**（记忆在 `~/.claude`，不入仓库）

---

### Task 5.3: 最终 F0 验收

- [ ] **Step 1: 逐条核对 spec §2 DoD**

对照 spec/index 的 DoD 清单逐条确认：
- [ ] 仓库：main=Multica base、archive 保留、upstream 配好、已 push（Phase 1）
- [ ] 本地 e2e 闭环跑通 + 证据（Phase 2）
- [ ] 身份：README/CLAUDE.md/NOTICE/共标（Phase 4）
- [ ] 退役：无旧服务运行/无旧代码引用（Phase 5.1）
- [ ] runbook：dev-windows.md（Phase 4.5）

- [ ] **Step 2: 把验收结论写入 index 完成状态**

更新 `<DOCDIR>/plans/atlas-2026-05-30/index.md` 的 Phase 表状态为 ✅，DoD 勾完。

- [ ] **Step 3: 最终提交 + push**

Run:
```bash
cd <forge-repo-path>
git add <DOCDIR>/plans/atlas-2026-05-30/index.md
git commit -m "docs(plan): mark atlas F0 foundation complete"
git push origin main
```

---

### Task 5.4: 清理临时备份

- [ ] **Step 1: 确认规划文档已在新 main 后删除仓库外备份**

Run (PowerShell):
```powershell
Test-Path D:\shulex_work\forge\<DOCDIR>\plans\atlas-2026-05-30\index.md
# 确认为 True 后再删备份
Remove-Item -Recurse -Force D:\shulex_work\_atlas_planning_backup
```
Expected: 计划已在新 main，备份可安全删除。`_atlas_notes.md` 内容已转 runbook，可一并清理或留存。

---

## Phase 5 完成检查

- [ ] 旧架构运行态退役，端口无冲突，新 main 无旧代码引用
- [ ] 项目记忆更新（pivot / remotes / a2）
- [ ] F0 DoD 全部勾完且有证据
- [ ] index 状态更新为完成，最终 push
- [ ] 仓库外临时备份清理
