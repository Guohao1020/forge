# Codeup API 能力清单

> **来源**: 阿里云云效 Codeup 官方文档
> **用途**: Forge 平台所有代码操作通过 Codeup API 完成，不做本地克隆

---

## 1. 代码读取

| API | 用途 | Forge 调用方 |
|-----|------|-------------|
| ListRepositoryTree | 获取仓库目录树结构 | Context Builder（项目结构分析） |
| GetFileBlobs | 获取单个文件完整内容 | Context Builder（加载相关代码） |
| GetFileLastCommit | 查询文件最近一次提交 | Context Builder（变更历史） |
| ListRepositoryCommits | 查询提交历史 | Context Builder（最近变更） |
| ListRepositoryCommitDiff | 获取单个 commit 的 diff | AI Review（查看变更详情） |

---

## 2. 代码写入

| API | 用途 | Forge 调用方 |
|-----|------|-------------|
| CreateBranch | 创建 ai/feature-xxx 分支 | AI 引擎（任务开始时） |
| CreateCommitWithMultipleFiles | **核心 API：一次性提交多个文件变更** | 执行服务（代码生成后原子提交） |
| CreateFile | 创建单个文件 | 执行服务（补充文件） |
| UpdateFile | 修改单个文件 | 执行服务（修复代码） |
| DeleteFile | 删除文件 | 执行服务（清理文件） |
| DeleteBranch | 删除分支 | DevOps（MR 合并后清理） |

---

## 3. 合并请求

| API | 用途 | Forge 调用方 |
|-----|------|-------------|
| CreateMergeRequest | 创建 MR（ai/feature-xxx → develop） | AI 引擎（Review 通过后） |
| GetMergeRequest | 查询 MR 详情 | Web 工作台（MR 审批页） |
| ListMergeRequests | 查询 MR 列表 | Web 工作台（任务看板） |
| MergeMergeRequest | 合并 MR | AI 引擎（低风险自动合并）/ 人工审批 |
| ReviewMergeRequest | 发表评审意见 | AI Review（自动评审） |
| CreateComment | 创建行内评论 | AI Review（逐行批注） |
| UpdateMergeRequestPersonnel | 更新评审人 | AI 引擎（分配审批人） |
| CloseMergeRequest | 关闭 MR | AI 引擎（任务取消时） |
| GetMergeRequestChangeTree | 查询 MR 变更文件统计 | 风险评估（计算影响范围） |

---

## 4. 仓库管理

| API | 用途 | Forge 调用方 |
|-----|------|-------------|
| CreateRepository | 创建新代码仓库 | AI 引擎（从零孵化新项目时） |
| GetRepository | 查询仓库信息 | AI 引擎（获取仓库元数据） |
| ListRepositories | 查询仓库列表 | Web 工作台（项目管理页） |
| GetBranchInfo | 查询分支信息 | AI 引擎（冲突检测） |
| ListRepositoryBranches | 查询分支列表 | Web 工作台（代码浏览器） |

---

## 5. Webhook 事件

| 事件类型 | 触发时机 | Forge 用途 |
|---------|---------|-----------|
| Push Hook | 代码推送 | 增量刷新代码索引 |
| Merge Request Hook | MR 创建/更新/合并/关闭 | 更新任务状态、触发临时环境清理 |
| Tag Push Hook | 标签创建/删除 | 版本发布追踪 |
| Note Hook | 评论事件 | 人工 Review 评论同步 |

Webhook 配置要点：
- Secret Token 验签（X-Codeup-Token 头）
- 出口 IP 白名单：47.98.116.130, 47.111.186.29

---

## 6. 限流应对策略

Codeup API 存在调用频率限制（具体限值需实际测试）。应对措施：

| 策略 | 说明 |
|------|------|
| 文件内容缓存 | GetFileBlobs 结果缓存到 Redis，设置合理 TTL |
| Webhook 驱动刷新 | 收到 Push Hook 时才刷新缓存，不轮询 |
| 项目结构持久化 | ListRepositoryTree 结果持久化到 DB，增量更新 |
| 令牌桶限流 | API 调用端自行限流，避免触发服务端限制 |
| 批量操作优先 | 优先使用 CreateCommitWithMultipleFiles 减少调用次数 |
