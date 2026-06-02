# Harness 健康总分 · 设计（hygieia）

> F5 健康面板的收口指标。在已有 `GET /api/forge/health` 上加一个 **0–100 Harness 健康分 + 绿/黄/红状态**,
> 后端纯函数从 F5 已采集的聚合计数算出。**无迁移、零凭证依赖、纯函数可单测**。

**Status:** Approved（2026-06-01 brainstorming）
**Plan:** `docs/forge/plans/hygieia-2026-06-01/`（待 writing-plans 产出）。

---

## 1. 决策链（brainstorming 2026-06-01）

| # | 决策 | 选择 |
|---|------|------|
| 评分哲学 | 一个 0–100 衡量什么 | **覆盖感知混合**(coverage + 有数据时的 quality) |
| 覆盖层 | 哪些算"配了" | F1 标准 / F2 检查 / F3 reviewer / F4 扫描 4 层(F4b 是 F4 的开关,不单列) |
| 质量子项 | 哪些信号 | 门禁通过率 + 评审完成率 + 熵控制(发现/修复);**分母为 0 → 排除,不当 0 算** |
| 权重 | coverage vs quality | **0.4 / 0.6**(有质量数据时);无质量数据 → 纯覆盖分 + "no activity" |
| 阈值 | 绿/黄/红 | 绿 ≥ 80 · 黄 50–79 · 红 < 50 |
| 放置 | 后端 vs 前端 | **后端纯函数** + `GetForgeHealth` 响应加 `score`/`status`/`no_activity` |

---

## 2. 目标 / 非目标

**目标**
- 给 F5 健康面板一个**一眼可读**的综合分 + 状态灯,回答"Harness 在不在健康运作"。
- 后端纯函数计算(权威、可单测、将来可接告警),从 F5 `GetForgeHealth` 已有计数派生。
- 对凭证稀疏鲁棒:刚配好未跑的 Harness 评为"已就绪"而非"坏"。
- **无新表 / 无迁移**;Forge 隔离(`forge_` 前缀)。

**非目标(本切片不做)**
- 阈值告警 / 通知(分数是前置,告警另立)。
- 历史分数趋势(只算当前快照分;趋势后置)。
- 可配置权重 / 自定义评分(权重硬编码,够用)。

---

## 3. 评分模型（覆盖感知混合）

**覆盖**(配置层,always 有数据):
```
coverage = count(StandardsTotal>0, Checks>0, ReviewConfigs>0, Scans>0) / 4   // 0..1
```

**质量子项**(0..1,各自仅在分母>0 时计入,否则排除):
```
gatePass       = GatePassed / (GatePassed + GateFailed)          if (GatePassed+GateFailed) > 0
reviewDone     = ReviewCompleted / ReviewTotal                   if ReviewTotal > 0
entropyControl = OpenFindings==0 ? 1.0
                 : FixPRsOpened / (OpenFindings + FixPRsOpened)   if (OpenFindings+FixPRsOpened) > 0
```
（`entropyControl`:无开放发现=1.0 满分;有发现则看修复占比;有发现且零修复=0。）

**合成**:
```
qual = 可用质量子项的算术平均
若无任何可用质量子项:  score = round(100 * coverage);          noActivity = true
否则:                  score = round(100 * (0.4*coverage + 0.6*qual)); noActivity = false
status = score>=80 ? "green" : score>=50 ? "yellow" : "red"
```

> 权重 0.4/0.6、`entropyControl` 子公式、阈值 80/50 是硬编码旋钮(本切片不暴露配置)。

---

## 4. 后端

新包 `server/internal/forgehealth/score.go`(service-free 纯逻辑):
```go
package forgehealth

type ScoreInput struct {
    StandardsTotal, Checks, ReviewConfigs, Scans int32   // coverage
    GatePassed, GateFailed                       int32   // F2
    ReviewTotal, ReviewCompleted                 int32   // F3
    OpenFindings, FixPRsOpened                   int32   // F4/F4b
}
type ScoreResult struct {
    Score      int    // 0..100
    Status     string // "green" | "yellow" | "red"
    NoActivity bool   // configured but unexercised (no quality data)
}
func Score(in ScoreInput) ScoreResult
```
纯函数,无 I/O,**表驱动单测**。

`handler/forge_health.go` `GetForgeHealth`:组装完 `out ForgeHealthResponse` 后,调
`forgehealth.Score(ScoreInput{ from out 的计数 })`,把 `Score`/`Status`/`NoActivity` 加进响应
（响应 struct 加 `Score int json:"score"` + `Status string json:"status"` + `NoActivity bool json:"no_activity"`）。

> 注:`server/internal/forgehealth` 是新包,与 handler 同名前缀但不同包,纯逻辑、不 import service/handler,无环。

---

## 5. 前端
- `types/forge-health.ts` `ForgeHealth` 加 `score: number; status: string; no_activity: boolean`;
  `schemas.ts` `ForgeHealthSchema` 加 `score: z.number()`, `status: z.string()`, `no_activity: z.boolean()`(均 `.loose()` 已在 object 上);`EMPTY_FORGE_HEALTH` 补默认(score:0, status:"red", no_activity:true)。
- `forge-health-page.tsx` 顶部加**健康分 badge**:大数字 `{h.score}` + 绿/黄/红圆点(由 `h.status` 决定颜色,用语义 token:green→`bg-green-500`?——**改用语义**:状态色用现有 token,如 `text-foreground` + 一个小圆点 `bg-emerald-500`/`bg-amber-500`/`bg-red-500`——注:CLAUDE.md 禁硬编码色,故用语义 token 或 shadcn Badge 的 variant)。一行分项注记("coverage 100% · gate 100% · reviews 0%" 或 `no_activity` 时"configured, no activity yet")。

> **状态色实现细节**(留给 plan 定稿):优先用 shadcn `Badge` variant / 语义 token,避免硬编码 Tailwind 颜色;若无合适语义色,用最小一组状态色 class 并在 plan 注明这是状态语义色(非装饰色)。

---

## 6. 错误处理
- `Score` 纯函数对全零输入返回 `{Score:0, Status:"red", NoActivity:true}`(没配没跑 = 红)。
- coverage 分母固定 4,无除零;质量子项分母为 0 即排除,无 NaN。
- 前端 score/status 经 zod;`EMPTY_FORGE_HEALTH` fallback 含 score:0/status:"red"。

---

## 7. 测试 / 验收（零凭证最硬）
- **纯单测**(本切片核心):`Score()` 表驱动 ——
  - 全配 + 满质量(gate 1.0/review 1.0/entropy 1.0)→ score 100 green。
  - 全配 + 无活动 → score 100, NoActivity true(纯覆盖)。
  - 全配 + gate 全挂(0 pass / N fail)+ 其余无数据 → 低分(0.4*1+0.6*0=40)yellow→ 实为 40 < 50 → red。
  - 零配置零活动 → 0 red。
  - 部分配置(2/4 层)+ 部分质量 → 中段;断言 status 边界(80/50)。
- **绕凭证集成**:源码构建栈 `GET /api/forge/health` 返回 `score`/`status`/`no_activity`,与 DB 实际计数手算一致
  (当前真实数据 standards=1/checks=1/reviewers=1/scans=1 全配、gate.pass=1、review.completed=0、open_findings=0/fix_prs.opened=1
  → coverage=1.0、gatePass=1.0、reviewDone=0.0、entropyControl=1.0、qual=0.667、score=round(100*(0.4+0.4))=**80 green**)。

## 8. 范围
单切片,小:`forgehealth.Score` 纯函数 + 3 个响应字段 + 前端类型/zod/badge。**无迁移**。

## 9. 后续
阈值告警 / 历史分数趋势 / 可配置权重。
