# Legacy Forge (pre-Multica)

旧 forge 架构（Go forge-core + Python ai-worker + Temporal + Next.js forge-portal）
已于 2026-05-30 归档到分支 `archive/forge-legacy`（origin 与 codeup 均有）。

本 `main` 自该日起以 [Multica](https://github.com/multica-ai/multica) 为新底座（fork/rebase），
Forge 作为其上的 Harness 工程层（规范 / 约束 / 验证 / 熵管理）。
详见 F0 设计 spec 与 atlas 计划（Phase 3 文档迁移后入 docs/）。

取回旧代码：

```bash
git checkout archive/forge-legacy
```

跟进 Multica 上游：

```bash
git fetch upstream && git merge upstream/main
```
