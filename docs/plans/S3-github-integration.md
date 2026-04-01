# S3 — GitHub OAuth 接入 + 仓库同步 + 项目导入

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户可通过 GitHub OAuth 授权接入代码平台，系统自动同步 GitHub 仓库列表，用户选择仓库导入为 Forge 项目。导入后项目卡片展示 GitHub 仓库信息。

**Architecture:** forge-core 新增 GitHub OAuth handler（auth 模块扩展）和 GitHub adapter（adapter 模块）。forge-portal 新增 OAuth 回调页、平台接入对话框、仓库导入对话框。

**Tech Stack:** Go 1.22 + Gin + go-github/v63, Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 + shadcn/ui

**Depends on:** S1 (auth + login + frontend skeleton), S2 (project CRUD + project hall + project detail)

---

## 前置说明

### GitHub OAuth App 注册（前置条件）

在开始开发前，必须在 GitHub 上注册一个 OAuth App：

1. 打开 https://github.com/settings/developers
2. 点击 **OAuth Apps** → **New OAuth App**
3. 填写信息：
   - **Application name**: `Forge Dev`
   - **Homepage URL**: `http://localhost:3000`
   - **Authorization callback URL**: `http://localhost:3000/auth/github/callback`
4. 创建后记录 **Client ID** 和 **Client Secret**
5. 在本地环境设置环境变量：
   ```bash
   export GITHUB_CLIENT_ID="Ov23liZInCCCo4fv4rre"
   export GITHUB_CLIENT_SECRET="a1252b8ad3e805df9ef3de56e455af33a381a7cf"
   ```

### 本切片交付后你可以做什么

1. 在项目大厅点击 "接入代码平台" 按钮
2. 在弹出的平台选择对话框中选择 GitHub
3. 浏览器跳转到 GitHub OAuth 授权页面
4. 授权后自动回调，系统保存 access token 并同步仓库列表
5. 在仓库导入对话框中看到所有 GitHub 仓库（按组织分组）
6. 勾选想导入的仓库，点击 "导入选中" 按钮
7. 导入的仓库作为 Forge 项目出现在项目大厅
8. 项目卡片展示 GitHub 图标、仓库 URL、默认分支、主要语言

### 安全说明

- Phase 1 将 GitHub access token 明文存储在 `auth.user_identities` 表中
- **Future improvement**: 生产环境应通过 Vault 加密存储 token
- GitHub access token 不设过期时间（除非用户在 GitHub 端撤销授权）

---

## 文件结构

### forge-core（后端新增/修改）

```
forge-core/
├── internal/
│   ├── config/
│   │   └── config.go                          # 修改: 新增 GitHub OAuth 配置字段
│   ├── adapter/
│   │   └── github/
│   │       ├── client.go                      # 新建: GitHub API 客户端封装
│   │       └── types.go                       # 新建: GitHub 数据类型
│   ├── module/
│   │   ├── auth/
│   │   │   ├── model.go                       # 修改: 新增 UserIdentity 模型
│   │   │   ├── repository.go                  # 修改: 新增 identity CRUD 方法
│   │   │   ├── handler.go                     # 修改: 新增 GitHub OAuth handlers
│   │   │   └── service.go                     # 修改: 新增 GitHub OAuth 业务逻辑
│   │   └── project/
│   │       ├── model.go                       # 修改: 新增 ImportRequest DTO
│   │       ├── handler.go                     # 修改: 新增 Import handler
│   │       ├── service.go                     # 修改: 新增 Import 业务逻辑
│   │       └── repository.go                  # 修改: 新增 BatchCreate 方法
│   └── router/
│       └── router.go                          # 修改: 注册新路由
├── migrations/
│   └── 003_user_identities.sql                # 新建: user_identities 表
```

### forge-portal（前端新增/修改）

```
forge-portal/
├── app/
│   └── auth/
│       └── github/
│           └── callback/
│               └── page.tsx                   # 新建: OAuth 回调页
├── components/
│   ├── connect-platform-dialog.tsx            # 新建: 平台选择对话框
│   ├── import-repos-dialog.tsx                # 新建: 仓库导入对话框
│   └── project-card.tsx                       # 修改: 展示 GitHub 仓库信息
├── lib/
│   └── api.ts                                 # 修改: 新增 GitHub 相关 API 函数
```

---

## Task 1: 数据库迁移 — user_identities 表

**Files:**
- Create: `forge-core/migrations/003_user_identities.sql`

- [ ] **Step 1: 创建 user_identities 迁移脚本**

`forge-core/migrations/003_user_identities.sql`:

```sql
-- User external identity bindings (GitHub, Codeup, etc.)
CREATE TABLE IF NOT EXISTS auth.user_identities (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    provider      VARCHAR(50) NOT NULL,
    provider_uid  VARCHAR(200) NOT NULL,
    access_token  TEXT,
    refresh_token TEXT,
    token_expires TIMESTAMPTZ,
    profile       JSONB DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_uid)
);

-- Index for fast lookup by user + provider
CREATE INDEX IF NOT EXISTS idx_user_identities_user_provider
    ON auth.user_identities(user_id, provider);
```

- [ ] **Step 2: 验证迁移执行**

```bash
# 确保基础设施运行中
docker compose -f docker-compose.dev.yml up -d

# 启动 forge-core，自动执行迁移
cd forge-core && go run ./cmd/forge-core

# 验证表已创建
docker exec forge-postgres psql -U forge -d forge_main -c "\d auth.user_identities"
# 预期: 显示 user_identities 表的列定义
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/003_user_identities.sql
git commit -m "feat: add user_identities migration for external OAuth providers"
```

---

## Task 2: 后端配置 — 新增 GitHub OAuth 环境变量

**Files:**
- Modify: `forge-core/internal/config/config.go`

- [ ] **Step 1: 在 Config struct 中新增 GitHub 配置字段**

修改 `forge-core/internal/config/config.go`，在 Config struct 中添加 GitHub 相关字段，在 Load() 函数中读取环境变量：

```go
package config

import "os"

type Config struct {
	ServerPort     string
	DatabaseURL    string
	RedisAddr      string
	RedisPassword  string
	JWTSecret      string
	JWTExpireHours int

	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURI  string
}

func Load() *Config {
	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://forge:forge_dev_2026@localhost:5432/forge_main?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", "forge_redis_2026"),
		JWTSecret:      getEnv("JWT_SECRET", "forge-dev-secret-key-change-in-production"),
		JWTExpireHours: 8,

		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURI:  getEnv("GITHUB_REDIRECT_URI", "http://localhost:3000/auth/github/callback"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: 验证编译**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/internal/config/config.go
git commit -m "feat: add GitHub OAuth config (client ID, secret, redirect URI)"
```

---

## Task 3: GitHub Adapter — REST API 客户端

**Files:**
- Create: `forge-core/internal/adapter/github/types.go`
- Create: `forge-core/internal/adapter/github/client.go`

- [ ] **Step 1: 创建 types.go — 数据类型定义**

`forge-core/internal/adapter/github/types.go`:

```go
package github

import "time"

// Repository represents a GitHub repository.
type Repository struct {
	ID            int64     `json:"id"`
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	HTMLURL       string    `json:"html_url"`
	CloneURL      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	Language      string    `json:"language"`
	Private       bool      `json:"private"`
	Fork          bool      `json:"fork"`
	StarCount     int       `json:"star_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Branch represents a GitHub branch.
type Branch struct {
	Name      string `json:"name"`
	CommitSHA string `json:"commit_sha"`
	Protected bool   `json:"protected"`
}

// OAuthTokenResponse represents the response from GitHub OAuth token exchange.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// GitHubUser represents a GitHub user profile.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}
```

- [ ] **Step 2: 创建 client.go — GitHub API 客户端**

`forge-core/internal/adapter/github/client.go`:

```go
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	ghlib "github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API for Forge operations.
type Client struct {
	token  string
	client *ghlib.Client
}

// NewClient creates a GitHub API client with the given access token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		token:  token,
		client: ghlib.NewClient(tc),
	}
}

// ExchangeCode exchanges an OAuth authorization code for an access token.
// This is a static function that does not require a Client instance.
func ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (*OAuthTokenResponse, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github oauth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response")
	}

	return &tokenResp, nil
}

// GetAuthenticatedUser returns the authenticated GitHub user.
func (c *Client) GetAuthenticatedUser(ctx context.Context) (*GitHubUser, error) {
	user, resp, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("get authenticated user: %w", err)
	}
	c.logRateLimit(resp)

	return &GitHubUser{
		ID:        user.GetID(),
		Login:     user.GetLogin(),
		Name:      user.GetName(),
		AvatarURL: user.GetAvatarURL(),
		Email:     user.GetEmail(),
	}, nil
}

// ListRepos returns all repositories accessible to the authenticated user.
// Handles pagination automatically.
func (c *Client) ListRepos(ctx context.Context) ([]Repository, error) {
	var allRepos []Repository

	opts := &ghlib.RepositoryListByAuthenticatedUserOptions{
		Sort:      "updated",
		Direction: "desc",
		ListOptions: ghlib.ListOptions{
			PerPage: 100,
		},
	}

	for {
		repos, resp, err := c.client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}
		c.logRateLimit(resp)

		for _, r := range repos {
			allRepos = append(allRepos, repoFromGitHub(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRepo returns a single repository by owner and name.
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (*Repository, error) {
	r, resp, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get repo %s/%s: %w", owner, repo, err)
	}
	c.logRateLimit(resp)

	result := repoFromGitHub(r)
	return &result, nil
}

// ListBranches returns all branches for a repository.
func (c *Client) ListBranches(ctx context.Context, owner, repo string) ([]Branch, error) {
	var allBranches []Branch

	opts := &ghlib.BranchListOptions{
		ListOptions: ghlib.ListOptions{PerPage: 100},
	}

	for {
		branches, resp, err := c.client.Repositories.ListBranches(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list branches %s/%s: %w", owner, repo, err)
		}
		c.logRateLimit(resp)

		for _, b := range branches {
			allBranches = append(allBranches, Branch{
				Name:      b.GetName(),
				CommitSHA: b.GetCommit().GetSHA(),
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

// GetFileContent returns the content of a file in a repository.
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	opts := &ghlib.RepositoryContentGetOptions{Ref: ref}
	fileContent, _, resp, err := c.client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return nil, fmt.Errorf("get file %s/%s/%s@%s: %w", owner, repo, path, ref, err)
	}
	c.logRateLimit(resp)

	if fileContent == nil {
		return nil, fmt.Errorf("path %s is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}

	return []byte(content), nil
}

// logRateLimit logs GitHub API rate limit information.
func (c *Client) logRateLimit(resp *ghlib.Response) {
	if resp == nil || resp.Response == nil {
		return
	}

	remaining := resp.Header.Get("X-RateLimit-Remaining")
	limit := resp.Header.Get("X-RateLimit-Limit")
	resetStr := resp.Header.Get("X-RateLimit-Reset")

	remainingInt, _ := strconv.Atoi(remaining)
	if remainingInt < 100 {
		resetUnix, _ := strconv.ParseInt(resetStr, 10, 64)
		resetTime := time.Unix(resetUnix, 0)
		slog.Warn("github rate limit low",
			"remaining", remaining,
			"limit", limit,
			"reset_at", resetTime.Format(time.RFC3339),
		)
	}
}

// repoFromGitHub converts a go-github Repository to our Repository type.
func repoFromGitHub(r *ghlib.Repository) Repository {
	return Repository{
		ID:            r.GetID(),
		Owner:         r.GetOwner().GetLogin(),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		HTMLURL:       r.GetHTMLURL(),
		CloneURL:      r.GetCloneURL(),
		DefaultBranch: r.GetDefaultBranch(),
		Language:      r.GetLanguage(),
		Private:       r.GetPrivate(),
		Fork:          r.GetFork(),
		StarCount:     r.GetStargazersCount(),
		UpdatedAt:     r.GetUpdatedAt().Time,
	}
}
```

- [ ] **Step 3: 安装依赖**

```bash
cd forge-core
go get github.com/google/go-github/v63
go get golang.org/x/oauth2
go mod tidy
go build ./cmd/forge-core
```

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/adapter/github/
git commit -m "feat: add GitHub adapter with OAuth exchange, repo listing, branch listing, file read"
```

---

## Task 4: Auth 模块扩展 — GitHub OAuth 流程

**Files:**
- Modify: `forge-core/internal/module/auth/model.go`
- Modify: `forge-core/internal/module/auth/repository.go`
- Modify: `forge-core/internal/module/auth/service.go`
- Modify: `forge-core/internal/module/auth/handler.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

- [ ] **Step 1: 扩展 model.go — 新增 UserIdentity 模型**

在 `forge-core/internal/module/auth/model.go` 文件末尾追加：

```go
// UserIdentity represents an external OAuth identity binding.
type UserIdentity struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	Provider     string     `json:"provider"`
	ProviderUID  string     `json:"provider_uid"`
	AccessToken  string     `json:"-"`
	RefreshToken string     `json:"-"`
	TokenExpires *time.Time `json:"token_expires,omitempty"`
	Profile      string     `json:"profile"`
	CreatedAt    time.Time  `json:"created_at"`
}

// GitHubAuthorizeResponse is returned by the authorize endpoint.
type GitHubAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// GitHubCallbackRequest is the query parameters from GitHub OAuth callback.
type GitHubCallbackRequest struct {
	Code  string `form:"code" binding:"required"`
	State string `form:"state"`
}

// GitHubCallbackResponse is returned after successful OAuth callback.
type GitHubCallbackResponse struct {
	User     UserInfo `json:"user"`
	Provider string   `json:"provider"`
	GitHubUser struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"github_user"`
}
```

- [ ] **Step 2: 扩展 repository.go — 新增 identity CRUD 方法**

在 `forge-core/internal/module/auth/repository.go` 文件末尾追加：

```go
// UpsertUserIdentity creates or updates an external identity binding.
// If a binding already exists for the same provider + provider_uid, it updates the tokens.
func (r *Repository) UpsertUserIdentity(ctx context.Context, identity *UserIdentity) (*UserIdentity, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO auth.user_identities (user_id, provider, provider_uid, access_token, refresh_token, token_expires, profile)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (provider, provider_uid)
		 DO UPDATE SET access_token = EXCLUDED.access_token,
		               refresh_token = EXCLUDED.refresh_token,
		               token_expires = EXCLUDED.token_expires,
		               profile = EXCLUDED.profile
		 RETURNING id, created_at`,
		identity.UserID, identity.Provider, identity.ProviderUID,
		identity.AccessToken, identity.RefreshToken, identity.TokenExpires, identity.Profile,
	).Scan(&identity.ID, &identity.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user identity: %w", err)
	}
	return identity, nil
}

// FindUserIdentity finds an identity by user ID and provider.
func (r *Repository) FindUserIdentity(ctx context.Context, userID int64, provider string) (*UserIdentity, error) {
	identity := &UserIdentity{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_uid, access_token, refresh_token, token_expires, profile, created_at
		 FROM auth.user_identities
		 WHERE user_id = $1 AND provider = $2`,
		userID, provider,
	).Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderUID,
		&identity.AccessToken, &identity.RefreshToken, &identity.TokenExpires, &identity.Profile, &identity.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user identity: %w", err)
	}
	return identity, nil
}

// FindIdentityByProviderUID finds an identity by provider and external UID.
func (r *Repository) FindIdentityByProviderUID(ctx context.Context, provider, providerUID string) (*UserIdentity, error) {
	identity := &UserIdentity{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_uid, access_token, refresh_token, token_expires, profile, created_at
		 FROM auth.user_identities
		 WHERE provider = $1 AND provider_uid = $2`,
		provider, providerUID,
	).Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderUID,
		&identity.AccessToken, &identity.RefreshToken, &identity.TokenExpires, &identity.Profile, &identity.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find identity by provider uid: %w", err)
	}
	return identity, nil
}

// DeleteUserIdentity removes an identity binding.
func (r *Repository) DeleteUserIdentity(ctx context.Context, userID int64, provider string) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM auth.user_identities WHERE user_id = $1 AND provider = $2",
		userID, provider,
	)
	return err
}
```

- [ ] **Step 3: 扩展 service.go — 新增 GitHub OAuth 业务逻辑**

在 `forge-core/internal/module/auth/service.go` 中，首先在 Service struct 中新增字段：

```go
type Service struct {
	repo              *Repository
	jwtSecret         []byte
	jwtExpire         time.Duration
	githubClientID    string
	githubSecret      string
	githubRedirectURI string
}
```

更新 NewService 构造函数：

```go
func NewService(repo *Repository, jwtSecret string, jwtExpireHours int, githubClientID, githubSecret, githubRedirectURI string) *Service {
	return &Service{
		repo:              repo,
		jwtSecret:         []byte(jwtSecret),
		jwtExpire:         time.Duration(jwtExpireHours) * time.Hour,
		githubClientID:    githubClientID,
		githubSecret:      githubSecret,
		githubRedirectURI: githubRedirectURI,
	}
}
```

在文件末尾追加以下方法：

```go
import (
	ghAdapter "github.com/shulex/forge/forge-core/internal/adapter/github"
	"encoding/json"
	"strconv"
)

// GetGitHubAuthorizeURL generates the GitHub OAuth authorization URL.
func (s *Service) GetGitHubAuthorizeURL(state string) string {
	params := url.Values{
		"client_id":    {s.githubClientID},
		"redirect_uri": {s.githubRedirectURI},
		"scope":        {"repo,read:org,read:user,user:email"},
		"state":        {state},
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// HandleGitHubCallback exchanges the OAuth code for a token, fetches the GitHub user,
// and saves/updates the identity binding for the current Forge user.
func (s *Service) HandleGitHubCallback(ctx context.Context, userID int64, code string) (*GitHubCallbackResponse, error) {
	// Step 1: Exchange code for access token
	tokenResp, err := ghAdapter.ExchangeCode(ctx, s.githubClientID, s.githubSecret, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	// Step 2: Get GitHub user profile
	ghClient := ghAdapter.NewClient(tokenResp.AccessToken)
	ghUser, err := ghClient.GetAuthenticatedUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get github user: %w", err)
	}

	// Step 3: Save identity binding
	profileJSON, _ := json.Marshal(ghUser)
	identity := &UserIdentity{
		UserID:      userID,
		Provider:    "github",
		ProviderUID: strconv.FormatInt(ghUser.ID, 10),
		AccessToken: tokenResp.AccessToken,
		Profile:     string(profileJSON),
	}

	if _, err := s.repo.UpsertUserIdentity(ctx, identity); err != nil {
		return nil, fmt.Errorf("save identity: %w", err)
	}

	// Step 4: Get current user info for response
	user, err := s.GetCurrentUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	return &GitHubCallbackResponse{
		User:     *user,
		Provider: "github",
		GitHubUser: struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}{
			Login:     ghUser.Login,
			AvatarURL: ghUser.AvatarURL,
		},
	}, nil
}

// GetGitHubToken retrieves the stored GitHub access token for a user.
func (s *Service) GetGitHubToken(ctx context.Context, userID int64) (string, error) {
	identity, err := s.repo.FindUserIdentity(ctx, userID, "github")
	if err != nil {
		return "", fmt.Errorf("github not connected: %w", err)
	}
	return identity.AccessToken, nil
}

// HasGitHubConnection checks if a user has a GitHub identity binding.
func (s *Service) HasGitHubConnection(ctx context.Context, userID int64) bool {
	_, err := s.repo.FindUserIdentity(ctx, userID, "github")
	return err == nil
}

// DisconnectGitHub removes the GitHub identity binding for a user.
func (s *Service) DisconnectGitHub(ctx context.Context, userID int64) error {
	return s.repo.DeleteUserIdentity(ctx, userID, "github")
}
```

**Important**: 需要在 service.go 中新增 import：
- `"net/url"`（用于 GetGitHubAuthorizeURL）
- `"encoding/json"`（用于 HandleGitHubCallback）
- `"strconv"`（用于 HandleGitHubCallback）
- `ghAdapter "github.com/shulex/forge/forge-core/internal/adapter/github"`

- [ ] **Step 4: 扩展 handler.go — 新增 GitHub OAuth handlers**

在 `forge-core/internal/module/auth/handler.go` 文件末尾追加：

```go
// GET /api/auth/github/authorize
// Returns the GitHub OAuth authorize URL for the frontend to redirect to.
func (h *Handler) GitHubAuthorize(c *gin.Context) {
	// Generate a random state parameter for CSRF protection
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	// In production, store state in Redis with expiry and validate on callback.
	// For Phase 1, we skip state validation.

	authorizeURL := h.service.GetGitHubAuthorizeURL(state)
	response.OK(c, GitHubAuthorizeResponse{
		AuthorizeURL: authorizeURL,
	})
}

// GET /api/auth/github/callback?code=xxx
// Exchanges the OAuth code for a token and saves the identity binding.
func (h *Handler) GitHubCallback(c *gin.Context) {
	var req GitHubCallbackRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "缺少 code 参数")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	result, err := h.service.HandleGitHubCallback(c.Request.Context(), userID.(int64), req.Code)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "GitHub 授权失败: "+err.Error())
		return
	}

	response.OK(c, result)
}

// GET /api/auth/github/status
// Returns whether the current user has connected GitHub.
func (h *Handler) GitHubStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	connected := h.service.HasGitHubConnection(c.Request.Context(), userID.(int64))
	response.OK(c, gin.H{"connected": connected})
}

// DELETE /api/auth/github/disconnect
// Removes the GitHub identity binding.
func (h *Handler) GitHubDisconnect(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	if err := h.service.DisconnectGitHub(c.Request.Context(), userID.(int64)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "断开 GitHub 失败")
		return
	}

	response.OK(c, nil)
}
```

**Important**: 在 handler.go 的 import 中追加 `"fmt"` 和 `"time"`。

- [ ] **Step 5: 更新 router.go — 注册 GitHub OAuth 路由**

修改 `forge-core/internal/router/router.go`，在路由注册中新增 GitHub OAuth 相关路由：

```go
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
)

type Deps struct {
	AuthHandler *auth.Handler
	AuthService *auth.Service
}

func Setup(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		// Public routes
		api.POST("/auth/login", deps.AuthHandler.Login)

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.JWTAuth(deps.AuthService))
		{
			protected.POST("/auth/logout", deps.AuthHandler.Logout)
			protected.GET("/auth/me", deps.AuthHandler.Me)

			// GitHub OAuth
			protected.GET("/auth/github/authorize", deps.AuthHandler.GitHubAuthorize)
			protected.GET("/auth/github/callback", deps.AuthHandler.GitHubCallback)
			protected.GET("/auth/github/status", deps.AuthHandler.GitHubStatus)
			protected.DELETE("/auth/github/disconnect", deps.AuthHandler.GitHubDisconnect)
		}
	}

	return r
}
```

- [ ] **Step 6: 更新 main.go — 传递 GitHub 配置到 auth service**

修改 `forge-core/cmd/forge-core/main.go` 中 auth module 的构建：

```go
// Auth module
authRepo := auth.NewRepository(db)
authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours,
	cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURI)
authHandler := auth.NewHandler(authService)
```

- [ ] **Step 7: 验证编译和测试**

```bash
cd forge-core
go mod tidy
go build ./cmd/forge-core
```

**测试 GitHub OAuth 流程**（需要先设置环境变量）：

```bash
# 设置 GitHub OAuth 环境变量
export GITHUB_CLIENT_ID="your_client_id"
export GITHUB_CLIENT_SECRET="your_client_secret"

cd forge-core && go run ./cmd/forge-core
```

```bash
# 1. 登录获取 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 2. 获取 GitHub 授权 URL
curl -s http://localhost:8080/api/auth/github/authorize \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"data":{"authorize_url":"https://github.com/login/oauth/authorize?client_id=..."}}

# 3. 检查 GitHub 连接状态
curl -s http://localhost:8080/api/auth/github/status \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"data":{"connected":false}}
```

- [ ] **Step 8: Commit**

```bash
git add forge-core/internal/module/auth/ forge-core/internal/router/router.go forge-core/cmd/forge-core/main.go
git commit -m "feat: implement GitHub OAuth flow (authorize URL, callback, identity binding)"
```

---

## Task 5: GitHub Repos API + 项目导入

**Files:**
- Modify: `forge-core/internal/module/auth/handler.go` (新增 ListGitHubRepos handler)
- Modify: `forge-core/internal/module/auth/service.go` (新增 ListGitHubRepos 逻辑)
- Modify: `forge-core/internal/module/project/model.go` (新增 ImportRequest DTO)
- Modify: `forge-core/internal/module/project/repository.go` (新增 BatchCreate)
- Modify: `forge-core/internal/module/project/service.go` (新增 ImportFromGitHub)
- Modify: `forge-core/internal/module/project/handler.go` (新增 Import handler)
- Modify: `forge-core/internal/router/router.go` (注册新路由)

- [ ] **Step 1: 在 auth service 中添加 ListGitHubRepos**

在 `forge-core/internal/module/auth/service.go` 文件末尾追加：

```go
// ListGitHubRepos lists all GitHub repositories for the authenticated user.
func (s *Service) ListGitHubRepos(ctx context.Context, userID int64) ([]ghAdapter.Repository, error) {
	token, err := s.GetGitHubToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	client := ghAdapter.NewClient(token)
	return client.ListRepos(ctx)
}
```

- [ ] **Step 2: 在 auth handler 中添加 ListGitHubRepos handler**

在 `forge-core/internal/module/auth/handler.go` 文件末尾追加：

```go
// GET /api/github/repos
// Lists all GitHub repositories for the authenticated user.
func (h *Handler) ListGitHubRepos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	repos, err := h.service.ListGitHubRepos(c.Request.Context(), userID.(int64))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "获取仓库列表失败: "+err.Error())
		return
	}

	response.OK(c, repos)
}
```

- [ ] **Step 3: 扩展 project model — 新增 ImportRequest DTO**

在 `forge-core/internal/module/project/model.go` 文件末尾追加：

```go
// ImportRepoItem represents a single GitHub repo to import.
type ImportRepoItem struct {
	FullName      string `json:"full_name" binding:"required"`  // e.g. "owner/repo"
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url" binding:"required"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Language      string `json:"language"`
}

// ImportRequest is the request body for POST /api/projects/import.
type ImportRequest struct {
	Repos []ImportRepoItem `json:"repos" binding:"required,min=1"`
}

// ImportResponse contains the result of a batch import.
type ImportResponse struct {
	Imported int            `json:"imported"`
	Skipped  int            `json:"skipped"`
	Projects []ProjectBrief `json:"projects"`
	Errors   []string       `json:"errors,omitempty"`
}

// ProjectBrief is a minimal project representation for import response.
type ProjectBrief struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	CodeRepoURL   string `json:"code_repo_url"`
	DefaultBranch string `json:"default_branch"`
}
```

- [ ] **Step 4: 扩展 project repository — 新增 CreateFromImport 方法**

在 `forge-core/internal/module/project/repository.go` 文件末尾追加：

```go
// CreateFromImport creates a project from an imported GitHub repo.
// Returns nil if a project with the same name already exists in the tenant (skip).
func (r *Repository) CreateFromImport(ctx context.Context, tenantID, userID int64, item *ImportRepoItem) (*ProjectBrief, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.projects (tenant_id, name, description, status, code_platform, code_repo_url, default_branch, created_by)
		 VALUES ($1, $2, $3, 'ACTIVE', 'github', $4, $5, $6)
		 ON CONFLICT (tenant_id, name) DO NOTHING
		 RETURNING id`,
		tenantID, item.Name, item.Description, item.HTMLURL, item.DefaultBranch, userID,
	).Scan(&id)
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows — check for this
		if err.Error() == "no rows in result set" {
			return nil, nil // Skipped: already exists
		}
		return nil, fmt.Errorf("create project from import: %w", err)
	}

	return &ProjectBrief{
		ID:            id,
		Name:          item.Name,
		CodeRepoURL:   item.HTMLURL,
		DefaultBranch: item.DefaultBranch,
	}, nil
}
```

- [ ] **Step 5: 扩展 project service — 新增 ImportFromGitHub**

在 `forge-core/internal/module/project/service.go` 文件末尾追加：

```go
// ImportFromGitHub imports selected GitHub repos as Forge projects.
func (s *Service) ImportFromGitHub(ctx context.Context, tenantID, userID int64, req *ImportRequest) (*ImportResponse, error) {
	resp := &ImportResponse{}

	for _, item := range req.Repos {
		brief, err := s.repo.CreateFromImport(ctx, tenantID, userID, &item)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %s", item.FullName, err.Error()))
			continue
		}
		if brief == nil {
			resp.Skipped++
			continue
		}
		resp.Imported++
		resp.Projects = append(resp.Projects, *brief)
	}

	return resp, nil
}
```

- [ ] **Step 6: 扩展 project handler — 新增 Import handler**

在 `forge-core/internal/module/project/handler.go` 文件末尾追加：

```go
// POST /api/projects/import
// Imports selected GitHub repos as Forge projects.
func (h *Handler) Import(c *gin.Context) {
	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请选择至少一个仓库导入")
		return
	}

	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	result, err := h.service.ImportFromGitHub(
		c.Request.Context(),
		tenantID.(int64),
		userID.(int64),
		&req,
	)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "导入失败: "+err.Error())
		return
	}

	response.OK(c, result)
}
```

- [ ] **Step 7: 更新 router.go — 注册新路由**

修改 `forge-core/internal/router/router.go`：

1. 在 Deps struct 中新增 ProjectHandler：

```go
import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/project"
)

type Deps struct {
	AuthHandler    *auth.Handler
	AuthService    *auth.Service
	ProjectHandler *project.Handler
}
```

2. 在 protected 路由组中添加：

```go
		protected.Use(middleware.JWTAuth(deps.AuthService))
		{
			protected.POST("/auth/logout", deps.AuthHandler.Logout)
			protected.GET("/auth/me", deps.AuthHandler.Me)

			// GitHub OAuth
			protected.GET("/auth/github/authorize", deps.AuthHandler.GitHubAuthorize)
			protected.GET("/auth/github/callback", deps.AuthHandler.GitHubCallback)
			protected.GET("/auth/github/status", deps.AuthHandler.GitHubStatus)
			protected.DELETE("/auth/github/disconnect", deps.AuthHandler.GitHubDisconnect)

			// GitHub repos
			protected.GET("/github/repos", deps.AuthHandler.ListGitHubRepos)

			// Project import
			protected.POST("/projects/import", deps.ProjectHandler.Import)
		}
```

**Note**: S2 中已有的 project CRUD 路由（GET/POST/PUT/DELETE /projects）保持不变，这里只添加新路由。

- [ ] **Step 8: 更新 main.go — 组装 project handler**

在 `forge-core/cmd/forge-core/main.go` 中，router.Setup 调用时传递 ProjectHandler（假设 S2 已有 project 模块组装，这里确保 router.Deps 包含 ProjectHandler）：

```go
// Router
r := router.Setup(&router.Deps{
	AuthHandler:    authHandler,
	AuthService:    authService,
	ProjectHandler: projectHandler,
})
```

- [ ] **Step 9: 验证编译和测试**

```bash
cd forge-core
go mod tidy
go build ./cmd/forge-core
```

**测试（需要先完成 GitHub OAuth 授权）**：

```bash
# 登录
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 列出 GitHub 仓库（需要先完成 OAuth 授权）
curl -s http://localhost:8080/api/github/repos \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"data":[{"id":123,"owner":"xxx","name":"repo1",...},...]}

# 导入仓库
curl -s -X POST http://localhost:8080/api/projects/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repos": [
      {
        "full_name": "owner/repo1",
        "name": "repo1",
        "description": "My repo",
        "html_url": "https://github.com/owner/repo1",
        "clone_url": "https://github.com/owner/repo1.git",
        "default_branch": "main",
        "language": "Go"
      }
    ]
  }'
# 预期: {"code":0,"data":{"imported":1,"skipped":0,"projects":[{"id":1,"name":"repo1",...}]}}

# 再次导入同一个仓库 — 应该 skip
curl -s -X POST http://localhost:8080/api/projects/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repos": [{"full_name":"owner/repo1","name":"repo1","description":"My repo","html_url":"https://github.com/owner/repo1","default_branch":"main","language":"Go"}]
  }'
# 预期: {"code":0,"data":{"imported":0,"skipped":1,"projects":[]}}
```

- [ ] **Step 10: Commit**

```bash
git add forge-core/internal/module/auth/ forge-core/internal/module/project/ forge-core/internal/router/router.go forge-core/cmd/forge-core/main.go
git commit -m "feat: add GitHub repos listing API and batch project import from GitHub"
```

---

## Task 6: 前端 — GitHub OAuth 回调页

**Files:**
- Create: `forge-portal/app/auth/github/callback/page.tsx`
- Modify: `forge-portal/lib/api.ts`

- [ ] **Step 1: 扩展 api.ts — 新增 GitHub 相关 API 函数**

在 `forge-portal/lib/api.ts` 文件中追加以下函数：

```typescript
// ========== GitHub OAuth APIs ==========

export interface GitHubAuthorizeResponse {
  authorize_url: string;
}

export interface GitHubStatusResponse {
  connected: boolean;
}

export interface GitHubRepo {
  id: number;
  owner: string;
  name: string;
  full_name: string;
  description: string;
  html_url: string;
  clone_url: string;
  default_branch: string;
  language: string;
  private: boolean;
  fork: boolean;
  star_count: number;
  updated_at: string;
}

export interface ImportRepoItem {
  full_name: string;
  name: string;
  description: string;
  html_url: string;
  clone_url: string;
  default_branch: string;
  language: string;
}

export interface ImportResponse {
  imported: number;
  skipped: number;
  projects: Array<{
    id: number;
    name: string;
    code_repo_url: string;
    default_branch: string;
  }>;
  errors?: string[];
}

export async function getGitHubAuthorizeURL(): Promise<GitHubAuthorizeResponse> {
  return fetchAPI<GitHubAuthorizeResponse>('/api/auth/github/authorize');
}

export async function exchangeGitHubCode(code: string): Promise<unknown> {
  return fetchAPI<unknown>(`/api/auth/github/callback?code=${encodeURIComponent(code)}`);
}

export async function getGitHubStatus(): Promise<GitHubStatusResponse> {
  return fetchAPI<GitHubStatusResponse>('/api/auth/github/status');
}

export async function listGitHubRepos(): Promise<GitHubRepo[]> {
  return fetchAPI<GitHubRepo[]>('/api/github/repos');
}

export async function importGitHubRepos(repos: ImportRepoItem[]): Promise<ImportResponse> {
  return fetchAPI<ImportResponse>('/api/projects/import', {
    method: 'POST',
    body: JSON.stringify({ repos }),
  });
}

export async function disconnectGitHub(): Promise<void> {
  return fetchAPI<void>('/api/auth/github/disconnect', {
    method: 'DELETE',
  });
}
```

**Note**: 这里假设 `fetchAPI<T>` 是 S1 中已有的通用 fetch wrapper，它自动添加 Authorization header 和 base URL，解析 `Result[T]` 中的 `data` 字段。如果命名不同，按实际代码调整。

- [ ] **Step 2: 创建 OAuth 回调页**

`forge-portal/app/auth/github/callback/page.tsx`:

```tsx
'use client';

import { useEffect, useState } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { exchangeGitHubCode } from '@/lib/api';
import { Loader2, CheckCircle2, XCircle } from 'lucide-react';

export default function GitHubCallbackPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [message, setMessage] = useState('正在连接 GitHub...');

  useEffect(() => {
    const code = searchParams.get('code');

    if (!code) {
      setStatus('error');
      setMessage('授权失败：缺少授权码');
      return;
    }

    exchangeGitHubCode(code)
      .then(() => {
        setStatus('success');
        setMessage('GitHub 连接成功！正在跳转...');
        // Redirect to project hall after 1.5 seconds
        setTimeout(() => {
          router.push('/projects?github_connected=true');
        }, 1500);
      })
      .catch((err) => {
        setStatus('error');
        setMessage(`GitHub 授权失败：${err instanceof Error ? err.message : '未知错误'}`);
      });
  }, [searchParams, router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-4 rounded-lg border border-border bg-card p-8 shadow-lg">
        {status === 'loading' && (
          <>
            <Loader2 className="h-12 w-12 animate-spin text-purple-500" />
            <p className="text-lg text-muted-foreground">{message}</p>
          </>
        )}
        {status === 'success' && (
          <>
            <CheckCircle2 className="h-12 w-12 text-green-500" />
            <p className="text-lg text-foreground">{message}</p>
          </>
        )}
        {status === 'error' && (
          <>
            <XCircle className="h-12 w-12 text-red-500" />
            <p className="text-lg text-red-400">{message}</p>
            <button
              onClick={() => router.push('/projects')}
              className="mt-4 rounded-md bg-purple-600 px-4 py-2 text-sm text-white hover:bg-purple-700 transition-colors"
            >
              返回项目大厅
            </button>
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 验证页面可访问**

```bash
cd forge-portal && npm run dev
```

打开 `http://localhost:3000/auth/github/callback` — 应该看到 "缺少授权码" 的错误页面（因为没有 code 参数）。

打开 `http://localhost:3000/auth/github/callback?code=test` — 应该看到 loading 状态，然后显示 API 请求失败（因为 code 无效）。

- [ ] **Step 4: Commit**

```bash
git add forge-portal/app/auth/github/callback/ forge-portal/lib/api.ts
git commit -m "feat: add GitHub OAuth callback page and API client functions"
```

---

## Task 7: 前端 — 平台接入对话框

**Files:**
- Create: `forge-portal/components/connect-platform-dialog.tsx`

- [ ] **Step 1: 创建平台接入对话框**

`forge-portal/components/connect-platform-dialog.tsx`:

```tsx
'use client';

import { useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { getGitHubAuthorizeURL } from '@/lib/api';
import { Loader2 } from 'lucide-react';

interface ConnectPlatformDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const platforms = [
  {
    id: 'github',
    name: 'GitHub',
    icon: (
      <svg viewBox="0 0 24 24" className="h-8 w-8 fill-current">
        <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
      </svg>
    ),
    description: '连接 GitHub 账号，同步仓库并导入为 Forge 项目',
    available: true,
  },
  {
    id: 'codeup',
    name: '云效 Codeup',
    icon: (
      <svg viewBox="0 0 24 24" className="h-8 w-8 fill-current">
        <rect width="24" height="24" rx="4" className="fill-orange-500/20" />
        <text x="12" y="16" textAnchor="middle" className="fill-orange-500 text-[10px] font-bold">CU</text>
      </svg>
    ),
    description: '连接阿里云效 Codeup，同步仓库（即将支持）',
    available: false,
  },
];

export function ConnectPlatformDialog({ open, onOpenChange }: ConnectPlatformDialogProps) {
  const [connecting, setConnecting] = useState(false);

  const handleConnect = async (platformId: string) => {
    if (platformId !== 'github') return;

    setConnecting(true);
    try {
      const { authorize_url } = await getGitHubAuthorizeURL();
      // Redirect to GitHub OAuth page
      window.location.href = authorize_url;
    } catch (err) {
      console.error('Failed to get authorize URL:', err);
      setConnecting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>接入代码平台</DialogTitle>
          <DialogDescription>
            选择代码托管平台，授权后 Forge 将自动同步仓库列表
          </DialogDescription>
        </DialogHeader>
        <div className="mt-4 space-y-3">
          {platforms.map((platform) => (
            <button
              key={platform.id}
              disabled={!platform.available || connecting}
              onClick={() => handleConnect(platform.id)}
              className={`flex w-full items-center gap-4 rounded-lg border p-4 text-left transition-colors
                ${platform.available
                  ? 'border-border hover:border-purple-500/50 hover:bg-purple-500/5 cursor-pointer'
                  : 'border-border/50 opacity-50 cursor-not-allowed'
                }
              `}
            >
              <div className="shrink-0 text-foreground">
                {platform.icon}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-foreground">{platform.name}</span>
                  {!platform.available && (
                    <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                      即将支持
                    </span>
                  )}
                </div>
                <p className="mt-1 text-sm text-muted-foreground">{platform.description}</p>
              </div>
              {connecting && platform.id === 'github' && (
                <Loader2 className="h-5 w-5 animate-spin text-purple-500" />
              )}
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add forge-portal/components/connect-platform-dialog.tsx
git commit -m "feat: add connect platform dialog with GitHub OAuth redirect"
```

---

## Task 8: 前端 — 仓库导入对话框

**Files:**
- Create: `forge-portal/components/import-repos-dialog.tsx`

- [ ] **Step 1: 创建仓库导入对话框**

`forge-portal/components/import-repos-dialog.tsx`:

```tsx
'use client';

import { useEffect, useState, useMemo } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import {
  listGitHubRepos,
  importGitHubRepos,
  type GitHubRepo,
  type ImportRepoItem,
} from '@/lib/api';
import { Loader2, Search, GitFork, Star, Lock, Globe } from 'lucide-react';

interface ImportReposDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImportComplete: () => void;
}

export function ImportReposDialog({ open, onOpenChange, onImportComplete }: ImportReposDialogProps) {
  const [repos, setRepos] = useState<GitHubRepo[]>([]);
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ imported: number; skipped: number } | null>(null);

  // Fetch repos when dialog opens
  useEffect(() => {
    if (!open) {
      setSelected(new Set());
      setSearch('');
      setError(null);
      setResult(null);
      return;
    }

    setLoading(true);
    setError(null);
    listGitHubRepos()
      .then((data) => {
        setRepos(data);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : '获取仓库列表失败');
      })
      .finally(() => setLoading(false));
  }, [open]);

  // Group repos by owner/org
  const grouped = useMemo(() => {
    const filtered = repos.filter(
      (r) =>
        r.full_name.toLowerCase().includes(search.toLowerCase()) ||
        (r.description && r.description.toLowerCase().includes(search.toLowerCase()))
    );

    const groups: Record<string, GitHubRepo[]> = {};
    for (const repo of filtered) {
      const owner = repo.owner;
      if (!groups[owner]) groups[owner] = [];
      groups[owner].push(repo);
    }

    // Sort groups alphabetically, repos by updated_at desc
    return Object.entries(groups)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([owner, repos]) => ({
        owner,
        repos: repos.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()),
      }));
  }, [repos, search]);

  const toggleRepo = (fullName: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(fullName)) {
        next.delete(fullName);
      } else {
        next.add(fullName);
      }
      return next;
    });
  };

  const toggleAll = () => {
    const allVisible = grouped.flatMap((g) => g.repos.map((r) => r.full_name));
    if (allVisible.every((fn) => selected.has(fn))) {
      // Deselect all visible
      setSelected((prev) => {
        const next = new Set(prev);
        allVisible.forEach((fn) => next.delete(fn));
        return next;
      });
    } else {
      // Select all visible
      setSelected((prev) => {
        const next = new Set(prev);
        allVisible.forEach((fn) => next.add(fn));
        return next;
      });
    }
  };

  const handleImport = async () => {
    if (selected.size === 0) return;

    setImporting(true);
    setError(null);

    const items: ImportRepoItem[] = repos
      .filter((r) => selected.has(r.full_name))
      .map((r) => ({
        full_name: r.full_name,
        name: r.name,
        description: r.description || '',
        html_url: r.html_url,
        clone_url: r.clone_url,
        default_branch: r.default_branch,
        language: r.language || '',
      }));

    try {
      const resp = await importGitHubRepos(items);
      setResult({ imported: resp.imported, skipped: resp.skipped });
      if (resp.errors && resp.errors.length > 0) {
        setError(`部分导入失败: ${resp.errors.join(', ')}`);
      }
      // Notify parent to refresh project list
      setTimeout(() => {
        onImportComplete();
        onOpenChange(false);
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入失败');
    } finally {
      setImporting(false);
    }
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    if (diffDays === 0) return '今天';
    if (diffDays === 1) return '昨天';
    if (diffDays < 30) return `${diffDays} 天前`;
    if (diffDays < 365) return `${Math.floor(diffDays / 30)} 个月前`;
    return `${Math.floor(diffDays / 365)} 年前`;
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[640px] max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>导入 GitHub 仓库</DialogTitle>
          <DialogDescription>
            选择要导入为 Forge 项目的仓库
          </DialogDescription>
        </DialogHeader>

        {/* Search bar */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="搜索仓库..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        {/* Repo list */}
        <div className="flex-1 overflow-y-auto min-h-0 max-h-[400px] space-y-4 pr-1">
          {loading && (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-purple-500" />
              <span className="ml-3 text-muted-foreground">正在同步仓库列表...</span>
            </div>
          )}

          {error && !loading && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-4 text-sm text-red-400">
              {error}
            </div>
          )}

          {result && (
            <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-4 text-sm text-green-400">
              成功导入 {result.imported} 个项目
              {result.skipped > 0 && `，跳过 ${result.skipped} 个（已存在）`}
            </div>
          )}

          {!loading && !error && repos.length === 0 && (
            <div className="py-12 text-center text-muted-foreground">
              未找到任何仓库
            </div>
          )}

          {!loading && grouped.map(({ owner, repos: groupRepos }) => (
            <div key={owner}>
              <div className="sticky top-0 z-10 bg-card/95 backdrop-blur-sm px-1 py-2">
                <span className="text-sm font-medium text-muted-foreground">{owner}</span>
                <span className="ml-2 text-xs text-muted-foreground">({groupRepos.length})</span>
              </div>
              <div className="space-y-1">
                {groupRepos.map((repo) => (
                  <label
                    key={repo.full_name}
                    className={`flex items-center gap-3 rounded-md border px-3 py-2.5 cursor-pointer transition-colors
                      ${selected.has(repo.full_name)
                        ? 'border-purple-500/50 bg-purple-500/5'
                        : 'border-transparent hover:bg-muted/50'
                      }
                    `}
                  >
                    <Checkbox
                      checked={selected.has(repo.full_name)}
                      onCheckedChange={() => toggleRepo(repo.full_name)}
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-sm text-foreground truncate">
                          {repo.name}
                        </span>
                        {repo.private ? (
                          <Lock className="h-3 w-3 shrink-0 text-yellow-500" />
                        ) : (
                          <Globe className="h-3 w-3 shrink-0 text-muted-foreground" />
                        )}
                        {repo.fork && (
                          <GitFork className="h-3 w-3 shrink-0 text-muted-foreground" />
                        )}
                      </div>
                      {repo.description && (
                        <p className="text-xs text-muted-foreground truncate mt-0.5">
                          {repo.description}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-3 shrink-0 text-xs text-muted-foreground">
                      {repo.language && (
                        <span className="flex items-center gap-1">
                          <span className="h-2 w-2 rounded-full bg-purple-400" />
                          {repo.language}
                        </span>
                      )}
                      {repo.star_count > 0 && (
                        <span className="flex items-center gap-1">
                          <Star className="h-3 w-3" />
                          {repo.star_count}
                        </span>
                      )}
                      <span>{formatDate(repo.updated_at)}</span>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          ))}
        </div>

        {/* Footer */}
        <DialogFooter className="flex items-center justify-between border-t border-border pt-4">
          <div className="flex items-center gap-3">
            {!loading && repos.length > 0 && (
              <Button variant="ghost" size="sm" onClick={toggleAll}>
                {grouped.flatMap(g => g.repos).every(r => selected.has(r.full_name))
                  ? '取消全选'
                  : '全选'}
              </Button>
            )}
            <span className="text-sm text-muted-foreground">
              已选择 {selected.size} 个仓库
            </span>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button
              onClick={handleImport}
              disabled={selected.size === 0 || importing}
              className="bg-purple-600 hover:bg-purple-700"
            >
              {importing ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  导入中...
                </>
              ) : (
                `导入选中 (${selected.size})`
              )}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add forge-portal/components/import-repos-dialog.tsx
git commit -m "feat: add import repos dialog with search, grouping, and batch selection"
```

---

## Task 9: 前端 — 接入项目大厅 + 项目卡片更新

**Files:**
- Modify: `forge-portal/app/(dashboard)/projects/page.tsx`
- Modify: `forge-portal/components/project-card.tsx` (or equivalent)

- [ ] **Step 1: 更新项目大厅 — 添加 "接入代码平台" 按钮和导入流程**

修改 `forge-portal/app/(dashboard)/projects/page.tsx`，在已有的项目大厅页面中添加：

1. "接入代码平台" 按钮（在页面顶部操作栏中）
2. ConnectPlatformDialog 和 ImportReposDialog 的状态管理
3. URL 参数检测（`?github_connected=true` 时自动打开导入对话框）

在页面组件中新增以下状态和逻辑：

```tsx
'use client';

import { useState, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import { ConnectPlatformDialog } from '@/components/connect-platform-dialog';
import { ImportReposDialog } from '@/components/import-repos-dialog';
import { getGitHubStatus } from '@/lib/api';
// ... other existing imports ...

export default function ProjectsPage() {
  // ... existing state from S2 ...

  const searchParams = useSearchParams();
  const [connectDialogOpen, setConnectDialogOpen] = useState(false);
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [githubConnected, setGithubConnected] = useState(false);

  // Check GitHub connection status on mount
  useEffect(() => {
    getGitHubStatus()
      .then(({ connected }) => setGithubConnected(connected))
      .catch(() => {}); // Ignore error — user may not have connected
  }, []);

  // Auto-open import dialog after OAuth callback redirect
  useEffect(() => {
    if (searchParams.get('github_connected') === 'true') {
      setGithubConnected(true);
      setImportDialogOpen(true);
      // Clean up URL param
      window.history.replaceState({}, '', '/projects');
    }
  }, [searchParams]);

  const handleConnectPlatform = () => {
    if (githubConnected) {
      // Already connected — go straight to import
      setImportDialogOpen(true);
    } else {
      setConnectDialogOpen(true);
    }
  };

  const handleImportComplete = () => {
    // Refresh project list — call whatever fetch function S2 uses
    // e.g. fetchProjects() or mutate()
    setGithubConnected(true);
  };

  // In the JSX, add the button to the page header area (alongside existing "新建项目" button):
  // <Button variant="outline" onClick={handleConnectPlatform}>
  //   <Github className="mr-2 h-4 w-4" />
  //   {githubConnected ? '导入 GitHub 仓库' : '接入代码平台'}
  // </Button>

  // And render the dialogs at the bottom of the component:
  // <ConnectPlatformDialog open={connectDialogOpen} onOpenChange={setConnectDialogOpen} />
  // <ImportReposDialog
  //   open={importDialogOpen}
  //   onOpenChange={setImportDialogOpen}
  //   onImportComplete={handleImportComplete}
  // />

  // ... rest of existing page ...
}
```

**具体集成点**（取决于 S2 实际代码结构）：

在项目大厅页面的操作区域（通常在页面标题右侧），添加按钮：

```tsx
import { Github } from 'lucide-react';

// In the header/action area:
<div className="flex items-center gap-3">
  <Button variant="outline" onClick={handleConnectPlatform}>
    <Github className="mr-2 h-4 w-4" />
    {githubConnected ? '导入 GitHub 仓库' : '接入代码平台'}
  </Button>
  {/* Existing "新建项目" button from S2 */}
</div>
```

在组件底部渲染对话框：

```tsx
<ConnectPlatformDialog
  open={connectDialogOpen}
  onOpenChange={setConnectDialogOpen}
/>
<ImportReposDialog
  open={importDialogOpen}
  onOpenChange={setImportDialogOpen}
  onImportComplete={handleImportComplete}
/>
```

- [ ] **Step 2: 更新项目卡片 — 展示 GitHub 信息**

修改项目卡片组件（S2 中的 `forge-portal/components/project-card.tsx` 或 projects 页面中的内联卡片），为 `code_platform === 'github'` 的项目展示额外信息：

```tsx
import { Github, GitBranch, Code2 } from 'lucide-react';

// In the project card component, add after the project name/description:

{project.code_platform === 'github' && (
  <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
    <span className="flex items-center gap-1">
      <Github className="h-3 w-3" />
      GitHub
    </span>
    {project.default_branch && (
      <span className="flex items-center gap-1">
        <GitBranch className="h-3 w-3" />
        {project.default_branch}
      </span>
    )}
    {project.language && (
      <span className="flex items-center gap-1">
        <Code2 className="h-3 w-3" />
        {project.language}
      </span>
    )}
    {project.code_repo_url && (
      <a
        href={project.code_repo_url}
        target="_blank"
        rel="noopener noreferrer"
        className="hover:text-purple-400 transition-colors truncate max-w-[200px]"
        onClick={(e) => e.stopPropagation()}
      >
        {project.code_repo_url.replace('https://github.com/', '')}
      </a>
    )}
  </div>
)}
```

**Note**: 项目卡片的具体修改取决于 S2 的项目数据模型中是否已包含 `code_platform`, `code_repo_url`, `default_branch` 字段。如果 S2 的 project list API 没有返回这些字段，需要确保后端 API 返回它们。

- [ ] **Step 3: 验证完整流程**

```bash
# 启动后端
cd forge-core && go run ./cmd/forge-core

# 启动前端
cd forge-portal && npm run dev
```

1. 打开 `http://localhost:3000/login`，用 `admin / admin123` 登录
2. 在项目大厅点击 "接入代码平台" 按钮
3. 在弹出的平台选择对话框中点击 GitHub
4. 浏览器跳转到 GitHub OAuth 授权页面
5. 点击 "Authorize" 授权
6. 浏览器自动回调到 `http://localhost:3000/auth/github/callback?code=xxx`
7. 看到 "GitHub 连接成功！正在跳转..." 提示
8. 自动跳转到项目大厅，仓库导入对话框自动打开
9. 看到 GitHub 仓库列表（按组织分组）
10. 勾选仓库，点击 "导入选中"
11. 看到 "成功导入 N 个项目" 提示
12. 对话框关闭后，项目大厅刷新，看到导入的项目
13. 项目卡片显示 GitHub 图标、仓库 URL、默认分支

- [ ] **Step 4: Commit**

```bash
git add forge-portal/app/ forge-portal/components/
git commit -m "feat: integrate GitHub connect + import flow into project hall, show repo info on project cards"
```

---

## 端到端流程总结

```
用户点击 "接入代码平台"
      │
      ▼
ConnectPlatformDialog 打开
      │ 选择 GitHub
      ▼
GET /api/auth/github/authorize
      │ 返回 authorize_url
      ▼
浏览器跳转到 GitHub OAuth 页面
      │ 用户授权
      ▼
GitHub 回调 → /auth/github/callback?code=xxx
      │
      ▼
前端 callback page → GET /api/auth/github/callback?code=xxx
      │ 后端：exchange code → save token → save identity
      ▼
跳转到 /projects?github_connected=true
      │
      ▼
ImportReposDialog 自动打开 → GET /api/github/repos
      │ 后端：用 saved token 调 GitHub API
      ▼
用户选择仓库 → POST /api/projects/import
      │ 后端：批量创建 project 记录
      ▼
项目大厅刷新 → 显示导入的项目（带 GitHub 信息）
```

---

## 注意事项和未来改进

1. **Token 安全**: Phase 1 明文存储 access token。生产环境应使用 Vault 加密存储。
2. **State 参数**: Phase 1 跳过 OAuth state 验证。生产环境应生成随机 state 存入 Redis，回调时验证以防 CSRF。
3. **Webhook 同步**: Phase 1 仅在用户手动操作时同步仓库。后续可通过 GitHub Webhook 实现实时同步。
4. **Rate Limiting**: 已在 adapter 中添加 rate limit 日志。如遇 403 错误，需检查 X-RateLimit-Remaining。
5. **Token 刷新**: GitHub OAuth App token 不过期（除非用户撤销）。如果切换到 GitHub App，需实现 token 刷新。
6. **Codeup 支持**: ConnectPlatformDialog 已预留 Codeup 入口（灰色不可用），后续切片实现。
7. **多账号**: Phase 1 每个 Forge 用户只能绑定一个 GitHub 账号（UNIQUE(provider, provider_uid) 约束）。
