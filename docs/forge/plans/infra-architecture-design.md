# Forge SaaS 基础设施架构设计

> **版本**: 1.0
> **日期**: 2026-04-02
> **关联**: PRD v4.1, technical-design.md

---

## 1. 核心问题

Forge 作为多租户 SaaS 系统，需要解决：
1. AI 生成的代码存在哪里？→ Git 仓库（GitHub）+ K8s 临时容器工作区
2. 多租户多用户并行如何隔离？→ K8s namespace + 容器隔离
3. Git 认证怎么做？→ GitHub App（平台级）+ OAuth（用户级）混合
4. 用户代码怎么部署？→ K8s 集群（先单集群 namespace 隔离）

---

## 2. 系统拓扑

```
                    ┌────────────────────────────────────────┐
                    │          ACK 集群（单集群）              │
                    │                                        │
                    │  namespace: forge-system               │
                    │  ├── forge-core (Go API)               │
                    │  ├── forge-portal (Next.js)            │
                    │  ├── ai-worker (Python)                │
                    │  ├── temporal-server                   │
                    │  ├── postgresql                        │
                    │  └── redis                             │
                    │                                        │
                    │  namespace: forge-jobs                 │
                    │  ├── task-{id}-build  (K8s Job, 临时)   │
                    │  ├── task-{id}-test   (K8s Job, 临时)   │
                    │  └── task-{id}-deploy (K8s Job, 临时)   │
                    │                                        │
                    │  namespace: tenant-{id}-dev            │
                    │  └── {用户的业务服务 Pod}                │
                    │                                        │
                    │  namespace: tenant-{id}-staging         │
                    │  └── {用户的业务服务 Pod}                │
                    │                                        │
                    │  namespace: tenant-{id}-prod            │
                    │  └── {用户的业务服务 Pod}                │
                    │                                        │
                    └────────────────────────────────────────┘
                                      │
                    ┌─────────────────┼──────────────────┐
                    │                 │                  │
              ┌─────▼─────┐   ┌──────▼──────┐    ┌─────▼──────┐
              │  GitHub    │   │ 阿里云 ACR   │    │ 阿里云 OSS  │
              │ (代码仓库)  │   │ (镜像仓库)   │    │ (制品存储)  │
              └───────────┘   └─────────────┘    └────────────┘
```

---

## 3. Git 仓库归属模式（混合）

### 模式 A: Forge 托管仓库（新项目）

```
用户: "我要从零创建一个优惠券系统"
  ↓
Forge 自动创建: github.com/forge-managed/tenant-1-coupon-system
  ↓
使用 GitHub App Installation Token 操作
  ↓
用户通过 Forge Web 界面管理代码
```

- Forge 注册为 **GitHub App**（不是 OAuth App）
- GitHub App 安装到 Forge 自己的 Organization `forge-managed`
- 每个租户的项目: `forge-managed/{tenant-slug}-{project-slug}`
- 优势: 用户不需要自己的 GitHub 账号

### 模式 B: 用户仓库（已有项目）

```
用户: "接入我们公司的 github.com/acme-corp/user-service"
  ↓
方式 1: 用户个人 OAuth 授权 → 用用户 token 操作
方式 2: 企业安装 Forge GitHub App → 用 Installation Token 操作（推荐）
```

- **个人仓库**: 用户 OAuth 授权，token 存 `user_identities` 表
- **企业仓库**: 企业管理员安装 Forge GitHub App 到企业 Org
  - Installation Token 自动刷新（1h 有效期，平台自动续期）
  - 不依赖个人 token，团队成员离职不影响
  - 可以精确控制仓库权限（只授权特定仓库）

### 认证优先级

```
操作仓库时的 Token 选择逻辑:

1. 检查仓库所属 Org 是否安装了 Forge GitHub App
   → 有 → 使用 Installation Token（最稳定）
2. 检查操作用户是否有 OAuth token
   → 有 → 使用用户 OAuth Token
3. 都没有
   → 报错: "请先关联代码平台"
```

---

## 4. AI 任务容器生命周期

### 4.1 容器化工作流

```
                    Temporal Workflow
                          │
          ┌───────────────┼────────────────┐
          │               │                │
    ┌─────▼─────┐  ┌──────▼──────┐  ┌─────▼──────┐
    │ AI 生成   │  │  测试执行    │  │  构建部署   │
    │ (Python)  │  │ (K8s Job)   │  │ (K8s Job)  │
    └───────────┘  └─────────────┘  └────────────┘

AI 生成 → 在 ai-worker 进程中完成（不需要容器）
测试/构建/部署 → 在 K8s Job 中完成（需要容器隔离）
```

### 4.2 任务容器 Spec

```yaml
# 测试/构建 Job 模板
apiVersion: batch/v1
kind: Job
metadata:
  name: task-{taskId}-{step}
  namespace: forge-jobs
  labels:
    forge.io/tenant: "{tenantId}"
    forge.io/task: "{taskId}"
    forge.io/step: "test|build|deploy"
spec:
  ttlSecondsAfterFinished: 300  # 完成后 5 分钟清理
  activeDeadlineSeconds: 1800   # 30 分钟超时
  template:
    spec:
      containers:
      - name: worker
        image: forge-task-runner:latest
        resources:
          requests: { cpu: "500m", memory: "1Gi" }
          limits:   { cpu: "2",    memory: "4Gi" }
        env:
        - name: GIT_TOKEN
          valueFrom:
            secretKeyRef: { name: task-{taskId}-creds, key: git-token }
        - name: REPO_URL
          value: "https://github.com/owner/repo"
        - name: BRANCH
          value: "ai/{taskId}-feature"
        volumeMounts:
        - name: workspace
          mountPath: /workspace
      volumes:
      - name: workspace
        emptyDir: { sizeLimit: "10Gi" }
      restartPolicy: Never
```

### 4.3 容器内执行步骤

```bash
# 1. Clone 代码
git clone --depth=1 --branch=${DEFAULT_BRANCH} https://x-access-token:${GIT_TOKEN}@github.com/${OWNER}/${REPO} /workspace
cd /workspace

# 2. 创建 AI 分支
git checkout -b ${BRANCH}

# 3. 写入 AI 生成的文件（从 Redis/API 获取）
curl -s ${FORGE_API}/internal/tasks/${TASK_ID}/generated-files | jq -r '.files[] | .path + "\t" + .content' | while IFS=$'\t' read path content; do
  mkdir -p $(dirname "$path")
  echo "$content" > "$path"
done

# 4. 运行 Lint
# 检测语言 → 自动选择 linter
if [ -f go.mod ]; then golangci-lint run ./...; fi
if [ -f package.json ]; then npx eslint .; fi

# 5. 运行测试
if [ -f go.mod ]; then go test ./... -v -cover; fi
if [ -f package.json ]; then npm test; fi

# 6. Docker Build（如有 Dockerfile）
if [ -f Dockerfile ]; then
  docker build -t ${ACR_REGISTRY}/${IMAGE}:${VERSION} .
  docker push ${ACR_REGISTRY}/${IMAGE}:${VERSION}
fi

# 7. Git Push + 创建 PR
git add -A
git commit -m "${COMMIT_MESSAGE}"
git push origin ${BRANCH}
```

---

## 5. 用户代码部署

### 5.1 部署流水线

```
代码合并到 main
  ↓
Forge 检测到 push event (Webhook)
  ↓
启动部署 K8s Job
  ├── git clone
  ├── docker build → push to ACR
  ├── 生成 K8s 资源清单
  │   ├── Deployment (副本数、资源限制)
  │   ├── Service (端口映射)
  │   ├── Ingress (域名路由)
  │   ├── ConfigMap / Secret
  │   └── HPA (自动扩缩容)
  ├── kubectl apply -n tenant-{id}-{env}
  └── 等待 Pod Ready + 健康检查
```

### 5.2 环境管理

| 环境 | Namespace | 触发条件 | URL |
|------|-----------|---------|-----|
| 预览 | `forge-jobs` | PR 创建时 | `{taskId}.preview.forge.example.com` |
| 开发 | `tenant-{id}-dev` | 合并到 develop | `{project}.dev.forge.example.com` |
| 预发 | `tenant-{id}-staging` | 合并到 release | `{project}.staging.forge.example.com` |
| 生产 | `tenant-{id}-prod` | 手动审批后 | `{project}.forge.example.com` |

### 5.3 多租户资源隔离

```yaml
# 每个租户 namespace 的 ResourceQuota
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tenant-quota
  namespace: tenant-{id}-dev
spec:
  hard:
    requests.cpu: "4"
    requests.memory: "8Gi"
    limits.cpu: "8"
    limits.memory: "16Gi"
    pods: "20"
    services: "10"
    persistentvolumeclaims: "5"
```

---

## 6. GitHub App 设计

### 6.1 App 权限

```
Forge GitHub App 权限:
├── Repository permissions:
│   ├── Contents: Read & Write     (读写代码)
│   ├── Pull requests: Read & Write (创建/管理 PR)
│   ├── Metadata: Read             (仓库基础信息)
│   ├── Webhooks: Read & Write     (接收 push/PR 事件)
│   └── Actions: Read              (查看 CI 状态)
├── Organization permissions:
│   └── Members: Read              (组织成员列表)
└── Subscribe to events:
    ├── Push
    ├── Pull request
    └── Create (branch/tag)
```

### 6.2 Token 管理

```
GitHub App Token 生命周期:

1. Forge 启动时生成 JWT (用 App 私钥签名, 10min 有效)
2. 用 JWT 获取 Installation Access Token (1h 有效)
3. Token 缓存在 Redis, key: github:installation:{installationId}
4. 过期前 5 分钟自动刷新
5. 每个 Installation (Org) 独立的 Token

比用户 OAuth Token 更稳定:
- 不依赖个人账号
- 自动刷新, 不会过期
- 权限精确可控
```

---

## 7. 数据流总览

```
用户提需求
  ↓
AI 分析 + 规划 + 生成代码 (ai-worker, 内存中)
  ↓
代码暂存到 Redis: code:task:{taskId} (TTL 1h)
  ↓
启动 K8s Job: task-{taskId}-build
  ├── 从 Redis 获取生成的代码
  ├── git clone 仓库
  ├── 写入文件 + git commit + push
  ├── 创建 PR
  ├── 运行测试 (在容器内)
  ├── 构建 Docker 镜像 → push ACR
  └── 汇报结果到 Forge API
  ↓
Forge 更新任务状态 + SSE 推送前端
  ↓
用户在 Web 界面查看 PR / Diff / 测试结果
  ↓
审批通过 → 合并 PR → 触发部署
  ↓
启动 K8s Job: task-{taskId}-deploy
  ├── kubectl apply 到 tenant-{id}-{env}
  └── 健康检查
```

---

## 8. 实施优先级

### 阶段 1 (当前): GitHub API 模式
- AI 生成代码通过 GitHub API 推送（已实现 S7）
- 不需要本地文件系统
- 不需要 K8s Job
- 适用于: 代码生成 + PR 创建

### 阶段 2: K8s Job 任务容器
- 接入 ACK 集群
- AI 任务在 K8s Job 中执行
- 支持运行测试、构建 Docker
- 容器隔离保证安全

### 阶段 3: 完整部署流水线
- 用户代码部署到 K8s
- 多环境管理 (dev/staging/prod)
- PR 预览环境
- 灰度发布

### 阶段 4: GitHub App
- 替代 OAuth，支持企业仓库
- Installation Token 自动管理
- Webhook 事件驱动
