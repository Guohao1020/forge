# Hygieia — Harness 健康总分实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 F5 `GET /api/forge/health` 上加一个 **0–100 Harness 健康分 + 绿/黄/红状态**,
后端纯函数从已有聚合计数算出(覆盖感知混合),前端顶部渲染健康分 badge。

**Architecture:** 新 service-free 纯包 `server/internal/forgehealth`(`Score(ScoreInput) ScoreResult`,TDD)+
`GetForgeHealth` 响应加 `score`/`status`/`no_activity` 三字段 + 前端 `forge-health` 顶部 badge。**无迁移、零凭证依赖**。

**Tech Stack:** Go 1.26 · Next.js 16 · zod · Multica monorepo。

**Source spec:** [`docs/forge/specs/2026-06-01-harness-health-score-design.md`](../../specs/2026-06-01-harness-health-score-design.md)

> 评分(覆盖感知混合):`coverage = 已配层(F1/F2/F3/F4)/4`;质量子项 `gatePass`/`reviewDone`/`entropyControl`
> 各 0–1、分母为 0 则**排除**;`score = 100×(0.4·cov + 0.6·mean(可用质量))`,无质量数据则 `100·cov` + `no_activity`;
> 阈值 绿≥80 / 黄≥50 / 红。状态色用既有 status-semantic 色(chat-window `bg-amber-500` 状态点 / billing 同款),非装饰色。

---

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | 后端 Score 纯函数(TDD)+ handler 接线 + 响应字段 | — | ☐ | [phase-0-score.md](phase-0-score.md) |
| 1 | 前端 types/zod/badge | Phase 0 | ☐ | [phase-1-frontend.md](phase-1-frontend.md) |
| 2 | 验收 + 文档 | Phase 0, 1 | ☐ | [phase-2-verify.md](phase-2-verify.md) |

## 完成门禁（DoD）
- [ ] `forgehealth.Score` 纯函数 + 表驱动单测(全配满质量→100绿 / 配了没跑→纯覆盖+no_activity / 门禁全挂→40红 / 零配置→0红 / live→80绿)
- [ ] `GetForgeHealth` 响应加 `score`/`status`/`no_activity`,`go build`+vet 绿
- [ ] 前端 types/zod/EMPTY 加三字段 + 顶部健康分 badge,三包 typecheck 绿
- [ ] **绕凭证集成**:源码构建栈 `GET /api/forge/health` 返回 `score`/`status` 与 DB 实际计数手算一致(当前真实数据 → score≈80 green)

## 后续
阈值告警 / 历史分数趋势 / 可配置权重。
