# SP-2 -- AI 方案推荐系统

**Duration**: 2 days
**Priority**: P2 -- 提升 AI 决策透明度，降低用户信任成本
**Dependencies**: SH-2 (上下文工具), S8 (需求澄清增强)
**Outputs**: RecommendationCard 数据结构 + AI Worker 推荐输出 + 前端推荐卡片 UI

---

## 1. Goal

当 AI 在分析/规划阶段识别出多种可行方案时，不再单方面选择一种执行，而是以结构化的 RecommendationCard 呈现给用户：每个方案的优缺点、风险等级、AI 推荐理由，让用户做出知情选择。推荐是上下文感知的 -- AI 使用项目画像（API 数量、模块图、架构类型）来评估每个方案的适用性。

---

## 2. Current State Analysis

### 2.1 What exists

1. **AnalystAgent** (`ai-worker/src/agents/analyst.py`): 分析用户需求，输出结构化的需求理解 + 风险评估 + 任务拆解
2. **PlannerAgent** (规划阶段): 将需求拆分为 DAG 任务图
3. **Chat UI** (`forge-portal/components/chat/`): 支持 OptionButtons、RiskAlert、ConfirmationCard 等结构化消息类型
4. **Project Profile** (`ProjectTypeProfile` from SP-1): 提供项目类型、技术栈、架构等上下文

### 2.2 What is missing

| Gap | Impact |
|-----|--------|
| AI 直接选择方案，用户无感知 | 用户不知道 AI 为什么这样拆分/设计，信任度低 |
| 无结构化推荐输出格式 | 即使 AI 在 system prompt 中考虑了多方案，结果也是纯文本 |
| 前端无推荐卡片组件 | 无法可视化展示方案对比 |
| 推荐不感知项目上下文 | AI 不知道项目当前有多少个 API、模块是否臃肿 |

---

## 3. Day 1 -- Recommendation Engine (AI Worker)

### 3.1 RecommendationCard data structure

在 AI Worker 输出中定义新的结构化类型：

```python
# ai-worker/src/models/recommendation.py (new file)

from dataclasses import dataclass, field
from typing import Optional

@dataclass
class RecommendationOption:
    id: str                          # "A", "B", "C"
    title: str                       # "方案 A: 在现有 UserService 中添加积分逻辑"
    description: str                 # 2-3 句话描述方案要点
    pros: list[str]                  # ["改动最小", "复用现有事务"]
    cons: list[str]                  # ["UserService 会变臃肿"]
    risk: str                        # "LOW" | "MEDIUM" | "HIGH"
    estimated_effort: Optional[str]  # "约 2 小时" | "约 1 天"
    affected_files: list[str]        # ["src/service/UserService.java"]
    recommended: bool                # True for the AI's pick
    reason: str                      # Why this is/isn't recommended for this project

@dataclass
class RecommendationCard:
    type: str = "recommendation"     # Message type identifier
    question: str = ""               # "积分功能应该放在哪里？"
    options: list[RecommendationOption] = field(default_factory=list)
    ai_recommendation: str = ""      # "A" -- the recommended option ID
    context_factors: list[str] = field(default_factory=list)  # Project context that influenced the recommendation
    trigger: str = ""                # "architecture_decision" | "task_decomposition" | "tech_stack" | "deploy_strategy"
```

### 3.2 Recommendation triggers

AI 在以下 4 个场景中可能输出 RecommendationCard：

| Trigger | Agent | When | Example |
|---------|-------|------|---------|
| `architecture_decision` | AnalystAgent | 需求分析发现多种架构可选 | 新功能放在现有服务 vs 独立服务 |
| `task_decomposition` | PlannerAgent | 任务拆分有多种策略 | 并行开发 vs 串行开发，先改 API 还是先改 DB |
| `tech_stack` | AnalystAgent | 项目类型允许多种框架 | ORM 选择 GORM vs sqlx，状态管理 Zustand vs Redux |
| `deploy_strategy` | PlannerAgent | 部署方式有多种选择 | 滚动部署 vs 蓝绿部署，风险等级模糊时 |

### 3.3 Modify AnalystAgent prompt

在 `ai-worker/src/agents/analyst.py` 的 system prompt 中添加推荐输出指令：

```python
RECOMMENDATION_INSTRUCTION = """
When you identify a decision point where multiple valid approaches exist,
output a recommendation block in the following JSON format embedded in your response:

```json
{
  "type": "recommendation",
  "question": "<the decision question>",
  "options": [
    {
      "id": "A",
      "title": "<concise option title>",
      "description": "<2-3 sentence description>",
      "pros": ["<pro 1>", "<pro 2>"],
      "cons": ["<con 1>"],
      "risk": "LOW|MEDIUM|HIGH",
      "estimated_effort": "<effort estimate>",
      "affected_files": ["<file paths>"],
      "recommended": true|false,
      "reason": "<why recommended/not for THIS project>"
    }
  ],
  "ai_recommendation": "<recommended option ID>",
  "context_factors": ["<project fact 1>", "<project fact 2>"],
  "trigger": "architecture_decision|task_decomposition|tech_stack|deploy_strategy"
}
```

Guidelines for recommendations:
- Only output recommendations when there are genuinely distinct approaches (not trivial variations)
- Maximum 3 options (if more exist, consolidate the less viable ones)
- The recommendation MUST be informed by the project profile context (API count, architecture type, team size, module complexity)
- Each option's reason field must reference specific project facts
- Pre-select the recommended option (recommended: true)
- If all options are roughly equivalent, say so and recommend the simplest one
"""
```

### 3.4 Context-aware recommendation logic

AI 使用以下项目上下文来评估方案适用性：

```python
# In ContextBuilder, include project metrics for recommendations
def build_recommendation_context(self, project_profile: dict) -> str:
    """Build a concise project context summary for recommendation decisions."""
    metrics = []

    # API scale
    api_count = len(project_profile.get("api_catalog", []))
    metrics.append(f"项目当前有 {api_count} 个 API 接口")

    # Module complexity
    modules = project_profile.get("module_graph", {}).get("nodes", [])
    metrics.append(f"有 {len(modules)} 个模块")

    # Architecture type
    arch = project_profile.get("architecture", {}).get("type", "unknown")
    metrics.append(f"架构类型: {arch}")

    # DB tables
    tables = project_profile.get("database_schema", [])
    metrics.append(f"数据库有 {len(tables)} 张表")

    # Team cadence (from SP-1)
    cadence = project_profile.get("teamCadence", "unknown")
    if cadence != "unknown":
        metrics.append(f"发布节奏: {cadence}")

    return "\n".join(f"- {m}" for m in metrics)
```

### 3.5 Response parsing

修改 AI Worker 的响应解析逻辑，从 AI 输出中提取 RecommendationCard：

```python
# ai-worker/src/utils/response_parser.py

import json
import re

def extract_recommendations(ai_response: str) -> list[dict]:
    """Extract RecommendationCard JSON blocks from AI response text."""
    pattern = r'```json\s*(\{[^`]*"type"\s*:\s*"recommendation"[^`]*\})\s*```'
    matches = re.findall(pattern, ai_response, re.DOTALL)

    recommendations = []
    for match in matches:
        try:
            card = json.loads(match)
            if card.get("type") == "recommendation" and card.get("options"):
                recommendations.append(card)
        except json.JSONDecodeError:
            continue

    return recommendations
```

当 AI 响应包含 RecommendationCard 时，将其作为结构化消息存入 conversation messages（`message_type = "recommendation"`），前端根据类型渲染。

### 3.6 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 1.1 | Define RecommendationCard dataclass | `ai-worker/src/models/recommendation.py` (new) | 30min |
| 1.2 | Add recommendation instruction to AnalystAgent prompt | `ai-worker/src/agents/analyst.py` | 45min |
| 1.3 | Add recommendation instruction to PlannerAgent prompt | `ai-worker/src/agents/planner.py` | 30min |
| 1.4 | Build recommendation context from project profile | `ai-worker/src/context/builder.py` | 45min |
| 1.5 | Implement response parser for recommendation extraction | `ai-worker/src/utils/response_parser.py` | 45min |
| 1.6 | Wire recommendation into conversation message flow | `ai-worker/src/activities/analyze.py` | 1h |
| 1.7 | Backend: handle "recommendation" message type in conversation service | `forge-core/internal/module/conversation/` | 45min |

---

## 4. Day 2 -- Frontend Recommendation UI

### 4.1 New component: `recommendation-card.tsx`

```
forge-portal/components/chat/recommendation-card.tsx (new)
```

**Layout**:

```
┌──────────────────────────────────────────────────────────────────┐
│  积分功能应该放在哪里？                                            │
│                                                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐ │
│  │ [AI 推荐]        │  │                  │  │                │ │
│  │ 方案 A           │  │ 方案 B           │  │ 方案 C         │ │
│  │ 在 UserService   │  │ 新建独立         │  │ 事件驱动       │ │
│  │ 中添加积分逻辑    │  │ PointsService   │  │ 积分系统       │ │
│  │                  │  │                  │  │                │ │
│  │ + 改动最小       │  │ + 职责清晰       │  │ + 完全解耦     │ │
│  │ + 复用现有事务   │  │ + 独立测试部署   │  │ + 可扩展性强   │ │
│  │ - Service 变臃肿 │  │ - 新增服务间调用 │  │ - 引入 MQ      │ │
│  │                  │  │ - 需定义新 API   │  │ - 复杂度高     │ │
│  │ 风险: LOW        │  │ 风险: MEDIUM     │  │ 风险: HIGH     │ │
│  │ 工时: ~2h        │  │ 工时: ~4h        │  │ 工时: ~1d      │ │
│  │                  │  │                  │  │                │ │
│  │ [选择此方案]     │  │ [选择此方案]     │  │ [选择此方案]   │ │
│  └──────────────────┘  └──────────────────┘  └────────────────┘ │
│                                                                  │
│  [v] 查看 AI 推荐依据                                             │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ 项目当前有 10 个 API，UserService 有 3 个方法               │  │
│  │ 项目未使用微服务架构                                        │  │
│  │ 积分功能与用户强关联                                        │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 4.2 Visual design specs

| Element | Style |
|---------|-------|
| Recommended card | Purple border (#8B5CF6), "AI 推荐" badge in top-left corner |
| Non-recommended card | Default border (zinc-700), no badge |
| Pros | Green text (emerald-400), "+" prefix |
| Cons | Red text (red-400), "-" prefix |
| Risk badge LOW | Green background |
| Risk badge MEDIUM | Yellow/amber background |
| Risk badge HIGH | Red background |
| Selected card | Filled purple background (subtle), check icon |
| Context factors section | Collapsible, muted text, monospace for metrics |
| Card hover | Subtle scale(1.02) + shadow transition |

### 4.3 User interaction flow

```
1. AI sends recommendation message
2. Chat renders RecommendationCard
3. Recommended option is visually highlighted (purple border + badge)
4. User clicks "选择此方案" on any card
5. Selection sent as user message: "我选择方案 {id}: {title}"
6. AI acknowledges and proceeds with the chosen approach
7. If user doesn't click but types a message instead, AI interprets freely
```

### 4.4 Integration with chat message types

修改 `forge-portal/components/chat/chat-message.tsx` 或等效消息渲染组件：

```typescript
// In message renderer
switch (message.type) {
    case "text":
        return <TextMessage ... />;
    case "confirmation":
        return <ConfirmationCard ... />;
    case "recommendation":
        return <RecommendationCard
            data={message.metadata as RecommendationData}
            onSelect={(optionId) => handleRecommendationSelect(optionId)}
        />;
    // ...
}
```

### 4.5 RecommendationCard TypeScript types

```typescript
// forge-portal/types/recommendation.ts (new)

export interface RecommendationOption {
    id: string;
    title: string;
    description: string;
    pros: string[];
    cons: string[];
    risk: "LOW" | "MEDIUM" | "HIGH";
    estimatedEffort?: string;
    affectedFiles?: string[];
    recommended: boolean;
    reason: string;
}

export interface RecommendationData {
    type: "recommendation";
    question: string;
    options: RecommendationOption[];
    aiRecommendation: string;
    contextFactors: string[];
    trigger: "architecture_decision" | "task_decomposition" | "tech_stack" | "deploy_strategy";
}
```

### 4.6 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 2.1 | TypeScript types for RecommendationData | `forge-portal/types/recommendation.ts` (new) | 20min |
| 2.2 | RecommendationCard component (UI + interaction) | `components/chat/recommendation-card.tsx` (new) | 3h |
| 2.3 | Integrate into chat message renderer | `components/chat/chat-message.tsx` or equivalent | 45min |
| 2.4 | Handle selection: send user message + disable re-selection | `components/chat/recommendation-card.tsx` | 30min |
| 2.5 | Context factors collapsible section | included in 2.2 | -- |
| 2.6 | Manual test: trigger each recommendation scenario | -- | 1h |

---

## 5. Recommendation Quality Guidelines

### 5.1 When to recommend (and when NOT to)

**Recommend when**:
- Multiple architecturally distinct approaches exist (new service vs extend existing)
- Deployment strategy has trade-offs (performance vs simplicity)
- Tech stack choice matters (ORM vs raw SQL, REST vs gRPC)
- Task decomposition order affects risk (DB-first vs API-first)

**Do NOT recommend when**:
- Only one viable approach exists (just do it)
- Differences are trivial (variable naming, comment style)
- User has already specified their preference
- Options are just different orderings of the same tasks

### 5.2 Recommendation quality metrics (future)

Track for continuous improvement:
- Recommendation acceptance rate (did user pick the AI's recommendation?)
- Override rate (how often does user pick a non-recommended option?)
- Post-selection satisfaction (did the chosen approach succeed without issues?)
- Recommendation frequency (too many recommendations = decision fatigue)

Target: 70%+ acceptance rate for the recommended option. If below 50%, the recommendation logic needs recalibration.

---

## 6. Risk & Mitigation

| Risk | Mitigation |
|------|-----------|
| AI outputs too many recommendations, causing decision fatigue | Rate limit: max 2 recommendations per conversation; only for genuinely distinct approaches |
| Recommendation JSON malformed in AI response | Robust parser with fallback to plain text display |
| Project profile data missing (new project, no scan yet) | Fallback: generic recommendations without context factors; note "项目画像未就绪，推荐基于通用判断" |
| Users ignore recommendations and just type | That's fine -- AI continues the conversation normally |

---

## 7. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | AI generates RecommendationCard when analyzing a requirement with multiple approaches | Submit "加一个积分系统" to a project with 10+ APIs |
| 2 | RecommendationCard renders with 2-3 option cards side by side | Visual check in chat |
| 3 | AI's recommended option has purple border + "AI 推荐" badge | Visual check |
| 4 | Clicking "选择此方案" sends selection as user message | Check conversation messages |
| 5 | Context factors section shows project-specific metrics | Expand and verify |
| 6 | Non-recommended options have clear risk/effort comparison | Visual check |
| 7 | Recommendation works even when project profile is empty | Test with a brand new project |

---

*Plan version: 1.0 | Author: Claude + Harvey | Date: 2026-04-03*
