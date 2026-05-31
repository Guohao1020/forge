# S15 — 代码浏览与分支管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 关联 GitHub 后，用户能在 Forge Web 界面浏览仓库完整代码（文件树 + Shiki 语法高亮），切换分支，查看 PR 列表和 Diff。

**Architecture:** 扩展 GitHub adapter 添加 ListBranches/ListPRs 方法，在 project handler 新增 5 个代码浏览 API，前端新增代码浏览器页面（复用已有的 ShikiCodeViewer + FileTree 组件）。

**Tech Stack:** Go 1.22 + go-github/v63, Next.js + React + Shiki + shadcn/ui

**Dependencies:** S3 (GitHub OAuth + adapter)

---

## File Structure

### Go 后端

```
forge-core/
├── internal/adapter/github/
│   ├── client.go                         # MODIFY: +ListBranches, +ListPRs, +GetPRDetail
│   └── types.go                          # MODIFY: +Branch, +PullRequestSummary types
├── internal/module/project/
│   ├── handler.go                        # MODIFY: +5 code browsing handlers
│   └── service.go                        # MODIFY: +5 code browsing service methods
└── internal/router/router.go             # MODIFY: register code browsing routes
```

### 前端

```
forge-portal/
├── app/(dashboard)/projects/[id]/
│   └── code/page.tsx                     # NEW: 代码浏览器主页面
├── components/
│   ├── code-browser/
│   │   ├── branch-selector.tsx           # NEW: 分支选择下拉框
│   │   ├── file-breadcrumb.tsx           # NEW: 文件路径面包屑导航
│   │   └── repo-file-tree.tsx            # NEW: 仓库文件树（适配 GitHub tree 数据）
│   └── project-sidebar.tsx               # MODIFY: 添加 "代码" 导航项
└── lib/code.ts                           # NEW: 代码浏览 API 客户端
```

---

## Task 1: GitHub Adapter — ListBranches + ListPRs

**Files:**
- Modify: `forge-core/internal/adapter/github/client.go`
- Modify: `forge-core/internal/adapter/github/types.go`

- [ ] **Step 1: 添加类型定义到 types.go**

读取 `forge-core/internal/adapter/github/types.go`，追加：

```go
// Branch represents a GitHub branch
type Branch struct {
	Name      string `json:"name"`
	SHA       string `json:"sha"`
	Protected bool   `json:"protected"`
}

// PullRequestSummary for listing PRs
type PullRequestSummary struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	Head      string `json:"head"`
	Base      string `json:"base"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      string `json:"user"`
}
```

- [ ] **Step 2: 实现 ListBranches**

读取 `client.go`，在文件末尾添加：

```go
// ListBranches returns all branches for a repository.
func (c *Client) ListBranches(ctx context.Context, owner, repo string) ([]Branch, error) {
	var allBranches []Branch
	opts := &ghlib.BranchListOptions{ListOptions: ghlib.ListOptions{PerPage: 100}}
	for {
		branches, resp, err := c.client.Repositories.ListBranches(ctx, owner, repo, opts)
		if resp != nil {
			c.logRateLimit(resp.Response)
		}
		if err != nil {
			return nil, fmt.Errorf("list branches: %w", err)
		}
		for _, b := range branches {
			allBranches = append(allBranches, Branch{
				Name:      b.GetName(),
				SHA:       b.GetCommit().GetSHA(),
				Protected: b.GetProtected(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allBranches, nil
}
```

- [ ] **Step 3: 实现 ListPRs**

```go
// ListPRs returns pull requests for a repository.
func (c *Client) ListPRs(ctx context.Context, owner, repo, state string) ([]PullRequestSummary, error) {
	if state == "" {
		state = "open"
	}
	opts := &ghlib.PullRequestListOptions{
		State:       state,
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: ghlib.ListOptions{PerPage: 30},
	}
	prs, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
	if resp != nil {
		c.logRateLimit(resp.Response)
	}
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}
	result := make([]PullRequestSummary, 0, len(prs))
	for _, pr := range prs {
		result = append(result, PullRequestSummary{
			Number:    pr.GetNumber(),
			Title:     pr.GetTitle(),
			State:     pr.GetState(),
			HTMLURL:   pr.GetHTMLURL(),
			Head:      pr.GetHead().GetRef(),
			Base:      pr.GetBase().GetRef(),
			CreatedAt: pr.GetCreatedAt().Format(time.RFC3339),
			UpdatedAt: pr.GetUpdatedAt().Format(time.RFC3339),
			User:      pr.GetUser().GetLogin(),
		})
	}
	return result, nil
}
```

- [ ] **Step 4: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/adapter/github/
git commit -m "feat(s15): add ListBranches and ListPRs to GitHub adapter"
```

---

## Task 2: 后端 API — 代码浏览 5 个端点

**Files:**
- Modify: `forge-core/internal/module/project/service.go`
- Modify: `forge-core/internal/module/project/handler.go`
- Modify: `forge-core/internal/router/router.go`

**重要**: 先完整读取 `service.go`、`handler.go`、`router.go` 了解现有模式。

- [ ] **Step 1: Service 层添加代码浏览方法**

在 `service.go` 末尾追加 5 个方法。每个方法模式相同：获取项目 → 解析 owner/repo → 获取 token → 创建 GitHub client → 调用 adapter。

```go
// GetCodeTree returns the file tree for a branch
func (s *Service) GetCodeTree(ctx context.Context, projectID, userID int64, ref string) ([]string, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	if ref == "" {
		ref = p.DefaultBranch
	}
	return ghClient.GetTree(ctx, owner, repo, ref)
}

// GetCodeFile returns file content at a specific ref
func (s *Service) GetCodeFile(ctx context.Context, projectID, userID int64, path, ref string) (string, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, userID)
	if err != nil {
		return "", err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	if ref == "" {
		ref = p.DefaultBranch
	}
	return ghClient.GetFileContent(ctx, owner, repo, path, ref)
}

// ListBranches returns all branches
func (s *Service) ListBranches(ctx context.Context, projectID, userID int64) ([]github.Branch, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	return ghClient.ListBranches(ctx, owner, repo)
}

// ListPRs returns pull requests
func (s *Service) ListPRs(ctx context.Context, projectID, userID int64, state string) ([]github.PullRequestSummary, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	return ghClient.ListPRs(ctx, owner, repo, state)
}

// GetPRDetail returns PR files/diff
func (s *Service) GetPRDetail(ctx context.Context, projectID, userID int64, prNumber int) ([]github.PRFile, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	return ghClient.GetPRFiles(ctx, owner, repo, prNumber)
}

// helper: get authenticated GitHub client for a project
func (s *Service) getGitHubClient(ctx context.Context, projectID, userID int64) (*project.Project, *github.Client, error) {
	p, err := s.repo.GetByID(ctx, projectID, userID)
	if err != nil {
		return nil, nil, err
	}
	if p.CodeRepoURL == "" {
		return nil, nil, fmt.Errorf("project has no repo URL")
	}
	token, err := s.authSvc.GetGitHubToken(ctx, userID)
	if err != nil || token == "" {
		return nil, nil, fmt.Errorf("no GitHub token available")
	}
	return p, github.NewClient(token), nil
}

func parseOwnerRepo(url string) (string, string) {
	parts := strings.Split(strings.TrimSuffix(url, ".git"), "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}
```

注意：`parseOwnerRepo` 可能已存在于 `DetectTechStack` 中（检查是否可复用，避免重复）。

- [ ] **Step 2: Handler 层添加 5 个端点**

在 `handler.go` 末尾追加：

```go
// GET /api/projects/:id/code/tree?ref=main
func (h *Handler) GetCodeTree(c *gin.Context) {
	projectID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	userID, _ := c.Get("user_id")
	ref := c.DefaultQuery("ref", "")
	tree, err := h.service.GetCodeTree(c.Request.Context(), projectID, userID.(int64), ref)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取文件树失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"files": tree, "ref": ref})
}

// GET /api/projects/:id/code/file?path=src/main.go&ref=main
func (h *Handler) GetCodeFile(c *gin.Context) {
	projectID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	userID, _ := c.Get("user_id")
	path := c.Query("path")
	ref := c.DefaultQuery("ref", "")
	if path == "" {
		response.Fail(c, http.StatusBadRequest, "path 参数必填")
		return
	}
	content, err := h.service.GetCodeFile(c.Request.Context(), projectID, userID.(int64), path, ref)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取文件内容失败")
		return
	}
	response.OK(c, gin.H{"path": path, "content": content, "ref": ref})
}

// GET /api/projects/:id/code/branches
func (h *Handler) ListBranches(c *gin.Context) { /* similar pattern */ }

// GET /api/projects/:id/code/prs?state=open
func (h *Handler) ListPRs(c *gin.Context) { /* similar pattern */ }

// GET /api/projects/:id/code/prs/:prNumber
func (h *Handler) GetPRDetail(c *gin.Context) { /* similar pattern */ }
```

- [ ] **Step 3: 注册路由**

在 `router.go` protected group 中添加：

```go
// Code browsing
protected.GET("/projects/:id/code/tree", deps.ProjectHandler.GetCodeTree)
protected.GET("/projects/:id/code/file", deps.ProjectHandler.GetCodeFile)
protected.GET("/projects/:id/code/branches", deps.ProjectHandler.ListBranches)
protected.GET("/projects/:id/code/prs", deps.ProjectHandler.ListPRs)
protected.GET("/projects/:id/code/prs/:prNumber", deps.ProjectHandler.GetPRDetail)
```

- [ ] **Step 4: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/
git commit -m "feat(s15): add code browsing API — tree, file, branches, PRs"
```

---

## Task 3: 前端 API 客户端 + 组件

**Files:**
- Create: `forge-portal/lib/code.ts`
- Create: `forge-portal/components/code-browser/branch-selector.tsx`
- Create: `forge-portal/components/code-browser/file-breadcrumb.tsx`
- Create: `forge-portal/components/code-browser/repo-file-tree.tsx`

- [ ] **Step 1: 创建 API 客户端 lib/code.ts**

```typescript
import { api } from "./api";

export interface Branch {
  name: string;
  sha: string;
  protected: boolean;
}

export interface PRSummary {
  number: number;
  title: string;
  state: string;
  html_url: string;
  head: string;
  base: string;
  created_at: string;
  user: string;
}

export interface PRFile {
  filename: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
}

export async function getCodeTree(projectId: number, ref?: string): Promise<{ files: string[]; ref: string }> {
  const params = ref ? `?ref=${encodeURIComponent(ref)}` : "";
  return api.get(`/projects/${projectId}/code/tree${params}`);
}

export async function getCodeFile(projectId: number, path: string, ref?: string): Promise<{ path: string; content: string; ref: string }> {
  const params = new URLSearchParams({ path });
  if (ref) params.set("ref", ref);
  return api.get(`/projects/${projectId}/code/file?${params}`);
}

export async function listBranches(projectId: number): Promise<Branch[]> {
  const res = await api.get<{ branches: Branch[] }>(`/projects/${projectId}/code/branches`);
  return res.branches || [];
}

export async function listPRs(projectId: number, state?: string): Promise<PRSummary[]> {
  const params = state ? `?state=${state}` : "";
  const res = await api.get<{ prs: PRSummary[] }>(`/projects/${projectId}/code/prs${params}`);
  return res.prs || [];
}

export async function getPRDetail(projectId: number, prNumber: number): Promise<PRFile[]> {
  const res = await api.get<{ files: PRFile[] }>(`/projects/${projectId}/code/prs/${prNumber}`);
  return res.files || [];
}
```

- [ ] **Step 2: 创建 BranchSelector 组件**

`forge-portal/components/code-browser/branch-selector.tsx`:
- 下拉菜单显示分支列表
- 当前选中分支高亮
- 切换分支时 callback
- 使用 shadcn Select 组件
- 图标：GitBranch from lucide-react

- [ ] **Step 3: 创建 FileBreadcrumb 组件**

`forge-portal/components/code-browser/file-breadcrumb.tsx`:
- 显示当前路径：`repo / src / main / java / Calculator.java`
- 每段可点击跳转到父目录
- 深空主题样式

- [ ] **Step 4: 创建 RepoFileTree 组件**

`forge-portal/components/code-browser/repo-file-tree.tsx`:
- 接收 `string[]` 文件路径列表
- 构建层级树结构
- 目录展开/折叠
- 点击文件触发 callback
- 复用 FileTree 的树构建逻辑，但不显示 create/modify 标记
- 文件图标根据扩展名显示（File, FileCode, etc.）

- [ ] **Step 5: 验证构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add forge-portal/lib/code.ts forge-portal/components/code-browser/
git commit -m "feat(s15): add code browsing API client and reusable components"
```

---

## Task 4: 代码浏览器页面

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/code/page.tsx`
- Modify: `forge-portal/components/project-sidebar.tsx`

- [ ] **Step 1: 创建代码浏览器页面**

`forge-portal/app/(dashboard)/projects/[id]/code/page.tsx`:

布局：
```
┌───────────────────────────────────────────────────┐
│  代码浏览    [BranchSelector ▾]    [PR 列表] 按钮  │
├─────────────────┬─────────────────────────────────┤
│                 │ [FileBreadcrumb: repo/src/...]   │
│  RepoFileTree   │                                 │
│  (左侧 260px)   │  ShikiCodeViewer                │
│                 │  （右侧 flex-1）                  │
│                 │                                 │
└─────────────────┴─────────────────────────────────┘
```

功能：
- 页面加载时获取默认分支的文件树
- BranchSelector 切换分支 → 重新加载文件树
- 点击文件 → 右侧显示文件内容（ShikiCodeViewer）
- 点击目录 → 展开/折叠
- FileBreadcrumb 显示当前路径，可点击跳转
- PR 列表按钮跳转到 PR 子页面（或 modal）
- 空状态：未关联 GitHub 时提示 "请先关联 GitHub 仓库"

- [ ] **Step 2: 更新 ProjectSidebar**

读取 `forge-portal/components/project-sidebar.tsx`，在导航项中添加 "代码" 入口：

```tsx
{ icon: Code2, label: "代码", href: `/projects/${projectId}/code` },
```

放在 "任务" 和 "变更" 之间。图标用 `Code2` from lucide-react。

- [ ] **Step 3: 验证构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 4: Commit**

```bash
git add forge-portal/
git commit -m "feat(s15): add code browser page with file tree, branch selector, and Shiki viewer"
```

---

## Task 5: 构建验证 + 端到端测试

- [ ] **Step 1: Go 构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: 前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: 端到端验证清单**

1. 启动所有服务（docker compose + forge-core + 前端）
2. 登录 → 进入已关联 GitHub 的项目
3. 左侧导航点击 "代码"
4. 文件树显示仓库完整目录结构
5. 点击文件 → 右侧 Shiki 语法高亮显示
6. 切换分支 → 文件树刷新
7. PR 列表显示 open PRs

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(s15): complete code browsing and branch management"
```

---

## 验收标准

- [ ] 代码浏览器页面：文件树 + Shiki 语法高亮 + 行号
- [ ] 分支选择器：切换分支后文件树刷新
- [ ] PR 列表：显示 open/closed PRs
- [ ] 面包屑导航：路径可点击
- [ ] 未关联 GitHub 时显示友好提示
- [ ] `go build` + `npm run build` 通过
