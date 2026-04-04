# Session 2026-04-04: 需求对话 UX 全面升级

## 背景

用户反馈任务创建后的对话体验极差：AI 返回原始 JSON、没有引导感、没有选项交互、没有流式输出。本次 session 围绕需求分析对话体验进行了大量改造。

---

## 已完成的修复

### 1. AI 返回格式化中文 Markdown（不再是 JSON）

**问题：** AI 返回 `{"status":"clarify","questions":["..."],...}` 原始 JSON 显示在聊天中。

**修复：**
- `ai-worker/src/activities/analyze.py` — 新增 `format_human_response(status, structured)` 函数
- 将 JSON 转为中文 Markdown：💡 我的理解 → ❓ 问题 → ⚠️ 风险
- `ai-worker/src/activities/analyze.py` — 新增 `normalize_clarify_response()` 后处理
  - 旧格式 `questions[]` 数组 → 新格式单个 `question` + `options[]`
  - `partial_summary` → `understanding`
  - 自动填充 `phase` 字段
- 前端 `forge-portal/components/chat/message-bubble.tsx` — 新增 `formatAIContent()` 兜底
  - 即使 DB 中存的是旧 JSON 消息，前端也能自动检测并格式化显示

**验证：** 41 个 Python 单元测试覆盖（`tests/test_analyze_flow.py`）

### 2. Superpowers 方法论融入需求分析

**问题：** AI 一次抛出 5-6 个问题，没有引导感，没有阶段概念。

**修复：**
- `ai-worker/src/agents/analyst.py` — 完全重写 System Prompt
  - 核心原则：一次只问一个问题、多选题优先、递进式深入、每轮复述理解
  - 四阶段流程：初步理解 → 场景澄清 → 约束确认 → 需求确认
  - confirmed 输出包含：功能需求列表、验收标准、不在范围内、风险识别
  - 包含两个完整的 JSON 示例，强制 AI 遵循格式
- `ai-worker/src/activities/analyze.py` — `_generate_fallback_options()` 兜底函数
  - 如果 AI 没返回 options，根据问题语义自动生成（是否型/规模型/功能型）

### 3. 可点击选项按钮（多选+确认）

**问题：** 用户需要可点击的 A/B/C/D 选项，而不是文字描述。且应支持多选。

**修复：**
- `forge-portal/components/chat/option-buttons.tsx` — 新建组件
  - Toggle 多选模式：点击选中/取消，选中后显示 ✓
  - "确认选择（N项）" 按钮 + "可多选" 提示
  - 确认后将选项用"；"连接发送给 AI
- `forge-portal/components/chat/chat-panel.tsx` — 新增 `latestOptions` prop
  - 在最后一条 assistant 消息下方渲染 OptionButtons
- `forge-portal/app/.../[taskId]/page.tsx` — 从 API 响应和历史消息 metadata 中提取 options
- Go 后端 `conversation/service.go` — SaveMessage 时将 metadata（含 options）存入 DB
  - 修复了之前 assistant 消息不存 metadata 的问题

### 4. PENDING 步骤不可点击 + 视觉禁用

**问题：** 所有步骤（包括未执行的）都可点击。

**修复：**
- `forge-portal/components/tasks/step-timeline.tsx`
  - PENDING 步骤：opacity-40 + 不响应点击
  - 但当任务已终态（COMPLETED/FAILED/CANCELLED）时，所有步骤都可点击（查看历史）

### 5. 已完成步骤可点击查看

**问题：** conversation phase 中点击左侧步骤不会切换到步骤详情视图。

**修复：**
- `forge-portal/app/.../[taskId]/page.tsx`
  - 当用户手动点击 COMPLETED 步骤时，切换到 TaskWorkspace 视图（不再固定显示 ChatPanel）
- `forge-portal/components/tasks/step-timeline.tsx`
  - 新增 `taskTerminal` prop，任务终态时所有步骤可点击

### 6. 确认卡片增强 + 中文化

**问题：** 确认卡片全英文，且缺少功能需求/验收标准等字段。

**修复：**
- `forge-portal/components/chat/confirmation-card.tsx`
  - 全中文化（确认需求、继续完善、正在生成方案...）
  - 新增字段：functionalRequirements、acceptanceCriteria、outOfScope
  - 紫色主题边框

### 7. 方案规划人工审查卡片

**问题：** 需求确认后自动跑完所有步骤，没有方案审查环节。

**修复：**
- Go 后端拆分 Workflow：
  - `PlanOnlyWorkflow` — 只执行 PLAN 步骤，返回方案结果
  - `TaskExecutionWorkflow` — 执行 TEST_WRITING → GENERATE → REVIEW → TEST → DEPLOY
  - `ConfirmPlan` — 确认需求 → 跑 PlanOnlyWorkflow → 返回方案数据
  - `ApprovePlan` — 审批方案 → 启动 TaskExecutionWorkflow
- `forge-portal/components/chat/plan-review-card.tsx` — 新建方案审查卡片
  - DAG 任务列表、风险等级、总工时、并行通道
  - 三个按钮：批准方案并执行 / 修改方案 / 取消

### 8. 任务取消功能（不可恢复）

**问题：** 未生成代码的任务无法取消。

**修复：**
- Go 后端 `task/service.go` — `CancelTask()` 方法
  - 仅允许 SUBMITTED/ANALYZING/PLANNING 状态取消
  - 状态改为 `CANCELLED`（新增常量）
- `task/handler.go` — `POST /tasks/:taskId/cancel` 端点
- `forge-portal/lib/conversation.ts` — `cancelTask()` API
- 前端取消按钮加确认弹窗

### 9. 步骤级模型配置

**问题：** 所有步骤用同一个模型。

**修复：**
- `ai-worker/src/models/router.py` — ROUTING_RULES 按 Purpose 配置不同模型
  - ANALYZE / PLAN / REVIEW → `qwen3-max`（强推理）
  - TEST_WRITING / GENERATE → `qwen3-coder-plus`（代码专精）
  - DashScope 优先，Anthropic/OpenAI/DeepSeek 降级

### 10. 结构化 JSON 输出强制

**问题：** AI 有时不遵循 JSON 格式要求。

**修复：**
- `ai-worker/src/models/client.py` — 所有 OpenAI-compatible 调用支持 `response_format` 参数
- `ai-worker/src/models/router.py` — ANALYZE purpose 自动传 `response_format: {"type": "json_object"}`

### 11. K8s 基础设施修复

**问题1：** `golang:1.22-alpine` 镜像无法从 docker.io 拉取（国内网络超时）。
**修复：** 改为阿里云镜像 `registry.cn-hangzhou.aliyuncs.com/shulex/golang:1.22-alpine`

**问题2：** namespace `tenant-1-DEV` 包含大写字母，K8s 拒绝。
**修复：** `strings.ToLower(env.EnvType)`

**问题3：** 部署时 `version`（如 `v0.1.1953`）被当作镜像名。
**修复：** 版本号不含 `/` 时，用 `nginx:alpine` 占位镜像做 mock 部署。

### 12. Docker 旧 Worker 冲突根因定位

**问题：** 所有代码改动都不生效。

**根因：** Docker 中运行着 23 小时前的旧 `forge-ai-worker` 容器，与本地 Python worker 争抢同一个 Temporal task queue。任务被旧容器处理。同时本地系统环境变量 `DASHSCOPE_API_KEY` 覆盖了 .env 文件中的正确 key。

**修复：**
- `docker stop forge-ai-worker`
- 本地 worker 启动时显式传正确的 `DASHSCOPE_API_KEY`

---

## 未完成 / 待下个 Session 继续

### 1. 流式输出 + 思考过程展示（P0 — 计划已批准）

**目标：** 用户发消息后立即看到 AI 思考过程（逐字出现），完成后折叠思考、显示格式化结果。

**架构设计：**
```
用户发消息 → Go 返回 "streaming"（不阻塞）
              ↓
         Temporal Workflow → Python Activity → LLM (流式)
                                                 ↓
                              Redis: analyze:stream:{taskId}
                              事件: thinking / stream_end / result
                                                 ↓
         Go SSE Handler ← Redis ← ─────────────┘
              ↓
         SSE → 前端（思考过程渐进 + 最终结果）
```

**8 步实施计划：**

| 步骤 | 文件 | 改动 |
|------|------|------|
| 1 | `ai-worker/src/activities/analyze.py` | 流式调用 LLM + Redis 发布 thinking/result 事件 |
| 2 | `forge-core/internal/module/task/sse.go` | 新增订阅 `analyze:stream:{taskId}` 频道 |
| 3 | `forge-core/internal/module/conversation/service.go` | SendMessage/TriggerAnalysis 异步化（不再阻塞 3 分钟） |
| 4 | `forge-core/internal/temporal/activity/task_activities.go` | 新增 `SaveConversationResult` activity |
| 5 | `forge-core/internal/temporal/workflow/task_workflow.go` | AnalyzeRequirementWorkflow 加保存步骤 |
| 6 | `forge-portal/lib/use-task-stream.ts` | 扩展 analyze 事件处理 |
| 7 | `forge-portal/components/chat/streaming-thinking.tsx` | 新建：可折叠思考过程组件 |
| 8 | `forge-portal/components/chat/chat-panel.tsx` + page.tsx | 集成流式状态 |

**计划文件：** `C:\Users\86157\.claude\plans\recursive-squishing-fog.md`

### 2. 需求分析完成后生成需求文档（P1）

**现状：** 需求确认后只在对话中显示，没有独立的需求规格文档。

**建议：** 当 AI 返回 `status: "confirmed"` 时，自动生成 Markdown 需求文档，保存到任务的 step output 中。点击 ANALYZE 步骤（COMPLETED 状态）时展示完整文档。

### 3. 前端显示使用的模型名称 + 头像（P2）

**现状：** 聊天中不显示 AI 使用了哪个模型。

**建议：**
- API 响应中包含 `model` 和 `provider` 字段（Python activity 已返回）
- 前端 MessageBubble 显示模型标签（如 "qwen3-max"）和 AI 头像
- Go 后端需要将 model/provider 信息存入 conversation metadata

### 4. 部署通过制品模块（P2）

**现状：** DEPLOY 步骤直接用版本号做镜像名，没有真正的 CI 构建流程。

**建议：** 部署应该关联制品（Artifact）模块，用户选择已构建的制品进行部署。当前用 mock nginx 镜像占位。

### 5. 对话中选项的持久化与历史回显（P2）

**现状：** 刷新页面后，历史消息的选项从 metadata 中提取并显示，但多选状态丢失。

**建议：** 已选选项应该在用户消息中体现（当前已实现——选项作为文本发送）。历史消息中的选项可以考虑以"已回答"样式展示。

---

## 关键文件变更清单

### Python (ai-worker)
| 文件 | 改动 |
|------|------|
| `src/agents/analyst.py` | System Prompt 全面重写（Superpowers 方法论） |
| `src/activities/analyze.py` | format_human_response + normalize_clarify_response + _generate_fallback_options |
| `src/models/router.py` | 步骤级模型配置 + response_format 支持 |
| `src/models/client.py` | response_format 参数透传 |
| `tests/test_analyze_flow.py` | 41 个测试用例（新建） |

### Go (forge-core)
| 文件 | 改动 |
|------|------|
| `internal/module/conversation/service.go` | 异步化 + metadata 存 DB + ConfirmPlan/ApprovePlan/TriggerAnalysis |
| `internal/module/conversation/handler.go` | TriggerAnalysis + ApprovePlan + CancelTask 端点 |
| `internal/module/conversation/model.go` | PlanConfirmResponse + SendMessageResponse |
| `internal/module/task/service.go` | CancelTask（CANCELLED 状态） |
| `internal/module/task/handler.go` | CancelTask handler |
| `internal/module/task/model.go` | StatusCancelled 常量 |
| `internal/module/task/sse.go` | （待改：analyze 频道订阅） |
| `internal/temporal/workflow/task_workflow.go` | PlanOnlyWorkflow + TaskExecutionWorkflow |
| `internal/temporal/activity/task_activities.go` | 阿里云镜像 + （待加 SaveConversationResult） |
| `internal/temporal/worker.go` | 注册新 workflow |
| `internal/module/pipeline/service.go` | namespace 小写 + 镜像名修复 |
| `internal/router/router.go` | 新路由 /analyze /approve-plan /cancel |

### Frontend (forge-portal)
| 文件 | 改动 |
|------|------|
| `components/chat/option-buttons.tsx` | 新建：多选选项按钮组件 |
| `components/chat/message-bubble.tsx` | formatAIContent 兜底 + options 不渲染为文本 |
| `components/chat/chat-panel.tsx` | latestOptions prop + OptionButtons 渲染 |
| `components/chat/confirmation-card.tsx` | 中文化 + 新字段 |
| `components/chat/plan-review-card.tsx` | 新建：方案审查卡片 |
| `components/tasks/step-timeline.tsx` | PENDING 不可点击 + taskTerminal 支持 |
| `lib/conversation.ts` | triggerAnalysis + approvePlan + cancelTask API |
| `lib/tasks.ts` | CANCELLED 状态标签和颜色 |
| `app/.../[taskId]/page.tsx` | 全面重构：auto-trigger + options + plan review + cancel + step 切换 |

---

## 运行环境注意事项

1. **Docker 旧 worker 必须停掉**：`docker stop forge-ai-worker`
2. **本地 AI worker 启动命令**：
   ```bash
   cd ai-worker && DASHSCOPE_API_KEY=sk-***REDACTED*** python -B -m src.worker
   ```
3. **forge-core 重新编译**：`cd forge-core && go build ./cmd/forge-core && ./forge-core.exe`
4. **前端 dev server**：`cd forge-portal && npm run dev`
5. **测试**：`cd ai-worker && pytest tests/` — 55 tests all pass
