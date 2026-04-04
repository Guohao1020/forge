# Forge Harness Engineering 架构设计

> **版本**: 1.0
> **日期**: 2026-04-05
> **作者**: Claude (设计) + Harvey (审阅)
> **前置**: [phase2-strategic-replan.md](phase2-strategic-replan.md) | [learn-claude-code](https://github.com/shareAI-lab/learn-claude-code)

---

## 一、设计动机

### 1.1 核心洞察

learn-claude-code 的核心教训：**"The model is the agent. The code is the harness."**

Forge 当前 AI Worker 的问题：
- 每个 Agent 调用重复获取相同上下文（20 次 HTTP / 工作流）
- 所有上下文塞进 system prompt（Analyzer 收到 DB Schema，Reviewer 收到业务规则）
- 单 Agent 串行执行，无法并行处理独立任务
- **没有并发任务协调** — 同一项目多个需求并行时无冲突检测

### 1.2 真实场景需求

一个项目 v1.2 迭代中，PM 同时提了 3 个需求：
```
需求 A: 给用户加积分功能（修改 UserService + 新建 PointsService）
需求 B: 优化订单列表性能（修改 OrderService + OrderRepository）
需求 C: 加一个导出 Excel 功能（修改 UserController + 新建 ExportService）
```

需求 A 和 C 都要改 User 相关文件 → 可能冲突。
需求 B 独立 → 可以并行。
三个需求都完成后 → 打 v1.2 tag，部署。

**当前 Forge 完全无法处理这个场景。** 没有版本概念，没有冲突检测，没有并行协调。

---

## 二、架构总览

### 2.1 三层架构

```
┌─────────────────────────────────────────────────────────┐
│  L1: Harness 基座层                                      │
│  ContextCache · ContextScope · AgentLoop · ModelRouter    │
│  纯基础设施，零质量风险                                     │
├─────────────────────────────────────────────────────────┤
│  L2: 上下文工具层                                         │
│  query_db_schema · query_api_catalog · read_project_file │
│  query_business_rules · query_module_graph                │
│  Agent 按需查询，质量可能更好                               │
├─────────────────────────────────────────────────────────┤
│  L3: 项目协调层                                           │
│  ProjectOrchestrator · ConflictDetector · MergeQueue     │
│  VersionManager · ReleaseManager                          │
│  多需求并行 + 版本迭代 + 冲突兜底                           │
└─────────────────────────────────────────────────────────┘
```

### 2.2 各层职责边界

| 层 | 改什么 | 不改什么 | 质量影响 |
|----|--------|---------|---------|
| L1 | ContextBuilder、BaseAgent、Activity 调用链 | Agent prompt、输出格式 | 零（纯优化） |
| L2 | BaseAgent.run() 改为 agent loop + tools | Agent 的核心 prompt 逻辑 | 正面（更精准的信息） |
| L3 | 新增 ProjectOrchestrator 工作流 + 数据模型 | 现有 TaskWorkflow | 正面（冲突预防） |

---

## 三、L1 — Harness 基座层

### 3.1 ContextCache（工作流级缓存）

**问题**: 5 个 Activity 各自调 ContextBuilder.build()，20 次 HTTP。

**方案**: Redis 缓存，一次获取全工作流复用。

```python
class ContextCache:
    """工作流级别的上下文缓存"""

    CACHE_KEY = "ctx:{workflow_id}"
    TTL = 1800  # 30 分钟

    async def get_or_build(self, workflow_id, project_id, purpose):
        cached = await redis.get(self.CACHE_KEY.format(workflow_id=workflow_id))
        if cached:
            return ProjectContext.from_json(cached)

        context = await ContextBuilder().build(project_id, purpose)
        await redis.setex(
            self.CACHE_KEY.format(workflow_id=workflow_id),
            self.TTL,
            context.to_json()
        )
        return context
```

**接入方式**: Activity 函数接收 `workflow_id` 参数，用 ContextCache 替代直接调用 ContextBuilder.build()。

**质量影响**: 零。数据完全一致。

### 3.2 并行上下文获取

**问题**: ContextBuilder.build() 串行调 4 个 API。

**方案**: asyncio.gather 并行。

```python
async def build(self, project_id, purpose):
    project, specs, prompts, profiles = await asyncio.gather(
        self._fetch_project(project_id),
        self._fetch_effective_specs(project_id),
        self._fetch_prompt_template(purpose),
        self._fetch_profiles(project_id),
    )
    return ProjectContext(
        project_name=project["name"],
        # ... 组装
    )
```

**质量影响**: 零。只是网络请求并行。

### 3.3 Agent Loop 改造

**问题**: BaseAgent.run() 是单轮 LLM 调用，无法使用工具。

**方案**: 改为 learn-claude-code s01 的标准 agent loop。

```python
class BaseAgent:
    MAX_TOOL_ROUNDS = 5

    async def run(self, user_input, context, tools=None):
        system = self._build_system_prompt(context)
        messages = self._build_messages(user_input, context)

        for round in range(self.MAX_TOOL_ROUNDS + 1):
            response = await self.router.chat(
                system=system,
                messages=messages,
                purpose=self.purpose,
                tools=tools,  # 新增：传入可用工具
            )

            if response.stop_reason != "tool_use":
                # 最终输出，解析 JSON
                return self._build_result(response)

            # 处理工具调用
            tool_results = []
            for tool_call in response.tool_calls:
                result = await self._execute_tool(tool_call, context)
                tool_results.append(result)

            messages.append({"role": "assistant", "content": response.raw_content})
            messages.append({"role": "user", "content": tool_results})

        # 超出轮次限制，强制要求最终输出
        messages.append({"role": "user", "content": "请立即输出最终 JSON 结果，不要再调用工具。"})
        response = await self.router.chat(system=system, messages=messages, purpose=self.purpose)
        return self._build_result(response)
```

**关键约束**:
- 最多 5 轮工具调用（防止无限循环和 token 浪费）
- 不使用工具时行为和旧版完全一致（tools=None 时单轮调用）
- 每个 Agent 子类决定自己要不要启用工具
- 每次工具调用有 10 秒超时；相同参数的重复调用返回缓存结果（不计入轮次）
- 累积 token 超过 budget 的 80% 时提前终止工具循环
- 强制输出轮如果仍然产出无效 JSON，返回空结果并标记 `parse_failed=True`

**ModelRouter 扩展**: `router.chat()` 需新增 `tools` 参数。不同 provider 的 tool calling 格式差异由 router 内部适配：
- Anthropic: 原生 `tools` 参数
- OpenAI/DashScope/DeepSeek: `tools` 参数（OpenAI 兼容格式）
- 不支持 tool calling 的 provider: 跳过，尝试下一个

**质量影响**: 零到正面。不用工具时行为不变；用工具时信息更精准。

---

## 四、L2 — 上下文工具层

### 4.1 上下文查询工具定义

```python
CONTEXT_TOOLS = [
    {
        "name": "query_api_catalog",
        "description": "查询项目的 API 接口清单。当你需要了解现有 API 路径、参数、返回值时调用。传入关键词过滤。",
        "input_schema": {
            "type": "object",
            "properties": {
                "keyword": {"type": "string", "description": "搜索关键词，如 'user' 或 '/api/orders'"}
            },
            "required": ["keyword"]
        }
    },
    {
        "name": "query_db_schema",
        "description": "查询项目的数据库表结构（字段、类型、索引、外键）。当你需要设计数据模型或写 SQL 时调用。",
        "input_schema": {
            "type": "object",
            "properties": {
                "table_name": {"type": "string", "description": "表名或关键词"}
            },
            "required": ["table_name"]
        }
    },
    {
        "name": "query_business_rules",
        "description": "查询项目的业务规则约束（如'积分不能为负'、'订单超时30分钟自动取消'）。当你需要了解业务逻辑边界时调用。",
        "input_schema": {
            "type": "object",
            "properties": {
                "domain": {"type": "string", "description": "业务域关键词，如 'user' 或 'payment'"}
            },
            "required": ["domain"]
        }
    },
    {
        "name": "query_module_graph",
        "description": "查询项目的模块依赖关系。当你需要了解代码组织结构和 import 路径时调用。",
        "input_schema": {
            "type": "object",
            "properties": {
                "module_name": {"type": "string", "description": "模块名"}
            },
            "required": ["module_name"]
        }
    },
    {
        "name": "read_project_file",
        "description": "读取项目仓库中的源代码文件。当你需要参考现有代码实现风格、理解已有逻辑时调用。",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "文件路径，如 'internal/module/user/service.go'"}
            },
            "required": ["path"]
        }
    }
]
```

### 4.2 工具执行器

```python
class ContextToolExecutor:
    """执行上下文查询工具，从缓存的 ProjectContext 或 API 获取数据"""

    def __init__(self, context: ProjectContext, project_id: int):
        self.context = context
        self.project_id = project_id

    async def execute(self, tool_call) -> str:
        name = tool_call["name"]
        args = tool_call["input"]

        if name == "query_api_catalog":
            return self._search_profiles("api_catalog", args["keyword"])
        elif name == "query_db_schema":
            return self._search_profiles("db_schema", args["table_name"])
        elif name == "query_business_rules":
            return self._search_profiles("business_rules", args["domain"])
        elif name == "query_module_graph":
            return self._search_profiles("module_graph", args["module_name"])
        elif name == "read_project_file":
            return await self._read_file(args["path"])
        else:
            return f"Unknown tool: {name}"

    def _search_profiles(self, dimension, keyword):
        """从缓存的画像数据中按关键词搜索"""
        profile = self.context.project_profiles.get(dimension, {})
        if not profile:
            return f"项目画像中没有 {dimension} 数据。请先触发项目画像扫描。"

        # 关键词过滤（简单实现，后续可用 pgvector 语义搜索）
        keyword_lower = keyword.lower()
        results = []
        # ... 按 dimension 结构遍历，匹配 keyword

        if not results:
            return f"未找到与 '{keyword}' 相关的 {dimension} 数据。"

        return json.dumps(results, ensure_ascii=False, indent=2)

    async def _read_file(self, path):
        """通过 API 读取项目文件（使用注入的 httpx client，避免连接泄漏）"""
        async with httpx.AsyncClient(timeout=10) as client:
            resp = await client.get(
                f"{settings.forge_api_url}/api/projects/{self.project_id}/code/file",
                params={"path": path, "ref": "main"},
                headers={"Authorization": f"Bearer {settings.forge_api_token}"},
            )
        if resp.status_code == 200:
            data = resp.json().get("data", {})
            content = data.get("content", "")
            if len(content) > 20000:
                return content[:20000] + "\n\n... [文件截断，仅显示前 20000 字符]"
            return content
        return f"无法读取文件 {path}（状态码 {resp.status_code}）"
```

### 4.3 各 Agent 的工具策略

| Agent | 启用工具？ | 理由 |
|-------|-----------|------|
| AnalystAgent | 否 | 需求分析阶段不需要代码细节（前置条件：SX-1 已接线） |
| PlannerAgent | 是（query_api_catalog, query_module_graph, read_project_file） | 规划任务需要了解现有 API 和模块结构 |
| TestWriterAgent | 是（query_db_schema, query_api_catalog, read_project_file） | 写测试需要了解数据结构和 API 契约 |
| CoderAgent | 是（全部 5 个工具） | 代码生成需要最完整的上下文 |
| ReviewerAgent | 是（read_project_file, query_business_rules） | Review 时参考现有代码风格和业务规则 |

**画像数据可用性感知**: 在 system prompt 中注入画像摘要行，让 Agent 知道哪些维度有数据：
```
Available profile data: api_catalog (23 endpoints), db_schema (12 tables), business_rules (8 rules), module_graph (empty), coding_habits (empty)
```
Agent 看到 "empty" 的维度不会浪费工具调用轮次去查询。如果项目画像尚未扫描（SX-3 未完成），提示 "项目画像未扫描，建议先触发扫描"。

**前置依赖**: SH-2 的工具层假设 SX-1（AnalystAgent 接线）和 SX-3（画像扫描接线）已完成。如果未完成，工具返回降级响应但不阻塞 Agent 执行。

### 4.4 System Prompt 调整

**变化**: 把画像数据从 system prompt 移到工具中。编码规范保留在 system prompt（这是核心约束，不能延迟加载）。

```
旧 system prompt:
  Agent 基础 prompt
  + ## Coding Standards (保留)
  + ## Project (保留: name, description, tech_stack)
  + ## 项目画像（移除 — 改为工具查询）
    - API 接口清单 (移除)
    - 数据库结构 (移除)
    - 模块依赖图 (移除)
    - 业务规则 (移除)

新 system prompt:
  Agent 基础 prompt
  + ## Coding Standards (保留)
  + ## Project (保留: name, description, tech_stack)
  + ## Available Tools (新增: 工具列表说明)
  + "你可以通过工具查询项目的 API、数据库、模块和业务规则。在生成代码前，先查询相关上下文。"

效果: system prompt 从 ~8000 tokens 降到 ~3000 tokens
     省出的空间用于更多的 user message（代码上下文、对话历史）
```

---

## 五、L3 — 项目协调层（多需求并行 + 版本管理）

### 5.1 核心数据模型扩展

**新增 project_versions 表**:
```sql
CREATE TABLE engine.project_versions (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    BIGINT NOT NULL,       -- 多租户隔离（必需）
    project_id   BIGINT NOT NULL REFERENCES engine.projects(id),
    version      VARCHAR(50) NOT NULL,  -- "1.2.0"
    status       VARCHAR(20) NOT NULL DEFAULT 'PLANNING',
    -- PLANNING: 收集需求中
    -- IN_PROGRESS: 有任务在执行
    -- TESTING: 所有任务完成，等待验证
    -- RELEASED: 已发布
    -- CANCELLED: 已取消（关联任务状态不变，仅版本停止跟踪）
    description  TEXT,                  -- 版本描述
    released_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_project_version UNIQUE (project_id, version)
);
CREATE INDEX idx_project_versions_tenant ON engine.project_versions(tenant_id);

-- 注意：task_ids 不存储在此表。通过 tasks.version_id 外键反查。
-- 查询版本关联任务: SELECT * FROM engine.tasks WHERE version_id = ? AND tenant_id = ?
```

**扩展 tasks 表**:
```sql
ALTER TABLE engine.tasks
    ADD COLUMN version_id BIGINT REFERENCES engine.project_versions(id),
    ADD COLUMN conflict_status VARCHAR(20) DEFAULT 'NONE',
    -- NONE: 无冲突
    -- DETECTED: 检测到潜在冲突（文件重叠）
    -- WAITING: 等待前置任务合并
    -- RESOLVED: 冲突已解决
    ADD COLUMN blocked_by JSONB DEFAULT '[]',  -- 阻塞当前任务的其他 task_id
    ADD COLUMN touched_files JSONB DEFAULT '[]'; -- AI 预计修改的文件列表
```

### 5.2 版本工作流

**用户视角（非技术人员）**:
```
PM 操作:
1. 进入项目，点击 "创建新版本" → 输入 "v1.2 — 用户积分功能"
2. 在该版本下创建需求:
   - "给用户加积分功能"
   - "优化订单列表性能"
   - "加一个导出 Excel 功能"
3. 每个需求独立进入 AI 流水线
4. 看板展示: 版本维度 → 需求列表 → 各需求状态
5. 所有需求完成 → 版本状态变为 "待发布"
6. 点击 "发布版本" → 自动打 tag + 部署
```

**系统内部**:
```
创建版本 v1.2
  → 状态: PLANNING

创建任务 A（积分功能）→ 关联 v1.2
创建任务 B（订单性能）→ 关联 v1.2
创建任务 C（导出功能）→ 关联 v1.2
  → 版本状态: IN_PROGRESS

任务 A 进入 PLANNING 阶段
  → PlannerAgent 输出 touched_files: ["UserService.go", "PointsService.go"]
任务 C 进入 PLANNING 阶段
  → PlannerAgent 输出 touched_files: ["UserController.go", "ExportService.go"]
  → ConflictDetector: A 和 C 都涉及 User 相关文件
  → 但具体文件不重叠 → conflict_status: NONE，允许并行

任务 B 独立 → 直接并行执行

假设 A 和 C 都要改 UserController.go:
  → ConflictDetector: 文件重叠！
  → 任务 C 的 conflict_status: DETECTED
  → 策略: C.blocked_by = [A.id]，C 等 A 合并后再执行
  → 通知 PM: "导出功能需等待积分功能完成后再开发（文件冲突: UserController.go）"

所有任务 COMPLETED
  → 版本状态: TESTING
  → 运行集成测试

测试通过
  → PM 点击 "发布"
  → git tag v1.2.0 + 部署
  → 版本状态: RELEASED
```

### 5.3 ProjectOrchestrator（Temporal 工作流）

```go
// VersionOrchestrator 是每个版本的协调工作流（非项目级，避免无限历史膨胀）
// 生命周期：版本创建 → 所有任务完成/版本取消 → 工作流结束
func VersionOrchestrator(ctx workflow.Context, input VersionOrchestratorInput) error {
    taskCh := workflow.GetSignalChannel(ctx, "new_task")
    completedCh := workflow.GetSignalChannel(ctx, "task_completed")
    failedCh := workflow.GetSignalChannel(ctx, "task_failed")
    cancelCh := workflow.GetSignalChannel(ctx, "cancel_version")

    activeTasks := input.ActiveTasks // 从 ContinueAsNew 恢复状态
    eventCount := 0

    for {
        selector := workflow.NewSelector(ctx)

        // 新任务到达
        selector.AddReceive(taskCh, func(ch workflow.ReceiveChannel, more bool) {
            var task NewTaskSignal
            ch.Receive(ctx, &task)
            eventCount++

            // 冲突检测（文件级 + 包级）
            conflicts := detectConflicts(task, activeTasks)

            if len(conflicts) == 0 {
                startTaskWorkflow(ctx, task)
                activeTasks[task.ID] = &TaskState{Status: "RUNNING"}
            } else {
                activeTasks[task.ID] = &TaskState{
                    Status: "WAITING", BlockedBy: conflicts,
                }
                notifyConflict(ctx, task, conflicts)
            }
        })

        // 任务完成
        selector.AddReceive(completedCh, func(ch workflow.ReceiveChannel, more bool) {
            var completed TaskCompletedSignal
            ch.Receive(ctx, &completed)
            eventCount++
            delete(activeTasks, completed.TaskID)
            unblockWaitingTasks(ctx, activeTasks, completed.TaskID)

            if allTasksCompleted(ctx, input.VersionID) {
                updateVersionStatus(ctx, input.VersionID, "TESTING")
                return // 版本所有任务完成，工作流自然结束
            }
        })

        // 任务失败 → 级联解除或标记依赖任务
        selector.AddReceive(failedCh, func(ch workflow.ReceiveChannel, more bool) {
            var failed TaskFailedSignal
            ch.Receive(ctx, &failed)
            eventCount++
            delete(activeTasks, failed.TaskID)

            // 被阻塞的任务：从 blocked_by 中移除失败任务
            // 如果 blocked_by 变空，解除阻塞（冲突是推测性的，可以尝试）
            unblockWaitingTasks(ctx, activeTasks, failed.TaskID)
        })

        // 版本取消
        selector.AddReceive(cancelCh, func(ch workflow.ReceiveChannel, more bool) {
            ch.Receive(ctx, nil)
            updateVersionStatus(ctx, input.VersionID, "CANCELLED")
            return // 工作流结束
        })

        selector.Select(ctx)

        // ContinueAsNew 防止历史膨胀（每 50 个事件重置一次）
        if eventCount >= 50 {
            return workflow.NewContinueAsNewError(ctx, VersionOrchestrator,
                VersionOrchestratorInput{
                    VersionID:   input.VersionID,
                    ActiveTasks: activeTasks,
                })
        }
    }
}
```

### 5.4 冲突检测策略

**三层冲突检测**（全部必须执行，不是可选层）:

| 层 | 时机 | 方法 | 动作 | 可靠性 |
|----|------|------|------|--------|
| 规划期检测 | PlannerAgent 输出 touched_files 后 | 文件列表交集 + **包级**交集 | 阻塞后置任务或标记警告 | 启发式（可能漏报/误报） |
| 生成期检测 | CoderAgent 实际生成代码后 | git merge-tree 模拟 | 暂停 + 通知 | 精确 |
| 合并期检测 | PR 准备合并时 | GitHub API 冲突检查 | AI rebase（1次）→ 失败通知人 | 精确 |

**重要**: 规划期检测是启发式的，不保证准确。PlannerAgent 可能预测错误（遗漏文件或包含无关文件）。因此生成期和合并期检测是强制兜底层，不可跳过。

**当 touched_files 为空或解析失败时**: 默认标记为 "潜在冲突"（DETECTED），通知 PM 但不阻塞执行。生成期检测会给出精确结果。

**包级检测增强**: 如果两个任务的 touched_files 在同一个 Go package 或同一个目录下，即使具体文件不同也标记为 WARNING（不阻塞，但提示 PM）。

**规划期检测（成本最低，第一道防线）**:
```python
def detect_file_conflicts(new_task_files, active_tasks):
    """检测新任务与所有活跃任务的文件冲突"""
    conflicts = []
    for task_id, task_state in active_tasks.items():
        if task_state.status in ("RUNNING", "WAITING"):
            overlap = set(new_task_files) & set(task_state.touched_files)
            if overlap:
                conflicts.append({
                    "task_id": task_id,
                    "task_title": task_state.title,
                    "overlapping_files": list(overlap),
                })
    return conflicts
```

### 5.5 版本号管理

**Semantic Versioning 规则**:
- PM 创建版本时指定 major.minor（如 v1.2）
- patch 号自动递增（v1.2.0, v1.2.1 hotfix...）
- AI 生成的 commit 遵循 Conventional Commits 格式

**版本 API**:
```
POST /api/projects/:id/versions        创建新版本
GET  /api/projects/:id/versions        列表（含任务进度）
GET  /api/projects/:id/versions/:vid   详情（含所有任务状态）
PUT  /api/projects/:id/versions/:vid   更新描述/状态
POST /api/projects/:id/versions/:vid/release  发布版本
```

**发布版本流程**:
```
POST /release
  → 检查所有关联任务状态 == COMPLETED
  → 检查所有 PR 已合并
  → git tag v{major}.{minor}.{patch}
  → 触发部署流水线（如果配置了）
  → 版本状态 → RELEASED
```

### 5.6 并行执行与顺序合并

**执行**: 无冲突的任务并行执行（各自独立分支）。
**合并**: 按完成顺序依次合并到 default_branch。

```
Task A (积分) ──────生成──────Review──────合并✓
Task B (订单) ────生成────Review────合并✓
Task C (导出) ──等待A完成──生成──Review──rebase──合并✓
                                         ↑
                                   基于 A 合并后的 main rebase
```

**合并策略**:
1. 先完成的先合并（FIFO）
2. 合并前自动 rebase 到最新 main
3. Rebase 冲突 → AI 尝试自动解决（1 次）
4. 自动解决失败 → 标记 CONFLICT → 通知技术管理者

---

## 六、Phase 2b 重设计（Harness-First）

基于以上设计，Phase 2b 从原来的"S9/S10/S11 功能增强"重设计为：

### 新 Phase 2b 切片

| 切片 | 标题 | 内容 | 天数 |
|------|------|------|------|
| SH-1 | Harness 基座 | ContextCache + 并行获取 + Agent Loop + ModelRouter tools 支持 | 3 |
| SH-2 | 上下文工具 | 5 个 context tools + ContextToolExecutor + Agent 接入 + 画像可用性感知 | 2 |
| SH-3a | 版本模型 + API | project_versions 表 + 版本 CRUD API + tasks 扩展字段 | 2 |
| SH-3b | 版本协调器 | VersionOrchestrator + 冲突检测 + 信号协调 + ContinueAsNew | 3 |
| SH-4 | 版本管理 UI | 版本列表/详情页 + 任务关联 + 冲突状态展示 + 发布按钮 | 2 |
| S9' | 任务拆分增强 | DAG 可视化 + touched_files 输出 + 冲突预标记 | 2 |
| S10' | 测试先行 | TestWriterAgent + context tools 集成 + 测试预览 UI | 2 |
| S11' | 代码生成增强 | CoderAgent + context tools + Lint + 并行子任务（仅独立任务） | 2 |

**总计**: ~18 天（比原 Phase 2b 多 7 天，但增加了 Harness 基座、版本管理和并发协调）

### 执行顺序

```
SH-1 (基座) → SH-2 (工具) → S9' (拆分) → S10' (测试) → S11' (生成)
                                ↓
                         SH-3 (协调器，可并行)
                                ↓
                         SH-4 (版本 UI)
```

SH-3 和 S9' 之后可以并行开发 — SH-3 改的是 Temporal 工作流层，S9'/S10'/S11' 改的是 AI Agent 层，互不干扰。

---

## 七、更新后的完整 Phase 2 时间线

```
Phase 2a — 补洞（3-4 天）
├── SX-1 接线需求分析
├── SX-2 验证规范注入
├── SX-3 接线画像扫描
└── SX-4 端到端验证

Phase 2b — Harness Engineering（18 天）     ← 重设计
├── SH-1 Harness 基座 + ModelRouter tools（3天）
├── SH-2 上下文工具 + 画像可用性感知（2天）
├── SH-3a 版本模型 + CRUD API（2天）
├── SH-3b 版本协调器 + 冲突检测（3天）
├── SH-4 版本管理 UI（2天）
├── S9'  任务拆分增强（2天）
├── S10' 测试先行（2天）
└── S11' 代码生成增强（2天）

Phase 2c — 基础设施（5-7 天）
├── Infra-1 最小 K8s 环境
├── S12 自动化测试执行
└── S13 制品管理

Phase 2d — 部署闭环（5-7 天）
├── S14 K8s 部署
├── S16 画像 RAG 增强
└── S17 云端预览

总计: ~35 天工作日（7 周）
```

---

## 八、质量保障体系

### 8.1 不降质量的保证

| 改造 | 保证 |
|------|------|
| ContextCache | 数据完全一致，只是来源从 HTTP 变为 Redis |
| Agent Loop | tools=None 时行为和旧版 100% 一致 |
| Context Tools | Agent 不查询工具时，输出和旧版一致；查询时信息更精准 |
| 并行任务 | 冲突检测阻止有风险的并行；Review 仍然是最终门禁 |
| 子 Agent 并行 | 仅对 Planner 标记为独立（depends_on=[]）的任务并行 |

### 8.2 质量验证方案

每个切片完成后的验证：

| 切片 | 验证方法 |
|------|---------|
| SH-1 | 对比旧版：相同输入 → Agent 输出 diff 为零 |
| SH-2 | 用真实项目测试：CoderAgent 使用工具查询 vs 旧版全注入，Review 评分对比 |
| SH-3 | 模拟 3 个并发任务：验证冲突检测、等待/解除、版本状态流转 |
| SH-4 | 端到端：创建版本 → 添加 3 个任务 → 并行执行 → 发布 |
| S9'-S11' | 旧版对比：相同需求/项目，新旧版 Review 评分 + 测试通过率对比 |

---

## 附录 A: learn-claude-code 到 Forge 的映射

| learn-claude-code 概念 | Session | Forge 对应 | 状态 |
|------------------------|---------|-----------|------|
| Agent Loop | s01 | BaseAgent.run() → Agent Loop | SH-1 改造 |
| Tool Use | s02 | Context Tools (5 个) | SH-2 新增 |
| TodoWrite (计划) | s03 | Task DAG | S9' 增强 |
| Subagents (隔离) | s04 | 子 Agent 并行生成 | S11' 新增 |
| Skills (按需加载) | s05 | 画像从 prompt 移到工具 | SH-2 改造 |
| Context Compact | s06 | ContextCompressor (token 超限时) | SH-1 预留接口 |
| Tasks (持久化) | s07 | ProjectVersion + Task 关联 | SH-3 新增 |
| Background Tasks | s08 | Temporal async + SSE | 已实现 |
| Agent Teams | s09 | ProjectOrchestrator | SH-3 新增 |
| Team Protocols | s10 | 冲突检测 + 等待/解除 FSM | SH-3 新增 |
| Autonomous Agents | s11 | 不适用（当前单人团队） | — |
| Worktree Isolation | s12 | git worktree per task | 已实现 |
