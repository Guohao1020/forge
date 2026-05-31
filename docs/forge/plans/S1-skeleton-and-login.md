# S1 — 项目骨架 + 基础设施 + 前端框架 + 登录闭环

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从零搭建 Go + Next.js 新架构，交付一个可在浏览器中登录并看到空项目大厅的完整闭环。

**Architecture:** forge-core (Go/Gin 模块化单体) 提供 auth API，forge-portal (Next.js 15 App Router) 提供前端。PostgreSQL + Redis 通过 Docker Compose 运行。旧 Java 代码保留不动，新代码在新目录中构建。

**Tech Stack:** Go 1.22 + Gin, Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 + shadcn/ui, PostgreSQL 16, Redis 7, Docker Compose

---

## 前置说明

### 与旧代码的关系

旧的 Java 微服务代码（forge-engine, forge-identity 等）和 Vue 3 前端（forge-portal/）保留不动，作为需求参考。新架构代码在以下新目录中构建：

- `forge-core/` — Go API Server（新建）
- `forge-portal/` — **覆盖**旧 Vue 3 代码，替换为 Next.js（旧代码已无用）

### 旧代码清理策略

本切片**不删除**旧 Java 项目。它们作为需求参考保留，在后续切片中按需清理。旧 docker-compose.yml 重命名为 docker-compose.legacy.yml 备份。

### 本切片交付后你可以做什么

1. `docker compose up -d` 启动 PostgreSQL + Redis
2. `cd forge-core && go run ./cmd/forge-core` 启动后端
3. `cd forge-portal && npm run dev` 启动前端
4. 浏览器打开 `http://localhost:3000`，看到登录页
5. 用 `admin / admin123` 登录，进入空的项目大厅页面
6. Token 过期或手动登出后回到登录页

---

## 文件结构

### forge-core（Go API Server）

```
forge-core/
├── cmd/
│   └── forge-core/
│       └── main.go                    # 启动入口
├── internal/
│   ├── config/
│   │   └── config.go                  # 配置加载 (env vars)
│   ├── middleware/
│   │   ├── auth.go                    # JWT 鉴权中间件
│   │   └── cors.go                    # CORS 中间件
│   ├── module/
│   │   └── auth/
│   │       ├── handler.go             # HTTP handler (login/logout/me)
│   │       ├── service.go             # 业务逻辑
│   │       ├── repository.go          # 数据库操作
│   │       └── model.go               # 数据模型
│   ├── pkg/
│   │   ├── response/
│   │   │   └── response.go            # Result[T] 统一响应
│   │   ├── database/
│   │   │   └── postgres.go            # PostgreSQL 连接
│   │   └── redis/
│   │       └── redis.go               # Redis 连接
│   └── router/
│       └── router.go                  # 路由注册
├── migrations/
│   └── 001_init_auth.sql              # auth schema DDL + seed data
├── go.mod
└── go.sum
```

### forge-portal（Next.js 前端）

```
forge-portal/
├── app/
│   ├── layout.tsx                     # Root layout (fonts, providers)
│   ├── page.tsx                       # Redirect to /login
│   ├── globals.css                    # Tailwind + 深空主题 CSS 变量
│   ├── login/
│   │   └── page.tsx                   # 登录页
│   └── (dashboard)/
│       ├── layout.tsx                 # Dashboard layout (侧边栏 + 顶栏)
│       └── projects/
│           └── page.tsx               # 项目大厅 (空状态)
├── components/
│   ├── ui/                            # shadcn/ui 组件 (按需添加)
│   ├── aurora-background.tsx          # Aurora 极光背景动效
│   └── forge-logo.tsx                 # Forge Logo SVG
├── lib/
│   ├── api.ts                         # fetch wrapper (base URL + token)
│   └── auth.ts                        # auth context + useAuth hook
├── next.config.ts                     # API 代理配置
├── tailwind.config.ts                 # 深空主题色彩 tokens
├── tsconfig.json
├── package.json
└── postcss.config.mjs
```

### 根目录

```
docker-compose.dev.yml                 # 新 PostgreSQL + Redis
docker/
├── postgres/
│   └── init.sql                       # 创建 databases + schemas
```

---

## Task 1: Docker Compose 基础设施

**Files:**
- Create: `docker-compose.dev.yml`
- Create: `docker/postgres/init.sql`
- Rename: `docker-compose.yml` → `docker-compose.legacy.yml`

- [ ] **Step 1: 备份旧 docker-compose**

```bash
mv docker-compose.yml docker-compose.legacy.yml
```

- [ ] **Step 2: 创建 PostgreSQL 初始化脚本**

创建 `docker/postgres/init.sql`：
- 创建 `forge_main` 数据库
- 创建 `auth`, `engine`, `specs`, `pipeline`, `billing` 五个 Schema
- 创建 `forge_temporal` 数据库（Temporal 预留）

```sql
-- Create databases
CREATE DATABASE forge_main;
CREATE DATABASE forge_temporal;

-- Connect to forge_main and create schemas
\c forge_main;
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS engine;
CREATE SCHEMA IF NOT EXISTS specs;
CREATE SCHEMA IF NOT EXISTS pipeline;
CREATE SCHEMA IF NOT EXISTS billing;
```

- [ ] **Step 3: 创建 docker-compose.dev.yml**

仅 PostgreSQL 16 + Redis 7，轻量启动：

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: forge-postgres
    environment:
      POSTGRES_USER: forge
      POSTGRES_PASSWORD: forge_dev_2026
      POSTGRES_DB: forge_main
    ports:
      - "5432:5432"
    volumes:
      - forge-pg-data:/var/lib/postgresql/data
      - ./docker/postgres/init.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U forge"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: forge-redis
    ports:
      - "6379:6379"
    command: redis-server --requirepass forge_redis_2026

volumes:
  forge-pg-data:
```

- [ ] **Step 4: 启动并验证基础设施**

```bash
docker compose -f docker-compose.dev.yml up -d
```

**验证**：
```bash
# PostgreSQL 连通
docker exec forge-postgres psql -U forge -d forge_main -c "SELECT schema_name FROM information_schema.schemata WHERE schema_name IN ('auth','engine','specs','pipeline','billing');"
# 预期: 5 行结果

# Redis 连通
docker exec forge-redis redis-cli -a forge_redis_2026 ping
# 预期: PONG
```

- [ ] **Step 5: Commit**

```bash
git add docker-compose.dev.yml docker/postgres/init.sql
git mv docker-compose.yml docker-compose.legacy.yml 2>/dev/null || true
git commit -m "infra: add PostgreSQL + Redis dev environment, archive legacy docker-compose"
```

---

## Task 2: Go API Server 骨架

**Files:**
- Create: `forge-core/cmd/forge-core/main.go`
- Create: `forge-core/internal/config/config.go`
- Create: `forge-core/internal/pkg/response/response.go`
- Create: `forge-core/internal/pkg/database/postgres.go`
- Create: `forge-core/internal/pkg/redis/redis.go`
- Create: `forge-core/internal/middleware/cors.go`
- Create: `forge-core/internal/router/router.go`
- Create: `forge-core/go.mod`

- [ ] **Step 1: 初始化 Go module**

```bash
mkdir -p forge-core/cmd/forge-core
cd forge-core && go mod init github.com/shulex/forge/forge-core
```

- [ ] **Step 2: 创建 config.go — 环境变量配置加载**

`forge-core/internal/config/config.go`：
- 从环境变量读取配置，带合理默认值
- 字段：ServerPort, DatabaseURL, RedisAddr, RedisPassword, JWTSecret, JWTExpireHours

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
}

func Load() *Config {
    return &Config{
        ServerPort:     getEnv("SERVER_PORT", "8080"),
        DatabaseURL:    getEnv("DATABASE_URL", "postgres://forge:forge_dev_2026@localhost:5432/forge_main?sslmode=disable"),
        RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
        RedisPassword:  getEnv("REDIS_PASSWORD", "forge_redis_2026"),
        JWTSecret:      getEnv("JWT_SECRET", "forge-dev-secret-key-change-in-production"),
        JWTExpireHours: 8,
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

- [ ] **Step 3: 创建 response.go — Result[T] 统一响应**

`forge-core/internal/pkg/response/response.go`：

```go
package response

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type Result struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, Result{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus int, message string) {
    c.JSON(httpStatus, Result{Code: -1, Message: message})
}
```

- [ ] **Step 4: 创建 postgres.go — 数据库连接**

`forge-core/internal/pkg/database/postgres.go`：
- 使用 `pgxpool`（生产级连接池）
- 使用 `pgxpool`（生产级连接池）

```go
package database

import (
    "context"
    "fmt"
    "log/slog"
    "github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("parse database URL: %w", err)
    }
    config.MaxConns = 20
    config.MinConns = 2

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("create pool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }

    slog.Info("database connected", "host", config.ConnConfig.Host)
    return pool, nil
}
```

- [ ] **Step 5: 创建 redis.go — Redis 连接**

`forge-core/internal/pkg/redis/redis.go`：

```go
package redis

import (
    "context"
    "fmt"
    "log/slog"
    goredis "github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, addr, password string) (*goredis.Client, error) {
    client := goredis.NewClient(&goredis.Options{
        Addr:     addr,
        Password: password,
        DB:       0,
    })

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("ping redis: %w", err)
    }

    slog.Info("redis connected", "addr", addr)
    return client, nil
}
```

- [ ] **Step 6: 创建 cors.go — CORS 中间件**

`forge-core/internal/middleware/cors.go`：

注意：前端通过 Next.js rewrites 代理 `/api/*` 到 `:8080`，正常使用时不需要 CORS。
此中间件作为 fallback，用于直接用 curl/Postman 调试 API 或前端直连后端的场景。

```go
package middleware

import (
    "github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
        c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
        c.Header("Access-Control-Allow-Credentials", "true")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
```

- [ ] **Step 7: 创建 router.go — 路由骨架**

`forge-core/internal/router/router.go`：

```go
package router

import (
    "github.com/gin-gonic/gin"
    "github.com/shulex/forge/forge-core/internal/middleware"
)

func Setup() *gin.Engine {
    gin.SetMode(gin.ReleaseMode)
    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(middleware.CORS())

    // Health check
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    return r
}
```

- [ ] **Step 8: 创建 main.go — 启动入口**

`forge-core/cmd/forge-core/main.go`：

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/shulex/forge/forge-core/internal/config"
    "github.com/shulex/forge/forge-core/internal/pkg/database"
    forgeRedis "github.com/shulex/forge/forge-core/internal/pkg/redis"
    "github.com/shulex/forge/forge-core/internal/router"
)

func main() {
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

    cfg := config.Load()
    ctx := context.Background()

    db, err := database.NewPool(ctx, cfg.DatabaseURL)
    if err != nil {
        slog.Error("failed to connect database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    rdb, err := forgeRedis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword)
    if err != nil {
        slog.Error("failed to connect redis", "error", err)
        os.Exit(1)
    }
    defer rdb.Close()

    r := router.Setup()

    slog.Info("forge-core starting", "port", cfg.ServerPort)
    if err := r.Run(":" + cfg.ServerPort); err != nil {
        slog.Error("server failed", "error", err)
        os.Exit(1)
    }
}
```

- [ ] **Step 9: 安装依赖并验证编译**

```bash
cd forge-core
go get github.com/gin-gonic/gin
go get github.com/jackc/pgx/v5
go get github.com/redis/go-redis/v9
go mod tidy
go build ./cmd/forge-core
```

**验证**（需要 Docker 基础设施运行中）：
```bash
./forge-core.exe  # Windows
# 预期输出: {"level":"INFO","msg":"database connected",...}
#           {"level":"INFO","msg":"redis connected",...}
#           {"level":"INFO","msg":"forge-core starting","port":"8080"}

# 另一个终端:
curl http://localhost:8080/health
# 预期: {"status":"ok"}
```

- [ ] **Step 10: Commit**

```bash
git add forge-core/
git commit -m "feat: scaffold forge-core Go API server with Gin, PostgreSQL, Redis"
```

---

## Task 3: 数据库迁移 + Auth Schema

**Files:**
- Create: `forge-core/migrations/001_init_auth.sql`
- Modify: `forge-core/cmd/forge-core/main.go` (添加迁移执行)
- Create: `forge-core/internal/pkg/database/migrate.go`

- [ ] **Step 1: 创建 auth schema 迁移脚本**

`forge-core/migrations/001_init_auth.sql`：

从 technical-design.md 中的 auth schema DDL，取本切片需要的最小子集：

```sql
-- Tenants
CREATE TABLE IF NOT EXISTS auth.tenants (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(50) NOT NULL UNIQUE,
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    plan          VARCHAR(20) NOT NULL DEFAULT 'FREE',
    config        JSONB NOT NULL DEFAULT '{}',
    token_budget  BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE IF NOT EXISTS auth.users (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    username      VARCHAR(100) NOT NULL,
    email         VARCHAR(200),
    password_hash VARCHAR(255),
    display_name  VARCHAR(100),
    avatar_url    TEXT,
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, username)
);

-- Roles
CREATE TABLE IF NOT EXISTS auth.roles (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(50) NOT NULL,
    scope         VARCHAR(20) NOT NULL DEFAULT 'PLATFORM',
    description   TEXT,
    is_system     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

-- User-Role mapping
CREATE TABLE IF NOT EXISTS auth.user_roles (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    role_id       BIGINT NOT NULL REFERENCES auth.roles(id),
    scope         VARCHAR(20) NOT NULL DEFAULT 'PLATFORM',
    scope_id      BIGINT NOT NULL DEFAULT 0,  -- 0 = global/platform scope
    granted_by    BIGINT REFERENCES auth.users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, role_id, scope, scope_id)
);

-- Active tokens (for token blacklist/management)
CREATE TABLE IF NOT EXISTS auth.active_tokens (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    token_jti     VARCHAR(100) NOT NULL UNIQUE,
    token_type    VARCHAR(20) NOT NULL DEFAULT 'SESSION',
    device_info   VARCHAR(200),
    ip_address    INET,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed: default tenant
INSERT INTO auth.tenants (name, code) VALUES ('Default', 'default')
ON CONFLICT (code) DO NOTHING;

-- Seed: admin user (password: admin123)
-- IMPORTANT: The bcrypt hash below must be generated at implementation time.
-- Use this Go snippet to generate:
--   go run -mod=mod golang.org/x/crypto/bcrypt -e 'admin123'
-- Or generate programmatically in a seed script.
-- The placeholder hash is for "admin123" — regenerate to confirm correctness.
INSERT INTO auth.users (tenant_id, username, password_hash, display_name, status)
VALUES (
    (SELECT id FROM auth.tenants WHERE code = 'default'),
    'admin',
    '<GENERATE_BCRYPT_HASH_OF_admin123_AT_IMPLEMENTATION_TIME>',
    'Administrator',
    'ACTIVE'
) ON CONFLICT (tenant_id, username) DO NOTHING;

-- Seed: system roles
INSERT INTO auth.roles (tenant_id, name, code, scope, is_system) VALUES
    ((SELECT id FROM auth.tenants WHERE code = 'default'), '平台管理员', 'PLATFORM_ADMIN', 'PLATFORM', TRUE),
    ((SELECT id FROM auth.tenants WHERE code = 'default'), '技术管理者', 'TECH_LEAD', 'PROJECT', TRUE),
    ((SELECT id FROM auth.tenants WHERE code = 'default'), '产品经理', 'PM', 'PROJECT', TRUE)
ON CONFLICT (tenant_id, code) DO NOTHING;

-- Seed: assign admin role
INSERT INTO auth.user_roles (user_id, role_id, scope)
SELECT u.id, r.id, 'PLATFORM'
FROM auth.users u, auth.roles r
WHERE u.username = 'admin' AND r.code = 'PLATFORM_ADMIN'
ON CONFLICT (user_id, role_id, scope, scope_id) DO NOTHING;
```

**实现时必须**：用 Go 生成 admin123 的正确 bcrypt hash 替换占位符。在 Task 4 的 service 代码中添加一个 seed 辅助函数，或在实现迁移时先用 Go 生成 hash 再写入 SQL。

- [ ] **Step 2: 创建简易迁移执行器**

`forge-core/internal/pkg/database/migrate.go`：

```go
package database

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "sort"

    "github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
    // Create migrations tracking table
    _, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS public.schema_migrations (
            version VARCHAR(255) PRIMARY KEY,
            applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    `)
    if err != nil {
        return fmt.Errorf("create migrations table: %w", err)
    }

    files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
    if err != nil {
        return fmt.Errorf("glob migrations: %w", err)
    }
    sort.Strings(files)

    for _, file := range files {
        version := filepath.Base(file)

        var exists bool
        err := pool.QueryRow(ctx,
            "SELECT EXISTS(SELECT 1 FROM public.schema_migrations WHERE version = $1)",
            version,
        ).Scan(&exists)
        if err != nil {
            return fmt.Errorf("check migration %s: %w", version, err)
        }
        if exists {
            continue
        }

        sql, err := os.ReadFile(file)
        if err != nil {
            return fmt.Errorf("read migration %s: %w", version, err)
        }

        // Run migration + version recording in a transaction
        tx, err := pool.Begin(ctx)
        if err != nil {
            return fmt.Errorf("begin transaction for %s: %w", version, err)
        }

        if _, err := tx.Exec(ctx, string(sql)); err != nil {
            tx.Rollback(ctx)
            return fmt.Errorf("execute migration %s: %w", version, err)
        }

        if _, err := tx.Exec(ctx,
            "INSERT INTO public.schema_migrations (version) VALUES ($1)", version,
        ); err != nil {
            tx.Rollback(ctx)
            return fmt.Errorf("record migration %s: %w", version, err)
        }

        if err := tx.Commit(ctx); err != nil {
            return fmt.Errorf("commit migration %s: %w", version, err)
        }

        slog.Info("migration applied", "version", version)
    }

    return nil
}
```

- [ ] **Step 3: 在 main.go 中添加迁移调用**

在 database 连接成功后、router 启动前，添加：

```go
if err := database.RunMigrations(ctx, db, "migrations"); err != nil {
    slog.Error("failed to run migrations", "error", err)
    os.Exit(1)
}
```

- [ ] **Step 4: 验证迁移执行**

```bash
# 先清理数据库（如果有旧数据）
docker compose -f docker-compose.dev.yml down -v
docker compose -f docker-compose.dev.yml up -d

# 等待 PostgreSQL 就绪后启动
cd forge-core && go run ./cmd/forge-core

# 验证表已创建
docker exec forge-postgres psql -U forge -d forge_main -c "SELECT username, display_name FROM auth.users;"
# 预期: admin | Administrator

docker exec forge-postgres psql -U forge -d forge_main -c "SELECT code, name FROM auth.roles;"
# 预期: PLATFORM_ADMIN, TECH_LEAD, PM
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/migrations/ forge-core/internal/pkg/database/migrate.go
git commit -m "feat: add database migrations with auth schema and seed data"
```

---

## Task 4: Auth 模块 — Login / Logout / Me API

**Files:**
- Create: `forge-core/internal/module/auth/model.go`
- Create: `forge-core/internal/module/auth/repository.go`
- Create: `forge-core/internal/module/auth/service.go`
- Create: `forge-core/internal/module/auth/handler.go`
- Create: `forge-core/internal/middleware/auth.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

- [ ] **Step 1: 创建 model.go — 数据模型 + DTO**

`forge-core/internal/module/auth/model.go`：

```go
package auth

import "time"

// DB model
type User struct {
    ID           int64      `json:"id"`
    TenantID     int64      `json:"tenant_id"`
    Username     string     `json:"username"`
    Email        *string    `json:"email,omitempty"`
    PasswordHash string     `json:"-"`
    DisplayName  *string    `json:"display_name,omitempty"`
    AvatarURL    *string    `json:"avatar_url,omitempty"`
    Status       string     `json:"status"`
    LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
    CreatedAt    time.Time  `json:"created_at"`
}

type Role struct {
    ID   int64  `json:"id"`
    Code string `json:"code"`
    Name string `json:"name"`
}

// Request DTOs
type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

// Response DTOs
type LoginResponse struct {
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
    User      UserInfo  `json:"user"`
}

type UserInfo struct {
    ID          int64   `json:"id"`
    TenantID    int64   `json:"tenant_id"`
    Username    string  `json:"username"`
    DisplayName string  `json:"display_name"`
    AvatarURL   string  `json:"avatar_url"`
    Roles       []Role  `json:"roles"`
}
```

- [ ] **Step 2: 创建 repository.go — 数据库操作**

`forge-core/internal/module/auth/repository.go`：

```go
package auth

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
    db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
    return &Repository{db: db}
}

func (r *Repository) FindUserByUsername(ctx context.Context, tenantID int64, username string) (*User, error) {
    user := &User{}
    err := r.db.QueryRow(ctx,
        `SELECT id, tenant_id, username, email, password_hash, display_name, avatar_url, status, last_login_at, created_at
         FROM auth.users WHERE tenant_id = $1 AND username = $2`,
        tenantID, username,
    ).Scan(&user.ID, &user.TenantID, &user.Username, &user.Email, &user.PasswordHash,
        &user.DisplayName, &user.AvatarURL, &user.Status, &user.LastLoginAt, &user.CreatedAt)
    if err != nil {
        return nil, fmt.Errorf("find user: %w", err)
    }
    return user, nil
}

func (r *Repository) FindUserByID(ctx context.Context, userID int64) (*User, error) {
    user := &User{}
    err := r.db.QueryRow(ctx,
        `SELECT id, tenant_id, username, email, password_hash, display_name, avatar_url, status, last_login_at, created_at
         FROM auth.users WHERE id = $1`,
        userID,
    ).Scan(&user.ID, &user.TenantID, &user.Username, &user.Email, &user.PasswordHash,
        &user.DisplayName, &user.AvatarURL, &user.Status, &user.LastLoginAt, &user.CreatedAt)
    if err != nil {
        return nil, fmt.Errorf("find user by id: %w", err)
    }
    return user, nil
}

func (r *Repository) GetUserRoles(ctx context.Context, userID int64) ([]Role, error) {
    rows, err := r.db.Query(ctx,
        `SELECT r.id, r.code, r.name
         FROM auth.roles r
         JOIN auth.user_roles ur ON r.id = ur.role_id
         WHERE ur.user_id = $1`,
        userID,
    )
    if err != nil {
        return nil, fmt.Errorf("get user roles: %w", err)
    }
    defer rows.Close()

    var roles []Role
    for rows.Next() {
        var role Role
        if err := rows.Scan(&role.ID, &role.Code, &role.Name); err != nil {
            return nil, fmt.Errorf("scan role: %w", err)
        }
        roles = append(roles, role)
    }
    return roles, nil
}

func (r *Repository) UpdateLastLogin(ctx context.Context, userID int64) error {
    _, err := r.db.Exec(ctx, "UPDATE auth.users SET last_login_at = NOW() WHERE id = $1", userID)
    return err
}

func (r *Repository) SaveActiveToken(ctx context.Context, tenantID, userID int64, jti, tokenType, deviceInfo string, ipAddr string, expiresAt interface{}) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO auth.active_tokens (tenant_id, user_id, token_jti, token_type, device_info, ip_address, expires_at)
         VALUES ($1, $2, $3, $4, $5, $6::inet, $7)`,
        tenantID, userID, jti, tokenType, deviceInfo, ipAddr, expiresAt,
    )
    return err
}

func (r *Repository) DeleteActiveToken(ctx context.Context, jti string) error {
    _, err := r.db.Exec(ctx, "DELETE FROM auth.active_tokens WHERE token_jti = $1", jti)
    return err
}

func (r *Repository) IsTokenActive(ctx context.Context, jti string) (bool, error) {
    var exists bool
    err := r.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM auth.active_tokens WHERE token_jti = $1 AND expires_at > NOW())",
        jti,
    ).Scan(&exists)
    return exists, err
}
```

- [ ] **Step 3: 创建 service.go — 业务逻辑**

`forge-core/internal/module/auth/service.go`：

```go
package auth

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

type Service struct {
    repo      *Repository
    jwtSecret []byte
    jwtExpire time.Duration
}

func NewService(repo *Repository, jwtSecret string, jwtExpireHours int) *Service {
    return &Service{
        repo:      repo,
        jwtSecret: []byte(jwtSecret),
        jwtExpire: time.Duration(jwtExpireHours) * time.Hour,
    }
}

type Claims struct {
    jwt.RegisteredClaims
    UserID   int64  `json:"uid"`
    TenantID int64  `json:"tid"`
    Username string `json:"usr"`
}

func (s *Service) Login(ctx context.Context, req *LoginRequest, ipAddr string) (*LoginResponse, error) {
    // Phase 1: tenant_id = 1 (default tenant)
    const defaultTenantID int64 = 1

    user, err := s.repo.FindUserByUsername(ctx, defaultTenantID, req.Username)
    if err != nil {
        return nil, errors.New("用户名或密码错误")
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        return nil, errors.New("用户名或密码错误")
    }

    if user.Status != "ACTIVE" {
        return nil, errors.New("用户名或密码错误")  // 不泄露账号状态信息
    }

    // Generate JWT
    jti := uuid.New().String()
    expiresAt := time.Now().Add(s.jwtExpire)

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            ID:        jti,
            ExpiresAt: jwt.NewNumericDate(expiresAt),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
        UserID:   user.ID,
        TenantID: user.TenantID,
        Username: user.Username,
    })

    tokenString, err := token.SignedString(s.jwtSecret)
    if err != nil {
        return nil, fmt.Errorf("sign token: %w", err)
    }

    // Save active token
    _ = s.repo.SaveActiveToken(ctx, user.TenantID, user.ID, jti, "SESSION", "", ipAddr, expiresAt)
    _ = s.repo.UpdateLastLogin(ctx, user.ID)

    roles, _ := s.repo.GetUserRoles(ctx, user.ID)

    displayName := ""
    if user.DisplayName != nil {
        displayName = *user.DisplayName
    }
    avatarURL := ""
    if user.AvatarURL != nil {
        avatarURL = *user.AvatarURL
    }

    return &LoginResponse{
        Token:     tokenString,
        ExpiresAt: expiresAt,
        User: UserInfo{
            ID:          user.ID,
            TenantID:    user.TenantID,
            Username:    user.Username,
            DisplayName: displayName,
            AvatarURL:   avatarURL,
            Roles:       roles,
        },
    }, nil
}

func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        return s.jwtSecret, nil
    })
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, errors.New("invalid token")
    }

    // Check token is still active (not logged out)
    active, err := s.repo.IsTokenActive(ctx, claims.ID)
    if err != nil || !active {
        return nil, errors.New("token revoked")
    }

    return claims, nil
}

func (s *Service) Logout(ctx context.Context, jti string) error {
    return s.repo.DeleteActiveToken(ctx, jti)
}

func (s *Service) GetCurrentUser(ctx context.Context, userID int64) (*UserInfo, error) {
    user, err := s.repo.FindUserByID(ctx, userID)
    if err != nil {
        return nil, err
    }
    roles, _ := s.repo.GetUserRoles(ctx, userID)

    displayName := ""
    if user.DisplayName != nil {
        displayName = *user.DisplayName
    }
    avatarURL := ""
    if user.AvatarURL != nil {
        avatarURL = *user.AvatarURL
    }

    return &UserInfo{
        ID:          user.ID,
        TenantID:    user.TenantID,
        Username:    user.Username,
        DisplayName: displayName,
        AvatarURL:   avatarURL,
        Roles:       roles,
    }, nil
}
```

- [ ] **Step 4: 创建 handler.go — HTTP handlers**

`forge-core/internal/module/auth/handler.go`：

```go
package auth

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

// POST /api/auth/login
func (h *Handler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, http.StatusBadRequest, "请输入用户名和密码")
        return
    }

    resp, err := h.service.Login(c.Request.Context(), &req, c.ClientIP())
    if err != nil {
        response.Fail(c, http.StatusUnauthorized, err.Error())
        return
    }

    response.OK(c, resp)
}

// POST /api/auth/logout
func (h *Handler) Logout(c *gin.Context) {
    jti, exists := c.Get("token_jti")
    if !exists {
        response.Fail(c, http.StatusUnauthorized, "未登录")
        return
    }

    if err := h.service.Logout(c.Request.Context(), jti.(string)); err != nil {
        response.Fail(c, http.StatusInternalServerError, "登出失败")
        return
    }

    response.OK(c, nil)
}

// GET /api/auth/me
func (h *Handler) Me(c *gin.Context) {
    userID, exists := c.Get("user_id")
    if !exists {
        response.Fail(c, http.StatusUnauthorized, "未登录")
        return
    }

    user, err := h.service.GetCurrentUser(c.Request.Context(), userID.(int64))
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "获取用户信息失败")
        return
    }

    response.OK(c, user)
}
```

- [ ] **Step 5: 创建 auth middleware**

`forge-core/internal/middleware/auth.go`：

```go
package middleware

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/shulex/forge/forge-core/internal/module/auth"
    "github.com/shulex/forge/forge-core/internal/pkg/response"
)

func JWTAuth(authService *auth.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        header := c.GetHeader("Authorization")
        if header == "" || !strings.HasPrefix(header, "Bearer ") {
            response.Fail(c, http.StatusUnauthorized, "请先登录")
            c.Abort()
            return
        }

        tokenString := strings.TrimPrefix(header, "Bearer ")
        claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
        if err != nil {
            response.Fail(c, http.StatusUnauthorized, "登录已过期，请重新登录")
            c.Abort()
            return
        }

        c.Set("user_id", claims.UserID)
        c.Set("tenant_id", claims.TenantID)
        c.Set("username", claims.Username)
        c.Set("token_jti", claims.ID)
        c.Next()
    }
}
```

- [ ] **Step 6: 更新 router.go 注册 auth 路由**

修改 `forge-core/internal/router/router.go`，接受 auth handler 和 service 参数：

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
        }
    }

    return r
}
```

- [ ] **Step 7: 更新 main.go 组装依赖**

修改 `forge-core/cmd/forge-core/main.go`，在 router.Setup 前组装 auth 模块：

```go
// ... (db, rdb 连接代码不变) ...

// Migrations
if err := database.RunMigrations(ctx, db, "migrations"); err != nil {
    slog.Error("failed to run migrations", "error", err)
    os.Exit(1)
}

// Auth module
authRepo := auth.NewRepository(db)
authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours)
authHandler := auth.NewHandler(authService)

// Router
r := router.Setup(&router.Deps{
    AuthHandler: authHandler,
    AuthService: authService,
})
```

- [ ] **Step 8: 安装新依赖并验证**

```bash
cd forge-core
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
go get github.com/google/uuid
go mod tidy
go build ./cmd/forge-core
```

- [ ] **Step 9: 启动并测试 API**

```bash
cd forge-core && go run ./cmd/forge-core
```

**测试登录**：
```bash
# 登录成功
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
# 预期: {"code":0,"message":"ok","data":{"token":"eyJ...","expires_at":"...","user":{...}}}

# 密码错误
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"wrong"}'
# 预期: {"code":-1,"message":"用户名或密码错误"}

# 获取当前用户 (用上面返回的 token)
curl http://localhost:8080/api/auth/me \
  -H "Authorization: Bearer <token>"
# 预期: {"code":0,"message":"ok","data":{"id":1,"username":"admin","roles":[...]}}

# 未登录访问
curl http://localhost:8080/api/auth/me
# 预期: {"code":-1,"message":"请先登录"}
```

- [ ] **Step 10: Commit**

```bash
git add forge-core/internal/module/auth/ forge-core/internal/middleware/auth.go forge-core/internal/router/
git commit -m "feat: implement auth module with login/logout/me APIs and JWT middleware"
```

---

## Task 5: Next.js 前端骨架

**Files:**
- Delete all files in: `forge-portal/` (旧 Vue 3 代码)
- Create: `forge-portal/package.json`, `forge-portal/next.config.ts`, `forge-portal/tsconfig.json`
- Create: `forge-portal/tailwind.config.ts`, `forge-portal/postcss.config.mjs`
- Create: `forge-portal/app/layout.tsx`, `forge-portal/app/globals.css`, `forge-portal/app/page.tsx`

- [ ] **Step 1: 清理旧前端代码**

```bash
cd forge-portal
# 删除旧 Vue 3 项目文件（保留 .gitignore）
rm -rf node_modules src dist public .vscode
rm -f package.json package-lock.json tsconfig.json tsconfig.app.json tsconfig.node.json
rm -f vite.config.ts index.html Dockerfile nginx.conf README.md
```

- [ ] **Step 2: 初始化 Next.js 项目**

先确保目录干净（只留 .gitignore），然后初始化：

```bash
cd forge-portal
npx create-next-app@latest . --typescript --tailwind --eslint --app --no-src-dir --import-alias "@/*" --yes
```

`--yes` 跳过所有交互式提示（Windows 下自动化执行时很重要）。如果 `--yes` 不生效，手动回答以下选项：
- TypeScript: Yes
- ESLint: Yes
- Tailwind CSS: Yes
- src/ directory: No
- App Router: Yes
- Turbopack: Yes (if asked)
- import alias: @/*

- [ ] **Step 3: 安装 shadcn/ui 和项目依赖**

```bash
cd forge-portal
npx shadcn@latest init -d
npm install lucide-react zustand
```

- [ ] **Step 4: 配置 next.config.ts — API 代理**

`forge-portal/next.config.ts`：

```typescript
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
    ];
  },
};

export default nextConfig;
```

- [ ] **Step 5: 配置深空主题 CSS 变量**

替换 `forge-portal/app/globals.css`：

```css
@import "tailwindcss";

:root {
  /* 深空指挥中心 — 色彩体系 */
  --background: #050510;
  --surface-1: #0F0F1A;
  --surface-2: #1A1A2E;
  --border: #2A2A3E;
  --border-glow: rgba(139, 92, 246, 0.2);

  --primary: #8B5CF6;
  --primary-hover: #7C3AED;
  --primary-glow: rgba(139, 92, 246, 0.3);
  --accent: #06B6D4;
  --accent-glow: rgba(6, 182, 212, 0.3);

  --success: #10B981;
  --warning: #F59E0B;
  --error: #EF4444;
  --info: #3B82F6;

  --text-primary: #F1F1F3;
  --text-secondary: #8888A0;
  --text-muted: #555570;

  --input-bg: #0A0A15;
}

body {
  background: var(--background);
  color: var(--text-primary);
  font-family: "Geist Sans", "Inter", -apple-system, "PingFang SC", "Microsoft YaHei", system-ui, sans-serif;
}

/* Aurora background utility */
.aurora-bg {
  background:
    radial-gradient(ellipse at 20% 50%, rgba(139, 92, 246, 0.08), transparent 50%),
    radial-gradient(ellipse at 80% 20%, rgba(6, 182, 212, 0.06), transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(59, 130, 246, 0.05), transparent 50%),
    var(--background);
}
```

- [ ] **Step 6: 创建 Root Layout**

`forge-portal/app/layout.tsx`：

```tsx
import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Forge — Harness Engineering Platform",
  description: "AI-driven Harness Engineering Platform",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className="dark">
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased`}>
        {children}
      </body>
    </html>
  );
}
```

- [ ] **Step 7: 创建根页面（重定向到登录）**

`forge-portal/app/page.tsx`：

```tsx
import { redirect } from "next/navigation";

export default function Home() {
  redirect("/login");
}
```

- [ ] **Step 8: 验证 Next.js 启动**

```bash
cd forge-portal && npm run dev
```

**验证**：
- 浏览器打开 `http://localhost:3000`
- 应该重定向到 `/login` 并显示 Next.js 默认 404（因为 login page 还没创建）
- 页面背景应该是深色 (#050510)

- [ ] **Step 9: 更新 .gitignore**

更新 `forge-portal/.gitignore` 为 Next.js 版本：

```
node_modules/
.next/
out/
.env*.local
```

- [ ] **Step 10: Commit**

```bash
git add forge-portal/
git commit -m "feat: replace Vue 3 frontend with Next.js 15 + Tailwind + shadcn/ui skeleton"
```

---

## Task 6: 前端 Auth 基础设施

**Files:**
- Create: `forge-portal/lib/api.ts`
- Create: `forge-portal/lib/auth.tsx`

- [ ] **Step 1: 创建 API client**

`forge-portal/lib/api.ts`：

```typescript
const BASE_URL = "/api";

interface ApiResult<T> {
  code: number;
  message: string;
  data: T;
}

class ApiError extends Error {
  constructor(public code: number, message: string) {
    super(message);
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = typeof window !== "undefined"
    ? localStorage.getItem("forge_token")
    : null;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    if (typeof window !== "undefined") {
      localStorage.removeItem("forge_token");
      localStorage.removeItem("forge_user");
      window.location.href = "/login";
    }
    throw new ApiError(401, "登录已过期，请重新登录");
  }

  const json: ApiResult<T> = await res.json();

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message);
  }

  return json.data;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};

export { ApiError };
```

- [ ] **Step 2: 创建 Auth Context + Hook**

`forge-portal/lib/auth.tsx`：

```tsx
"use client";

import { createContext, useContext, useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "./api";

interface UserInfo {
  id: number;
  tenant_id: number;
  username: string;
  display_name: string;
  avatar_url: string;
  roles: { id: number; code: string; name: string }[];
}

interface LoginResponse {
  token: string;
  expires_at: string;
  user: UserInfo;
}

interface AuthContextType {
  user: UserInfo | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("forge_token");
    const savedUser = localStorage.getItem("forge_user");
    if (token && savedUser) {
      try {
        setUser(JSON.parse(savedUser));
      } catch {
        localStorage.removeItem("forge_token");
        localStorage.removeItem("forge_user");
      }
    }
    setLoading(false);
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const data = await api.post<LoginResponse>("/auth/login", { username, password });
    localStorage.setItem("forge_token", data.token);
    localStorage.setItem("forge_user", JSON.stringify(data.user));
    setUser(data.user);
    router.push("/projects");
  }, [router]);

  const logout = useCallback(async () => {
    try {
      await api.post("/auth/logout");
    } catch {
      // ignore errors during logout
    }
    localStorage.removeItem("forge_token");
    localStorage.removeItem("forge_user");
    setUser(null);
    router.push("/login");
  }, [router]);

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
```

- [ ] **Step 3: Commit**

```bash
git add forge-portal/lib/
git commit -m "feat: add API client and auth context for frontend"
```

---

## Task 7: 登录页

**Files:**
- Create: `forge-portal/components/aurora-background.tsx`
- Create: `forge-portal/components/forge-logo.tsx`
- Create: `forge-portal/app/login/page.tsx`
- Modify: `forge-portal/app/layout.tsx` (wrap with AuthProvider)

- [ ] **Step 1: 安装 shadcn/ui 组件**

```bash
cd forge-portal
npx shadcn@latest add button input label card
```

- [ ] **Step 2: 创建 Aurora 背景组件**

`forge-portal/components/aurora-background.tsx`：

```tsx
"use client";

export function AuroraBackground() {
  return (
    <div className="fixed inset-0 -z-10 aurora-bg">
      <div
        className="absolute inset-0 animate-aurora-1"
        style={{
          background: "radial-gradient(ellipse at 20% 50%, rgba(139, 92, 246, 0.12), transparent 50%)",
        }}
      />
      <div
        className="absolute inset-0 animate-aurora-2"
        style={{
          background: "radial-gradient(ellipse at 80% 20%, rgba(6, 182, 212, 0.08), transparent 50%)",
        }}
      />
      <div
        className="absolute inset-0 animate-aurora-3"
        style={{
          background: "radial-gradient(ellipse at 50% 80%, rgba(59, 130, 246, 0.06), transparent 50%)",
        }}
      />
    </div>
  );
}
```

在 `globals.css` 末尾追加动画 keyframes：

```css
@keyframes aurora-drift-1 {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(30px, -20px) scale(1.05); }
}
@keyframes aurora-drift-2 {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(-20px, 30px) scale(1.03); }
}
@keyframes aurora-drift-3 {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(15px, 15px) scale(1.04); }
}

.animate-aurora-1 { animation: aurora-drift-1 20s ease-in-out infinite; }
.animate-aurora-2 { animation: aurora-drift-2 25s ease-in-out infinite; }
.animate-aurora-3 { animation: aurora-drift-3 30s ease-in-out infinite; }
```

- [ ] **Step 3: 创建 Forge Logo 组件**

`forge-portal/components/forge-logo.tsx`：

```tsx
export function ForgeLogo({ className }: { className?: string }) {
  return (
    <div className={className}>
      <span className="text-3xl font-semibold tracking-tight bg-gradient-to-r from-[#8B5CF6] to-[#3B82F6] bg-clip-text text-transparent">
        Forge
      </span>
    </div>
  );
}
```

- [ ] **Step 4: 创建登录页**

`forge-portal/app/login/page.tsx`：

```tsx
"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth";
import { AuroraBackground } from "@/components/aurora-background";
import { ForgeLogo } from "@/components/forge-logo";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";

export default function LoginPage() {
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [shake, setShake] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      await login(username, password);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "登录失败，请重试";
      setError(message);
      setShake(true);
      setTimeout(() => setShake(false), 500);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <AuroraBackground />

      <div
        className={`w-[480px] p-8 rounded-2xl border backdrop-blur-xl transition-transform ${
          shake ? "animate-shake" : ""
        }`}
        style={{
          background: "rgba(15, 15, 26, 0.8)",
          borderColor: "rgba(255, 255, 255, 0.08)",
          borderTopColor: "rgba(255, 255, 255, 0.12)",
        }}
      >
        <div className="text-center mb-8">
          <ForgeLogo className="mb-2" />
          <p className="text-sm" style={{ color: "var(--text-secondary)" }}>
            Harness Engineering Platform
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="username" style={{ color: "var(--text-secondary)" }}>
              用户名
            </Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="输入用户名"
              autoComplete="username"
              className="h-11 border transition-all duration-150 focus:shadow-[0_0_0_2px_rgba(139,92,246,0.2)]"
              style={{
                background: "var(--input-bg)",
                borderColor: error ? "var(--error)" : "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="password" style={{ color: "var(--text-secondary)" }}>
              密码
            </Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              autoComplete="current-password"
              className="h-11 border transition-all duration-150 focus:shadow-[0_0_0_2px_rgba(139,92,246,0.2)]"
              style={{
                background: "var(--input-bg)",
                borderColor: error ? "var(--error)" : "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          {error && (
            <p className="text-sm" style={{ color: "var(--error)" }}>
              {error}
            </p>
          )}

          <Button
            type="submit"
            disabled={loading || !username || !password}
            className="w-full h-11 text-base font-medium rounded-lg transition-all active:scale-[0.97]"
            style={{
              background: "var(--primary)",
              boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)",
            }}
          >
            {loading ? "登录中..." : "登录"}
          </Button>
        </form>

        <p
          className="text-center mt-6 text-xs"
          style={{ color: "var(--text-muted)" }}
        >
          Forge v0.1.0
        </p>
      </div>
    </div>
  );
}
```

在 `globals.css` 追加 shake 动画：

```css
@keyframes shake {
  0%, 100% { transform: translateX(0); }
  20%, 60% { transform: translateX(-6px); }
  40%, 80% { transform: translateX(6px); }
}
.animate-shake { animation: shake 0.4s ease-in-out; }
```

- [ ] **Step 5: 更新 layout.tsx 包裹 AuthProvider**

修改 `forge-portal/app/layout.tsx`，在 `<body>` 内包裹 `AuthProvider`：

```tsx
import { AuthProvider } from "@/lib/auth";

// ... (metadata 不变)

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className="dark">
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased`}>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
```

- [ ] **Step 6: 验证登录页**

确保后端和 Docker 运行中，然后：

```bash
cd forge-portal && npm run dev
```

**验证**：
1. 打开 `http://localhost:3000/login`
2. 应看到深色毛玻璃卡片 + Aurora 背景 + Forge 品牌渐变标题
3. 输入 `admin / admin123`，点登录
4. 应跳转到 `/projects`（此时会 404，下一个 Task 创建）
5. 输入错误密码，应看到红色错误提示 + 输入框抖动

- [ ] **Step 7: Commit**

```bash
git add forge-portal/
git commit -m "feat: implement login page with Aurora background and glassmorphism design"
```

---

## Task 8: Dashboard Layout + 空项目大厅

**Files:**
- Create: `forge-portal/app/(dashboard)/layout.tsx`
- Create: `forge-portal/app/(dashboard)/projects/page.tsx`
- Create: `forge-portal/components/sidebar.tsx`
- Create: `forge-portal/components/topbar.tsx`

- [ ] **Step 1: 创建 Sidebar 组件**

`forge-portal/components/sidebar.tsx`：

```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ForgeLogo } from "./forge-logo";
import { FolderOpen } from "lucide-react";

const navItems = [
  { href: "/projects", label: "项目大厅", icon: FolderOpen },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside
      className="w-60 h-screen flex flex-col border-r"
      style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
    >
      <div className="h-14 flex items-center px-5 border-b" style={{ borderColor: "var(--border)" }}>
        <ForgeLogo />
      </div>

      <nav className="flex-1 p-3 space-y-1">
        {navItems.map((item) => {
          const active = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                active
                  ? "text-[var(--text-primary)]"
                  : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              }`}
              style={active ? { background: "rgba(139, 92, 246, 0.1)" } : {}}
            >
              <item.icon size={18} />
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
```

- [ ] **Step 2: 创建 Topbar 组件**

`forge-portal/components/topbar.tsx`：

```tsx
"use client";

import { useAuth } from "@/lib/auth";
import { LogOut, User } from "lucide-react";

export function Topbar() {
  const { user, logout } = useAuth();

  return (
    <header
      className="h-14 flex items-center justify-end px-6 border-b"
      style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2 text-sm" style={{ color: "var(--text-secondary)" }}>
          <User size={16} />
          <span>{user?.display_name || user?.username}</span>
        </div>
        <button
          onClick={logout}
          className="flex items-center gap-1.5 text-sm transition-colors hover:text-[var(--error)]"
          style={{ color: "var(--text-muted)" }}
        >
          <LogOut size={16} />
          登出
        </button>
      </div>
    </header>
  );
}
```

- [ ] **Step 3: 创建 Dashboard Layout**

`forge-portal/app/(dashboard)/layout.tsx`：

```tsx
"use client";

import { useAuth } from "@/lib/auth";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "var(--background)" }}>
        <div className="text-sm" style={{ color: "var(--text-muted)" }}>加载中...</div>
      </div>
    );
  }

  if (!user) return null;

  return (
    <div className="flex h-screen" style={{ background: "var(--background)" }}>
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Topbar />
        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 创建项目大厅空状态页**

`forge-portal/app/(dashboard)/projects/page.tsx`：

```tsx
"use client";

import { FolderOpen, Plus, GitBranch } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function ProjectsPage() {
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight" style={{ color: "var(--text-primary)" }}>
          项目大厅
        </h1>
        <div className="flex gap-3">
          <Button
            variant="outline"
            className="gap-2 border"
            style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
          >
            <GitBranch size={16} />
            接入代码平台
          </Button>
          <Button
            className="gap-2"
            style={{ background: "var(--primary)", boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)" }}
          >
            <Plus size={16} />
            创建新项目
          </Button>
        </div>
      </div>

      {/* Empty state */}
      <div
        className="flex flex-col items-center justify-center py-24 rounded-xl border"
        style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
      >
        <div
          className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
          style={{ background: "rgba(139, 92, 246, 0.1)" }}
        >
          <FolderOpen size={32} style={{ color: "var(--primary)" }} />
        </div>
        <h3 className="text-lg font-medium mb-2" style={{ color: "var(--text-primary)" }}>
          还没有项目
        </h3>
        <p className="text-sm mb-6" style={{ color: "var(--text-secondary)" }}>
          接入代码平台同步已有项目，或创建一个新项目开始
        </p>
        <div className="flex gap-3">
          <Button
            variant="outline"
            className="gap-2 border"
            style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
          >
            <GitBranch size={16} />
            接入代码平台
          </Button>
          <Button
            className="gap-2"
            style={{ background: "var(--primary)" }}
          >
            <Plus size={16} />
            创建新项目
          </Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: 验证完整登录流程**

确保后端和 Docker 运行中：

```bash
# Terminal 1: 后端
cd forge-core && go run ./cmd/forge-core

# Terminal 2: 前端
cd forge-portal && npm run dev
```

**完整验证清单**：

| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 打开 http://localhost:3000 | 重定向到 /login |
| 2 | 看到登录页 | 深色背景 + Aurora 动效 + 毛玻璃卡片 + Forge 渐变标题 |
| 3 | 输入 admin / wrong 并登录 | 红色错误提示 "用户名或密码错误" + 抖动 |
| 4 | 输入 admin / admin123 并登录 | 跳转到 /projects |
| 5 | 看到项目大厅 | 左侧边栏 + 顶栏用户名 + "还没有项目"空状态 |
| 6 | 点击顶栏"登出" | 回到 /login |
| 7 | 直接访问 /projects（未登录） | 自动跳转到 /login |

- [ ] **Step 6: Commit**

```bash
git add forge-portal/
git commit -m "feat: implement dashboard layout, sidebar, topbar and empty projects page"
```

---

## Task 9: 更新 CLAUDE.md 和 .gitignore

**Files:**
- Modify: `CLAUDE.md`
- Modify: `.gitignore`

- [ ] **Step 1: 更新 .gitignore**

追加 Go 和 Next.js 构建产物：

```gitignore
# Go
forge-core/forge-core
forge-core/forge-core.exe
*.exe

# Next.js
forge-portal/.next/
forge-portal/out/
forge-portal/node_modules/

# Environment
.env*.local
```

- [ ] **Step 2: 更新 CLAUDE.md Build Commands 部分**

确保 Build Commands 反映新的 Go + Next.js 命令（应该已经在之前的文档对齐中更新过，这里验证一下）。

- [ ] **Step 3: Commit**

```bash
git add .gitignore CLAUDE.md
git commit -m "chore: update gitignore for Go + Next.js stack"
```

---

## 验收标准

S1 完成后，你应该能够：

1. **一条命令启动基础设施**：`docker compose -f docker-compose.dev.yml up -d`
2. **一条命令启动后端**：`cd forge-core && go run ./cmd/forge-core`
3. **一条命令启动前端**：`cd forge-portal && npm run dev`
4. **浏览器完整流程**：打开 → 登录 → 看到项目大厅 → 登出 → 回到登录页
5. **API 可独立测试**：curl 登录/登出/获取用户信息

---

## 后续切片预告

| 切片 | 内容 | 前置 |
|------|------|------|
| S2 | 项目管理 CRUD + 项目列表/创建页面 | S1 |
| S3 | GitHub OAuth + 仓库同步 + 接入页面 | S1, S2 |
| S4 | Temporal 集成 + 任务基础 CRUD + 任务看板 | S2 |
| S5 | 规范中心 CRUD + 管理页面 | S2 |
| S6 | AI Worker (Python) + 需求对话页 | S4, S5 |
