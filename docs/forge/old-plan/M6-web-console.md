# M6 — Web 工作台（完整重做）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 `docs/product-design.md` 全面重做 forge-portal，实现"深空指挥中心"视觉风格、项目优先导航、角色分页、混合式需求输入、三级进度视图、AI 工作过程可视化、四层测试报告、分支管理、MR 审批的完整 Web 交互闭环。同时补充 forge-engine 缺失的 SSE 流式输出能力。

**Architecture:** forge-portal 是 Vue 3 SPA，通过 Vite 代理按路径前缀路由到各后端服务。前端采用项目优先导航 + 角色路由守卫。

**Tech Stack:**
- Vue 3.5, TypeScript, Vite
- Ant Design Vue 4.x（深度定制暗色主题）
- Pinia 3.x（状态管理）
- Vue Router 4.x（路由 + 角色守卫）
- Axios（HTTP 请求）
- Geist Sans + Geist Mono（字体）
- Lucide Icons（图标）

**关键设计参考:** `docs/product-design.md` Section 9（视觉规范）

---

## 前置工作：forge-engine SSE 支持

M4 实现中缺少 SSE 流式输出，M6 前端需要实时推送。需要先在 forge-engine 补充 SSE 端点。

---

## Task 0 — forge-engine SSE 端点补充

**目标：** 为 forge-engine 添加 SSE 推送能力，支持任务步骤状态变更、AI 流式输出的实时推送。

### 0.1 SSE 基础设施

- [ ] 创建 `forge-engine/src/main/java/com/shulex/forge/engine/entrance/controller/SseController.java`
  - `GET /api/tasks/{taskId}/stream` — 建立 SSE 连接，返回 `SseEmitter`
  - 超时设置 30 分钟（长连接）
  - 心跳：每 15 秒发送 `event: heartbeat`

- [ ] 创建 `forge-engine/src/main/java/com/shulex/forge/engine/service/SseService.java`
  - `ConcurrentHashMap<Long, Set<SseEmitter>>` 管理连接（按 taskId 分组）
  - `subscribe(taskId, emitter)` — 注册连接
  - `unsubscribe(taskId, emitter)` — 移除连接
  - `pushEvent(taskId, eventType, data)` — 向该任务所有连接推送
  - 连接断开自动清理

### 0.2 事件推送集成

- [ ] 修改 `StepResultListener` — 步骤完成时调用 `sseService.pushEvent(taskId, "step-update", stepData)`
- [ ] 修改 `TaskDispatcher` — 步骤开始时推送 `event: step-start`
- [ ] 修改 `TaskService.transitionStatus()` — 状态变更时推送 `event: task-status`
- [ ] 修改 `CodeGenerator` — 在代码生成过程中按文件推送 `event: file-generated`

### 0.3 SSE 事件格式定义

```
事件类型：
- step-start     {stepType, stepOrder, startTime}
- step-update    {stepType, status, output, tokens, duration}
- task-status    {taskId, oldStatus, newStatus}
- file-generated {fileName, fileType, lineCount}
- heartbeat      {}
```

### 0.4 测试

- [ ] 单元测试：SseService 连接管理、推送、清理
- [ ] 集成测试：创建任务 → 建立 SSE 连接 → 收到步骤事件流

**完成标准：** `curl -N http://localhost:8081/api/tasks/{id}/stream` 能收到实时事件流

---

## 文件结构总览（前端）

```
forge-portal/src/
├── main.ts
├── App.vue
├── assets/
│   └── fonts/                           ← Geist Sans + Geist Mono 字体文件
├── styles/
│   ├── variables.css                    ← 设计系统 CSS 变量（颜色、字体、间距）
│   ├── global.css                       ← 全局样式（暗色主题基础）
│   ├── aurora.css                       ← Aurora 极光背景动画
│   ├── components.css                   ← 通用组件样式覆盖（Ant Design 暗色定制）
│   └── animations.css                   ← 动画定义（过渡、呼吸灯、骨架屏）
├── api/
│   ├── request.ts                       ← Axios 实例 + Token 拦截器
│   ├── types.ts                         ← 全局类型定义
│   ├── auth.ts                          ← 鉴权 API
│   ├── project.ts                       ← 项目管理 API（新增）
│   ├── task.ts                          ← 任务 API（扩展 SSE）
│   ├── pipeline.ts                      ← 流水线 API
│   ├── specs.ts                         ← 规范 API（新增）
│   └── sse.ts                           ← SSE 客户端封装（新增）
├── stores/
│   ├── user.ts                          ← 用户状态（扩展角色信息）
│   ├── project.ts                       ← 当前项目状态（新增）
│   ├── task.ts                          ← 任务列表状态
│   └── ui.ts                            ← UI 状态（侧栏、视图模式）（新增）
├── router/
│   ├── index.ts                         ← 路由定义 + 角色守卫
│   └── guards.ts                        ← 路由守卫逻辑（新增）
├── composables/
│   ├── useSse.ts                        ← SSE 连接管理 composable（新增）
│   ├── useRole.ts                       ← 角色判断 composable（新增）
│   └── useTheme.ts                      ← 主题变量 composable（新增）
├── components/
│   ├── layout/
│   │   ├── AppLayout.vue                ← 重写：项目优先导航布局
│   │   ├── ProjectSidebar.vue           ← 新增：项目内侧栏（按角色显示菜单）
│   │   └── TopBar.vue                   ← 新增：顶部栏（项目切换、用户菜单）
│   ├── common/
│   │   ├── AiBadge.vue                  ← 新增：AI 生成内容标记
│   │   ├── SkeletonLoader.vue           ← 新增：骨架屏加载
│   │   ├── EmptyState.vue               ← 新增：空状态插画
│   │   ├── GlowButton.vue              ← 新增：发光按钮
│   │   ├── GlassCard.vue               ← 新增：毛玻璃卡片
│   │   └── StatusDot.vue               ← 新增：状态指示灯
│   ├── task/
│   │   ├── TaskCard.vue                 ← 重写：匹配新设计
│   │   ├── TaskKanban.vue               ← 新增：看板视图
│   │   ├── TaskTimeline.vue             ← 新增：AI 工作时间线
│   │   ├── TaskRealtimeView.vue         ← 新增：实时工作区
│   │   └── DecisionCard.vue             ← 新增：AI 决策卡片
│   ├── code/
│   │   ├── CodeBrowser.vue              ← 保留优化
│   │   ├── DiffViewer.vue               ← 保留优化
│   │   └── AiDiffAnnotation.vue         ← 新增：AI Diff 注释
│   ├── test/
│   │   ├── TestLayerCard.vue            ← 新增：单层测试结果卡片
│   │   └── TestReport.vue              ← 新增：四层测试报告组合
│   └── project/
│       ├── ProjectCard.vue              ← 新增：项目卡片
│       ├── ProjectImportModal.vue       ← 新增：一键接入弹窗
│       └── ProjectCreateWizard.vue      ← 新增：创建项目向导
├── views/
│   ├── LoginView.vue                    ← 重写：Aurora 背景 + 毛玻璃登录卡片
│   ├── ProjectLobbyView.vue             ← 新增：项目大厅
│   ├── project/
│   │   ├── RequirementChatView.vue      ← 新增：需求对话页
│   │   ├── TaskDashboardView.vue        ← 重写：三级进度视图
│   │   ├── TaskDetailView.vue           ← 重写：AI 工作过程可视化
│   │   ├── ChangeResultView.vue         ← 新增：变更结果三层展示
│   │   ├── TestReportView.vue           ← 新增：四层测试报告
│   │   ├── DeploymentView.vue           ← 重写：部署环境页
│   │   ├── BranchManageView.vue         ← 新增：分支管理页（技术管理者）
│   │   └── MrReviewView.vue             ← 新增：MR 审批页（技术管理者）
│   └── admin/
│       └── SystemSettingsView.vue       ← 新增：简化版系统设置
```

---

## Task 1 — 设计系统 + 全局样式基础

**目标：** 搭建"深空指挥中心"视觉基础，定义 CSS 变量、暗色主题、Aurora 背景、动画、Ant Design 暗色覆盖。

### 1.1 字体安装

- [ ] 下载 Geist Sans 和 Geist Mono 字体文件（woff2 格式）放入 `src/assets/fonts/`
- [ ] 在 `global.css` 中 `@font-face` 声明

### 1.2 CSS 变量系统 (`styles/variables.css`)

- [ ] 定义完整颜色变量（照搬 product-design.md Section 9.2）：
  ```css
  :root {
    /* Base */
    --bg: #050510;
    --surface-1: #0F0F1A;
    --surface-2: #1A1A2E;
    --border: #2A2A3E;
    --border-glow: rgba(139, 92, 246, 0.2);

    /* Brand */
    --primary: #8B5CF6;
    --primary-hover: #7C3AED;
    --primary-glow: rgba(139, 92, 246, 0.3);
    --accent: #06B6D4;
    --accent-glow: rgba(6, 182, 212, 0.3);

    /* Semantic */
    --success: #10B981;
    --warning: #F59E0B;
    --error: #EF4444;
    --info: #3B82F6;

    /* Text */
    --text-primary: #F1F1F3;
    --text-secondary: #8888A0;
    --text-muted: #555570;

    /* Gradients */
    --gradient-ai: linear-gradient(135deg, #8B5CF6, #06B6D4);
    --gradient-success: linear-gradient(135deg, #10B981, #06B6D4);
    --gradient-brand: linear-gradient(135deg, #8B5CF6, #3B82F6);

    /* Typography */
    --font-sans: 'Geist Sans', Inter, system-ui, -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;
    --font-mono: 'Geist Mono', 'JetBrains Mono', monospace;

    /* Spacing */
    --radius-sm: 6px;
    --radius-md: 8px;
    --radius-lg: 12px;

    /* Shadows */
    --shadow-glow-purple: 0 0 20px rgba(139, 92, 246, 0.3);
    --shadow-glow-cyan: 0 0 20px rgba(6, 182, 212, 0.3);
  }
  ```

### 1.3 全局样式 (`styles/global.css`)

- [ ] 全局 `body` 设置：`background: var(--bg); color: var(--text-primary); font-family: var(--font-sans);`
- [ ] 全局滚动条暗色定制
- [ ] 全局 `::selection` 紫色高亮
- [ ] 链接、输入框等基础元素暗色重置

### 1.4 Aurora 背景 (`styles/aurora.css`)

- [ ] `.aurora-bg` 类：2-3 个大模糊渐变块，低透明度（5-15%），慢速漂移动画（20-30s 循环）
- [ ] 颜色组合：深紫 + 青色 + 午夜蓝
- [ ] `position: fixed; z-index: -1;` 不干扰内容

### 1.5 动画定义 (`styles/animations.css`)

- [ ] `@keyframes skeleton-sweep` — 骨架屏渐变扫光（1.5s 循环）
- [ ] `@keyframes pulse-glow` — 呼吸灯效果（2s 循环）
- [ ] `@keyframes fade-in-up` — 页面淡入上移 8px（200-300ms）
- [ ] `@keyframes breathing-dots` — AI 思考状态三点呼吸
- [ ] CSS `transition` 工具类：`.transition-fast` (150ms)、`.transition-normal` (300ms)

### 1.6 Ant Design 暗色覆盖 (`styles/components.css`)

- [ ] 使用 Ant Design Vue 的 `ConfigProvider` theme token 覆盖：
  - `colorPrimary: '#8B5CF6'`
  - `colorBgContainer: '#0F0F1A'`
  - `colorBgElevated: '#1A1A2E'`
  - `colorBorder: '#2A2A3E'`
  - `colorText: '#F1F1F3'`
  - `colorTextSecondary: '#8888A0'`
  - `borderRadius: 8`
  - `fontFamily: var(--font-sans)`
- [ ] 卡片、按钮、输入框、表格、标签等组件样式微调

### 1.7 Lucide Icons 安装

- [ ] `npm install lucide-vue-next`
- [ ] 创建图标使用示例，确认图标风格一致

**完成标准：** 打开空白页面，Aurora 背景可见，暗色主题生效，Ant Design 组件颜色正确。

---

## Task 2 — 通用组件库

**目标：** 构建可复用的基础组件，后续页面直接使用。

### 2.1 GlowButton 组件

- [ ] `components/common/GlowButton.vue`
  - Props: `type` (primary/secondary/danger)、`loading`、`disabled`、`size`
  - Primary：紫色背景 + 外发光 `box-shadow`
  - Secondary：透明背景 + 边框
  - Danger：红色背景 + 红色外发光
  - 按下效果：`scale(0.97)`，100ms

### 2.2 GlassCard 组件

- [ ] `components/common/GlassCard.vue`
  - Props: `hoverable`、`padding`
  - 毛玻璃效果：`rgba(15, 15, 26, 0.8) + backdrop-filter: blur(20px)`
  - 顶部高光线：`border-top: 1px solid rgba(255, 255, 255, 0.12)`
  - Hover：边框亮度增加 + 微上移 1px

### 2.3 SkeletonLoader 组件

- [ ] `components/common/SkeletonLoader.vue`
  - Props: `type` (card/list/text/avatar)、`rows`
  - 骨架屏布局匹配真实内容
  - 渐变扫光动画（左到右）

### 2.4 EmptyState 组件

- [ ] `components/common/EmptyState.vue`
  - Props: `title`、`description`、`icon`、`actions` (按钮列表)
  - 居中显示插画 + 文字 + 操作按钮

### 2.5 AiBadge 组件

- [ ] `components/common/AiBadge.vue`
  - AI 生成内容标记：左侧 2px 紫→青渐变线 + 淡紫背景 + "AI" 小标签
  - Props: `showBadge`（是否显示"AI"标签）

### 2.6 StatusDot 组件

- [ ] `components/common/StatusDot.vue`
  - Props: `status` (running/waiting/error/success)
  - running：绿色脉冲呼吸动画
  - waiting：灰色静态
  - error：红色静态
  - success：绿色静态

**完成标准：** 每个组件独立可用，视觉效果符合 product-design.md Section 9.5。

---

## Task 3 — 路由 + 布局 + 角色守卫

**目标：** 实现项目优先导航架构、角色路由守卫、响应式布局。

### 3.1 路由定义 (`router/index.ts`)

- [ ] 路由结构：
  ```
  /login                              → LoginView
  /projects                           → ProjectLobbyView（项目大厅）
  /projects/:projectId/               → 项目内布局（AppLayout + ProjectSidebar）
    /requirements                     → RequirementChatView
    /tasks                            → TaskDashboardView
    /tasks/:taskId                    → TaskDetailView（AI 工作过程可视化）
    /tasks/:taskId/changes            → ChangeResultView
    /tasks/:taskId/tests              → TestReportView
    /deployments                      → DeploymentView
    /branches                         → BranchManageView（需要角色：tech_manager）
    /merge-requests                   → MrReviewView（需要角色：tech_manager）
  /admin/settings                     → SystemSettingsView（需要角色：admin）
  ```

### 3.2 路由守卫 (`router/guards.ts`)

- [ ] `beforeEach` 守卫：
  - 未登录 → 跳转 `/login`
  - 已登录访问 `/login` → 跳转 `/projects`
  - 访问需要特定角色的路由 → 检查用户角色
  - 访问项目内页面 → 检查 `projectId` 有效性

### 3.3 主布局 (`components/layout/AppLayout.vue`)

- [ ] 重写布局结构：
  - 顶部栏 `TopBar`（项目切换下拉 + 用户头像菜单 + 紧急停止入口）
  - 左侧栏 `ProjectSidebar`（项目内导航菜单，按角色显示不同菜单项）
  - 主内容区 `<router-view>` + 页面过渡动画 `fade-in-up`
  - 整体暗色基调

### 3.4 顶部栏 (`components/layout/TopBar.vue`)

- [ ] 左侧：Forge Logo + 当前项目名称（点击切换项目下拉）
- [ ] 中间：面包屑导航（项目 > 页面名）
- [ ] 右侧：紧急停止按钮（红色）+ 用户头像下拉（设置、登出）

### 3.5 项目侧栏 (`components/layout/ProjectSidebar.vue`)

- [ ] 使用 Lucide Icons
- [ ] 菜单项按角色条件渲染：
  - 所有用户：需求对话、任务看板、部署环境
  - 技术管理者+管理员：分支管理、MR 审批
- [ ] 当前激活项高亮（紫色左边框 + 淡紫背景）
- [ ] 底部：项目设置入口

### 3.6 用户 Store 扩展 (`stores/user.ts`)

- [ ] 扩展用户信息：`roles: string[]`、`tenantId: number`
- [ ] 添加 `hasRole(role)` 方法
- [ ] 添加 `isAdmin`、`isTechManager` 计算属性

### 3.7 项目 Store (`stores/project.ts`)

- [ ] `currentProject` — 当前选中的项目
- [ ] `projects` — 项目列表（星标置顶）
- [ ] `fetchProjects()` / `setCurrentProject(id)`
- [ ] 持久化 `currentProjectId` 到 `localStorage`

### 3.8 UI Store (`stores/ui.ts`)

- [ ] `sidebarCollapsed` — 侧栏折叠状态
- [ ] `progressViewMode` — 进度视图模式（overview/detail/realtime）

**完成标准：** 登录后进入项目大厅 → 选择项目 → 进入项目内页面 → 侧栏按角色显示菜单 → 路由守卫工作正常。

---

## Task 4 — 登录页

**目标：** 实现 Aurora 极光背景 + 毛玻璃登录卡片。

### 4.1 重写 LoginView

- [ ] 全屏 Aurora 背景（`.aurora-bg`）
- [ ] 居中毛玻璃登录卡片（480px 宽）：
  - Forge Logo + 渐变标题
  - 用户名输入框（暗色背景，聚焦紫色发光边框）
  - 密码输入框
  - GlowButton [登录]（紫色发光）
  - 底部版本号
- [ ] 交互：
  - 聚焦输入框：紫色发光边框过渡
  - 登录失败：卡片水平抖动 + 红色错误提示
  - 登录成功：卡片淡出 → 跳转项目大厅

### 4.2 登录 API 对接

- [ ] 调用 `POST /api/auth/login`
- [ ] 存储 `accessToken`、`username`、`userId`、`roles` 到 user store
- [ ] 错误处理：账号密码错误、网络异常

**完成标准：** 登录页视觉效果完整，Aurora 背景 + 毛玻璃卡片 + 发光按钮 + 交互动画。

---

## Task 5 — 项目大厅

**目标：** 实现项目列表、一键接入、创建新项目。

### 5.1 ProjectLobbyView

- [ ] 顶部：标题 "项目大厅" + 操作按钮 [一键接入] [创建项目]
- [ ] 搜索栏（暗色输入框）
- [ ] 项目卡片网格布局：
  - 星标置顶区
  - 按组织分组
  - 每个卡片：项目名、描述、技术栈标签、最近活动时间、星标按钮
  - 点击卡片 → 进入项目内页面

### 5.2 ProjectCard 组件

- [ ] `components/project/ProjectCard.vue`
  - Surface-1 背景 + hover 边框亮度增加
  - 项目名（text-lg）+ 描述（text-secondary）
  - 技术栈标签（语义色 badge）
  - 星标图标（点击切换）
  - 最近更新时间
  - 项目状态指示灯（StatusDot）

### 5.3 一键接入弹窗 (`ProjectImportModal`)

- [ ] 毛玻璃弹窗
- [ ] Step 1：选择平台（GitHub / Codeup）按钮选择
- [ ] Step 2：OAuth 授权（跳转授权页 → 回调）
- [ ] Step 3：显示同步到的仓库列表，批量勾选要关注的项目
- [ ] Step 4：确认导入 → 后台异步进行项目画像分析
- [ ] **Phase 1 简化：** OAuth 授权用手动填写 Token 替代（实际 OAuth 在 M7 完整实现），或直接输入仓库地址

### 5.4 创建项目向导 (`ProjectCreateWizard`)

- [ ] 分步向导（3 步）：
  - Step 1：选择模板（标准 Java 微服务 / Vue 3 前端 / 全栈 / SDK / 空项目）
  - Step 2：配置（项目名 + 描述 + 代码平台 + 部署环境 + CI/CD 平台）
  - Step 3：确认创建
- [ ] 底部进度指示器（1/3 → 2/3 → 3/3）

### 5.5 项目 API (`api/project.ts`)

- [ ] 项目 API 可能需要 forge-engine 新增端点（或在 forge-pipeline 中）：
  - `GET /api/projects` — 项目列表
  - `POST /api/projects` — 创建项目
  - `PUT /api/projects/:id/star` — 星标切换
  - `POST /api/projects/import` — 一键导入
- [ ] **注意：** 如果后端无项目管理 API，需要在 Task 5 中同时添加后端端点

**完成标准：** 项目大厅页面展示项目卡片列表，支持搜索、星标、创建。

---

## Task 6 — 需求对话页

**目标：** 实现混合式需求输入 — 自然语言对话 → AI 澄清 → 需求确认卡片 → SSE 流式输出。

### 6.1 RequirementChatView 布局

- [ ] 左侧：历史需求列表面板（可折叠）
  - 每条记录：需求摘要 + 时间 + 状态标签
  - 点击加载历史对话
- [ ] 右侧：对话区域
  - 聊天消息列表（用户消息 + AI 回复交替）
  - 底部输入区域（多行文本框 + 发送按钮）

### 6.2 消息组件

- [ ] 用户消息气泡：右侧对齐，Surface-2 背景
- [ ] AI 消息气泡：左侧对齐，AiBadge 标记，左侧紫色渐变线
- [ ] AI 流式输出：字符逐个出现 + 闪烁光标（紫色）
- [ ] AI 思考状态：三个紫色呼吸圆点 + "AI 正在分析..."

### 6.3 需求确认卡片组件

- [ ] 当 AI 生成完整需求理解后，渲染确认卡片：
  - 需求摘要（自然语言一段话）
  - 技术任务分解（编号列表）
  - 预估面板：影响文件数 ~N、Token 预估 ~NK、预计时间 ~Nmin、风险等级
  - 操作按钮：[确认执行] [修改需求] [取消]
- [ ] 确认执行 → 调用创建任务 API → 自动跳转任务看板
- [ ] 修改需求 → 卡片折叠 → AI 继续对话
- [ ] 取消 → 二次确认弹窗

### 6.4 SSE 客户端封装 (`api/sse.ts`)

- [ ] `createSseConnection(url): EventSource` — 创建 SSE 连接
- [ ] 自动重连（最多 3 次）
- [ ] 心跳检测
- [ ] 连接状态管理

### 6.5 useSse composable (`composables/useSse.ts`)

- [ ] `useSse(taskId)` — 建立 SSE 连接并返回响应式事件流
- [ ] `events: Ref<SseEvent[]>` — 事件列表
- [ ] `isConnected: Ref<boolean>` — 连接状态
- [ ] `disconnect()` — 断开连接
- [ ] 组件卸载自动断开

### 6.6 对话 API 对接

- [ ] 需求对话可能需要新的后端端点：
  - `POST /api/tasks/analyze` — 发送需求文本，返回 AI 分析（或通过 SSE 流式返回）
  - 或复用 `POST /api/tasks` 创建任务后监听 SSE
- [ ] **Phase 1 简化：** 对话可以是单轮 → 创建任务 → AI 分析步骤输出作为"AI 回复"

**完成标准：** 用户输入需求 → AI 流式回复 → 生成确认卡片 → 确认后创建任务并跳转看板。

---

## Task 7 — 任务看板 + 三级进度视图

**目标：** 实现三级可切换的任务进度视图（概览看板 / 详情 / 实时）。

### 7.1 TaskDashboardView

- [ ] 顶部：标题 + 视图切换按钮组（概览 | 详情 | 实时）
- [ ] 根据 `ui.progressViewMode` 渲染不同视图

### 7.2 概览视图（默认）— TaskKanban

- [ ] `components/task/TaskKanban.vue`
- [ ] 6 列看板：解析中 → 生成中 → 审查中 → 测试中 → 部署中 → 已完成
- [ ] 每列标题 + 任务计数
- [ ] TaskCard 组件：
  - 需求摘要（截断 2 行）
  - 风险标签（低/中/高，语义色）
  - 已用时间
  - 点击跳转 TaskDetailView

### 7.3 详情视图

- [ ] 任务列表（表格形式），每行展开显示：
  - 各阶段关键产出（子任务数、变更文件数、Review 分数、测试状态）
  - 进度条（分段式，每段 = 1 个步骤）

### 7.4 实时视图

- [ ] 显示当前正在执行的任务
- [ ] SSE 连接，实时显示 AI 正在做什么（哪个文件在分析、哪段代码在生成）
- [ ] 实时日志流

### 7.5 任务筛选

- [ ] 按状态筛选
- [ ] 按风险等级筛选
- [ ] 按时间排序

**完成标准：** 三种视图可切换，概览看板任务卡片正确分列，点击卡片进入详情。

---

## Task 8 — AI 工作过程可视化（任务详情页）

**目标：** 实现左右分栏的 AI 工作过程可视化 — 左侧任务时间线 + 右侧实时工作区。

### 8.1 TaskDetailView 重写

- [ ] 左右分栏布局（左 360px / 右 flex）
- [ ] 面包屑：任务看板 > 任务 #{id}

### 8.2 左侧 — TaskTimeline 组件

- [ ] `components/task/TaskTimeline.vue`
- [ ] 垂直时间线，每步显示：
  - 状态图标（✅ 完成 / 🔄 进行中 / ⬚ 待执行 / ⚠️ 需介入 / ❌ 失败）
  - 步骤名称 + 已用时间
  - 一句话摘要（AI 自动生成人话）
  - 关键指标（如 "3/5 文件生成"、"92 分"）
- [ ] 左侧 1px 连接线（Border 色）
- [ ] 活跃步骤：脉冲光晕效果
- [ ] 点击步骤 → 右侧显示该步骤详情

### 8.3 右侧 — TaskRealtimeView 组件

- [ ] `components/task/TaskRealtimeView.vue`
- [ ] 根据当前活跃步骤动态显示不同内容：
  - 需求理解：AI 理解摘要 + 任务拆分卡片
  - 方案规划：方案文档（影响模块、接口变更、数据库变更）
  - 代码生成：流式代码输出（类似 Claude）+ 文件树显示进度
  - AI 审查：审查发现流式输出 + 实时分数变化
  - 测试执行：四层测试进度条 + 实时结果
  - 流水线构建：构建日志流
  - 部署：部署进度 + 环境状态变化

### 8.4 DecisionCard 组件

- [ ] `components/task/DecisionCard.vue`
  - 紫色渐变顶部边框（2px）
  - AI 头像/图标
  - 决策内容 + 理由说明
  - 底部操作按钮（同意 / 替代方案）
  - 低风险自动通过标记
  - 高风险阻断等待用户确认

### 8.5 SSE 集成

- [ ] 进入任务详情页时建立 SSE 连接 `GET /api/tasks/{taskId}/stream`
- [ ] 监听事件：
  - `step-start` → 更新时间线步骤状态为"进行中"
  - `step-update` → 更新步骤产出和指标
  - `task-status` → 更新整体任务状态
  - `file-generated` → 在代码生成视图中追加文件
- [ ] 离开页面自动断开 SSE

### 8.6 历史回看

- [ ] 已完成任务的时间线完整保留
- [ ] 点击任意步骤可查看当时的完整输出

**完成标准：** 进入任务详情 → 左侧时间线实时更新步骤状态 → 右侧显示当前步骤的工作内容 → SSE 实时推送生效。

---

## Task 9 — 变更结果页 + 测试报告页

**目标：** 实现三层变更结果展示和四层测试报告展示。

### 9.1 ChangeResultView — 三层展示

- [ ] **Layer 1（默认展开）— AI 总结：**
  - 自然语言变更描述
  - 信任指标面板：
    - AI Review 分数：N/100
    - 安全扫描：通过/未通过
    - 单测覆盖率：N%
    - 接口测试：全部通过
    - 风险等级：低/中/高
  - 每个指标用语义色圆点标注

- [ ] **Layer 2（点击展开）— 变更摘要：**
  - 结构化变更列表：
    - 新增 API 列表
    - 修改 API 列表
    - 数据库变更
    - 影响模块
  - 不是代码 Diff，是人类可读的变更概要

- [ ] **Layer 3（钻取）— 代码详情：**
  - 左侧文件树 + 右侧代码查看器
  - AI Diff 视图：逐行变更 + AI 解释注释
  - 风险标注：高亮问题代码

### 9.2 AiDiffAnnotation 组件

- [ ] `components/code/AiDiffAnnotation.vue`
  - Diff 行旁边的 AI 注释气泡
  - 紫色连接线指向代码行
  - 注释内容：为什么改、有什么风险

### 9.3 TestReportView — 四层测试报告

- [ ] `views/project/TestReportView.vue`
- [ ] 四个测试层级卡片纵向排列：

  | 层级 | 内容 | 展示 |
  |------|------|------|
  | 单元测试 | AI 生成的对应单测 | 通过/失败计数、覆盖率 % |
  | 接口测试 | AI 从接口定义生成的 API 测试 | API 列表 + 场景（正常/异常/边界）结果 |
  | 集成测试 | 跨服务业务流程测试 | 流程图形式，每个节点标注通过/失败 |
  | 回归测试 | 完整运行已有测试 | 通过率趋势、失败用例列表 |

### 9.4 TestLayerCard 组件

- [ ] `components/test/TestLayerCard.vue`
  - 层级名 + 工具名（如 "单元测试 · JUnit 5"）
  - 通过/失败/跳过 计数
  - 折叠/展开详细结果
  - 语义色状态：全通过=绿，有失败=红，执行中=蓝

**完成标准：** 变更结果页三层展示逻辑正确，测试报告页四层卡片展示正确。

---

## Task 10 — 部署环境页

**目标：** 实现环境状态卡片 + 发布记录时间线 + 临时环境管理。

### 10.1 DeploymentView 重写

- [ ] 环境卡片区域：
  - dev / staging / prod 三个环境卡片
  - 每个卡片：当前版本、部署时间、健康状态（StatusDot）、最近部署者
  - 点击卡片展开详情

- [ ] 发布记录时间线：
  - 版本号、触发方式（自动/手动）、状态（成功/失败/回滚）
  - 每条记录可展开查看日志

- [ ] 临时环境列表：
  - AI 分支预览环境
  - 自动销毁倒计时
  - 手动销毁按钮

**完成标准：** 环境状态卡片正确显示，发布记录可查看。

---

## Task 11 — 分支管理 + MR 审批（技术管理者页面）

**目标：** 实现分支管理和 MR 审批页面，仅技术管理者及管理员可见。

### 11.1 BranchManageView

- [ ] 活跃分支列表（表格）：
  - 分支名、关联需求、创建时间、最新提交、状态
  - 操作：查看详情、手动合并触发
- [ ] 冲突检测面板：
  - 哪些分支存在冲突、冲突文件列表
  - 操作：AI 尝试解决、手动处理
- [ ] 合并历史：
  - 已合并分支、审批人、合并时间

### 11.2 MrReviewView

- [ ] 待审批 MR 列表（按风险等级排序）：
  - MR 标题、关联需求、风险等级标签、提交时间
- [ ] MR 详情面板（点击展开或侧边弹出）：
  - 完整 AI Review 报告（分数 + 问题列表 + 修复建议）
  - 风险评估详情（为什么判定为高风险）
  - Code Diff 查看器（复用 DiffViewer + AiDiffAnnotation）
  - 四层测试结果摘要
- [ ] 操作按钮：
  - [批准合并] — 确认弹窗后执行
  - [驳回] — 输入驳回理由
  - [请求 AI 修订] — 触发 AI 重新修复
  - [添加评论]

**完成标准：** 技术管理者可查看分支状态、审批或驳回 MR。

---

## Task 12 — 简化版系统设置（管理员页面）

**目标：** 实现 Phase 1 的最简系统设置。

### 12.1 SystemSettingsView

- [ ] Tab 页切换：紧急停止 | AI 模型配置 | 用户管理

- [ ] **紧急停止 Tab：**
  - L1 紧急停止开关（当前状态 + 切换按钮 + 操作日志）
  - 二次确认弹窗

- [ ] **AI 模型配置 Tab：**
  - 当前模型名称、API Key（脱敏显示）
  - Token 预算设置
  - 模型调用统计（总调用次数、总 Token 消耗）

- [ ] **用户管理 Tab：**
  - 用户列表（表格）：用户名、角色、创建时间、状态
  - 操作：修改角色、启用/禁用
  - 新增用户按钮

**完成标准：** 管理员可管理紧急停止、查看 AI 配置、管理用户。

---

## Task 13 — 后端 API 补充

**目标：** 补充前端页面所需但后端尚未提供的 API。

### 13.1 项目管理 API（forge-engine 或 forge-pipeline）

- [ ] 评估项目数据应该放在哪个服务中（建议 forge-engine，作为任务的上层概念）
- [ ] 数据库：`engine_project` 表（id, tenant_id, name, description, repo_url, platform_type, tech_stack, status, starred, org_group, gmt_create, gmt_modified）
- [ ] Flyway 迁移脚本
- [ ] API 端点：
  - `GET /api/projects` — 项目列表（支持搜索、筛选、排序）
  - `POST /api/projects` — 创建项目
  - `GET /api/projects/{id}` — 项目详情
  - `PUT /api/projects/{id}/star` — 星标切换
  - `POST /api/projects/import` — 批量导入
  - `GET /api/projects/{id}/profile` — 项目画像

### 13.2 需求对话 API（forge-engine）

- [ ] 评估需求对话是否需要独立存储（建议先复用 task + step 模型，对话作为 ANALYZE 步骤的输入/输出）
- [ ] 如果需要多轮对话，需新增 `engine_conversation` 表
- [ ] API：
  - `POST /api/projects/{projectId}/requirements/chat` — 发送需求消息（返回 SSE 流）
  - `GET /api/projects/{projectId}/requirements/history` — 历史需求列表

### 13.3 分支 & MR API（forge-pipeline 适配器透传）

- [ ] `GET /api/projects/{projectId}/branches` — 分支列表
- [ ] `GET /api/projects/{projectId}/merge-requests` — MR 列表
- [ ] `POST /api/merge-requests/{mrId}/approve` — 批准 MR
- [ ] `POST /api/merge-requests/{mrId}/reject` — 驳回 MR

### 13.4 测试报告 API

- [ ] `GET /api/tasks/{taskId}/test-report` — 四层测试结果聚合
- [ ] 可能需要从 MeterSphere API 拉取数据（Phase 1 可先 mock）

**完成标准：** 所有前端页面所需的 API 端点都已实现，接口文档与前端类型定义一致。

---

## Task 14 — 集成联调 + 端到端测试

**目标：** 全流程联调，验证 Phase 1 验收标准。

### 14.1 端到端流程验证

- [ ] 用户登录 → 进入项目大厅
- [ ] 创建新项目（或接入已有项目）
- [ ] 进入需求对话页 → 输入"创建一个用户管理服务"
- [ ] AI 流式回复 → 生成需求确认卡片
- [ ] 确认执行 → 跳转任务看板 → 看到任务卡片在"解析中"列
- [ ] 进入任务详情 → 左侧时间线实时更新 → 右侧显示代码生成过程
- [ ] 任务完成 → 查看变更结果（三层）
- [ ] 查看测试报告（四层）
- [ ] 查看部署环境页 → dev 环境显示已部署

### 14.2 角色测试

- [ ] 普通用户：看不到分支管理和 MR 审批
- [ ] 技术管理者：可以看到所有页面 + 审批 MR
- [ ] 管理员：可以访问系统设置

### 14.3 视觉走查

- [ ] 所有页面暗色主题一致
- [ ] Aurora 背景只在登录页
- [ ] Forge Purple (#8B5CF6) 品牌色统一
- [ ] 骨架屏加载（无 spinner）
- [ ] AI 生成内容有紫色渐变左边线标记
- [ ] 空状态页面有引导文案和操作按钮

### 14.4 SSE 连接测试

- [ ] 任务详情页 SSE 连接正常
- [ ] 连接断开自动重连
- [ ] 离开页面自动断开
- [ ] 多个页面同时连接不冲突

### 14.5 构建验证

- [ ] `npm run build` 无报错
- [ ] 构建产物可正常通过 nginx 或 Vite preview 访问
- [ ] 所有页面路由可直接访问（history mode 配置正确）

**完成标准：** Phase 1 完整验收标准通过 — 从需求输入到代码部署的完整流程在浏览器中可完成。

---

## 任务依赖关系

```
Task 0 (SSE 后端) ──────────────────────────────────┐
                                                      │
Task 1 (设计系统) ──→ Task 2 (通用组件) ──→ Task 3 (路由布局)
                                               │
                                    ┌──────────┼──────────┬──────────┐
                                    ▼          ▼          ▼          ▼
                              Task 4       Task 5     Task 12    Task 13
                              (登录)       (项目大厅)  (系统设置)  (后端 API)
                                    │          │                     │
                                    ▼          ▼                     │
                              Task 6 (需求对话) ←────────────────────┤
                                    │                                │
                                    ▼                                │
                              Task 7 (任务看板) ←────────────────────┤
                                    │                                │
                              ┌─────┼─────┐                         │
                              ▼           ▼                         │
                        Task 8        Task 9 ←──────────────────────┤
                        (AI 可视化)    (变更+测试)                    │
                              │           │                         │
                              ▼           ▼                         │
                        Task 10      Task 11 ←──────────────────────┘
                        (部署环境)    (分支+MR)
                              │           │
                              └─────┬─────┘
                                    ▼
                              Task 14 (集成联调)
```

**可并行路径：**
- Task 0 (SSE 后端) 和 Task 1~3 (前端基础) 可同时启动
- Task 4/5/12/13 可并行
- Task 8/9/10/11 可并行

---

## Phase 1 简化说明

以下功能在 Phase 1 中做简化处理：

| 功能 | Phase 1 实现 | 完整版（Phase 2+） |
|------|-------------|-------------------|
| OAuth 授权 | 手动输入 Token/仓库地址 | 完整 OAuth 流程 |
| 实时推送 | SSE 单向推送 | WebSocket 双向通信 |
| 需求对话 | 单轮 → 确认卡片 | 多轮对话 + 模板引导 |
| 项目画像 | 基础信息展示 | AI 深度分析 |
| 质量仪表盘 | 不实现 | 完整趋势图表 |
| 规范配置 | 不实现 | 继承链 + 可配置 |
| 测试报告数据 | Mock 数据 | MeterSphere 真实数据 |
| 代码浏览器 | 简单文本查看 | Monaco Editor |

---

## 完成标准汇总

1. ✅ 登录页：Aurora 背景 + 毛玻璃卡片 + 发光按钮
2. ✅ 项目大厅：项目列表 + 搜索 + 星标 + 创建/导入
3. ✅ 需求对话：自然语言输入 → AI 回复（SSE） → 确认卡片 → 创建任务
4. ✅ 任务看板：三级视图可切换（概览看板 / 详情 / 实时）
5. ✅ AI 工作可视化：左右分栏，时间线 + 实时工作区 + 决策卡片
6. ✅ 变更结果：三层展示（AI 总结 → 变更摘要 → 代码 Diff）
7. ✅ 测试报告：四层测试卡片（单测/接口/集成/回归）
8. ✅ 部署环境：环境卡片 + 发布时间线 + 临时环境
9. ✅ 分支管理：活跃分支 + 冲突检测 + 合并历史（技术管理者）
10. ✅ MR 审批：待审批列表 + Review 报告 + 批准/驳回（技术管理者）
11. ✅ 系统设置：紧急停止 + AI 配置 + 用户管理（管理员）
12. ✅ 视觉风格："深空指挥中心"暗色主题，Forge Purple 品牌色统一
13. ✅ 角色路由：不同角色看到不同菜单，路由守卫生效
14. ✅ SSE 实时推送：任务详情页实时接收步骤更新
