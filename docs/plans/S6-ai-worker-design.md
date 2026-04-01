# S6 — AI Worker 设计规格书

> **状态**: 设计完成，待实施
> **依赖**: S4 (Temporal + Tasks), S5 (Specs Center)
> **交付**: Python AI Worker + Go 对话 API + 前端 Chat UI + 代码预览
> **参考**: PRD §2.1/§2.7, 技术设计 §1.4/§2/§3/§10

---

## 1. 目标

交付完整的 AI 驱动代码生成流水线。AI 引擎是 Forge 平台的**中央大脑**，负责从需求输入到代码产出的全流程。

**S6 交付后用户可以做什么**：
1. 在项目中新建需求，用自然语言描述（"给优惠券系统加一个按用户等级发放的功能"）
2. AI 通过**对话式澄清**理解需求（多轮对话，主动提问）
3. AI 生成**需求确认卡片**，用户确认后才继续
4. 确认后 AI 自动执行：任务拆解 → 代码生成 → AI Review → 修复重试
5. **实时查看**每一步的进度、产出物、AI 决策原因
6. 在任务详情页**内嵌预览**生成的代码（文件树 + 代码 + Review findings）
7. Review 不通过时 AI 自动修复重试（最多 3 次），超限升级人工

---

## 2. 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│ forge-portal (Next.js)                                       │
│  需求对话 UI ─ 确认卡片 ─ 任务时间线 ─ 代码预览              │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP + SSE
┌──────────────────▼──────────────────────────────────────────┐
│ forge-core (Go)                                              │
│  Conversation API ─ Task API ─ Specs API ─ Temporal Client   │
│  ┌────────────────────────────────┐                          │
│  │ TaskGenerationWorkflow (Go)    │                          │
│  │  step 1: analyze (→ Python)    │  ← 多轮对话阶段          │
│  │  step 2: plan    (→ Python)    │  ← 确认后自动启动        │
│  │  step 3: generate(→ Python)    │                          │
│  │  step 4: review  (→ Python)    │  ← 失败则 fix loop      │
│  │  step 5: complete              │                          │
│  └────────────────────────────────┘                          │
└──────────────────┬──────────────────────────────────────────┘
                   │ Temporal (cross-language activity)
┌──────────────────▼──────────────────────────────────────────┐
│ ai-worker (Python 3.12)                                      │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Layer 1: Model Router (多模型路由 + 降级链)             │   │
│  │  Claude Sonnet → GPT-4o → Qwen Max → DeepSeek Chat  │   │
│  │  每个模型独立熔断器 (50% 错误率 → 降级)                  │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Layer 2: Context Builder (上下文工程)                    │   │
│  │  四层 Prompt 叠加:                                      │   │
│  │  System Prompt + Standards Injection + Context + User  │   │
│  │  Token 预算优化: 180k input (200k - 20k output reserve)│   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Layer 3: AI Agents                                     │   │
│  │  Analyst (需求分析) ─ Planner (任务拆解)                 │   │
│  │  Coder (代码生成)  ─ Reviewer (代码审查)                 │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Layer 4: Temporal Activities                           │   │
│  │  analyze_requirement ─ plan_task ─ generate_code      │   │
│  │  review_code                                          │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

---

## 3. Python AI Worker

### 3.1 项目结构

```
ai-worker/
├── pyproject.toml
├── requirements.txt
├── .env.example
├── src/
│   ├── __init__.py
│   ├── config.py              # 配置（环境变量, Pydantic Settings）
│   ├── worker.py              # Temporal Worker 入口
│   ├── models/
│   │   ├── __init__.py
│   │   ├── router.py          # 多模型路由器 + 熔断器
│   │   └── client.py          # LLM 客户端封装（Anthropic/OpenAI/DashScope/DeepSeek）
│   ├── context/
│   │   ├── __init__.py
│   │   └── builder.py         # 上下文构建器（四层 Prompt 叠加）
│   ├── agents/
│   │   ├── __init__.py
│   │   ├── base.py            # Agent 基类（统一 JSON 解析、错误处理）
│   │   ├── analyst.py         # 需求分析 Agent
│   │   ├── planner.py         # 任务规划 Agent
│   │   ├── coder.py           # 代码生成 Agent
│   │   └── reviewer.py        # 代码审查 Agent
│   └── activities/
│       ├── __init__.py
│       ├── analyze.py         # analyze_requirement_activity
│       ├── plan.py            # plan_task_activity
│       ├── generate.py        # generate_code_activity
│       └── review.py          # review_code_activity
├── tests/
│   ├── __init__.py
│   ├── test_router.py
│   ├── test_context_builder.py
│   └── test_agents.py
```

### 3.2 多模型路由与降级链

**设计原则**（PRD §2.1.4）：
- 按任务复杂度选择合适模型（高复杂度用强模型，简单任务用轻量模型）
- 故障降级：主模型不可用时自动切换到备选模型
- 模型可插拔：新增 AI 模型不需要修改核心逻辑

```python
class Purpose(Enum):
    ANALYZE = "analyze"       # 需求分析
    PLAN = "plan"             # 任务拆解
    GENERATE = "generate"     # 代码生成（需要强模型）
    REVIEW = "review"         # 代码审查（需要强模型）

# 任务感知的模型选择 + 降级链（技术设计 §3.2）
ROUTING_RULES = {
    Purpose.ANALYZE:  [
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
        ("dashscope", "qwen-max"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.PLAN:     [
        ("anthropic", "claude-sonnet-4-20250514"),
        ("openai", "gpt-4o"),
        ("dashscope", "qwen-max"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.GENERATE: [
        ("anthropic", "claude-sonnet-4-20250514"),   # 代码生成用强模型
        ("openai", "gpt-4o"),
        ("dashscope", "qwen-max"),
        ("deepseek", "deepseek-chat"),
    ],
    Purpose.REVIEW:   [
        ("anthropic", "claude-sonnet-4-20250514"),   # Review 需要强理解力
        ("openai", "gpt-4o"),
        ("dashscope", "qwen-max"),
    ],
}
```

**熔断机制**（技术设计 §3.2）：
- 每个模型有独立熔断器
- 错误率 > 50%（30s 窗口）→ 熔断 → 降级到下一个模型
- 熔断后 60s 半开状态 → 尝试恢复

**调用追踪**：
- 每次 LLM 调用记录到 `engine.model_calls` 表
- 字段：model, provider, purpose, input_tokens, output_tokens, cost_cents, latency_ms, status

### 3.3 上下文工程（Context Engineering）

**核心理念**（PRD §2.7.4）：Prompt 是平台最核心的资产。每个 Prompt 调用由四层叠加：

| 层级 | 内容 | 变化频率 |
|------|------|---------|
| System Prompt（固定层） | AI 角色定义、行为约束、输出格式 | 极少变化 |
| Standards Injection（项目层） | 编码规范、技术栈约束、命名规则 | 按项目不同 |
| Context（任务层） | 相关代码、Schema、API 契约 | 每次任务 |
| User Input（用户层） | 需求描述、澄清回复、修改要求 | 每次交互 |

```python
class ContextBuilder:
    """为 AI 调用构建最优上下文（技术设计 §3.4）"""

    async def build(self, project_id: int, purpose: str,
                    requirement: str = "", history: list = None) -> ProjectContext:
        context = ProjectContext()

        # 1. 静态上下文（规范中心 S5）
        context.coding_standards = await self._load_effective_specs(project_id)
        context.prompt_template = await self._load_prompt_template(project_id, purpose)

        # 2. 项目画像
        context.project = await self._load_project_profile(project_id)

        # 3. 对话历史
        context.conversation_history = history or []

        # 4. Token 预算优化
        context = self._optimize_for_budget(context, budget=180_000)

        return context

    def _optimize_for_budget(self, context, budget):
        """Token 预算内最大化上下文价值
        优先级: 编码规范 > Prompt 模板 > 对话历史 > 项目画像
        长对话: 结构化摘要旧对话，避免 context window 爆炸
        """
```

**数据来源**（通过 forge-core API 获取）：
- `GET /api/specs/effective/{projectId}` — 合并后的编码规范 + Review 规则
- `GET /api/specs/prompts?purpose={purpose}` — 对应用途的 Prompt 模板
- `GET /api/projects/{projectId}` — 项目画像（技术栈、描述）

### 3.4 AI Agents

#### 3.4.1 Agent 基类

所有 Agent 共享的基础能力：
- 统一 JSON 结构化输出解析（fallback: 如果 JSON 解析失败，将响应作为纯文本处理）
- 统一错误处理和日志
- 统一模型调用追踪

#### 3.4.2 Analyst Agent（需求分析）

**职责**（PRD §2.1.1）：
- 接收用户的自然语言需求描述，理解意图
- 需求模糊时**主动提问澄清**，支持多轮对话
- 需求清楚后生成结构化摘要，供用户确认

**输入**：用户需求 + 项目上下文 + 对话历史

**输出**：
```json
{
  "status": "clarify",           // "clarify" = 需要澄清, "confirmed" = 需求已确认
  "questions": [                  // status=clarify 时的澄清问题
    "需要支持哪些用户等级？",
    "发放规则是固定额度还是按比例？"
  ],
  "summary": "...",               // status=confirmed 时的需求摘要
  "task_title": "按用户等级发放优惠券",
  "affected_modules": ["coupon", "user"],
  "estimated_complexity": "MEDIUM"
}
```

#### 3.4.3 Planner Agent（任务规划）

**职责**（PRD §2.1.1 + 技术设计 §2.3）：
- 将确认的需求拆解为结构化技术任务清单
- 明确每个任务改哪些模块、哪些文件、新增哪些接口、数据库变更
- 输出风险评估

**输出**：
```json
{
  "title": "按用户等级发放优惠券",
  "tasks": [
    {
      "order": 1,
      "title": "新增用户等级枚举和配置表",
      "files": ["src/model/user_level.go", "migrations/007_user_level.sql"],
      "type": "SCHEMA_CHANGE",
      "estimate_hours": 1
    },
    {
      "order": 2,
      "title": "实现等级发放服务",
      "files": ["src/service/coupon_level_service.go"],
      "type": "BACKEND",
      "estimate_hours": 2
    }
  ],
  "risk_level": "MEDIUM",
  "risk_factors": ["涉及数据库变更", "影响优惠券发放核心逻辑"]
}
```

#### 3.4.4 Coder Agent（代码生成）

**职责**（PRD §2.1.2 + 技术设计 §3.3）：
- 基于任务列表、项目已有代码、编码规范生成代码
- 上下文感知：确保与现有架构一致
- 输出可直接提交的代码文件

**设计原则**：
- **契约先行**：先输出接口签名和 DTO 定义，再基于契约生成实现
- **编码规范强制**：System Prompt 注入 S5 规范中心的编码规范
- **多文件原子提交**：一次任务生成的多个文件作为一个原子提交

**输出**：
```json
{
  "files": [
    {
      "path": "src/service/coupon_level_service.go",
      "content": "package service\n\nimport ...",
      "action": "create",
      "language": "go"
    },
    {
      "path": "src/model/user_level.go",
      "content": "package model\n\ntype UserLevel ...",
      "action": "create",
      "language": "go"
    }
  ],
  "commit_message": "feat(coupon): add user level based coupon distribution",
  "files_changed": 2,
  "lines_added": 180,
  "lines_deleted": 0
}
```

#### 3.4.5 Reviewer Agent（代码审查）

**职责**（PRD §2.1.3 + 技术设计 §2.4）：
- 对照规范中心规则，检查代码是否合规
- 安全扫描：OWASP 规则 + SQL 注入 + XSS 检查
- 逻辑一致性：检查生成代码是否自洽、接口是否匹配
- 输出评分报告 + 问题列表 + 修复建议

**Review 维度**：
1. 编码规范合规性
2. 安全漏洞（SQL 注入、XSS、硬编码密码）
3. 性能问题（N+1 查询、大表全扫描）
4. 逻辑正确性
5. 可维护性

**输出**：
```json
{
  "passed": false,
  "score": 72,
  "findings": [
    {
      "severity": "ERROR",
      "file": "src/service/coupon_level_service.go",
      "line": 42,
      "message": "SQL 查询使用了字符串拼接，存在注入风险",
      "suggestion": "使用参数化查询 $1/$2",
      "rule": "SECURITY/sql-injection"
    },
    {
      "severity": "WARNING",
      "file": "src/model/user_level.go",
      "line": 15,
      "message": "枚举值未使用常量定义",
      "suggestion": "提取为 const 块",
      "rule": "CODING/magic-string"
    }
  ],
  "summary": "发现 1 个安全问题和 1 个编码规范问题，建议修复后重新提交",
  "fix_instructions": "1. coupon_level_service.go:42 将 SQL 改为参数化查询\n2. user_level.go:15 枚举值提取为常量"
}
```

**修复重试机制**（PRD §2.1.3）：
- Review 不通过 → `fix_instructions` 传回 Coder Agent 自动修复
- 最多 3 轮修复重试
- 超限 → 标记为需要人工介入，推送通知

### 3.5 Temporal Activities

每个 Activity 是 `@activity.defn` 装饰的 async 函数，作为 Temporal 跨语言 Activity 暴露：

```python
@dataclass
class AnalyzeInput:
    project_id: int
    task_id: int
    requirement: str
    conversation_history: list[dict] | None

@dataclass
class AnalyzeOutput:
    status: str        # "clarify" | "confirmed"
    content: str       # AI 回复文本
    metadata: dict     # 结构化数据（questions / summary / task_title）
    tokens_used: int

@activity.defn(name="analyze_requirement")
async def analyze_requirement_activity(input: AnalyzeInput) -> AnalyzeOutput:
    ctx = await ContextBuilder().build(input.project_id, purpose="analyze")
    result = await AnalystAgent(router).run(input.requirement, ctx, input.conversation_history)
    return AnalyzeOutput(...)
```

**Activity 配置**（技术设计 §2.1）：

| Activity | 超时 | 重试 | Task Queue |
|----------|------|------|-----------|
| analyze_requirement | 2min | 2次 | ai-worker |
| plan_task | 3min | 2次 | ai-worker |
| generate_code | 5min | 2次 | ai-worker |
| review_code | 3min | 2次 | ai-worker |

**Worker 入口**：

```python
async def main():
    client = await Client.connect(settings.temporal_host)
    worker = Worker(
        client,
        task_queue="ai-worker",
        activities=[
            analyze_requirement_activity,
            plan_task_activity,
            generate_code_activity,
            review_code_activity,
        ],
    )
    await worker.run()
```

---

## 4. Go 改造

### 4.1 数据库迁移

基于技术设计 §10 数据架构，S6 需要新增以下表：

```sql
-- 对话历史
CREATE TABLE engine.conversations (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    role            VARCHAR(20) NOT NULL,      -- user | assistant | system
    content         TEXT NOT NULL,
    metadata        JSONB,                      -- agent 结构化数据
    tokens_used     INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_conversations_task ON engine.conversations(task_id);

-- AI 模型调用记录（成本追踪）
CREATE TABLE engine.model_calls (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    task_id         BIGINT NOT NULL,
    step_type       VARCHAR(20),
    model           VARCHAR(50) NOT NULL,
    provider        VARCHAR(20) NOT NULL,       -- anthropic | openai | dashscope | deepseek
    purpose         VARCHAR(20) NOT NULL,       -- analyze | plan | generate | review
    input_tokens    INT NOT NULL DEFAULT 0,
    output_tokens   INT NOT NULL DEFAULT 0,
    total_tokens    INT NOT NULL DEFAULT 0,
    cost_cents      INT NOT NULL DEFAULT 0,
    latency_ms      INT NOT NULL DEFAULT 0,
    status          VARCHAR(10) NOT NULL,       -- success | error
    error_code      VARCHAR(50),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_model_calls_tenant ON engine.model_calls(tenant_id);
CREATE INDEX idx_model_calls_task ON engine.model_calls(task_id);

-- Review 结果
CREATE TABLE engine.review_results (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    step_id         BIGINT,
    review_type     VARCHAR(20) NOT NULL,       -- ai_review
    score           INT,
    passed          BOOLEAN NOT NULL,
    findings        JSONB NOT NULL DEFAULT '[]',
    summary         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_review_results_task ON engine.review_results(task_id);

-- 扩展 tasks 表（新增字段）
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS analysis JSONB;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS task_graph JSONB;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS risk_factors JSONB;
```

### 4.2 对话 API

```
POST /api/projects/:id/tasks/:taskId/messages     # 发送消息 → 调用 AI 分析 → 返回 AI 回复
GET  /api/projects/:id/tasks/:taskId/messages      # 获取对话历史
POST /api/projects/:id/tasks/:taskId/confirm       # 确认需求 → 启动 TaskGenerationWorkflow
```

**SendMessage 流程**：
1. 保存用户消息到 conversations 表
2. 查询对话历史
3. 调用 Temporal Activity `analyze_requirement`（同步等待结果）
4. 保存 AI 回复到 conversations 表
5. 如果 status=confirmed，更新 task.analysis 和 task.status=ANALYZING
6. 返回 AI 回复给前端

**ConfirmPlan 流程**：
1. 校验 task 状态必须是 ANALYZING（已确认需求）
2. 启动 TaskGenerationWorkflow（Temporal）
3. 保存 workflow_id 到 task
4. 更新 task.status = PLANNING
5. 返回成功

### 4.3 真实 Workflow（替换 S4 骨架）

```go
func TaskGenerationWorkflow(ctx workflow.Context, input TaskWorkflowInput) error {
    // AI Activities 运行在 "ai-worker" task queue（Python Worker）
    aiActivityOptions := workflow.ActivityOptions{
        TaskQueue:           "ai-worker",
        StartToCloseTimeout: 5 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
            InitialInterval: 5 * time.Second,
            BackoffCoefficient: 2.0,
        },
    }

    // Step 1: Plan（任务拆解）
    updateStepStatus(ctx, taskID, "PLAN", "RUNNING")
    var planOutput PlanOutput
    err := workflow.ExecuteActivity(aiCtx, "plan_task", planInput).Get(ctx, &planOutput)
    if err != nil { return markFailed(ctx, taskID, "PLAN", err) }
    updateStepStatus(ctx, taskID, "PLAN", "COMPLETED")

    // Step 2: Generate（代码生成）
    updateStepStatus(ctx, taskID, "GENERATE", "RUNNING")
    var codeOutput GenerateOutput
    err = workflow.ExecuteActivity(aiCtx, "generate_code", genInput).Get(ctx, &codeOutput)
    if err != nil { return markFailed(ctx, taskID, "GENERATE", err) }
    updateStepStatus(ctx, taskID, "GENERATE", "COMPLETED")

    // Step 3: Review + Fix Loop（审查 + 修复循环，最多 3 轮）
    for attempt := 1; attempt <= 3; attempt++ {
        updateStepStatus(ctx, taskID, "REVIEW", "RUNNING")
        var reviewOutput ReviewOutput
        err = workflow.ExecuteActivity(aiCtx, "review_code", reviewInput).Get(ctx, &reviewOutput)
        if err != nil { return markFailed(ctx, taskID, "REVIEW", err) }

        if reviewOutput.Passed {
            updateStepStatus(ctx, taskID, "REVIEW", "COMPLETED")
            break
        }

        if attempt == 3 {
            // 超过 3 轮仍未通过 → 标记需要人工介入
            return markNeedsHumanReview(ctx, taskID, reviewOutput)
        }

        // 自动修复：将 fix_instructions 传回 Coder Agent
        updateStepStatus(ctx, taskID, "GENERATE", "RUNNING")
        fixInput := GenerateInput{
            ...genInput,
            FixInstructions: reviewOutput.FixInstructions,
            PreviousCode:    codeOutput,
        }
        err = workflow.ExecuteActivity(aiCtx, "generate_code", fixInput).Get(ctx, &codeOutput)
        if err != nil { return markFailed(ctx, taskID, "GENERATE", err) }
    }

    // Step 4: Complete（S7 将在此处加入 GitHub commit + 部署）
    updateTaskStatus(ctx, taskID, "COMPLETED")
    return nil
}
```

---

## 5. 前端

### 5.1 需求对话页面

**交互模式**（产品设计 §3.3.1）：混合式（对话 + 确认卡片）

**交互流程**：
1. 用户输入自然语言需求
2. AI 流式响应，追问澄清问题
3. 多轮对话直到 AI 理解需求
4. AI 生成**需求确认卡片**，展示：
   - 需求摘要
   - 任务拆解预览
   - 影响的模块
   - 风险等级
   - 预估耗时
5. 用户操作：
   - **[确认执行]** → 启动 Workflow，跳转任务详情页
   - **[修改需求]** → 回到对话模式，卡片收起，补充修改意见
   - **[取消]** → 二次确认后清空对话

### 5.2 组件结构

```
forge-portal/
├── components/
│   ├── chat/
│   │   ├── chat-panel.tsx          # 对话面板主组件
│   │   ├── message-bubble.tsx      # 消息气泡（用户/AI/系统）
│   │   └── confirmation-card.tsx   # 需求确认卡片
│   └── code-preview/
│       ├── code-preview-panel.tsx  # 主面板（左文件树 + 右代码）
│       ├── file-tree.tsx           # 文件树（带 create/modify 图标）
│       └── code-viewer.tsx         # 代码展示（行号 + Geist Mono）
├── lib/
│   └── conversation.ts            # 对话 API 客户端
└── app/(dashboard)/projects/[id]/
    └── tasks/
        └── new/
            └── page.tsx            # 新建需求对话页
```

### 5.3 消息气泡样式

| 类型 | 对齐 | 背景色 | 渲染 |
|------|------|--------|------|
| 用户消息 | 右对齐 | #8B5CF6/10（紫色） | 纯文本 |
| AI 消息 | 左对齐 | white/5（深灰） | Markdown 渲染（复用 MarkdownPreview） |
| 系统消息 | 居中 | 无背景 | 小字灰色 |
| 确认卡片 | 全宽 | border + white/3 | 结构化卡片 + 操作按钮 |

### 5.4 任务详情页 — AI 工作过程可视化

**页面结构**（产品设计 §4.2）：左右分栏

**左侧 — 任务时间线（步骤流）**：
纵向时间线，每个步骤显示：
- 状态图标：✅ 已完成 / 🔄 进行中 / ⬚ 待执行 / ❌ 失败
- 步骤名称 + 耗时
- 一句话摘要
- 关键数据（如"3/5 文件已生成"、"评分 92 分"）

**右侧 — 实时工作区**：
根据当前活跃步骤动态展示内容：

| 当前步骤 | 右侧展示 |
|---------|---------|
| 需求分析 | AI 理解摘要 + 影响模块 |
| 方案规划 | 任务拆解列表 + 风险等级 |
| 代码生成 | **代码预览面板**（文件树 + 代码） |
| AI 审查 | Review 评分 + findings 列表 |
| 完成 | 变更摘要 + 统计信息 |

### 5.5 代码预览（任务详情页内嵌）

**数据存储**：Coder Agent 输出的 `files[]` 存入 task_steps.output 字段（JSON）。

**UI 布局**：

```
┌─────────────────────────────────────────────────┐
│ 代码生成  ✅ COMPLETED                           │
│                                                  │
│ ┌──────────┬────────────────────────────────┐   │
│ │ 文件列表  │ 代码内容                        │   │
│ │          │                                │   │
│ │ ▸ src/   │  // auth/service.go            │   │
│ │   auth/  │  package auth                  │   │
│ │   ✚ service.go │                          │   │
│ │   ✚ handler.go │  func NewService(...)    │   │
│ │   ✎ model.go   │    return &Service{...}  │   │
│ │          │  }                              │   │
│ └──────────┴────────────────────────────────┘   │
│                                                  │
│ commit: feat(auth): add OAuth service            │
│ 变更: 3 files (+180 / -0)                        │
└─────────────────────────────────────────────────┘
```

**交互**：
- 点击文件名 → 切换右侧代码内容
- 文件图标标记操作类型：✚ create（绿色）、✎ modify（黄色）
- 折叠/展开目录

**Review 结果叠加**：REVIEWING 步骤完成后，在代码预览区显示 findings 列表：
- 按文件分组
- 每条 finding 显示：severity badge + line + message + suggestion
- 颜色：ERROR 红色 / WARNING 黄色 / INFO 蓝色

### 5.6 SSE 实时通信

复用 S4 已有的 SSE 端点 `GET /api/stream/tasks/:taskId`，事件类型扩展：

| 事件类型 | 数据 | 触发时机 |
|---------|------|---------|
| TASK_PROGRESS | {status, step_type, progress} | 任务状态变更 |
| STEP_COMPLETE | {step_type, output_summary} | 步骤完成 |
| REVIEW_RESULT | {score, passed, findings_count} | Review 完成 |
| ERROR | {message, step_type} | 错误发生 |

---

## 6. 主编排流程

完整的端到端流程（技术设计 §2.5）：

```
用户提交需求 (SUBMITTED)
    │
    ▼
多轮对话澄清 (ANALYZING)
    │  POST /messages → analyze_requirement_activity
    │  循环直到 status=confirmed
    ▼
用户确认需求 → 生成确认卡片
    │  POST /confirm
    ▼
任务拆解 (PLANNING)
    │  plan_task_activity
    ▼
代码生成 (GENERATING)
    │  generate_code_activity
    ▼
AI 审查 (REVIEWING)
    │  review_code_activity
    │
    ├── passed=true → COMPLETED ✅
    │
    └── passed=false
        │  fix_instructions → generate_code_activity (修复)
        │  → review_code_activity (重审)
        │  最多 3 轮
        │
        ├── 通过 → COMPLETED ✅
        └── 超限 → FAILED (需人工介入) ❌
```

---

## 7. 错误处理与重试

| 失败场景 | 最大重试 | 处理 | 用户通知 |
|---------|---------|------|---------|
| LLM API 调用失败 | 3次 | Temporal 自动重试 + 模型降级 | 透明，用户无感 |
| LLM 返回格式错误 | 2次 | 重试解析，fallback 纯文本 | 透明 |
| AI 代码生成质量不达标 | 3轮 | Review → Fix 循环 | 显示重试进度 |
| 全部模型不可用 | 0次 | 标记 FAILED | "AI 服务暂不可用" |
| Temporal Worker 崩溃 | 自动 | Activity heartbeat 超时 → 重分配 | 透明，自动恢复 |

---

## 8. Phase 1 简化与后续计划

| 特性 | Phase 1 (S6) | 后续版本 |
|------|-------------|---------|
| Token 流式输出 | Activity 返回完整结果 | S7+ 可加 token-by-token |
| 文件生成 | 串行单次生成 | S7 并行（Temporal Child Workflow） |
| 外部 Linter | 仅 AI Review | S7 constraint-worker (golangci-lint/eslint/Semgrep) |
| GitHub 提交 | 不提交，仅生成代码 | S7 devops-worker (独立分支 + MR) |
| 自动化测试 | 不生成测试 | S7 TestGen Agent + 原生框架运行 |
| 成本控制 | 记录 model_calls，无硬限 | Phase 2 Token 预算 + 熔断 |
| 风险评估 | 简单规则 (HIGH/MEDIUM/LOW) | Phase 2 两阶段 AI 评估 |
| 代码预览语法高亮 | Geist Mono + pre 渲染 | 后续升级 shiki/Monaco |
| Review 行内标注 | 文件级 findings 列表 | 后续做行内标注 |
| 人工审批流 | 超限标记 FAILED | Phase 2 审批卡片 + IM 通知 |
| 并发冲突处理 | 不涉及（不提交代码） | S7 独立分支 + 冲突检测 |
| DB 迁移安全 | 不涉及 | S7 可逆迁移 + 危险操作拦截 |
| KillSwitch 紧急停止 | 不实现 | Phase 2 三级停止机制 |

---

## 9. 环境变量

```env
# AI Worker
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
DASHSCOPE_API_KEY=sk-...
DEEPSEEK_API_KEY=sk-...
TEMPORAL_HOST=localhost:7233
FORGE_API_URL=http://localhost:8080
FORGE_API_TOKEN=<internal service token>

# forge-core 新增
AI_WORKER_TASK_QUEUE=ai-worker
```

---

## 10. 实施任务拆解

| # | 任务 | 技术栈 | 核心交付 |
|---|------|--------|---------|
| 1 | Python 项目骨架 | Python 3.12 | pyproject.toml, config, 目录结构, requirements.txt |
| 2 | 多模型路由器 | Python | Claude/GPT/Qwen/DeepSeek 4 级降级链 + 熔断器 |
| 3 | 上下文构建器 | Python | 四层 Prompt 叠加 + Token 预算优化 |
| 4 | 4 个 AI Agent | Python | analyst/planner/coder/reviewer + JSON 结构化输出 |
| 5 | Temporal Activities + Worker | Python | 4 个 activity + worker 入口 + docker-compose |
| 6 | Go — DB 迁移 + 对话 API + 真实 Workflow | Go | conversations/model_calls/review_results 表, 对话 CRUD, 替换骨架 workflow |
| 7 | 前端 — Chat UI + 代码预览 | Next.js | 对话页面, 消息气泡, 确认卡片, 代码预览面板, 文件树 |
| 8 | 集成测试 + docker-compose | All | 端到端验证, AI Worker 容器化 |

---

## 11. 验收标准

- [ ] Python AI Worker 启动并连接 Temporal（docker-compose 一键启动）
- [ ] 创建任务 → 多轮对话 → AI 澄清问题 → 确认需求
- [ ] 确认后自动执行：plan → generate → review 完整流水线
- [ ] 4 级模型降级链工作（任一模型不可用时自动切换）
- [ ] Review 不通过 → 自动修复重试（最多 3 轮）
- [ ] model_calls 表记录每次 AI 调用的 model/tokens/cost/latency
- [ ] 前端 Chat UI 多轮对话 + 确认卡片
- [ ] 任务详情页显示步骤时间线 + 代码预览面板
- [ ] 代码预览：文件树导航 + 代码展示 + Review findings
- [ ] SSE 实时推送任务进度
