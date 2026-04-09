# Agent Terminal Variant B — 交付报告

**Session 日期：** 2026-04-07
**Baseline commit：** `13c566c` (fix(portal): add auth token to Agent chat API calls)
**HEAD：** `30ec4aa` (feat(agent): Stream 4c — language skill YAML + contextual suggestions API)
**Commits：** 12
**Diff：** 58 files, +5916 / -4602 lines

## 一句话概括

把 Agent Terminal 从一个"Dense Engineering 结构雏形"完整落地为 `variant-B-dense.html` mockup（Cursor/VS Code 美学）的生产级实现，覆盖 26 个 plan-eng-review + plan-design-review 抓出的架构/设计/测试/性能 issue，并补齐 Stream 5（质量门禁）、Stream 4b（双存储 + TaskSwitcher）、Stream 4c（语言检测 + 建议 API）。

## Commits

| # | Commit | 内容 |
|---|---|---|
| 1 | `a7c84be` | `fix(core): use encoding/json for Redis stream -> SSE encoding` — mapToJSON 从手写拼接改为 encoding/json，修复 quotes/backslashes/newlines 场景下 EventSource 丢帧 |
| 2 | `a681e65` | `fix(portal): serve agent SSE locally to bypass dev rewrite buffering` — Next Route Handler 绕过 dev server rewrite layer 的 gzip 缓冲 |
| 3 | `421fec5` | `docs: remove superseded OpenHarness refactor plan` — 删除已被 `a00bcda` 取代的 4242 行旧计划 |
| 4 | `eafae1a` | `chore: land deferred backlog + pair pipeline scaffold` — TODOS.md backlog + pair_pipeline.py + .gitignore 排除 .gstack/ |
| 5 | `6fdba56` | `design(portal): Stream 1 foundation` — Variant B tokens (3-layer bg/text/border + semantic bands), Inter + JetBrains Mono, DESIGN.md, WCAG AA --text-tertiary fix |
| 6 | `6c30a62` | `fix(portal): drop type scale tokens, use Tailwind arbitrary values` — 放弃与 Tailwind 4 `--text-*` 命名空间冲突的自定义 token，改用 `text-[9px]`/`text-[11px]` 等 arbitrary values |
| 7 | `c36778d` | `design(portal): Stream 2 components` — step-ribbon 去 glow、tool-execution 状态 badge、tool-formatters、agent-chat 空状态 CLI、code-panel Shiki 重写 |
| 8 | `45b16ed` | `design(portal): Stream 3 shell` — 4-row CSS Grid shell (40/40/1fr/20)、StatusBar、PanelDivider（rAF batched）、MobilePanelSwitcher、SSE 4-state enum + 指数退避 |
| 9 | `62f5c5e` | `feat(agent): Stream 4 backend` — Go handler goroutine+channel 重写 + 14 tests（mapToJSON 5 + Chat 5 + Stream 4 含泄漏检测）、pair_pipeline 发出 FixLoop/SessionComplete/Thinking 事件、ThinkingIndicator + SummaryCard 组件、消息 Retry 按钮 |
| 10 | `a8e54cc` | `test(agent): Stream 5 quality gate` — color-scheme dark/light 修复、ARIA 系统化补全（role=log/status/tablist/region）、Esc 关闭 tool card、60 vitest 组件测试 + 9 文件 axe-core 合规、5 个 Go Chat 端点测试 |
| 11 | `46e938d` | `feat(agent): Stream 4b dual storage` — migration 024 (agent_sessions + agent_messages)、Go repository、5 个新端点（Sessions/Messages CRUD）、ai-worker asyncpg 双写 + XADD MAXLEN 500、TaskSwitcher sidebar、前端 history 水化 via hydrateFromDurableLog |
| 12 | `30ec4aa` | `feat(agent): Stream 4c suggestions` — 5 个语言 skill YAML（java/python/go/node/rust）+ project_language.py 检测模块 + 18 个 Python 测试、suggestions.go 端点 + 17 个 Go 测试、前端 empty-state 从后端获取 contextual 建议 |

## Streams 对照

| Stream | 原计划 items | 交付状态 | 备注 |
|---|---|---|---|
| 1 Foundation | 7 | ✅ 全部 | WCAG 修正 + DESIGN.md 完整落地 |
| 2 UI Components | 6 | ✅ 全部 | Shiki 代码面板 + 语言彩色 dot |
| 3 Shell | 5 | ✅ 全部 | CSS Grid + rAF-batched divider + mobile swipe |
| 4 Backend | 6 of 10 | ✅ 全部 | 4/10 项拆到 4b/4c |
| 4b Task & Storage | 9 | ✅ 全部 | 决策：agent_sessions 独立于 engine.tasks，通过可选 FK 关联 |
| 4c Skill & Suggestions | 6 | ✅ 全部 | pair_pipeline 与 language profiles 的联动留待后续 |
| 5 A11y & Tests | 7 of 8 | ✅ 7/8 | Playwright e2e 延期（200MB 浏览器依赖） |

**总计 44/45 items 交付**（Playwright e2e 唯一延期项）。

## 测试总结

| 套件 | 数量 | 状态 |
|---|---|---|
| Go agent handler | 17 (mapToJSON 5 + Chat 5 + Stream 4 + Suggestions 3) | 全绿 |
| Python project_language | 18 (YAML loading + marker detection + command routing + end-to-end) | 全绿 |
| Vitest 组件 | 60 (9 files × 6-10 tests) | 全绿 |
| 预提交钩子 | trim whitespace, EOF, YAML check, JSON check, Go build/vet, Python compile, TypeScript, ESLint | 每个 commit 都通过 |

## 关键架构决策

### 双存储数据流
```
用户输入 → POST /chat → ai-worker engine
           ↓
       Redis XADD (MAXLEN ~500, hot buffer)
           ↓                    ↓
       Go SSE handler       asyncpg → engine.agent_messages (durable)
           ↓
       EventSource → 浏览器 UI

页面加载 → GET /sessions/:sid/messages → hydrateFromDurableLog → 完整对话复现
```

### agent_sessions vs engine.tasks
两者**独立**，不绑定。`agent_sessions.task_id` 是可选 FK，用户可以让 chat 关联到 Temporal workflow task，也可以让它做独立对话。这样避免污染原有 Task 模型的 SUBMITTED→COMPLETED 状态机。

### WCAG AA 修正
Mockup 的 `--text-tertiary: #868e96` 在白底上只有 3.4:1 对比度（WCAG AA 要求 4.5:1）。实现时暗化到 `#6b7280` (4.6:1) 并在 DESIGN.md 里明确标注这个偏离——"继承意图但修正 gap"。

### Tailwind 4 `@theme` 陷阱
`--text-*` 是 Tailwind 4 的保留 typography 命名空间，自定义条目在 `@theme inline` 或 `@theme` 里都会被 silently drop（cache clear + 冷启动都不生效）。最终改为 `text-[9px]`/`text-[11px]` arbitrary values + DESIGN.md 文档化"什么场景用什么 size"。这个发现通过 preview_eval 的运行时验证才看出来，tsc 和 ESLint 都没检测到。

### Goroutine 泄漏检测
原始 `select { default: XREAD }` 模式每秒调用 Redis 一次，默认分支永远优先于 ctx.Done 和 heartbeat。重写为 reader goroutine + channel 模式后，用 `runtime.Stack()` 过滤 `streamReader` 函数名来检测泄漏——比统计总 goroutine 数更健壮，因为 miniredis 的 `servePeer` 和 go-redis 连接池会污染总数。

## 部署检查单（上线前必做）

- [ ] `psql -d forge -f forge-core/migrations/024_agent_sessions.sql` — 应用 Stream 4b 的新表
- [ ] `cd ai-worker && pip install -r requirements.txt` — 安装 asyncpg
- [ ] `cd forge-core && go build -o forge-core ./cmd/forge-core && systemctl restart forge-core` — 重建重启
- [ ] `systemctl restart ai-worker` — 重启 Python 服务
- [ ] 验证 `curl http://localhost:8080/api/projects/1/agent/suggestions` 返回 200（不是 404）
- [ ] 验证 `curl http://localhost:8080/api/projects/1/agent/sessions` 返回 200（不是 503）

## 已知债务（不在本次交付范围）

见 `TODOS.md` + checkpoint `20260407-后续任务.md`：

1. 紫色品牌残留清理（7 files outside Agent Terminal）
2. Workspace 脚手架垃圾清理（`forge-core/workspaces/tenant-1/project-25/...`）
3. build-card vs mockup lines 505-554 细节对照
4. 登录按钮 `bg-primary` vs `--accent` 决策
5. TaskSwitcher 的嵌套 button 重构
6. Playwright happy-path e2e（Stream 5.7 延期）
7. pair_pipeline 与 Stream 4c language profiles 的联动
8. Git log / TODOS.md 集成到 suggestions heuristic
9. "Create Task from session" 把 chat 物化为 Temporal task
10. Skills / Projects / Settings 页面应用 DESIGN.md
11. Redis consumer group 替代 ai-worker 直写（提升 durable 一致性）

## Before / After

**Before this session（baseline 13c566c）：**
- Agent Terminal 存在但是"Dense Engineering 结构雏形"，细节上：
  - 没有 4 行 CSS Grid shell（缺 StatusBar）
  - Token 系统半缺（只有 --surface/--text-dim 旧别名）
  - 字体是 Geist（应该是 Inter + JetBrains Mono）
  - Code Panel 是带行号的 `<pre>`（没有 Shiki、breadcrumb、minimap）
  - Step ribbon 带紫色 glow 发光
  - 没有 Task 模型链接（header 显示 sessionId slice）
  - 没有 Fix Loop 可视化、Thinking Indicator、Summary Card
  - ARIA 几乎全缺、WCAG 对比度不合格
  - Go handler 有 SSE busy-loop bug（1 Redis call/sec/connection）
  - 零测试

**After this session（HEAD 30ec4aa）：**
- Mockup-aligned Dense Engineering 落地
- 4-row CSS Grid shell + draggable PanelDivider + 移动端 swipe switcher
- Inter + JetBrains Mono + mockup 完整 token 系统
- Shiki 代码面板 + 语言彩色 dot + breadcrumb + minimap + 错误行高亮
- Step ribbon 无 glow，1px 连接线
- TaskSwitcher sidebar 支持多会话管理
- Fix Loop system messages + Thinking Indicator + Summary Card
- ARIA 系统化（role=log/status/tablist/region），Esc 键盘关闭
- WCAG AA 合规（--text-tertiary 暗化）
- Go handler 用 goroutine+channel 模式，无 CPU busy-loop，覆盖 14 个测试（含 goroutine 泄漏检测）
- 60 vitest 组件测试 + axe-core 合规 + 18 Python 测试 + 17 Go 测试
- 双存储架构：Redis 热 buffer + PostgreSQL 耐久 history + 水化时复现完整对话
- Contextual suggestions API 根据 tech stack 推 3 条启动提示
