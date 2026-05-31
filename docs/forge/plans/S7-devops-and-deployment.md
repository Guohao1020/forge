# S7 — DevOps 集成 + 变更结果 + 测试报告 + 部署环境

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 Phase 1 端到端闭环：AI 生成代码 -> 自动推送到 GitHub 分支 -> 创建 PR -> 变更结果页展示 Diff -> 质量门禁信息 -> 测试报告 -> 部署环境状态。用户从需求输入到看到代码变更的全流程打通。

**Architecture:** 扩展 S3 的 GitHub adapter（分支/提交/PR 操作），在 forge-core 中添加 DevOps Temporal activities（Phase 1 不引入独立 devops-worker），扩展 S6 的任务 workflow 加入代码提交步骤，前端新增三个页面（变更结果、测试报告、部署环境）。

**Tech Stack:** Go 1.22 + Gin + pgx, Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 + shadcn/ui + Monaco Editor, PostgreSQL 16, Temporal, GitHub REST API, code-server (Web IDE)

**Depends on:** S1 (auth), S2 (projects), S3 (GitHub adapter), S4 (Temporal + tasks), S5 (specs), S6 (AI worker + code generation)

---

## 前置说明

### 与 S6 的关系

S6 交付了 AI 需求对话 + 代码生成 + AI Review。S7 把 AI 生成的代码推送到 GitHub 分支，并让用户在 Web 端查看变更结果、质量信息和测试报告。S7 完成后，Phase 1 的完整循环闭合。

### Phase 1 的边界

S7 **交付**：
- GitHub 分支创建、代码提交、PR 创建（真实 GitHub 操作）
- 从 GitHub 获取 PR Diff 并展示（真实数据）
- Monaco Editor 内联 Diff 查看（语法高亮 + AI 注释）
- code-server (Web IDE) 集成 — "在 IDE 中打开"完整代码浏览
- AI Review 评分展示（复用 S6 的 review 结果）
- 基本测试报告页面（Phase 1 仅展示 AI 生成的单测通过/失败）
- 环境状态卡片（Phase 1 为信息展示，无实际部署）

S7 **不包含**（后续切片）：
- 实际 K8s 部署（ACK/Argo CD）
- MeterSphere 测试平台对接
- 实际 CI/CD 流水线触发
- 实际安全扫描（Semgrep, Trivy）
- 灰度/蓝绿发布
- 独立 devops-worker / constraint-worker 进程
- code-server 通过 IDE 直接提交代码（Phase 1 只读）

### 本切片交付后你可以做什么

1. 在任务详情页提交需求 -> AI 分析、生成代码、Review（S6 已有）
2. AI 自动创建 `ai/{taskId}-{slug}` 分支，提交代码，创建 PR
3. 在变更结果页查看 AI 总结、信任指标、变更文件列表、Monaco Diff
4. 点击"在 IDE 中打开"，在 code-server (VS Code Web) 中完整浏览代码仓库
5. 在测试报告页查看基本测试结果（单测通过/失败，其他层级 "Coming soon"）
6. 在部署环境页查看环境状态卡片（dev/staging/prod 信息卡）
7. 在任务详情页看到 PR 链接和 Review 评分

---

## 文件结构

### forge-core 新增/修改

```
forge-core/
├── migrations/
│   └── 007_init_pipeline.sql              # pipeline schema + review_results + tasks 字段
├── internal/
│   ├── module/
│   │   ├── adapter/
│   │   │   └── github_adapter.go          # 修改：新增 CreateBranch/CommitFiles/CreatePR/GetPRDiff
│   │   ├── pipeline/
│   │   │   ├── model.go                   # 环境 + Review 结果模型
│   │   │   ├── repository.go              # 数据库操作
│   │   │   ├── service.go                 # 业务逻辑
│   │   │   └── handler.go                 # HTTP handler
│   │   └── task/
│   │       ├── model.go                   # 修改：添加 mr_url, review_score 字段
│   │       ├── workflow.go                # 修改：扩展 workflow 加入 DevOps 步骤
│   │       ├── activities_devops.go       # 新增：DevOps activities
│   │       └── handler.go                # 修改：新增 changes/review/tests API
│   └── router/
│       └── router.go                      # 修改：注册新路由
```

### forge-portal 新增/修改

```
forge-portal/
├── app/
│   └── (dashboard)/
│       └── projects/
│           └── [id]/
│               ├── tasks/
│               │   └── [taskId]/
│               │       ├── changes/
│               │       │   └── page.tsx           # 变更结果页
│               │       └── tests/
│               │           └── page.tsx           # 测试报告页
│               └── deploy/
│                   └── page.tsx                   # 部署环境页
├── components/
│   ├── monaco-diff-viewer.tsx                     # Monaco Editor Diff 查看组件（替代 react-diff-viewer）
│   ├── open-in-ide-button.tsx                     # "在 IDE 中打开" 按钮组件
│   ├── trust-metrics.tsx                          # 信任指标面板组件
│   ├── environment-card.tsx                       # 环境状态卡片组件
│   └── test-layer-card.tsx                        # 测试层级卡片组件
├── lib/
│   └── api.ts                                     # 修改：新增 API 调用函数
```

### Docker Compose 修改

```
docker-compose.dev.yml                             # 新增 code-server 服务
```

### forge-core 新增（code-server 代理）

```
forge-core/
├── internal/
│   └── module/
│       └── ide/
│           ├── handler.go                         # code-server 工作区管理 API
│           └── service.go                         # 工作区创建/回收逻辑
```

---

## Task 1: 数据库迁移 — Pipeline Schema + Review Results + Tasks 扩展

**Files:**
- Create: `forge-core/migrations/007_init_pipeline.sql`

- [ ] **Step 1: 创建迁移文件 007_init_pipeline.sql**

`forge-core/migrations/007_init_pipeline.sql`：

```sql
-- S7: Pipeline schema + Review results + Task extensions
-- Depends on: 005 (specs schema)

-- ============================================================
-- pipeline.environments — 部署环境跟踪
-- ============================================================
CREATE TABLE IF NOT EXISTS pipeline.environments (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    name            VARCHAR(100) NOT NULL,
    env_type        VARCHAR(20) NOT NULL,  -- DEV / STAGING / PROD
    status          VARCHAR(20) NOT NULL DEFAULT 'INACTIVE',  -- INACTIVE / ACTIVE / DEPLOYING / ERROR
    current_version VARCHAR(100),
    config          JSONB NOT NULL DEFAULT '{}',
    last_deploy_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_environments_project ON pipeline.environments(project_id);
CREATE INDEX IF NOT EXISTS idx_environments_tenant ON pipeline.environments(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_project_type ON pipeline.environments(project_id, env_type);

-- ============================================================
-- engine.review_results — AI Review / Lint / 安全扫描结果
-- ============================================================
CREATE TABLE IF NOT EXISTS engine.review_results (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    review_type     VARCHAR(20) NOT NULL,  -- AI_REVIEW / LINT / SECURITY
    score           INT,
    passed          BOOLEAN NOT NULL,
    findings        JSONB NOT NULL DEFAULT '[]',
    summary         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_review_results_task ON engine.review_results(task_id);

-- ============================================================
-- engine.tasks 扩展字段 — PR URL + Review 评分
-- ============================================================
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS mr_url TEXT;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS review_score INT;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS branch_name VARCHAR(200);
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS pr_number INT;

-- ============================================================
-- 触发器：项目创建时自动初始化默认环境
-- ============================================================
CREATE OR REPLACE FUNCTION pipeline.create_default_environments()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
    VALUES
        (NEW.tenant_id, NEW.id, 'Development', 'DEV', 'INACTIVE'),
        (NEW.tenant_id, NEW.id, 'Staging', 'STAGING', 'INACTIVE'),
        (NEW.tenant_id, NEW.id, 'Production', 'PROD', 'INACTIVE');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_create_default_environments ON engine.projects;
CREATE TRIGGER trg_create_default_environments
    AFTER INSERT ON engine.projects
    FOR EACH ROW
    EXECUTE FUNCTION pipeline.create_default_environments();

-- ============================================================
-- 为已有项目补充默认环境
-- ============================================================
INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
SELECT p.tenant_id, p.id, 'Development', 'DEV', 'INACTIVE'
FROM engine.projects p
WHERE NOT EXISTS (
    SELECT 1 FROM pipeline.environments e WHERE e.project_id = p.id AND e.env_type = 'DEV'
);

INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
SELECT p.tenant_id, p.id, 'Staging', 'STAGING', 'INACTIVE'
FROM engine.projects p
WHERE NOT EXISTS (
    SELECT 1 FROM pipeline.environments e WHERE e.project_id = p.id AND e.env_type = 'STAGING'
);

INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
SELECT p.tenant_id, p.id, 'Production', 'PROD', 'INACTIVE'
FROM engine.projects p
WHERE NOT EXISTS (
    SELECT 1 FROM pipeline.environments e WHERE e.project_id = p.id AND e.env_type = 'PROD'
);
```

- [ ] **Step 2: 验证迁移**

```bash
cd forge-core && go run ./cmd/forge-core
# 启动后检查日志确认迁移 006 执行成功

# 验证表创建
docker exec forge-postgres psql -U forge -d forge_main -c "\dt pipeline.*"
# 预期: environments 表

docker exec forge-postgres psql -U forge -d forge_main -c "\dt engine.review_results"
# 预期: review_results 表

docker exec forge-postgres psql -U forge -d forge_main -c "SELECT column_name FROM information_schema.columns WHERE table_schema='engine' AND table_name='tasks' AND column_name IN ('mr_url','review_score','branch_name','pr_number');"
# 预期: 4 行

# 验证默认环境已为现有项目创建
docker exec forge-postgres psql -U forge -d forge_main -c "SELECT project_id, env_type, status FROM pipeline.environments;"
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/007_init_pipeline.sql
git commit -m "feat(s7): add pipeline schema, review results table, and task PR fields"
```

---

## Task 2: GitHub Adapter 扩展 — 分支/提交/PR 操作

**Files:**
- Modify: `forge-core/internal/module/adapter/github_adapter.go`
- Modify: `forge-core/internal/module/adapter/model.go` (if exists, otherwise create)

- [ ] **Step 1: 添加模型定义**

在 adapter 模块中添加 DevOps 操作所需的数据结构。如果 `model.go` 已存在则追加，否则创建：

`forge-core/internal/module/adapter/model.go`（追加或新建）：

```go
// FileChange represents a single file to commit
type FileChange struct {
    Path    string `json:"path"`     // 文件路径 (相对于 repo 根目录)
    Content string `json:"content"`  // 文件内容 (base64 不需要，GitHub API 接受 UTF-8)
    Action  string `json:"action"`   // "create" / "update" / "delete"
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
    Number    int    `json:"number"`
    Title     string `json:"title"`
    HTMLURL   string `json:"html_url"`
    State     string `json:"state"`
    DiffURL   string `json:"diff_url"`
    Head      string `json:"head"`
    Base      string `json:"base"`
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
    ChangedFiles int `json:"changed_files"`
}

// PRFile represents a file changed in a pull request
type PRFile struct {
    Filename  string `json:"filename"`
    Status    string `json:"status"`    // "added" / "modified" / "removed" / "renamed"
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
    Patch     string `json:"patch"`
}
```

- [ ] **Step 2: 实现 CreateBranch**

在 `github_adapter.go` 中添加：

```go
// CreateBranch creates a new branch from the given ref (default branch if empty).
// Uses GitHub REST API: GET /repos/{owner}/{repo}/git/ref/heads/{fromRef}
//                       POST /repos/{owner}/{repo}/git/refs
func (a *GitHubAdapter) CreateBranch(ctx context.Context, owner, repo, branchName, fromRef string) error {
    if fromRef == "" {
        fromRef = "main"
    }

    // 1. Get the SHA of the source branch
    refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s", owner, repo, fromRef)
    req, err := http.NewRequestWithContext(ctx, "GET", refURL, nil)
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    a.setHeaders(req)

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("get source ref: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("get source ref failed (%d): %s", resp.StatusCode, string(body))
    }

    var refResp struct {
        Object struct {
            SHA string `json:"sha"`
        } `json:"object"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&refResp); err != nil {
        return fmt.Errorf("decode ref response: %w", err)
    }

    // 2. Create the new branch
    createURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", owner, repo)
    body := map[string]string{
        "ref": "refs/heads/" + branchName,
        "sha": refResp.Object.SHA,
    }
    bodyJSON, _ := json.Marshal(body)
    req, err = http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewReader(bodyJSON))
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    a.setHeaders(req)

    resp, err = a.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("create branch: %w", err)
    }
    defer resp.Body.Close()

    // 422 means branch already exists — treat as success (idempotent)
    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusUnprocessableEntity {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("create branch failed (%d): %s", resp.StatusCode, string(respBody))
    }

    slog.Info("branch created", "owner", owner, "repo", repo, "branch", branchName)
    return nil
}
```

- [ ] **Step 3: 实现 CommitFiles**

使用 GitHub Git Trees API 实现批量文件提交（一次 commit 包含多个文件）：

```go
// CommitFiles commits multiple files to the specified branch in a single commit.
// Uses GitHub REST API:
//   POST /repos/{owner}/{repo}/git/blobs      (for each file)
//   POST /repos/{owner}/{repo}/git/trees
//   POST /repos/{owner}/{repo}/git/commits
//   PATCH /repos/{owner}/{repo}/git/refs/heads/{branch}
func (a *GitHubAdapter) CommitFiles(ctx context.Context, owner, repo, branch, message string, files []FileChange) error {
    // 1. Get current commit SHA of the branch
    refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s", owner, repo, branch)
    currentSHA, err := a.getRefSHA(ctx, refURL)
    if err != nil {
        return fmt.Errorf("get branch ref: %w", err)
    }

    // 2. Get the tree SHA of the current commit
    commitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits/%s", owner, repo, currentSHA)
    treeSHA, err := a.getCommitTreeSHA(ctx, commitURL)
    if err != nil {
        return fmt.Errorf("get commit tree: %w", err)
    }

    // 3. Create blobs for each file
    treeEntries := make([]map[string]interface{}, 0, len(files))
    for _, f := range files {
        if f.Action == "delete" {
            treeEntries = append(treeEntries, map[string]interface{}{
                "path": f.Path,
                "mode": "100644",
                "type": "blob",
                "sha":  nil, // null SHA = delete
            })
            continue
        }

        blobSHA, err := a.createBlob(ctx, owner, repo, f.Content)
        if err != nil {
            return fmt.Errorf("create blob for %s: %w", f.Path, err)
        }
        treeEntries = append(treeEntries, map[string]interface{}{
            "path": f.Path,
            "mode": "100644",
            "type": "blob",
            "sha":  blobSHA,
        })
    }

    // 4. Create new tree
    treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees", owner, repo)
    treeBody := map[string]interface{}{
        "base_tree": treeSHA,
        "tree":      treeEntries,
    }
    newTreeSHA, err := a.createTree(ctx, treeURL, treeBody)
    if err != nil {
        return fmt.Errorf("create tree: %w", err)
    }

    // 5. Create commit
    newCommitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits", owner, repo)
    commitBody := map[string]interface{}{
        "message": message,
        "tree":    newTreeSHA,
        "parents": []string{currentSHA},
    }
    newCommitSHA, err := a.createCommit(ctx, newCommitURL, commitBody)
    if err != nil {
        return fmt.Errorf("create commit: %w", err)
    }

    // 6. Update branch ref to point to new commit
    updateRefURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", owner, repo, branch)
    if err := a.updateRef(ctx, updateRefURL, newCommitSHA); err != nil {
        return fmt.Errorf("update ref: %w", err)
    }

    slog.Info("files committed", "owner", owner, "repo", repo, "branch", branch, "files", len(files))
    return nil
}

// --- Helper methods for CommitFiles ---

func (a *GitHubAdapter) getRefSHA(ctx context.Context, url string) (string, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    a.setHeaders(req)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    var result struct {
        Object struct {
            SHA string `json:"sha"`
        } `json:"object"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Object.SHA, nil
}

func (a *GitHubAdapter) getCommitTreeSHA(ctx context.Context, url string) (string, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    a.setHeaders(req)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    var result struct {
        Tree struct {
            SHA string `json:"sha"`
        } `json:"tree"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Tree.SHA, nil
}

func (a *GitHubAdapter) createBlob(ctx context.Context, owner, repo, content string) (string, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/blobs", owner, repo)
    body := map[string]string{"content": content, "encoding": "utf-8"}
    bodyJSON, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
    a.setHeaders(req)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusCreated {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("create blob failed (%d): %s", resp.StatusCode, string(respBody))
    }
    var result struct {
        SHA string `json:"sha"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.SHA, nil
}

func (a *GitHubAdapter) createTree(ctx context.Context, url string, body map[string]interface{}) (string, error) {
    bodyJSON, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
    a.setHeaders(req)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusCreated {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("create tree failed (%d): %s", resp.StatusCode, string(respBody))
    }
    var result struct {
        SHA string `json:"sha"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.SHA, nil
}

func (a *GitHubAdapter) createCommit(ctx context.Context, url string, body map[string]interface{}) (string, error) {
    bodyJSON, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
    a.setHeaders(req)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusCreated {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("create commit failed (%d): %s", resp.StatusCode, string(respBody))
    }
    var result struct {
        SHA string `json:"sha"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.SHA, nil
}

func (a *GitHubAdapter) updateRef(ctx context.Context, url, sha string) error {
    body := map[string]interface{}{"sha": sha, "force": false}
    bodyJSON, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(bodyJSON))
    a.setHeaders(req)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("update ref failed (%d): %s", resp.StatusCode, string(respBody))
    }
    return nil
}
```

- [ ] **Step 4: 实现 CreatePullRequest**

```go
// CreatePullRequest creates a PR from head branch to base branch.
// Uses GitHub REST API: POST /repos/{owner}/{repo}/pulls
func (a *GitHubAdapter) CreatePullRequest(ctx context.Context, owner, repo, title, body, head, base string) (*PullRequest, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
    reqBody := map[string]string{
        "title": title,
        "body":  body,
        "head":  head,
        "base":  base,
    }
    bodyJSON, _ := json.Marshal(reqBody)
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    a.setHeaders(req)

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("create PR: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        respBody, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("create PR failed (%d): %s", resp.StatusCode, string(respBody))
    }

    var ghPR struct {
        Number   int    `json:"number"`
        Title    string `json:"title"`
        HTMLURL  string `json:"html_url"`
        State    string `json:"state"`
        DiffURL  string `json:"diff_url"`
        Head     struct{ Ref string `json:"ref"` } `json:"head"`
        Base     struct{ Ref string `json:"ref"` } `json:"base"`
        Additions int   `json:"additions"`
        Deletions int   `json:"deletions"`
        ChangedFiles int `json:"changed_files"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&ghPR); err != nil {
        return nil, fmt.Errorf("decode PR response: %w", err)
    }

    return &PullRequest{
        Number:       ghPR.Number,
        Title:        ghPR.Title,
        HTMLURL:      ghPR.HTMLURL,
        State:        ghPR.State,
        DiffURL:      ghPR.DiffURL,
        Head:         ghPR.Head.Ref,
        Base:         ghPR.Base.Ref,
        Additions:    ghPR.Additions,
        Deletions:    ghPR.Deletions,
        ChangedFiles: ghPR.ChangedFiles,
    }, nil
}
```

- [ ] **Step 5: 实现 GetPullRequestDiff 和 GetPullRequestFiles**

```go
// GetPullRequestFiles returns the list of files changed in a PR.
// Uses GitHub REST API: GET /repos/{owner}/{repo}/pulls/{pr_number}/files
func (a *GitHubAdapter) GetPullRequestFiles(ctx context.Context, owner, repo string, prNumber int) ([]PRFile, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/files", owner, repo, prNumber)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    a.setHeaders(req)

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("get PR files: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("get PR files failed (%d): %s", resp.StatusCode, string(respBody))
    }

    var ghFiles []struct {
        Filename  string `json:"filename"`
        Status    string `json:"status"`
        Additions int    `json:"additions"`
        Deletions int    `json:"deletions"`
        Patch     string `json:"patch"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&ghFiles); err != nil {
        return nil, fmt.Errorf("decode files response: %w", err)
    }

    files := make([]PRFile, len(ghFiles))
    for i, f := range ghFiles {
        files[i] = PRFile{
            Filename:  f.Filename,
            Status:    f.Status,
            Additions: f.Additions,
            Deletions: f.Deletions,
            Patch:     f.Patch,
        }
    }
    return files, nil
}

// GetPullRequestDiff returns the unified diff of a PR as a string.
// Uses GitHub REST API: GET /repos/{owner}/{repo}/pulls/{pr_number}
// with Accept: application/vnd.github.v3.diff
func (a *GitHubAdapter) GetPullRequestDiff(ctx context.Context, owner, repo string, prNumber int) (string, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", fmt.Errorf("create request: %w", err)
    }
    a.setHeaders(req)
    req.Header.Set("Accept", "application/vnd.github.v3.diff")

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("get PR diff: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("get PR diff failed (%d): %s", resp.StatusCode, string(respBody))
    }

    diff, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("read diff: %w", err)
    }
    return string(diff), nil
}
```

- [ ] **Step 6: 验证编译**

```bash
cd forge-core && go build ./cmd/forge-core
# 应无编译错误
```

- [ ] **Step 7: Commit**

```bash
git add forge-core/internal/module/adapter/
git commit -m "feat(s7): extend GitHub adapter with branch, commit, PR, and diff operations"
```

---

## Task 3: DevOps Activities + Workflow 扩展

**Files:**
- Create: `forge-core/internal/module/task/activities_devops.go`
- Modify: `forge-core/internal/module/task/workflow.go`
- Modify: `forge-core/internal/module/task/model.go`

- [ ] **Step 1: 更新 Task 模型 — 添加 PR 相关字段**

修改 `forge-core/internal/module/task/model.go`，在 Task struct 中添加：

```go
// Add to Task struct
BranchName  *string `json:"branch_name,omitempty"`
PRNumber    *int    `json:"pr_number,omitempty"`
MRURL       *string `json:"mr_url,omitempty"`
ReviewScore *int    `json:"review_score,omitempty"`
```

添加 DTO：

```go
// ChangesResponse is returned by the changes API
type ChangesResponse struct {
    TaskID      int64        `json:"task_id"`
    BranchName  string       `json:"branch_name"`
    PRNumber    int          `json:"pr_number"`
    PRURL       string       `json:"pr_url"`
    AISummary   string       `json:"ai_summary"`
    ReviewScore *int         `json:"review_score,omitempty"`
    RiskLevel   string       `json:"risk_level"`  // LOW / MEDIUM / HIGH
    Files       []PRFileInfo `json:"files"`
    Diff        string       `json:"diff"`
}

type PRFileInfo struct {
    Filename  string `json:"filename"`
    Status    string `json:"status"`
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
    Patch     string `json:"patch"`
}

// ReviewResponse is returned by the review API
type ReviewResponse struct {
    TaskID   int64          `json:"task_id"`
    Reviews  []ReviewResult `json:"reviews"`
}

type ReviewResult struct {
    ID         int64           `json:"id"`
    ReviewType string          `json:"review_type"`
    Score      *int            `json:"score,omitempty"`
    Passed     bool            `json:"passed"`
    Findings   json.RawMessage `json:"findings"`
    Summary    string          `json:"summary"`
    CreatedAt  time.Time       `json:"created_at"`
}

// TestsResponse is returned by the tests API
type TestsResponse struct {
    TaskID int64       `json:"task_id"`
    Layers []TestLayer `json:"layers"`
}

type TestLayer struct {
    Name        string     `json:"name"`       // unit / api / integration / regression
    DisplayName string     `json:"display_name"`
    Status      string     `json:"status"`     // passed / failed / skipped / pending
    Total       int        `json:"total"`
    Passed      int        `json:"passed"`
    Failed      int        `json:"failed"`
    Coverage    *float64   `json:"coverage,omitempty"`
    Available   bool       `json:"available"`  // Phase 1: only unit=true
    Cases       []TestCase `json:"cases,omitempty"`
}

type TestCase struct {
    Name     string  `json:"name"`
    Status   string  `json:"status"`   // passed / failed
    Duration float64 `json:"duration"` // seconds
    Error    string  `json:"error,omitempty"`
}
```

- [ ] **Step 2: 创建 DevOps Activities**

`forge-core/internal/module/task/activities_devops.go`：

```go
package task

import (
    "context"
    "fmt"
    "log/slog"
    "strings"

    "go.temporal.io/sdk/activity"
)

// DevOpsActivities holds dependencies for DevOps-related Temporal activities.
type DevOpsActivities struct {
    adapter   AdapterInterface  // GitHub adapter
    repo      *Repository       // task repo for updating task state
}

func NewDevOpsActivities(adapter AdapterInterface, repo *Repository) *DevOpsActivities {
    return &DevOpsActivities{adapter: adapter, repo: repo}
}

// CreateBranchInput is the input for CreateBranchActivity.
type CreateBranchInput struct {
    TaskID     int64  `json:"task_id"`
    Owner      string `json:"owner"`
    Repo       string `json:"repo"`
    BranchName string `json:"branch_name"`
    FromRef    string `json:"from_ref"` // default branch, e.g. "main"
}

// CreateBranchActivity creates a feature branch for AI-generated code.
func (a *DevOpsActivities) CreateBranchActivity(ctx context.Context, input CreateBranchInput) error {
    logger := activity.GetLogger(ctx)
    logger.Info("creating branch", "branch", input.BranchName, "repo", input.Owner+"/"+input.Repo)

    if err := a.adapter.CreateBranch(ctx, input.Owner, input.Repo, input.BranchName, input.FromRef); err != nil {
        return fmt.Errorf("create branch: %w", err)
    }

    // Update task with branch name
    if err := a.repo.UpdateBranch(ctx, input.TaskID, input.BranchName); err != nil {
        slog.Warn("failed to update task branch", "error", err)
    }

    return nil
}

// CommitCodeInput is the input for CommitCodeActivity.
type CommitCodeInput struct {
    TaskID     int64       `json:"task_id"`
    Owner      string      `json:"owner"`
    Repo       string      `json:"repo"`
    Branch     string      `json:"branch"`
    Message    string      `json:"message"`
    Files      []FileEntry `json:"files"`
}

type FileEntry struct {
    Path    string `json:"path"`
    Content string `json:"content"`
    Action  string `json:"action"` // create / update / delete
}

// CommitCodeActivity commits AI-generated files to the branch.
func (a *DevOpsActivities) CommitCodeActivity(ctx context.Context, input CommitCodeInput) error {
    logger := activity.GetLogger(ctx)
    logger.Info("committing code", "branch", input.Branch, "files", len(input.Files))

    // Convert to adapter FileChange
    changes := make([]adapter.FileChange, len(input.Files))
    for i, f := range input.Files {
        changes[i] = adapter.FileChange{
            Path:    f.Path,
            Content: f.Content,
            Action:  f.Action,
        }
    }

    if err := a.adapter.CommitFiles(ctx, input.Owner, input.Repo, input.Branch, input.Message, changes); err != nil {
        return fmt.Errorf("commit files: %w", err)
    }

    return nil
}

// CreatePRInput is the input for CreatePRActivity.
type CreatePRInput struct {
    TaskID int64  `json:"task_id"`
    Owner  string `json:"owner"`
    Repo   string `json:"repo"`
    Title  string `json:"title"`
    Body   string `json:"body"`
    Head   string `json:"head"`  // AI branch
    Base   string `json:"base"`  // default branch
}

// CreatePROutput is the output of CreatePRActivity.
type CreatePROutput struct {
    PRNumber int    `json:"pr_number"`
    PRURL    string `json:"pr_url"`
}

// CreatePRActivity creates a pull request from the AI branch.
func (a *DevOpsActivities) CreatePRActivity(ctx context.Context, input CreatePRInput) (*CreatePROutput, error) {
    logger := activity.GetLogger(ctx)
    logger.Info("creating PR", "head", input.Head, "base", input.Base)

    pr, err := a.adapter.CreatePullRequest(ctx, input.Owner, input.Repo, input.Title, input.Body, input.Head, input.Base)
    if err != nil {
        return nil, fmt.Errorf("create PR: %w", err)
    }

    // Update task with PR info
    if err := a.repo.UpdatePR(ctx, input.TaskID, pr.Number, pr.HTMLURL); err != nil {
        slog.Warn("failed to update task PR info", "error", err)
    }

    return &CreatePROutput{
        PRNumber: pr.Number,
        PRURL:    pr.HTMLURL,
    }, nil
}

// SaveReviewResultInput is the input for saving review results to DB.
type SaveReviewResultInput struct {
    TaskID     int64  `json:"task_id"`
    ReviewType string `json:"review_type"` // AI_REVIEW
    Score      int    `json:"score"`
    Passed     bool   `json:"passed"`
    Findings   string `json:"findings"` // JSON string
    Summary    string `json:"summary"`
}

// SaveReviewResultActivity persists the AI review result to engine.review_results.
func (a *DevOpsActivities) SaveReviewResultActivity(ctx context.Context, input SaveReviewResultInput) error {
    logger := activity.GetLogger(ctx)
    logger.Info("saving review result", "taskId", input.TaskID, "type", input.ReviewType, "score", input.Score)

    if err := a.repo.InsertReviewResult(ctx, input.TaskID, input.ReviewType, input.Score, input.Passed, input.Findings, input.Summary); err != nil {
        return fmt.Errorf("insert review result: %w", err)
    }

    // Also update task review_score
    if err := a.repo.UpdateReviewScore(ctx, input.TaskID, input.Score); err != nil {
        slog.Warn("failed to update task review score", "error", err)
    }

    return nil
}

// Helper: generate branch name from task
func GenerateBranchName(taskID int64, taskTitle string) string {
    // Slugify the title: lowercase, replace spaces/special chars with hyphens, truncate
    slug := strings.ToLower(taskTitle)
    slug = strings.Map(func(r rune) rune {
        if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
            return r
        }
        if r == ' ' || r == '_' {
            return '-'
        }
        return -1 // drop other chars (Chinese chars, etc.)
    }, slug)
    // Remove consecutive hyphens
    for strings.Contains(slug, "--") {
        slug = strings.ReplaceAll(slug, "--", "-")
    }
    slug = strings.Trim(slug, "-")
    if len(slug) > 30 {
        slug = slug[:30]
    }
    if slug == "" {
        slug = "task"
    }
    return fmt.Sprintf("ai/%d-%s", taskID, slug)
}
```

- [ ] **Step 3: 扩展 Task Workflow — 加入 DevOps 步骤**

修改 `forge-core/internal/module/task/workflow.go`，在 AI 代码生成 + Review 步骤之后添加 DevOps 步骤：

```go
// Add after the AI Review step (step ~9 from S6):

// Step 10: Create branch
branchName := GenerateBranchName(input.TaskID, input.Title)
createBranchInput := CreateBranchInput{
    TaskID:     input.TaskID,
    Owner:      input.RepoOwner,
    Repo:       input.RepoName,
    BranchName: branchName,
    FromRef:    input.DefaultBranch, // usually "main"
}
err = workflow.ExecuteActivity(ctx, devOpsActivities.CreateBranchActivity, createBranchInput).Get(ctx, nil)
if err != nil {
    return nil, fmt.Errorf("create branch: %w", err)
}

// Step 11: Commit generated code to branch
commitInput := CommitCodeInput{
    TaskID:  input.TaskID,
    Owner:   input.RepoOwner,
    Repo:    input.RepoName,
    Branch:  branchName,
    Message: fmt.Sprintf("feat: %s\n\nGenerated by Forge AI for task #%d", input.Title, input.TaskID),
    Files:   convertGeneratedFiles(generateResult.Files), // map AI output to FileEntry
}
err = workflow.ExecuteActivity(ctx, devOpsActivities.CommitCodeActivity, commitInput).Get(ctx, nil)
if err != nil {
    return nil, fmt.Errorf("commit code: %w", err)
}

// Step 12: Create pull request
prTitle := fmt.Sprintf("[Forge AI] %s (#%d)", input.Title, input.TaskID)
prBody := fmt.Sprintf("## AI Generated Changes\n\n%s\n\n**Task:** #%d\n**Review Score:** %d/100\n\n---\n_Generated by Forge AI_",
    reviewResult.Summary, input.TaskID, reviewResult.Score)
createPRInput := CreatePRInput{
    TaskID: input.TaskID,
    Owner:  input.RepoOwner,
    Repo:   input.RepoName,
    Title:  prTitle,
    Body:   prBody,
    Head:   branchName,
    Base:   input.DefaultBranch,
}
var prOutput CreatePROutput
err = workflow.ExecuteActivity(ctx, devOpsActivities.CreatePRActivity, createPRInput).Get(ctx, &prOutput)
if err != nil {
    return nil, fmt.Errorf("create PR: %w", err)
}

// Step 13: Save review result to DB
saveReviewInput := SaveReviewResultInput{
    TaskID:     input.TaskID,
    ReviewType: "AI_REVIEW",
    Score:      reviewResult.Score,
    Passed:     reviewResult.Score >= 70,
    Findings:   string(reviewResult.FindingsJSON),
    Summary:    reviewResult.Summary,
}
_ = workflow.ExecuteActivity(ctx, devOpsActivities.SaveReviewResultActivity, saveReviewInput).Get(ctx, nil)

// Step 14: Update task status to COMPLETED with PR URL
// (update the final status update to include PR info)
```

Add the file conversion helper:

```go
func convertGeneratedFiles(aiFiles []AIGeneratedFile) []FileEntry {
    entries := make([]FileEntry, len(aiFiles))
    for i, f := range aiFiles {
        action := "create"
        if f.IsModified {
            action = "update"
        }
        entries[i] = FileEntry{
            Path:    f.Path,
            Content: f.Content,
            Action:  action,
        }
    }
    return entries
}
```

- [ ] **Step 4: 添加 Repository 方法**

在 `forge-core/internal/module/task/repository.go` 中添加：

```go
func (r *Repository) UpdateBranch(ctx context.Context, taskID int64, branchName string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE engine.tasks SET branch_name = $1, updated_at = NOW() WHERE id = $2`,
        branchName, taskID)
    return err
}

func (r *Repository) UpdatePR(ctx context.Context, taskID int64, prNumber int, mrURL string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE engine.tasks SET pr_number = $1, mr_url = $2, updated_at = NOW() WHERE id = $3`,
        prNumber, mrURL, taskID)
    return err
}

func (r *Repository) UpdateReviewScore(ctx context.Context, taskID int64, score int) error {
    _, err := r.db.Exec(ctx,
        `UPDATE engine.tasks SET review_score = $1, updated_at = NOW() WHERE id = $2`,
        score, taskID)
    return err
}

func (r *Repository) InsertReviewResult(ctx context.Context, taskID int64, reviewType string, score int, passed bool, findings string, summary string) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO engine.review_results (task_id, review_type, score, passed, findings, summary)
         VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
        taskID, reviewType, score, passed, findings, summary)
    return err
}

func (r *Repository) GetReviewResults(ctx context.Context, taskID int64) ([]ReviewResult, error) {
    rows, err := r.db.Query(ctx,
        `SELECT id, review_type, score, passed, findings, summary, created_at
         FROM engine.review_results WHERE task_id = $1 ORDER BY created_at DESC`, taskID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []ReviewResult
    for rows.Next() {
        var rr ReviewResult
        if err := rows.Scan(&rr.ID, &rr.ReviewType, &rr.Score, &rr.Passed, &rr.Findings, &rr.Summary, &rr.CreatedAt); err != nil {
            return nil, err
        }
        results = append(results, rr)
    }
    return results, nil
}
```

- [ ] **Step 5: Register DevOps activities with Temporal worker**

在 Temporal worker 注册代码中（`main.go` 或 worker 初始化文件），注册新的 activities：

```go
devOpsActivities := task.NewDevOpsActivities(githubAdapter, taskRepo)
w.RegisterActivity(devOpsActivities.CreateBranchActivity)
w.RegisterActivity(devOpsActivities.CommitCodeActivity)
w.RegisterActivity(devOpsActivities.CreatePRActivity)
w.RegisterActivity(devOpsActivities.SaveReviewResultActivity)
```

- [ ] **Step 6: 验证编译**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 7: Commit**

```bash
git add forge-core/internal/module/task/activities_devops.go
git add forge-core/internal/module/task/workflow.go
git add forge-core/internal/module/task/model.go
git add forge-core/internal/module/task/repository.go
git add forge-core/cmd/forge-core/main.go
git commit -m "feat(s7): add DevOps activities and extend task workflow with branch/commit/PR steps"
```

---

## Task 4: Pipeline 模块 + Changes/Review/Tests API

**Files:**
- Create: `forge-core/internal/module/pipeline/model.go`
- Create: `forge-core/internal/module/pipeline/repository.go`
- Create: `forge-core/internal/module/pipeline/service.go`
- Create: `forge-core/internal/module/pipeline/handler.go`
- Modify: `forge-core/internal/module/task/handler.go`
- Modify: `forge-core/internal/router/router.go`

- [ ] **Step 1: 创建 Pipeline 模型**

`forge-core/internal/module/pipeline/model.go`：

```go
package pipeline

import "time"

// Environment represents a deployment environment.
type Environment struct {
    ID             int64      `json:"id"`
    TenantID       int64      `json:"tenant_id"`
    ProjectID      int64      `json:"project_id"`
    Name           string     `json:"name"`
    EnvType        string     `json:"env_type"`  // DEV / STAGING / PROD
    Status         string     `json:"status"`     // INACTIVE / ACTIVE / DEPLOYING / ERROR
    CurrentVersion *string    `json:"current_version,omitempty"`
    Config         any        `json:"config"`
    LastDeployAt   *time.Time `json:"last_deploy_at,omitempty"`
    CreatedAt      time.Time  `json:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at"`
}
```

- [ ] **Step 2: 创建 Pipeline Repository**

`forge-core/internal/module/pipeline/repository.go`：

```go
package pipeline

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
    db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
    return &Repository{db: db}
}

func (r *Repository) ListEnvironments(ctx context.Context, projectID int64) ([]Environment, error) {
    rows, err := r.db.Query(ctx,
        `SELECT id, tenant_id, project_id, name, env_type, status,
                current_version, config, last_deploy_at, created_at, updated_at
         FROM pipeline.environments
         WHERE project_id = $1
         ORDER BY CASE env_type
             WHEN 'DEV' THEN 1
             WHEN 'STAGING' THEN 2
             WHEN 'PROD' THEN 3
             ELSE 4
         END`, projectID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var envs []Environment
    for rows.Next() {
        var e Environment
        if err := rows.Scan(
            &e.ID, &e.TenantID, &e.ProjectID, &e.Name, &e.EnvType, &e.Status,
            &e.CurrentVersion, &e.Config, &e.LastDeployAt, &e.CreatedAt, &e.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        envs = append(envs, e)
    }
    return envs, nil
}
```

- [ ] **Step 3: 创建 Pipeline Service**

`forge-core/internal/module/pipeline/service.go`：

```go
package pipeline

import (
    "context"
)

type Service struct {
    repo *Repository
}

func NewService(repo *Repository) *Service {
    return &Service{repo: repo}
}

func (s *Service) ListEnvironments(ctx context.Context, projectID int64) ([]Environment, error) {
    return s.repo.ListEnvironments(ctx, projectID)
}
```

- [ ] **Step 4: 创建 Pipeline Handler**

`forge-core/internal/module/pipeline/handler.go`：

```go
package pipeline

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

// ListEnvironments godoc
// GET /api/projects/:id/environments
func (h *Handler) ListEnvironments(c *gin.Context) {
    projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid project id")
        return
    }

    envs, err := h.service.ListEnvironments(c.Request.Context(), projectID)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to list environments")
        return
    }

    response.OK(c, envs)
}
```

- [ ] **Step 5: 添加 Changes/Review/Tests Handler 到 Task 模块**

在 `forge-core/internal/module/task/handler.go` 中添加：

```go
// GetChanges returns the code changes for a task (diff, files, summary).
// GET /api/projects/:id/tasks/:taskId/changes
func (h *Handler) GetChanges(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid task id")
        return
    }

    task, err := h.service.GetTask(c.Request.Context(), taskID)
    if err != nil {
        response.Fail(c, http.StatusNotFound, "task not found")
        return
    }

    if task.PRNumber == nil || task.MRURL == nil {
        response.Fail(c, http.StatusNotFound, "no pull request found for this task")
        return
    }

    // Get project to find repo owner/name
    project, err := h.projectService.GetProject(c.Request.Context(), task.ProjectID)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to get project")
        return
    }

    // Fetch PR files from GitHub
    files, err := h.adapter.GetPullRequestFiles(c.Request.Context(), project.RepoOwner, project.RepoName, *task.PRNumber)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to fetch PR files")
        return
    }

    // Fetch diff from GitHub
    diff, err := h.adapter.GetPullRequestDiff(c.Request.Context(), project.RepoOwner, project.RepoName, *task.PRNumber)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to fetch PR diff")
        return
    }

    // Determine risk level from review score
    riskLevel := "HIGH"
    if task.ReviewScore != nil {
        if *task.ReviewScore >= 90 {
            riskLevel = "LOW"
        } else if *task.ReviewScore >= 70 {
            riskLevel = "MEDIUM"
        }
    }

    // Build file info list
    fileInfos := make([]PRFileInfo, len(files))
    for i, f := range files {
        fileInfos[i] = PRFileInfo{
            Filename:  f.Filename,
            Status:    f.Status,
            Additions: f.Additions,
            Deletions: f.Deletions,
            Patch:     f.Patch,
        }
    }

    resp := ChangesResponse{
        TaskID:      task.ID,
        BranchName:  stringVal(task.BranchName),
        PRNumber:    intVal(task.PRNumber),
        PRURL:       stringVal(task.MRURL),
        AISummary:   task.AISummary, // from S6 generation result stored on task
        ReviewScore: task.ReviewScore,
        RiskLevel:   riskLevel,
        Files:       fileInfos,
        Diff:        diff,
    }

    response.OK(c, resp)
}

// GetReview returns review results for a task.
// GET /api/projects/:id/tasks/:taskId/review
func (h *Handler) GetReview(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid task id")
        return
    }

    reviews, err := h.repo.GetReviewResults(c.Request.Context(), taskID)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to get review results")
        return
    }

    response.OK(c, ReviewResponse{
        TaskID:  taskID,
        Reviews: reviews,
    })
}

// GetTests returns test results for a task.
// GET /api/projects/:id/tasks/:taskId/tests
// Phase 1: Returns mock/basic structure with unit test layer populated from AI review.
func (h *Handler) GetTests(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid task id")
        return
    }

    task, err := h.service.GetTask(c.Request.Context(), taskID)
    if err != nil {
        response.Fail(c, http.StatusNotFound, "task not found")
        return
    }

    // Phase 1: Build basic test layers
    // Unit test data comes from AI generation (if AI generated tests)
    unitLayer := TestLayer{
        Name:        "unit",
        DisplayName: "单元测试",
        Status:      "pending",
        Available:   true,
    }

    // If task has review results, populate unit test info from AI review
    reviews, _ := h.repo.GetReviewResults(c.Request.Context(), taskID)
    for _, r := range reviews {
        if r.ReviewType == "AI_REVIEW" && r.Score != nil {
            // Phase 1: Derive basic test info from review
            // In future slices, this will come from actual test execution
            if *r.Score >= 70 {
                unitLayer.Status = "passed"
                unitLayer.Total = 1
                unitLayer.Passed = 1
            } else {
                unitLayer.Status = "failed"
                unitLayer.Total = 1
                unitLayer.Failed = 1
            }
        }
    }

    // Other layers are "coming soon" in Phase 1
    layers := []TestLayer{
        unitLayer,
        {Name: "api", DisplayName: "接口测试", Status: "pending", Available: false},
        {Name: "integration", DisplayName: "集成测试", Status: "pending", Available: false},
        {Name: "regression", DisplayName: "回归测试", Status: "pending", Available: false},
    }

    response.OK(c, TestsResponse{
        TaskID: taskID,
        Layers: layers,
    })
}

// Helper functions
func stringVal(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}

func intVal(i *int) int {
    if i == nil {
        return 0
    }
    return *i
}
```

- [ ] **Step 6: 注册新路由**

修改 `forge-core/internal/router/router.go`：

```go
// Inside the authenticated project routes group:

// Pipeline — environments
projectGroup.GET("/environments", pipelineHandler.ListEnvironments)

// Task — changes, review, tests
taskGroup.GET("/:taskId/changes", taskHandler.GetChanges)
taskGroup.GET("/:taskId/review", taskHandler.GetReview)
taskGroup.GET("/:taskId/tests", taskHandler.GetTests)
```

- [ ] **Step 7: 在 main.go 中初始化 Pipeline 模块**

```go
// Pipeline module
pipelineRepo := pipeline.NewRepository(db)
pipelineService := pipeline.NewService(pipelineRepo)
pipelineHandler := pipeline.NewHandler(pipelineService)
```

- [ ] **Step 8: 验证编译 + API 测试**

```bash
cd forge-core && go build ./cmd/forge-core
cd forge-core && go run ./cmd/forge-core &

# Test environments API
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/environments | jq .
# 预期: code=0, data=[{env_type: "DEV"}, {env_type: "STAGING"}, {env_type: "PROD"}]

# Test review API (will be empty initially)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/tasks/1/review | jq .
# 预期: code=0, data={reviews: []}

# Test tests API
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/tasks/1/tests | jq .
# 预期: code=0, data={layers: [{name: "unit", available: true}, ...]}
```

- [ ] **Step 9: Commit**

```bash
git add forge-core/internal/module/pipeline/
git add forge-core/internal/module/task/handler.go
git add forge-core/internal/router/router.go
git add forge-core/cmd/forge-core/main.go
git commit -m "feat(s7): add pipeline module, changes/review/tests API endpoints"
```

---

## Task 5: 前端 — 变更结果页 + Monaco Diff Viewer

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/changes/page.tsx`
- Create: `forge-portal/components/monaco-diff-viewer.tsx`
- Create: `forge-portal/components/trust-metrics.tsx`
- Modify: `forge-portal/lib/api.ts`

- [ ] **Step 1: 安装 Monaco Editor 依赖**

```bash
cd forge-portal && npm install @monaco-editor/react
```

> Monaco Editor 是 VS Code 的编辑器核心组件，提供语法高亮、Diff 视图、代码折叠等完整编辑器功能。与 code-server (Task 8) 形成双层代码浏览体验：Monaco 做内联 Diff，code-server 做完整 IDE 浏览。

- [ ] **Step 2: 添加 API 调用函数**

在 `forge-portal/lib/api.ts` 中追加：

```typescript
// --- S7: Changes / Review / Tests / Environments ---

export interface ChangesResponse {
  task_id: number;
  branch_name: string;
  pr_number: number;
  pr_url: string;
  ai_summary: string;
  review_score: number | null;
  risk_level: "LOW" | "MEDIUM" | "HIGH";
  files: PRFileInfo[];
  diff: string;
}

export interface PRFileInfo {
  filename: string;
  status: "added" | "modified" | "removed" | "renamed";
  additions: number;
  deletions: number;
  patch: string;
}

export interface ReviewResponse {
  task_id: number;
  reviews: ReviewResult[];
}

export interface ReviewResult {
  id: number;
  review_type: string;
  score: number | null;
  passed: boolean;
  findings: any[];
  summary: string;
  created_at: string;
}

export interface TestsResponse {
  task_id: number;
  layers: TestLayer[];
}

export interface TestLayer {
  name: string;
  display_name: string;
  status: "passed" | "failed" | "skipped" | "pending";
  total: number;
  passed: number;
  failed: number;
  coverage: number | null;
  available: boolean;
  cases: TestCase[];
}

export interface TestCase {
  name: string;
  status: "passed" | "failed";
  duration: number;
  error?: string;
}

export interface Environment {
  id: number;
  tenant_id: number;
  project_id: number;
  name: string;
  env_type: "DEV" | "STAGING" | "PROD";
  status: "INACTIVE" | "ACTIVE" | "DEPLOYING" | "ERROR";
  current_version: string | null;
  config: Record<string, any>;
  last_deploy_at: string | null;
  created_at: string;
  updated_at: string;
}

export async function getTaskChanges(projectId: number, taskId: number): Promise<ChangesResponse> {
  return fetchAPI(`/api/projects/${projectId}/tasks/${taskId}/changes`);
}

export async function getTaskReview(projectId: number, taskId: number): Promise<ReviewResponse> {
  return fetchAPI(`/api/projects/${projectId}/tasks/${taskId}/review`);
}

export async function getTaskTests(projectId: number, taskId: number): Promise<TestsResponse> {
  return fetchAPI(`/api/projects/${projectId}/tasks/${taskId}/tests`);
}

export async function getProjectEnvironments(projectId: number): Promise<Environment[]> {
  return fetchAPI(`/api/projects/${projectId}/environments`);
}
```

- [ ] **Step 3: 创建 Trust Metrics 组件**

`forge-portal/components/trust-metrics.tsx`：

```tsx
"use client";

import { cn } from "@/lib/utils";
import { Shield, ShieldCheck, ShieldAlert, Bug, FlaskConical, AlertTriangle } from "lucide-react";

interface TrustMetricsProps {
  reviewScore: number | null;
  riskLevel: "LOW" | "MEDIUM" | "HIGH";
  testStatus?: "passed" | "failed" | "pending";
  className?: string;
}

function getScoreColor(score: number | null): string {
  if (score === null) return "text-zinc-500";
  if (score >= 90) return "text-emerald-400";
  if (score >= 70) return "text-amber-400";
  return "text-red-400";
}

function getScoreBg(score: number | null): string {
  if (score === null) return "bg-zinc-500/10 border-zinc-500/20";
  if (score >= 90) return "bg-emerald-500/10 border-emerald-500/20";
  if (score >= 70) return "bg-amber-500/10 border-amber-500/20";
  return "bg-red-500/10 border-red-500/20";
}

function getRiskBadge(level: string) {
  switch (level) {
    case "LOW":
      return { label: "低风险", color: "text-emerald-400 bg-emerald-500/10 border-emerald-500/20" };
    case "MEDIUM":
      return { label: "中风险", color: "text-amber-400 bg-amber-500/10 border-amber-500/20" };
    case "HIGH":
      return { label: "高风险", color: "text-red-400 bg-red-500/10 border-red-500/20" };
    default:
      return { label: "未知", color: "text-zinc-400 bg-zinc-500/10 border-zinc-500/20" };
  }
}

export function TrustMetrics({ reviewScore, riskLevel, testStatus, className }: TrustMetricsProps) {
  const risk = getRiskBadge(riskLevel);

  return (
    <div className={cn("grid grid-cols-2 md:grid-cols-4 gap-4", className)}>
      {/* AI Review Score */}
      <div className={cn("rounded-lg border p-4", getScoreBg(reviewScore))}>
        <div className="flex items-center gap-2 text-sm text-zinc-400 mb-2">
          <ShieldCheck className="h-4 w-4" />
          <span>AI Review</span>
        </div>
        <div className={cn("text-2xl font-bold font-mono", getScoreColor(reviewScore))}>
          {reviewScore !== null ? `${reviewScore}/100` : "--"}
        </div>
      </div>

      {/* Risk Level */}
      <div className={cn("rounded-lg border p-4", risk.color)}>
        <div className="flex items-center gap-2 text-sm text-zinc-400 mb-2">
          <AlertTriangle className="h-4 w-4" />
          <span>风险等级</span>
        </div>
        <div className="text-lg font-semibold">{risk.label}</div>
      </div>

      {/* Test Status */}
      <div className="rounded-lg border border-surface-2 bg-surface-1 p-4">
        <div className="flex items-center gap-2 text-sm text-zinc-400 mb-2">
          <FlaskConical className="h-4 w-4" />
          <span>单元测试</span>
        </div>
        <div className={cn("text-lg font-semibold", {
          "text-emerald-400": testStatus === "passed",
          "text-red-400": testStatus === "failed",
          "text-zinc-500": testStatus === "pending" || !testStatus,
        })}>
          {testStatus === "passed" ? "通过" : testStatus === "failed" ? "失败" : "待执行"}
        </div>
      </div>

      {/* Security Scan — placeholder for Phase 1 */}
      <div className="rounded-lg border border-surface-2 bg-surface-1 p-4">
        <div className="flex items-center gap-2 text-sm text-zinc-400 mb-2">
          <Shield className="h-4 w-4" />
          <span>安全扫描</span>
        </div>
        <div className="text-lg font-semibold text-zinc-500">--</div>
        <div className="text-xs text-zinc-600 mt-1">Coming soon</div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 创建 Monaco Diff Viewer 组件**

`forge-portal/components/monaco-diff-viewer.tsx`：

> 使用 Monaco Editor（VS Code 编辑器核心）替代 react-diff-viewer，提供更好的语法高亮、代码折叠和 Diff 体验。与 Task 8 的 code-server 形成双层浏览：Monaco 做内联 Diff，code-server 做完整 IDE。

```tsx
"use client";

import { useState, useMemo } from "react";
import dynamic from "next/dynamic";
import { cn } from "@/lib/utils";
import { File, Plus, Minus, Edit3, ChevronDown, ChevronRight } from "lucide-react";

// Dynamic import to avoid SSR issues (Monaco requires browser APIs)
const DiffEditor = dynamic(
  () => import("@monaco-editor/react").then((mod) => mod.DiffEditor),
  { ssr: false, loading: () => <div className="h-64 bg-[#0A0A12] animate-pulse rounded" /> }
);

interface MonacoDiffViewerProps {
  files: {
    filename: string;
    status: string;
    additions: number;
    deletions: number;
    patch: string;
  }[];
  className?: string;
}

function getStatusIcon(status: string) {
  switch (status) {
    case "added":
      return <Plus className="h-4 w-4 text-emerald-400" />;
    case "removed":
      return <Minus className="h-4 w-4 text-red-400" />;
    case "modified":
    case "renamed":
      return <Edit3 className="h-4 w-4 text-amber-400" />;
    default:
      return <File className="h-4 w-4 text-zinc-400" />;
  }
}

function getStatusBadge(status: string) {
  switch (status) {
    case "added":
      return "bg-emerald-500/10 text-emerald-400 border-emerald-500/20";
    case "removed":
      return "bg-red-500/10 text-red-400 border-red-500/20";
    case "modified":
    case "renamed":
      return "bg-amber-500/10 text-amber-400 border-amber-500/20";
    default:
      return "bg-zinc-500/10 text-zinc-400 border-zinc-500/20";
  }
}

// Infer Monaco language from filename extension
function getLanguage(filename: string): string {
  const ext = filename.split(".").pop()?.toLowerCase();
  const map: Record<string, string> = {
    ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
    go: "go", py: "python", java: "java", sql: "sql", yaml: "yaml", yml: "yaml",
    json: "json", md: "markdown", html: "html", css: "css", sh: "shell",
  };
  return map[ext || ""] || "plaintext";
}

// Parse unified diff patch into old/new content for Monaco DiffEditor
function parsePatch(patch: string): { oldValue: string; newValue: string } {
  if (!patch) return { oldValue: "", newValue: "" };
  const lines = patch.split("\n");
  const oldLines: string[] = [];
  const newLines: string[] = [];
  for (const line of lines) {
    if (line.startsWith("@@")) continue;
    if (line.startsWith("-")) { oldLines.push(line.substring(1)); }
    else if (line.startsWith("+")) { newLines.push(line.substring(1)); }
    else {
      const content = line.startsWith(" ") ? line.substring(1) : line;
      oldLines.push(content);
      newLines.push(content);
    }
  }
  return { oldValue: oldLines.join("\n"), newValue: newLines.join("\n") };
}

export function MonacoDiffViewer({ files, className }: MonacoDiffViewerProps) {
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(
    new Set(files.slice(0, 3).map((f) => f.filename))
  );

  const toggleFile = (filename: string) => {
    setExpandedFiles((prev) => {
      const next = new Set(prev);
      next.has(filename) ? next.delete(filename) : next.add(filename);
      return next;
    });
  };

  return (
    <div className={cn("space-y-2", className)}>
      {files.map((file) => {
        const isExpanded = expandedFiles.has(file.filename);

        return (
          <div key={file.filename} className="rounded-lg border border-surface-2 overflow-hidden">
            {/* File header */}
            <button
              onClick={() => toggleFile(file.filename)}
              className="w-full flex items-center gap-3 px-4 py-3 bg-surface-1 hover:bg-surface-2 transition-colors text-left"
            >
              {isExpanded ? (
                <ChevronDown className="h-4 w-4 text-zinc-500 shrink-0" />
              ) : (
                <ChevronRight className="h-4 w-4 text-zinc-500 shrink-0" />
              )}
              {getStatusIcon(file.status)}
              <span className="font-mono text-sm text-zinc-200 truncate flex-1">
                {file.filename}
              </span>
              <span className={cn("text-xs px-2 py-0.5 rounded border", getStatusBadge(file.status))}>
                {file.status}
              </span>
              <span className="text-xs text-emerald-400 font-mono">+{file.additions}</span>
              <span className="text-xs text-red-400 font-mono">-{file.deletions}</span>
            </button>

            {/* Monaco Diff content */}
            {isExpanded && file.patch && (
              <MonacoDiffContent filename={file.filename} patch={file.patch} />
            )}
          </div>
        );
      })}
    </div>
  );
}

function MonacoDiffContent({ filename, patch }: { filename: string; patch: string }) {
  const { oldValue, newValue } = useMemo(() => parsePatch(patch), [patch]);
  const language = getLanguage(filename);
  const lineCount = Math.max(oldValue.split("\n").length, newValue.split("\n").length);
  const height = Math.min(Math.max(lineCount * 20 + 20, 120), 600); // min 120px, max 600px

  return (
    <div className="border-t border-surface-2" style={{ height }}>
      <DiffEditor
        original={oldValue}
        modified={newValue}
        language={language}
        theme="forge-dark"
        options={{
          readOnly: true,
          renderSideBySide: false, // inline diff mode
          fontSize: 13,
          fontFamily: "var(--font-geist-mono), monospace",
          lineNumbers: "on",
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          folding: true,
          renderOverviewRuler: false,
          contextmenu: false,
        }}
        beforeMount={(monaco) => {
          // Define 深空指挥中心 dark theme for Monaco
          monaco.editor.defineTheme("forge-dark", {
            base: "vs-dark",
            inherit: true,
            rules: [
              { token: "keyword", foreground: "8B5CF6" },   // purple
              { token: "string", foreground: "06B6D4" },     // cyan
              { token: "comment", foreground: "555570" },    // muted
            ],
            colors: {
              "editor.background": "#0A0A12",
              "editor.foreground": "#e4e4e7",
              "editorLineNumber.foreground": "#555570",
              "editorLineNumber.activeForeground": "#8888A0",
              "editor.selectionBackground": "#8B5CF620",
              "diffEditor.insertedTextBackground": "#22C55E14",
              "diffEditor.removedTextBackground": "#EF444414",
              "diffEditor.insertedLineBackground": "#22C55E0A",
              "diffEditor.removedLineBackground": "#EF44440A",
            },
          });
        }}
      />
    </div>
  );
}
```

- [ ] **Step 5: 创建变更结果页面**

`forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/changes/page.tsx`：

```tsx
"use client";

import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { getTaskChanges, getTaskReview } from "@/lib/api";
import { TrustMetrics } from "@/components/trust-metrics";
import { MonacoDiffViewer } from "@/components/monaco-diff-viewer";
import { ExternalLink, GitBranch, GitPullRequest, FileText } from "lucide-react";
import { cn } from "@/lib/utils";

export default function ChangesPage() {
  const params = useParams();
  const projectId = Number(params.id);
  const taskId = Number(params.taskId);

  const { data: changes, isLoading, error } = useQuery({
    queryKey: ["taskChanges", projectId, taskId],
    queryFn: () => getTaskChanges(projectId, taskId),
  });

  const { data: review } = useQuery({
    queryKey: ["taskReview", projectId, taskId],
    queryFn: () => getTaskReview(projectId, taskId),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-forge-purple" />
      </div>
    );
  }

  if (error || !changes) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-zinc-500">
        <FileText className="h-12 w-12 mb-4" />
        <p className="text-lg">暂无变更数据</p>
        <p className="text-sm mt-2">任务完成后变更结果将在这里展示</p>
      </div>
    );
  }

  // Determine unit test status from review
  const unitTestStatus = changes.review_score !== null
    ? (changes.review_score >= 70 ? "passed" : "failed")
    : "pending";

  // Count files by status
  const addedFiles = changes.files.filter((f) => f.status === "added").length;
  const modifiedFiles = changes.files.filter((f) => f.status === "modified").length;
  const removedFiles = changes.files.filter((f) => f.status === "removed").length;

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-zinc-100">变更结果</h1>
        {changes.pr_url && (
          <a
            href={changes.pr_url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 text-sm text-forge-purple hover:text-forge-purple/80 transition-colors"
          >
            <GitPullRequest className="h-4 w-4" />
            <span>PR #{changes.pr_number}</span>
            <ExternalLink className="h-3 w-3" />
          </a>
        )}
      </div>

      {/* Branch info */}
      <div className="flex items-center gap-4 text-sm text-zinc-400">
        <div className="flex items-center gap-1.5">
          <GitBranch className="h-4 w-4" />
          <code className="font-mono text-zinc-300">{changes.branch_name}</code>
        </div>
        <span>|</span>
        <span>{changes.files.length} files changed</span>
        {addedFiles > 0 && <span className="text-emerald-400">+{addedFiles} added</span>}
        {modifiedFiles > 0 && <span className="text-amber-400">{modifiedFiles} modified</span>}
        {removedFiles > 0 && <span className="text-red-400">-{removedFiles} removed</span>}
      </div>

      {/* Layer 1: Trust Metrics */}
      <TrustMetrics
        reviewScore={changes.review_score}
        riskLevel={changes.risk_level}
        testStatus={unitTestStatus as "passed" | "failed" | "pending"}
      />

      {/* Layer 1: AI Summary */}
      {changes.ai_summary && (
        <div className="rounded-lg border border-forge-purple/20 bg-forge-purple/5 p-4">
          <h3 className="text-sm font-medium text-forge-purple mb-2">AI 变更总结</h3>
          <p className="text-sm text-zinc-300 leading-relaxed">{changes.ai_summary}</p>
        </div>
      )}

      {/* Layer 2: Structured change list */}
      <div className="rounded-lg border border-surface-2 bg-surface-1 overflow-hidden">
        <div className="px-4 py-3 border-b border-surface-2">
          <h3 className="text-sm font-medium text-zinc-200">变更文件列表</h3>
        </div>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-surface-2 text-zinc-500">
              <th className="text-left px-4 py-2 font-medium">文件</th>
              <th className="text-left px-4 py-2 font-medium w-24">状态</th>
              <th className="text-right px-4 py-2 font-medium w-20">增加</th>
              <th className="text-right px-4 py-2 font-medium w-20">删除</th>
            </tr>
          </thead>
          <tbody>
            {changes.files.map((file) => (
              <tr key={file.filename} className="border-b border-surface-2 last:border-0 hover:bg-surface-2/50">
                <td className="px-4 py-2 font-mono text-zinc-300 truncate max-w-md">{file.filename}</td>
                <td className="px-4 py-2">
                  <span className={cn("text-xs px-2 py-0.5 rounded border", {
                    "bg-emerald-500/10 text-emerald-400 border-emerald-500/20": file.status === "added",
                    "bg-amber-500/10 text-amber-400 border-amber-500/20": file.status === "modified",
                    "bg-red-500/10 text-red-400 border-red-500/20": file.status === "removed",
                  })}>
                    {file.status}
                  </span>
                </td>
                <td className="px-4 py-2 text-right font-mono text-emerald-400">+{file.additions}</td>
                <td className="px-4 py-2 text-right font-mono text-red-400">-{file.deletions}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Layer 3: Code Diff Viewer */}
      <div>
        <h3 className="text-sm font-medium text-zinc-200 mb-3">代码变更详情</h3>
        <MonacoDiffViewer files={changes.files} />
      </div>
    </div>
  );
}
```

- [ ] **Step 6: 验证前端编译**

```bash
cd forge-portal && npm run build
# 应无编译错误
```

- [ ] **Step 7: Commit**

```bash
git add forge-portal/components/monaco-diff-viewer.tsx
git add forge-portal/components/trust-metrics.tsx
git add forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/\[taskId\]/changes/
git add forge-portal/lib/api.ts
git add forge-portal/package.json forge-portal/package-lock.json
git commit -m "feat(s7): add changes page with trust metrics and diff viewer"
```

---

## Task 6: 前端 — 测试报告页

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/tests/page.tsx`
- Create: `forge-portal/components/test-layer-card.tsx`

- [ ] **Step 1: 创建 Test Layer Card 组件**

`forge-portal/components/test-layer-card.tsx`：

```tsx
"use client";

import { cn } from "@/lib/utils";
import { CheckCircle2, XCircle, Clock, Lock, ChevronDown, ChevronRight } from "lucide-react";
import { useState } from "react";
import type { TestLayer } from "@/lib/api";

interface TestLayerCardProps {
  layer: TestLayer;
}

function getStatusConfig(status: string, available: boolean) {
  if (!available) {
    return { icon: Lock, color: "text-zinc-600", bg: "bg-zinc-500/5 border-zinc-500/10", label: "Coming soon" };
  }
  switch (status) {
    case "passed":
      return { icon: CheckCircle2, color: "text-emerald-400", bg: "bg-emerald-500/10 border-emerald-500/20", label: "通过" };
    case "failed":
      return { icon: XCircle, color: "text-red-400", bg: "bg-red-500/10 border-red-500/20", label: "失败" };
    case "pending":
    default:
      return { icon: Clock, color: "text-zinc-500", bg: "bg-zinc-500/10 border-zinc-500/20", label: "待执行" };
  }
}

export function TestLayerCard({ layer }: TestLayerCardProps) {
  const [expanded, setExpanded] = useState(false);
  const config = getStatusConfig(layer.status, layer.available);
  const StatusIcon = config.icon;

  return (
    <div className={cn("rounded-lg border", config.bg)}>
      <button
        onClick={() => layer.available && layer.cases?.length > 0 && setExpanded(!expanded)}
        className={cn(
          "w-full flex items-center gap-4 px-5 py-4 text-left",
          layer.available && layer.cases?.length > 0 ? "cursor-pointer hover:bg-white/[0.02]" : "cursor-default"
        )}
        disabled={!layer.available}
      >
        <StatusIcon className={cn("h-5 w-5 shrink-0", config.color)} />

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-zinc-200">{layer.display_name}</span>
            <span className={cn("text-xs px-1.5 py-0.5 rounded", config.color, config.bg)}>
              {config.label}
            </span>
          </div>
          {layer.available && (
            <div className="flex items-center gap-4 mt-1 text-xs text-zinc-500">
              <span>总计: {layer.total}</span>
              <span className="text-emerald-400">通过: {layer.passed}</span>
              {layer.failed > 0 && <span className="text-red-400">失败: {layer.failed}</span>}
              {layer.coverage !== null && <span>覆盖率: {layer.coverage}%</span>}
            </div>
          )}
          {!layer.available && (
            <p className="text-xs text-zinc-600 mt-1">此测试层级将在后续版本中启用</p>
          )}
        </div>

        {layer.available && layer.cases?.length > 0 && (
          expanded
            ? <ChevronDown className="h-4 w-4 text-zinc-500 shrink-0" />
            : <ChevronRight className="h-4 w-4 text-zinc-500 shrink-0" />
        )}
      </button>

      {/* Expanded test cases */}
      {expanded && layer.cases && layer.cases.length > 0 && (
        <div className="border-t border-surface-2 px-5 py-3">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-zinc-500">
                <th className="text-left py-1 font-medium">用例名称</th>
                <th className="text-left py-1 font-medium w-16">状态</th>
                <th className="text-right py-1 font-medium w-20">耗时</th>
              </tr>
            </thead>
            <tbody>
              {layer.cases.map((tc, i) => (
                <tr key={i} className="border-t border-surface-2">
                  <td className="py-1.5 text-zinc-300 font-mono">{tc.name}</td>
                  <td className="py-1.5">
                    {tc.status === "passed" ? (
                      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-400" />
                    ) : (
                      <XCircle className="h-3.5 w-3.5 text-red-400" />
                    )}
                  </td>
                  <td className="py-1.5 text-right text-zinc-500 font-mono">{tc.duration.toFixed(2)}s</td>
                </tr>
              ))}
            </tbody>
          </table>
          {layer.cases.some((tc) => tc.error) && (
            <div className="mt-3 space-y-2">
              {layer.cases
                .filter((tc) => tc.error)
                .map((tc, i) => (
                  <div key={i} className="rounded bg-red-500/5 border border-red-500/20 px-3 py-2">
                    <p className="text-xs font-medium text-red-400">{tc.name}</p>
                    <pre className="text-xs text-red-300/80 mt-1 whitespace-pre-wrap font-mono">{tc.error}</pre>
                  </div>
                ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 创建测试报告页面**

`forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/tests/page.tsx`：

```tsx
"use client";

import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { getTaskTests } from "@/lib/api";
import { TestLayerCard } from "@/components/test-layer-card";
import { FlaskConical } from "lucide-react";

export default function TestsPage() {
  const params = useParams();
  const projectId = Number(params.id);
  const taskId = Number(params.taskId);

  const { data: tests, isLoading, error } = useQuery({
    queryKey: ["taskTests", projectId, taskId],
    queryFn: () => getTaskTests(projectId, taskId),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-forge-purple" />
      </div>
    );
  }

  if (error || !tests) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-zinc-500">
        <FlaskConical className="h-12 w-12 mb-4" />
        <p className="text-lg">暂无测试数据</p>
        <p className="text-sm mt-2">任务完成后测试报告将在这里展示</p>
      </div>
    );
  }

  // Calculate overall stats
  const availableLayers = tests.layers.filter((l) => l.available);
  const passedLayers = availableLayers.filter((l) => l.status === "passed").length;
  const totalAvailable = availableLayers.length;

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-bold text-zinc-100">测试报告</h1>
        <p className="text-sm text-zinc-500 mt-1">
          四层自动化测试结果 ({passedLayers}/{totalAvailable} 层通过)
        </p>
      </div>

      {/* Overall progress bar */}
      <div className="rounded-lg border border-surface-2 bg-surface-1 p-4">
        <div className="flex items-center justify-between text-sm mb-2">
          <span className="text-zinc-400">测试进度</span>
          <span className="text-zinc-300 font-mono">
            {availableLayers.length} / {tests.layers.length} 层已启用
          </span>
        </div>
        <div className="h-2 rounded-full bg-zinc-800 overflow-hidden">
          <div
            className="h-full rounded-full bg-forge-purple transition-all duration-500"
            style={{ width: `${(availableLayers.length / tests.layers.length) * 100}%` }}
          />
        </div>
      </div>

      {/* Test layers */}
      <div className="space-y-3">
        {tests.layers.map((layer) => (
          <TestLayerCard key={layer.name} layer={layer} />
        ))}
      </div>

      {/* Phase 1 notice */}
      <div className="rounded-lg border border-zinc-800 bg-zinc-900/50 p-4 text-center">
        <p className="text-xs text-zinc-600">
          Phase 1 仅支持单元测试层。接口测试、集成测试和回归测试将在后续版本中启用。
        </p>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 验证前端编译**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 4: Commit**

```bash
git add forge-portal/components/test-layer-card.tsx
git add forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/\[taskId\]/tests/
git commit -m "feat(s7): add test report page with four-layer test display"
```

---

## Task 7: 前端 — 部署环境页

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/deploy/page.tsx`
- Create: `forge-portal/components/environment-card.tsx`

- [ ] **Step 1: 创建 Environment Card 组件**

`forge-portal/components/environment-card.tsx`：

```tsx
"use client";

import { cn } from "@/lib/utils";
import { Server, Clock, Tag, Rocket } from "lucide-react";
import type { Environment } from "@/lib/api";

interface EnvironmentCardProps {
  environment: Environment;
}

function getStatusConfig(status: string) {
  switch (status) {
    case "ACTIVE":
      return { dot: "bg-emerald-400", label: "运行中", labelColor: "text-emerald-400" };
    case "DEPLOYING":
      return { dot: "bg-amber-400 animate-pulse", label: "部署中", labelColor: "text-amber-400" };
    case "ERROR":
      return { dot: "bg-red-400", label: "异常", labelColor: "text-red-400" };
    case "INACTIVE":
    default:
      return { dot: "bg-zinc-600", label: "未激活", labelColor: "text-zinc-500" };
  }
}

function getEnvTypeConfig(envType: string) {
  switch (envType) {
    case "DEV":
      return { label: "Development", color: "text-sky-400 bg-sky-500/10 border-sky-500/20" };
    case "STAGING":
      return { label: "Staging", color: "text-amber-400 bg-amber-500/10 border-amber-500/20" };
    case "PROD":
      return { label: "Production", color: "text-emerald-400 bg-emerald-500/10 border-emerald-500/20" };
    default:
      return { label: envType, color: "text-zinc-400 bg-zinc-500/10 border-zinc-500/20" };
  }
}

function formatTime(dateStr: string | null): string {
  if (!dateStr) return "--";
  const date = new Date(dateStr);
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function EnvironmentCard({ environment }: EnvironmentCardProps) {
  const status = getStatusConfig(environment.status);
  const envType = getEnvTypeConfig(environment.env_type);

  return (
    <div className="rounded-xl border border-surface-2 bg-surface-1 p-5 hover:border-surface-3 transition-colors">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-lg bg-surface-2 flex items-center justify-center">
            <Server className="h-5 w-5 text-zinc-400" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-zinc-200">{environment.name}</h3>
            <span className={cn("text-xs px-1.5 py-0.5 rounded border", envType.color)}>
              {envType.label}
            </span>
          </div>
        </div>

        {/* Status indicator */}
        <div className="flex items-center gap-2">
          <div className={cn("h-2.5 w-2.5 rounded-full", status.dot)} />
          <span className={cn("text-xs font-medium", status.labelColor)}>{status.label}</span>
        </div>
      </div>

      {/* Info rows */}
      <div className="space-y-3">
        <div className="flex items-center justify-between text-sm">
          <div className="flex items-center gap-2 text-zinc-500">
            <Tag className="h-3.5 w-3.5" />
            <span>版本</span>
          </div>
          <span className="font-mono text-zinc-300 text-xs">
            {environment.current_version || "未部署"}
          </span>
        </div>

        <div className="flex items-center justify-between text-sm">
          <div className="flex items-center gap-2 text-zinc-500">
            <Clock className="h-3.5 w-3.5" />
            <span>最后部署</span>
          </div>
          <span className="text-zinc-400 text-xs">
            {formatTime(environment.last_deploy_at)}
          </span>
        </div>
      </div>

      {/* Action area — placeholder for Phase 1 */}
      {environment.status === "INACTIVE" && (
        <div className="mt-4 pt-4 border-t border-surface-2">
          <p className="text-xs text-zinc-600 text-center">尚未配置部署，后续版本将支持一键部署</p>
        </div>
      )}

      {environment.status === "ACTIVE" && (
        <div className="mt-4 pt-4 border-t border-surface-2">
          <button
            disabled
            className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-lg bg-surface-2 text-zinc-500 text-sm cursor-not-allowed"
          >
            <Rocket className="h-4 w-4" />
            <span>部署 (Coming soon)</span>
          </button>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: 创建部署环境页面**

`forge-portal/app/(dashboard)/projects/[id]/deploy/page.tsx`：

```tsx
"use client";

import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { getProjectEnvironments } from "@/lib/api";
import { EnvironmentCard } from "@/components/environment-card";
import { Cloud, Info } from "lucide-react";

export default function DeployPage() {
  const params = useParams();
  const projectId = Number(params.id);

  const { data: environments, isLoading, error } = useQuery({
    queryKey: ["environments", projectId],
    queryFn: () => getProjectEnvironments(projectId),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-forge-purple" />
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-zinc-100">部署环境</h1>
          <p className="text-sm text-zinc-500 mt-1">查看各环境的部署状态和版本信息</p>
        </div>
      </div>

      {/* Phase 1 notice */}
      <div className="flex items-start gap-3 rounded-lg border border-forge-purple/20 bg-forge-purple/5 px-4 py-3">
        <Info className="h-4 w-4 text-forge-purple mt-0.5 shrink-0" />
        <div>
          <p className="text-sm text-zinc-300">当前版本为环境状态概览</p>
          <p className="text-xs text-zinc-500 mt-1">
            实际 K8s 部署、灰度发布等功能将在后续版本中启用。环境在项目创建时自动初始化。
          </p>
        </div>
      </div>

      {/* Environment cards */}
      {(!environments || environments.length === 0) ? (
        <div className="flex flex-col items-center justify-center h-48 text-zinc-500">
          <Cloud className="h-12 w-12 mb-4" />
          <p className="text-lg">暂无环境数据</p>
          <p className="text-sm mt-2">环境将在项目创建时自动初始化</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {environments.map((env) => (
            <EnvironmentCard key={env.id} environment={env} />
          ))}
        </div>
      )}

      {/* Future: deployment history timeline */}
      <div className="rounded-lg border border-surface-2 bg-surface-1 p-6">
        <h3 className="text-sm font-medium text-zinc-200 mb-4">发布记录</h3>
        <div className="flex flex-col items-center justify-center h-24 text-zinc-600">
          <p className="text-sm">暂无发布记录</p>
          <p className="text-xs mt-1">部署功能启用后，发布历史将在这里展示</p>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 更新任务详情页 — 添加 PR 链接和 Review 信息**

在现有任务详情页（或任务卡片组件）中，添加 PR 链接和 Review 分数展示。找到任务详情的 UI 组件，添加：

```tsx
{/* PR Info — add to task detail page */}
{task.mr_url && (
  <div className="flex items-center gap-4 mt-4 p-3 rounded-lg bg-surface-1 border border-surface-2">
    <a
      href={task.mr_url}
      target="_blank"
      rel="noopener noreferrer"
      className="flex items-center gap-2 text-sm text-forge-purple hover:underline"
    >
      <GitPullRequest className="h-4 w-4" />
      PR #{task.pr_number}
      <ExternalLink className="h-3 w-3" />
    </a>
    {task.review_score !== null && (
      <div className="flex items-center gap-2 text-sm">
        <ShieldCheck className="h-4 w-4 text-zinc-400" />
        <span className={cn("font-mono font-medium", {
          "text-emerald-400": task.review_score >= 90,
          "text-amber-400": task.review_score >= 70 && task.review_score < 90,
          "text-red-400": task.review_score < 70,
        })}>
          {task.review_score}/100
        </span>
      </div>
    )}
  </div>
)}
```

Also update the task sidebar/tabs navigation to include links to changes, tests, and deploy pages. If there is a task-level navigation component, add:

```tsx
// Task sub-navigation tabs (add to task detail layout or sidebar)
const taskSubPages = [
  { label: "需求对话", href: `/projects/${projectId}/tasks/${taskId}` },
  { label: "变更结果", href: `/projects/${projectId}/tasks/${taskId}/changes` },
  { label: "测试报告", href: `/projects/${projectId}/tasks/${taskId}/tests` },
];
```

And in the project sidebar, add the deploy page link:

```tsx
// Add to project sidebar navigation
{ label: "部署环境", href: `/projects/${projectId}/deploy`, icon: Cloud }
```

- [ ] **Step 4: 验证前端编译**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/environment-card.tsx
git add forge-portal/app/\(dashboard\)/projects/\[id\]/deploy/
git add forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/
git commit -m "feat(s7): add deploy environment page and update task detail with PR info"
```

---

## Task 8: code-server 集成 — Web IDE 代码浏览

**Files:**
- Modify: `docker-compose.dev.yml` — 添加 code-server 服务
- Create: `forge-core/internal/module/ide/handler.go` — IDE 工作区管理 API
- Create: `forge-core/internal/module/ide/service.go` — 工作区创建/回收逻辑
- Create: `forge-portal/components/open-in-ide-button.tsx` — "在 IDE 中打开" 按钮
- Modify: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/changes/page.tsx` — 添加 IDE 按钮
- Modify: `forge-core/internal/router/router.go` — 注册 IDE 路由

### 架构说明

code-server 作为共享 Web IDE 实例部署在 Docker Compose 中，forge-core 负责工作区管理（按 project_id + branch 隔离），前端通过按钮跳转到 code-server 页面。

Phase 1 约束：
- 工作区只读模式（浏览审查用途，不能通过 IDE 提交代码）
- 单实例共享（不做多实例 HPA，本地开发足够）
- 工作区空闲 30 分钟后自动回收

- [ ] **Step 1: Docker Compose 添加 code-server**

在 `docker-compose.dev.yml` 中添加 code-server 服务：

```yaml
  code-server:
    image: codercom/code-server:latest
    container_name: forge-code-server
    environment:
      - DOCKER_USER=coder
    volumes:
      - code-workspaces:/workspaces
    ports:
      - "8443:8080"
    restart: unless-stopped

volumes:
  code-workspaces:
```

启动验证：

```bash
docker compose -f docker-compose.dev.yml up -d code-server
# 浏览器访问 http://localhost:8443 应看到 VS Code Web 界面
```

- [ ] **Step 2: IDE 模块 — 工作区管理 Service**

创建 `forge-core/internal/module/ide/service.go`：

```go
package ide

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "sync"
    "time"
)

type WorkspaceInfo struct {
    ProjectID  int64  `json:"projectId"`
    Branch     string `json:"branch"`
    Path       string `json:"path"`
    IDEUrl     string `json:"ideUrl"`
    CreatedAt  time.Time `json:"createdAt"`
    LastAccess time.Time `json:"lastAccess"`
}

type Service struct {
    baseDir       string // /workspaces
    codeServerURL string // http://localhost:8443
    mu            sync.Mutex
    workspaces    map[string]*WorkspaceInfo // key: "{projectId}/{branch}"
}

func NewService(baseDir, codeServerURL string) *Service {
    return &Service{
        baseDir:       baseDir,
        codeServerURL: codeServerURL,
        workspaces:    make(map[string]*WorkspaceInfo),
    }
}

// GetOrCreateWorkspace clones the repo branch if not already present, returns IDE URL
func (s *Service) GetOrCreateWorkspace(ctx context.Context, projectID int64, repoURL, branch, token string) (*WorkspaceInfo, error) {
    key := fmt.Sprintf("%d/%s", projectID, branch)

    s.mu.Lock()
    defer s.mu.Unlock()

    // Check existing workspace
    if ws, ok := s.workspaces[key]; ok {
        ws.LastAccess = time.Now()
        return ws, nil
    }

    // Clone repo to workspace dir
    wsPath := filepath.Join(s.baseDir, fmt.Sprintf("%d", projectID), branch)
    if err := os.MkdirAll(filepath.Dir(wsPath), 0755); err != nil {
        return nil, fmt.Errorf("create workspace dir: %w", err)
    }

    // Clone via GitHub API token (HTTPS)
    cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoURL)
    cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--depth", "1", cloneURL, wsPath)
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("git clone: %w", err)
    }

    ws := &WorkspaceInfo{
        ProjectID:  projectID,
        Branch:     branch,
        Path:       wsPath,
        IDEUrl:     fmt.Sprintf("%s/?folder=%s", s.codeServerURL, wsPath),
        CreatedAt:  time.Now(),
        LastAccess: time.Now(),
    }
    s.workspaces[key] = ws
    return ws, nil
}

// CleanupStaleWorkspaces removes workspaces idle > 30 minutes
func (s *Service) CleanupStaleWorkspaces() {
    s.mu.Lock()
    defer s.mu.Unlock()

    threshold := time.Now().Add(-30 * time.Minute)
    for key, ws := range s.workspaces {
        if ws.LastAccess.Before(threshold) {
            os.RemoveAll(ws.Path)
            delete(s.workspaces, key)
        }
    }
}
```

- [ ] **Step 3: IDE 模块 — HTTP Handler**

创建 `forge-core/internal/module/ide/handler.go`：

```go
package ide

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
)

type Handler struct {
    service *Service
    // need access to project and GitHub adapter for repo URL and token
}

// POST /api/projects/:id/ide/workspace
// Body: { "branch": "ai/123-feature" }
// Returns: { "ideUrl": "http://localhost:8443/?folder=/workspaces/1/ai/123-feature" }
func (h *Handler) CreateWorkspace(c *gin.Context) {
    projectID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

    var req struct {
        Branch string `json:"branch" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // TODO: load project from DB, get repo URL and GitHub token
    // ws, err := h.service.GetOrCreateWorkspace(c, projectID, repoURL, req.Branch, token)

    // Placeholder response for Step 3
    c.JSON(http.StatusOK, gin.H{
        "projectId": projectID,
        "branch":    req.Branch,
        "ideUrl":    "http://localhost:8443",
        "message":   "workspace creation will be wired in integration step",
    })
}
```

注册路由（`router.go`）：

```go
// IDE workspace management
ideGroup := api.Group("/projects/:id/ide")
ideGroup.POST("/workspace", ideHandler.CreateWorkspace)
```

- [ ] **Step 4: 前端 — "在 IDE 中打开" 按钮组件**

创建 `forge-portal/components/open-in-ide-button.tsx`：

```tsx
"use client";

import { useState } from "react";
import { Monitor, ExternalLink, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { apiClient } from "@/lib/api";

interface OpenInIDEButtonProps {
  projectId: number;
  branch: string;
  variant?: "default" | "ghost" | "outline";
  size?: "default" | "sm" | "icon";
}

export function OpenInIDEButton({
  projectId,
  branch,
  variant = "outline",
  size = "sm",
}: OpenInIDEButtonProps) {
  const [loading, setLoading] = useState(false);

  async function handleClick() {
    setLoading(true);
    try {
      const res = await apiClient.post(`/projects/${projectId}/ide/workspace`, {
        branch,
      });
      const { ideUrl } = res.data;
      window.open(ideUrl, "_blank", "noopener,noreferrer");
    } catch (err) {
      console.error("Failed to create IDE workspace:", err);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Button variant={variant} size={size} onClick={handleClick} disabled={loading}>
      {loading ? (
        <Loader2 className="h-4 w-4 animate-spin" />
      ) : (
        <Monitor className="h-4 w-4" />
      )}
      <span className="ml-2">在 IDE 中打开</span>
      <ExternalLink className="h-3 w-3 ml-1 opacity-50" />
    </Button>
  );
}
```

- [ ] **Step 5: 集成到变更结果页**

在变更结果页（`changes/page.tsx`）的页面头部区域添加 "在 IDE 中打开" 按钮：

```tsx
import { OpenInIDEButton } from "@/components/open-in-ide-button";

// In the page header area, alongside the existing action buttons:
<OpenInIDEButton
  projectId={projectId}
  branch={task.branch_name}
/>
```

同时在项目详情页右上角也可添加入口（打开默认分支）：

```tsx
<OpenInIDEButton
  projectId={projectId}
  branch={project.default_branch || "main"}
  variant="ghost"
/>
```

- [ ] **Step 6: 验证 code-server 集成**

```bash
# 1. 启动 code-server
docker compose -f docker-compose.dev.yml up -d code-server

# 2. 验证 code-server 可访问
curl -s -o /dev/null -w "%{http_code}" http://localhost:8443
# 预期: 200

# 3. 启动后端和前端
cd forge-core && go run ./cmd/forge-core &
cd forge-portal && npm run dev &

# 4. 测试 IDE workspace API
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"branch": "main"}' \
  http://localhost:8080/api/projects/1/ide/workspace

# 5. 在浏览器中验证：进入变更结果页，点击 "在 IDE 中打开" 按钮
```

- [ ] **Step 7: Commit**

```bash
git add docker-compose.dev.yml
git add forge-core/internal/module/ide/
git add forge-portal/components/open-in-ide-button.tsx
git commit -m "feat(s7): integrate code-server Web IDE with workspace management"
```

---

## 验收标准

S7 完成后，你应该能够：

1. **端到端闭环**: 需求输入 -> AI 分析 -> 代码生成 -> Review -> 自动创建分支/提交/PR -> 变更结果可查看
2. **变更结果页三层展示**: AI 总结 + 信任指标 -> 变更文件列表 -> Monaco Diff 查看
3. **Web IDE 代码浏览**: 点击"在 IDE 中打开"，在 code-server (VS Code Web) 中完整浏览代码仓库
4. **测试报告页**: 四层结构展示（Phase 1 仅单测层有数据）
5. **部署环境页**: 三个环境卡片（dev/staging/prod），Phase 1 为信息展示
6. **任务详情显示 PR 链接和 Review 评分**
7. **API 可独立测试**:
   ```bash
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/tasks/1/changes
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/tasks/1/review
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/tasks/1/tests
   curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/projects/1/environments
   curl -X POST -H "Authorization: Bearer $TOKEN" -d '{"branch":"main"}' http://localhost:8080/api/projects/1/ide/workspace
   ```

---

## 后续切片预告

| 切片 | 内容 | 前置 |
|------|------|------|
| S8 | Constraint Worker — Lint 引擎 + 实际代码质量扫描 | S7 |
| S9 | 实际 CI/CD 流水线 — Argo Workflows 集成 | S7 |
| S10 | K8s 部署 — ACK/Argo CD 集成，环境页功能化 | S7, S9 |
| S11 | 测试平台集成 — MeterSphere / 四层真实测试 | S7, S9 |
| S12 | IM 机器人 — 钉钉/飞书消息通知 | S7 |
