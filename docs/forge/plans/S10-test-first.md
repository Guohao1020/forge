# S10 — 测试先行体系 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AI 在生成代码之前先生成测试用例（原生框架：Go→go test, Java→JUnit, JS→Jest），测试用例作为代码生成的约束输入，确保生成的代码可测试。

**Architecture:** 新增 TestWriterAgent（Python），在 PLAN 和 GENERATE 之间插入 TEST_WRITING 步骤。AI 根据计划输出 + 技术栈选择对应的测试框架生成测试代码。测试用例传递给 CoderAgent 作为约束。Workflow 扩展为 PLAN → TEST_WRITING → GENERATE → REVIEW → DEPLOY。

**Tech Stack:** Python 3.12 (TestWriterAgent), Go 1.22 (workflow 扩展), Next.js (前端展示)

**Dependencies:** S9 (任务拆分, 已完成)

---

## File Structure

### Python AI Worker
```
ai-worker/src/
├── agents/test_writer.py           # NEW: 测试用例生成 Agent
├── activities/test_writing.py      # NEW: 测试生成 Temporal activity
├── models/router.py                # MODIFY: 添加 Purpose.TEST_WRITING
└── worker.py                       # MODIFY: 注册新 activity
```

### Go 后端
```
forge-core/
├── internal/module/task/
│   └── model.go                    # MODIFY: 添加 StepTypeTestWriting + AllSteps 更新
├── internal/temporal/
│   ├── workflow/task_workflow.go    # MODIFY: 插入 TEST_WRITING 步骤
│   └── activity/task_activities.go # MODIFY: 无新 activity（复用 SaveStepOutput）
```

### 前端
```
forge-portal/
├── components/tasks/
│   └── task-workspace.tsx          # MODIFY: 添加 TEST_WRITING 步骤展示
├── components/tasks/
│   └── step-timeline.tsx           # MODIFY: 添加测试步骤摘要
```

---

## Task 1: Python — TestWriterAgent + Activity

**Files:**
- Create: `ai-worker/src/agents/test_writer.py`
- Create: `ai-worker/src/activities/test_writing.py`
- Modify: `ai-worker/src/models/router.py`
- Modify: `ai-worker/src/worker.py`

**重要**: 先完整读取 `agents/base.py`、`agents/coder.py`、`activities/generate.py`、`models/router.py`、`worker.py`、`context/builder.py` 了解现有模式。

- [ ] **Step 1: 添加 Purpose.TEST_WRITING 到 router.py**

读取 `ai-worker/src/models/router.py`，找到 `Purpose` 枚举，添加：

```python
class Purpose(Enum):
    ANALYZE = "analyze"
    PLAN = "plan"
    TEST_WRITING = "test_writing"  # NEW
    GENERATE = "generate"
    REVIEW = "review"
```

同时在 `ROUTING_RULES` 中为 `TEST_WRITING` 添加模型路由（与 GENERATE 相同的 fallback 链）。

- [ ] **Step 2: 创建 TestWriterAgent**

`ai-worker/src/agents/test_writer.py`:

```python
from __future__ import annotations
from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

TEST_WRITER_SYSTEM_PROMPT = """You are a senior test engineer. Your task is to write test cases BEFORE the implementation code exists. These tests define the expected behavior.

## Rules
1. Select test framework based on the project's tech stack:
   - Go → use "testing" package (go test)
   - Java → use JUnit 5
   - Python → use pytest
   - JavaScript/TypeScript → use Jest
   - If unknown → use pytest as default
2. For each implementation task in the plan, write corresponding test cases
3. Cover: happy path, edge cases, error cases (at least 2 cases per function)
4. Tests should be compilable/runnable even before implementation exists (use interfaces/mocks)
5. Use descriptive test names that explain the expected behavior

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown.

{"test_files": [{"path": "tests/user_test.go", "content": "package tests\\n\\nimport ...", "language": "go", "framework": "go_test", "covers_task": 1}], "test_count": 6, "framework": "go_test", "coverage_targets": ["UserService.Create", "UserService.Delete"]}
"""


class TestWriterAgent(BaseAgent):
    purpose = Purpose.TEST_WRITING

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = TEST_WRITER_SYSTEM_PROMPT

        # Inject tech stack for framework selection
        tech = context.tech_stack
        if tech:
            frameworks = tech.get("frameworks", [])
            languages = tech.get("languages", {})
            if frameworks or languages:
                base += f"\n\n## Detected Tech Stack\nLanguages: {languages}\nFrameworks: {frameworks}"

        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
```

- [ ] **Step 3: 创建 test_writing activity**

`ai-worker/src/activities/test_writing.py`:

```python
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.test_writer import TestWriterAgent
from src.context.builder import ContextBuilder
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)


@dataclass
class TestWritingInput:
    task_id: int
    tenant_id: int
    project_id: int
    plan: Optional[Dict[str, Any]] = None
    requirement_summary: Optional[str] = None


@dataclass
class TestWritingOutput:
    test_files: List[Dict[str, Any]] = field(default_factory=list)
    test_count: int = 0
    framework: str = ""
    coverage_targets: List[str] = field(default_factory=list)
    tokens_used: int = 0
    model: str = ""
    provider: str = ""
    latency_ms: int = 0


@activity.defn(name="generate_test_cases")
async def generate_test_cases_activity(input: TestWritingInput) -> TestWritingOutput:
    logger.info(f"Generating test cases for task {input.task_id}")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-generation")

        # Build user prompt from plan
        user_prompt = ""
        if input.requirement_summary:
            user_prompt += f"## Requirement\n{input.requirement_summary}\n\n"

        if input.plan:
            tasks = input.plan.get("tasks", [])
            if tasks:
                import json
                user_prompt += f"## Implementation Plan\n{json.dumps(tasks, indent=2, ensure_ascii=False)}\n\n"

        if not user_prompt.strip():
            user_prompt = "Generate test cases based on the project context."

        user_prompt += "\nGenerate test cases for the implementation tasks above. Write tests that will validate the code BEFORE it is written."

        router = ModelRouter()
        agent = TestWriterAgent(router)
        result = await agent.run(user_prompt, ctx)

        return TestWritingOutput(
            test_files=result.structured.get("test_files", []),
            test_count=result.structured.get("test_count", 0),
            framework=result.structured.get("framework", ""),
            coverage_targets=result.structured.get("coverage_targets", []),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await builder.close()
```

- [ ] **Step 4: 注册 activity 到 worker.py**

读取 `ai-worker/src/worker.py`，在 activities 列表中添加 `generate_test_cases_activity`。

- [ ] **Step 5: 验证 imports**

```bash
cd ai-worker && python -c "from src.activities.test_writing import generate_test_cases_activity; print('OK')"
```

- [ ] **Step 6: Commit**

```bash
git add ai-worker/
git commit -m "feat(s10): add TestWriterAgent and test case generation activity"
```

---

## Task 2: Go 后端 — 步骤类型 + Workflow 扩展

**Files:**
- Modify: `forge-core/internal/module/task/model.go`
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`

**重要**: 先完整读取 `model.go` 和 `task_workflow.go`。

- [ ] **Step 1: 添加 StepTypeTestWriting 到 model.go**

在 step type 常量中添加：
```go
StepTypeTestWriting = "TEST_WRITING"
```

更新 AllSteps 数组，在 PLAN 之后插入：
```go
{"方案规划", StepTypePlan},
{"测试设计", StepTypeTestWriting},  // NEW
{"代码生成", StepTypeGenerate},
```

同时添加对应的 task status：
```go
StatusTestWriting = "TEST_WRITING"
```

- [ ] **Step 2: Workflow 插入 TEST_WRITING 步骤**

在 `task_workflow.go` 中，PLAN 的 SaveStepOutput 之后、GENERATE 的 ExecuteStep 之前，插入：

```go
// ---- Step 2: Test Writing ----
err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
    TaskID: input.TaskID, StepType: "TEST_WRITING", TaskStatus: "TEST_WRITING", Duration: 0,
}).Get(ctx, nil)
if err != nil {
    _ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
    return err
}

var testResult map[string]interface{}
err = workflow.ExecuteActivity(aiCtx, "generate_test_cases", map[string]interface{}{
    "task_id":             input.TaskID,
    "tenant_id":           input.TenantID,
    "project_id":          input.ProjectID,
    "plan":                planResult,
    "requirement_summary": input.Requirement,
}).Get(ctx, &testResult)
if err != nil {
    logger.Warn("test case generation failed, continuing without tests", "error", err)
    // Non-blocking: if test generation fails, continue with code generation
    testResult = map[string]interface{}{}
}

_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "TEST_WRITING", testResult).Get(ctx, nil)
```

**关键设计决策**: TEST_WRITING 失败不阻断流程（warn + continue），因为代码生成不强依赖测试。

- [ ] **Step 3: 将测试用例传递给 GENERATE 步骤**

在 generate_code activity 调用中，添加 `test_cases` 参数：

```go
err = workflow.ExecuteActivity(aiCtx, "generate_code", map[string]interface{}{
    "task_id":             input.TaskID,
    "tenant_id":           input.TenantID,
    "project_id":          input.ProjectID,
    "requirement_summary": input.Requirement,
    "plan":                planResult,
    "test_cases":          testResult,  // NEW: pass test cases as constraint
}).Get(ctx, &generateResult)
```

- [ ] **Step 4: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/
git commit -m "feat(s10): insert TEST_WRITING step into workflow between PLAN and GENERATE"
```

---

## Task 3: Python — CoderAgent 接收测试约束

**Files:**
- Modify: `ai-worker/src/activities/generate.py`

- [ ] **Step 1: 读取 generate.py**

完整读取 `ai-worker/src/activities/generate.py`。

- [ ] **Step 2: GenerateInput 添加 test_cases 字段**

```python
@dataclass
class GenerateInput:
    task_id: int
    tenant_id: int
    project_id: int
    plan: Optional[Dict[str, Any]] = None
    requirement_summary: Optional[str] = None
    task_plan: Optional[List[Dict[str, Any]]] = None
    test_cases: Optional[Dict[str, Any]] = None  # NEW: from TEST_WRITING step
    fix_instructions: Optional[str] = None
    code: Optional[Dict[str, Any]] = None
    review: Optional[Dict[str, Any]] = None
```

- [ ] **Step 3: 在 user_prompt 中注入测试约束**

在 generate_code_activity 的 user_prompt 构建逻辑中，添加测试用例注入：

```python
# Inject test cases as constraints
if input.test_cases:
    test_files = input.test_cases.get("test_files", [])
    if test_files:
        import json
        user_prompt += f"\n## Test Cases (MUST PASS)\nThe following test cases have been written. Your code MUST make these tests pass.\n"
        for tf in test_files:
            user_prompt += f"\n### {tf.get('path', 'test')}\n```{tf.get('language', '')}\n{tf.get('content', '')}\n```\n"
        user_prompt += "\nGenerate implementation code that satisfies all the test cases above.\n"
```

- [ ] **Step 4: 验证 imports**

```bash
cd ai-worker && python -c "from src.activities.generate import generate_code_activity; print('OK')"
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/activities/generate.py
git commit -m "feat(s10): inject test cases as constraints into code generation"
```

---

## Task 4: 前端 — TEST_WRITING 步骤展示

**Files:**
- Modify: `forge-portal/components/tasks/task-workspace.tsx`
- Modify: `forge-portal/components/tasks/step-timeline.tsx`

- [ ] **Step 1: 读取现有代码**

完整读取 `task-workspace.tsx` 和 `step-timeline.tsx`。

- [ ] **Step 2: task-workspace.tsx 添加 TEST_WRITING 路由**

在步骤类型判断逻辑中（找到处理 PLAN/GENERATE/REVIEW 的 case），添加 TEST_WRITING：

```tsx
if (stepType === "TEST_WRITING") {
  if (status === "RUNNING") {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <Loader2 className="h-8 w-8 animate-spin text-purple-400" />
        <p className="text-muted-foreground">AI 正在生成测试用例...</p>
      </div>
    );
  }
  if (status === "COMPLETED" && output) {
    const testFiles = output.test_files || [];
    const framework = output.framework || "unknown";
    const testCount = output.test_count || testFiles.length;
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold">测试用例</h3>
          <div className="flex gap-2">
            <Badge variant="outline">{framework}</Badge>
            <Badge className="bg-purple-500/20 text-purple-300">{testCount} 个用例</Badge>
          </div>
        </div>
        {testFiles.map((f, i) => (
          <div key={i} className="rounded-lg border border-white/10 overflow-hidden">
            <div className="px-3 py-2 bg-white/5 text-sm text-muted-foreground">{f.path}</div>
            <ShikiCodeViewer code={f.content} language={f.language} fileName={f.path} />
          </div>
        ))}
      </div>
    );
  }
}
```

- [ ] **Step 3: step-timeline.tsx 添加 TEST_WRITING 摘要**

在步骤摘要提取逻辑中，添加 TEST_WRITING：

```tsx
case "TEST_WRITING":
  const testCount = parsed.test_count || (parsed.test_files?.length ?? 0);
  const framework = parsed.framework || "";
  summary = `生成 ${testCount} 个测试用例` + (framework ? ` (${framework})` : "");
  break;
```

- [ ] **Step 4: 验证前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(s10): add TEST_WRITING step display in task workspace and timeline"
```

---

## Task 5: 构建验证 + Docker 重建

- [ ] **Step 1: Go 构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: 前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: 重建 AI Worker**

```bash
docker compose -f docker-compose.dev.yml up -d --build ai-worker
```

- [ ] **Step 4: 端到端验证**

1. 重启 Go 后端（新 binary）
2. 创建新任务 → AI 分析 → 确认
3. Workflow 执行顺序验证：
   - PLAN 完成 → TEST_WRITING 开始（AI 生成测试用例）
   - TEST_WRITING 完成 → GENERATE 开始（代码生成收到测试约束）
   - GENERATE/REVIEW/DEPLOY 正常
4. 前端验证：
   - 步骤时间线显示 "测试设计" 步骤
   - 点击 TEST_WRITING 步骤 → 右侧显示测试代码
   - 步骤摘要显示 "生成 N 个测试用例 (go_test)"
5. 代码中包含测试文件（验证约束注入生效）

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s10): complete test-first system with test case generation before code"
```

---

## 验收标准

- [ ] Workflow 顺序：PLAN → TEST_WRITING → GENERATE → REVIEW → DEPLOY
- [ ] AI 根据技术栈选择正确的测试框架（Go→go test, Java→JUnit, JS→Jest）
- [ ] 测试用例包含 happy path + edge case + error case
- [ ] 测试用例作为约束传递给代码生成
- [ ] TEST_WRITING 失败不阻断流程（降级为无测试约束）
- [ ] 前端步骤时间线显示 "测试设计" + 摘要
- [ ] 前端工作区显示测试代码（Shiki 高亮）
- [ ] `go build` + `npm run build` + ai-worker 重建通过
