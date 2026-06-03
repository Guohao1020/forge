# Forge docs

本目录是 Forge 的设计 / 规范 / 计划文档。2026-05-30 起 Forge 以
[Multica](https://github.com/multica-ai/multica) 为新底座（fork/rebase），Forge 作为其上的
**Harness 工程层**（规范 / 约束 / 验证 / 熵管理）。

> 顶层 `../docs/` 与 `../apps/docs/`（Fumadocs 站）属于 Multica 上游，本 `docs/forge/`
> 是 Forge 自己的文档树。

## ✅ Current（Multica 时代）

- **[specs/2026-05-30-forge-on-multica-f0-foundation-design.md](specs/2026-05-30-forge-on-multica-f0-foundation-design.md)**
  —— 当前方向的设计 spec：以 Multica 为底座的 F0 基座切片。
- **[plans/atlas-2026-05-30/](plans/atlas-2026-05-30/index.md)** —— F0 实施计划（6 phase）。
- **[references/coding-standards.md](references/coding-standards.md)** —— 编码规范（F1 规范中心的种子）。
- **[references/2026-06-03-live-execution-runbook.md](references/2026-06-03-live-execution-runbook.md)**
  —— 活体执行 Runbook：真实 provider 凭证接入（路由器 / Claude / Codex）+ 真实 gate→review 闭环搭法 +
  关键坑根因修复（stale daemon、OAuth 遮蔽、Codex Windows elevated UAC）。
- **PRD.md / product-design.md / DESIGN.md** —— 产品需求与设计（多数仍适用，但需按 Multica 底座复核）。
- **harness-data-flow.mmd / architecture.mmd** —— Harness 概念图（部分概念沿用，技术栈以 Multica 为准）。

## ⚠️ SUPERSEDED（pre-Multica，旧 Go+Python+Temporal+forge-portal 架构）

以下文档基于旧自建架构，**仅作需求 / 设计 / 历史参考，不要据此执行**。对应代码在
`archive/forge-legacy` 分支：

- `technical-design.md`、`milestone-plan.md`、`api-reference.md` —— 旧架构技术设计 / 里程碑 / API。
- `old-plan/M0–M6*` —— 旧里程碑计划。
- `plans/SH-*`、`plans/SP-*`、`plans/SX-*`、`plans/S1`–`S17`（在 plans/ 中）、`plans/chronos-*`、
  `plans/OH-*`、`plans/*pair-pipeline*`、`plans/harness-engineering-design.md`、
  `plans/infra-architecture-design.md`、`plans/phase2-*`、`plans/phase3-*`、`plans/session-*`
  —— 旧 ai-worker / pair_pipeline / chronos / 自建 agent loop 时代的计划与设计。

> 这些被 [F0 spec](specs/2026-05-30-forge-on-multica-f0-foundation-design.md) 取代。
> Harness 概念（规范中心、熵管理、验证门禁、AI Review）将以 Multica 原语
> （Skills / Autopilot / 完成门禁 / Squad）重新实现，见 F0 spec §0.2。
