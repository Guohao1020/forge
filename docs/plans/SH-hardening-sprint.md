# S4→S5 加固 Sprint 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 解决 P0 安全硬伤 + 搭建测试基础设施 + 统一错误处理，让后续 S5-S7 在安全、可测试的基线上推进。

**Architecture:** 横切关注点加固，不引入新功能模块。后端新增 `crypto` 包（token 加密）、`requestid` 中间件、结构化错误码；前端新增 Error Boundary + Toast + HttpOnly Cookie 认证流。

**Tech Stack:** Go crypto/aes, crypto/rand; React Error Boundary; vitest + testing-library; gin middleware

**预计工时:** 1.5-2 天

---

## 文件结构

### 新建文件

| 文件 | 职责 |
|------|------|
| `forge-core/internal/pkg/crypto/crypto.go` | AES-256-GCM 加密/解密工具 |
| `forge-core/internal/pkg/crypto/crypto_test.go` | 加密工具测试 |
| `forge-core/internal/pkg/errcode/errcode.go` | 结构化错误码定义 |
| `forge-core/internal/pkg/errcode/errcode_test.go` | 错误码测试 |
| `forge-core/internal/middleware/requestid.go` | Request ID 中间件 |
| `forge-core/internal/middleware/requestid_test.go` | Request ID 中间件测试 |
| `forge-core/internal/middleware/auth_test.go` | JWT 中间件测试 |
| `forge-portal/components/error-boundary.tsx` | React Error Boundary 组件 |
| `forge-portal/components/ui/toaster-provider.tsx` | Toast 通知封装 |
| `forge-portal/vitest.config.ts` | Vitest 配置 |
| `forge-portal/lib/__tests__/api.test.ts` | API 层测试 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `forge-core/internal/config/config.go` | 添加 `ENCRYPTION_KEY` 配置 + 启动校验 |
| `forge-core/internal/pkg/response/response.go` | 支持结构化错误码 |
| `forge-core/internal/module/auth/service.go` | JWT 算法锁定 + token 加密存储 + crypto/rand state |
| `forge-core/internal/module/auth/handler.go` | OAuth state 用 crypto/rand |
| `forge-core/internal/module/auth/repository.go` | GitHub token 加密读写 |
| `forge-core/internal/module/auth/model.go` | 输入校验增强 |
| `forge-core/internal/middleware/cors.go` | 添加安全响应头 |
| `forge-core/internal/middleware/auth.go` | JWT 算法验证 |
| `forge-core/internal/router/router.go` | 注册新中间件 |
| `forge-portal/lib/api.ts` | 错误码处理增强 |
| `forge-portal/lib/auth.tsx` | 增强错误处理 |
| `forge-portal/app/layout.tsx` | 挂载 Error Boundary + Toaster |
| `forge-portal/package.json` | 添加 vitest 依赖 |

---

## Task 1: AES-256-GCM 加密工具

**Files:**
- Create: `forge-core/internal/pkg/crypto/crypto.go`
- Create: `forge-core/internal/pkg/crypto/crypto_test.go`

- [ ] **Step 1: Write the failing test**

```go
// forge-core/internal/pkg/crypto/crypto_test.go
package crypto

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef" // 32-byte hex key
	plaintext := "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("encrypted text should differ from plaintext")
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	plaintext := "same-input"

	c1, _ := Encrypt(key, plaintext)
	c2, _ := Encrypt(key, plaintext)

	if c1 == c2 {
		t.Fatal("two encryptions of same plaintext should differ (unique nonce)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := "0123456789abcdef0123456789abcdef"
	key2 := "fedcba9876543210fedcba9876543210"

	encrypted, _ := Encrypt(key1, "secret")
	_, err := Decrypt(key2, encrypted)
	if err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestInvalidKeyLength(t *testing.T) {
	_, err := Encrypt("short", "data")
	if err == nil {
		t.Fatal("short key should fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd forge-core && go test ./internal/pkg/crypto/... -v`
Expected: FAIL — package/functions not found

- [ ] **Step 3: Write minimal implementation**

```go
// forge-core/internal/pkg/crypto/crypto.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Key must be a 32-byte hex-encoded string (64 hex chars).
// Returns hex-encoded nonce+ciphertext.
func Encrypt(hexKey, plaintext string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("key must be 64 hex chars (32 bytes), got %d", len(hexKey))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts hex-encoded ciphertext produced by Encrypt.
func Decrypt(hexKey, hexCiphertext string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("key must be 64 hex chars (32 bytes)")
	}

	ciphertext, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd forge-core && go test ./internal/pkg/crypto/... -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/pkg/crypto/
git commit -m "feat: add AES-256-GCM crypto utility for token encryption"
```

---

## Task 2: 结构化错误码

**Files:**
- Create: `forge-core/internal/pkg/errcode/errcode.go`
- Create: `forge-core/internal/pkg/errcode/errcode_test.go`
- Modify: `forge-core/internal/pkg/response/response.go`

- [ ] **Step 1: Write the failing test**

```go
// forge-core/internal/pkg/errcode/errcode_test.go
package errcode

import "testing"

func TestAppErrorImplementsError(t *testing.T) {
	err := New(InvalidInput, "bad field")
	if err.Error() != "bad field" {
		t.Fatalf("got %q, want %q", err.Error(), "bad field")
	}
	if err.Code != InvalidInput {
		t.Fatalf("got code %d, want %d", err.Code, InvalidInput)
	}
}

func TestAppErrorHTTPStatus(t *testing.T) {
	tests := []struct {
		code   int
		status int
	}{
		{InvalidInput, 400},
		{Unauthorized, 401},
		{Forbidden, 403},
		{NotFound, 404},
		{InternalError, 500},
	}
	for _, tt := range tests {
		err := New(tt.code, "test")
		if err.HTTPStatus() != tt.status {
			t.Errorf("code %d: got HTTP %d, want %d", tt.code, err.HTTPStatus(), tt.status)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd forge-core && go test ./internal/pkg/errcode/... -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// forge-core/internal/pkg/errcode/errcode.go
package errcode

import "net/http"

// Error codes — 4-digit, grouped by category
const (
	// 1xxx: Input / validation
	InvalidInput = 1001
	MissingField = 1002

	// 2xxx: Authentication
	Unauthorized   = 2001
	TokenExpired   = 2002
	TokenRevoked   = 2003
	InvalidCredentials = 2004

	// 3xxx: Authorization
	Forbidden = 3001

	// 4xxx: Resource
	NotFound  = 4001
	Conflict  = 4002

	// 5xxx: Internal
	InternalError = 5001
	ExternalAPI   = 5002
)

// AppError is a structured application error with a code.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) HTTPStatus() int {
	switch {
	case e.Code >= 1000 && e.Code < 2000:
		return http.StatusBadRequest
	case e.Code >= 2000 && e.Code < 3000:
		return http.StatusUnauthorized
	case e.Code >= 3000 && e.Code < 4000:
		return http.StatusForbidden
	case e.Code >= 4000 && e.Code < 5000:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd forge-core && go test ./internal/pkg/errcode/... -v`
Expected: PASS

- [ ] **Step 5: Update response.go to support AppError**

Modify `forge-core/internal/pkg/response/response.go`:

```go
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/errcode"
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

// FailWithError responds with a structured AppError.
func FailWithError(c *gin.Context, err error) {
	var appErr *errcode.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus(), Result{Code: appErr.Code, Message: appErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, Result{Code: errcode.InternalError, Message: "内部错误"})
}
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/pkg/errcode/ forge-core/internal/pkg/response/response.go
git commit -m "feat: add structured error codes and FailWithError response helper"
```

---

## Task 3: Request ID 中间件

**Files:**
- Create: `forge-core/internal/middleware/requestid.go`
- Create: `forge-core/internal/middleware/requestid_test.go`
- Modify: `forge-core/internal/router/router.go`

- [ ] **Step 1: Write the failing test**

```go
// forge-core/internal/middleware/requestid_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		rid := c.GetString("request_id")
		c.String(200, rid)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	// Response should have X-Request-ID header
	headerVal := w.Header().Get("X-Request-ID")
	if headerVal == "" {
		t.Fatal("X-Request-ID header should be set")
	}

	// Body should match header (we echoed the context value)
	if w.Body.String() != headerVal {
		t.Fatalf("context request_id %q != header %q", w.Body.String(), headerVal)
	}
}

func TestRequestIDForwardsExisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.String(200, c.GetString("request_id"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "incoming-id-123")
	r.ServeHTTP(w, req)

	if w.Body.String() != "incoming-id-123" {
		t.Fatalf("should forward incoming request ID, got %q", w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd forge-core && go test ./internal/middleware/... -v`
Expected: FAIL — RequestID not defined

- [ ] **Step 3: Write implementation**

```go
// forge-core/internal/middleware/requestid.go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID generates or forwards X-Request-ID for every request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd forge-core && go test ./internal/middleware/... -v`
Expected: PASS

- [ ] **Step 5: Register in router**

Modify `forge-core/internal/router/router.go` — add `r.Use(middleware.RequestID())` after `gin.Recovery()`:

```go
r.Use(gin.Recovery())
r.Use(middleware.RequestID())
r.Use(middleware.CORS())
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/middleware/requestid.go forge-core/internal/middleware/requestid_test.go forge-core/internal/router/router.go
git commit -m "feat: add Request ID middleware for request tracing"
```

---

## Task 4: 安全响应头 + CORS 加固

**Files:**
- Modify: `forge-core/internal/middleware/cors.go`

- [ ] **Step 1: Update CORS middleware to add security headers**

```go
// forge-core/internal/middleware/cors.go
package middleware

import (
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")

		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "0") // modern browsers: use CSP instead
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Verify build**

Run: `cd forge-core && go build ./cmd/forge-core`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add forge-core/internal/middleware/cors.go
git commit -m "feat: add security response headers (X-Frame-Options, nosniff, referrer)"
```

---

## Task 5: JWT 算法锁定 + OAuth State 加固

**Files:**
- Modify: `forge-core/internal/module/auth/service.go:108-111` — 添加算法检查
- Modify: `forge-core/internal/module/auth/handler.go:72` — crypto/rand state
- Modify: `forge-core/internal/module/auth/model.go:68` — state 必填

- [ ] **Step 1: Write unit test for JWT algorithm validation**

JWT 算法检查发生在 token parse 阶段（repo 调用之前），所以可以不依赖 DB 进行单元测试：

```go
// forge-core/internal/module/auth/jwt_test.go
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateTokenRejectsNoneAlgorithm(t *testing.T) {
	s := &Service{jwtSecret: []byte("test-secret")}

	// Create a token with alg:none
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: 1, TenantID: 1, Username: "admin",
	})
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := s.ValidateToken(t.Context(), tokenString)
	if err == nil {
		t.Fatal("should reject alg:none token")
	}
}

func TestValidateTokenAcceptsHS256(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	s := &Service{
		jwtSecret: secret,
		repo:      nil, // Will panic at IsTokenActive — that's OK, we test parse logic
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: 1, TenantID: 1, Username: "admin",
	})
	tokenString, _ := token.SignedString(secret)

	// This will pass JWT parse but panic at repo.IsTokenActive (nil repo).
	// We use recover to verify the JWT parsing succeeded.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo, meaning JWT parse succeeded")
			}
			// Panic from nil repo = JWT parse passed = correct behavior
		}()
		s.ValidateToken(t.Context(), tokenString)
	}()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd forge-core && go test ./internal/module/auth/ -run TestValidateToken -v`
Expected: FAIL — algorithm check not yet implemented

- [ ] **Step 3: Lock JWT algorithm in ValidateToken**

Modify `forge-core/internal/module/auth/service.go` — `ValidateToken` method:

```go
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Reject algorithm confusion attacks
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	active, err := s.repo.IsTokenActive(ctx, claims.ID)
	if err != nil || !active {
		return nil, errors.New("token revoked")
	}

	return claims, nil
}
```

- [ ] **Step 3: Fix OAuth state to use crypto/rand**

Modify `forge-core/internal/module/auth/handler.go`:

```go
import (
	"crypto/rand"
	"encoding/hex"
	// ... existing imports, remove "fmt" and "time" if no longer needed
)

func (h *Handler) GitHubAuthorize(c *gin.Context) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成安全令牌失败")
		return
	}
	state := hex.EncodeToString(b)
	authorizeURL := h.service.GetGitHubAuthorizeURL(state)
	response.OK(c, GitHubAuthorizeResponse{AuthorizeURL: authorizeURL})
}
```

- [ ] **Step 4: Make State required in GitHubCallbackRequest**

Modify `forge-core/internal/module/auth/model.go:68`:

```go
type GitHubCallbackRequest struct {
	Code  string `form:"code" binding:"required"`
	State string `form:"state" binding:"required"`
}
```

- [ ] **Step 5: Verify build**

Run: `cd forge-core && go build ./cmd/forge-core`

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/module/auth/service.go forge-core/internal/module/auth/handler.go forge-core/internal/module/auth/model.go
git commit -m "fix(security): lock JWT to HS256, use crypto/rand for OAuth state"
```

---

## Task 6: GitHub Token 加密存储

**Files:**
- Modify: `forge-core/internal/config/config.go` — 添加 ENCRYPTION_KEY
- Modify: `forge-core/internal/module/auth/service.go` — 注入加密 key，加密/解密 token
- Modify: `forge-core/internal/module/auth/repository.go` — 不变（加密在 service 层）
- Modify: `forge-core/cmd/forge-core/main.go` — 传递 encryption key

- [ ] **Step 1: Add ENCRYPTION_KEY to config**

Modify `forge-core/internal/config/config.go`:

```go
type Config struct {
	ServerPort     string
	DatabaseURL    string
	RedisAddr      string
	RedisPassword  string
	JWTSecret      string
	JWTExpireHours int
	EncryptionKey  string // 64 hex chars = 32 bytes AES-256

	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURI  string

	// Temporal
	TemporalAddress string
}

func Load() *Config {
	cfg := &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://forge:forge_dev_2026@localhost:5432/forge_main?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", "forge_redis_2026"),
		JWTSecret:      getEnv("JWT_SECRET", "forge-dev-secret-key-change-in-production"),
		JWTExpireHours: 8,
		EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),

		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURI:  getEnv("GITHUB_REDIRECT_URI", "http://localhost:3000/auth/github/callback"),

		TemporalAddress: getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
	}
	return cfg
}
```

- [ ] **Step 2: Add encryption key to Service struct and constructor**

Modify `forge-core/internal/module/auth/service.go`:

```go
// Add import
import (
	"github.com/shulex/forge/forge-core/internal/pkg/crypto"
	// ... existing imports
)

// Update Service struct — add encryptionKey field
type Service struct {
	repo              *Repository
	jwtSecret         []byte
	jwtExpire         time.Duration
	githubClientID    string
	githubSecret      string
	githubRedirectURI string
	encryptionKey     string // 64 hex chars for AES-256-GCM, empty = no encryption
}

// Update constructor — add encryptionKey parameter (last position)
func NewService(repo *Repository, jwtSecret string, jwtExpireHours int,
	githubClientID, githubSecret, githubRedirectURI, encryptionKey string) *Service {
	return &Service{
		repo:              repo,
		jwtSecret:         []byte(jwtSecret),
		jwtExpire:         time.Duration(jwtExpireHours) * time.Hour,
		githubClientID:    githubClientID,
		githubSecret:      githubSecret,
		githubRedirectURI: githubRedirectURI,
		encryptionKey:     encryptionKey,
	}
}
```

- [ ] **Step 3: Encrypt token on write in HandleGitHubCallback**

In `HandleGitHubCallback`, after creating the `identity` struct and before `UpsertUserIdentity`:

```go
	// Encrypt access token before storage
	if s.encryptionKey != "" {
		encrypted, err := crypto.Encrypt(s.encryptionKey, tokenResp.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt token: %w", err)
		}
		identity.AccessToken = encrypted
	}
```

- [ ] **Step 4: Decrypt token on read in GetGitHubToken**

Replace the `GetGitHubToken` method:

```go
func (s *Service) GetGitHubToken(ctx context.Context, userID int64) (string, error) {
	identity, err := s.repo.FindUserIdentity(ctx, userID, "github")
	if err != nil {
		return "", fmt.Errorf("github not connected: %w", err)
	}
	token := identity.AccessToken
	if s.encryptionKey != "" {
		decrypted, err := crypto.Decrypt(s.encryptionKey, token)
		if err != nil {
			return "", fmt.Errorf("decrypt token: %w", err)
		}
		token = decrypted
	}
	return token, nil
}
```

- [ ] **Step 5: Update main.go call site**

In `forge-core/cmd/forge-core/main.go:45-46`, update the `NewService` call to add `cfg.EncryptionKey`:

```go
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours,
		cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURI, cfg.EncryptionKey)
```

- [ ] **Step 6: Generate a dev encryption key and add to `.env.example`**

Add to `.env.example`:
```
ENCRYPTION_KEY=your-64-hex-char-key-here-for-aes256-encryption
```

Generate a dev key: `openssl rand -hex 32` → add to `.env` for local dev.

- [ ] **Step 7: Build and verify**

Run: `cd forge-core && go build ./cmd/forge-core`

- [ ] **Step 8: Commit**

```bash
git add forge-core/internal/config/config.go forge-core/internal/module/auth/service.go forge-core/cmd/forge-core/main.go .env.example
git commit -m "feat(security): encrypt GitHub access tokens at rest with AES-256-GCM"
```

---

## Task 7: 输入校验增强

**Files:**
- Modify: `forge-core/internal/module/auth/model.go`
- Modify: `forge-core/internal/module/task/model.go`

- [ ] **Step 1: Add validation tags to auth models**

```go
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=2,max=50,alphanum"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}
```

- [ ] **Step 2: Add validation to task model**

Read `forge-core/internal/module/task/model.go` and add min/max to Requirement and other string fields.

- [ ] **Step 3: Build and verify**

Run: `cd forge-core && go build ./cmd/forge-core`

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/module/auth/model.go forge-core/internal/module/task/model.go
git commit -m "feat: add input length/format validation to auth and task models"
```

---

## Task 8: 前端测试基础设施

**Files:**
- Create: `forge-portal/vitest.config.ts`
- Modify: `forge-portal/package.json` — add vitest + testing-library deps
- Create: `forge-portal/lib/__tests__/api.test.ts`

- [ ] **Step 1: Install test dependencies**

```bash
cd forge-portal && npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom
```

- [ ] **Step 2: Create vitest config**

```ts
// forge-portal/vitest.config.ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: [],
    globals: true,
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '.'),
    },
  },
})
```

- [ ] **Step 3: Add test script to package.json**

Add to `scripts` in `package.json`:
```json
"test": "vitest run",
"test:watch": "vitest"
```

- [ ] **Step 4: Write a basic API layer test**

```ts
// forge-portal/lib/__tests__/api.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

describe('api module', () => {
  it('should be importable', async () => {
    // Basic smoke test — verify the module can be imported
    const mod = await import('../api')
    expect(mod.api).toBeDefined()
    expect(mod.api.get).toBeTypeOf('function')
    expect(mod.api.post).toBeTypeOf('function')
    expect(mod.api.put).toBeTypeOf('function')
    expect(mod.api.delete).toBeTypeOf('function')
  })
})
```

- [ ] **Step 5: Run tests**

Run: `cd forge-portal && npm test`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add forge-portal/vitest.config.ts forge-portal/package.json forge-portal/package-lock.json forge-portal/lib/__tests__/
git commit -m "chore: add vitest + testing-library frontend test infrastructure"
```

---

## Task 9: React Error Boundary + Toast

**Files:**
- Create: `forge-portal/components/error-boundary.tsx`
- Modify: `forge-portal/app/layout.tsx` — wrap with ErrorBoundary

- [ ] **Step 1: Check if sonner/toast is already available**

Check `forge-portal/package.json` for `sonner` or existing toast library. shadcn/ui often includes `sonner`. If not installed:

```bash
cd forge-portal && npx shadcn@latest add sonner
```

- [ ] **Step 2: Create Error Boundary component**

```tsx
// forge-portal/components/error-boundary.tsx
"use client"

import { Component, type ReactNode } from "react"

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback ?? (
        <div className="flex min-h-screen items-center justify-center bg-background">
          <div className="text-center space-y-4">
            <h2 className="text-xl font-semibold text-foreground">出了点问题</h2>
            <p className="text-muted-foreground text-sm">
              {this.state.error?.message || "发生了未知错误"}
            </p>
            <button
              onClick={() => this.setState({ hasError: false, error: null })}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm"
            >
              重试
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
```

- [ ] **Step 3: Add ErrorBoundary to root layout**

Read `forge-portal/app/layout.tsx`, wrap the children with `<ErrorBoundary>`. Also add `<Toaster />` from sonner if not already present.

- [ ] **Step 4: Verify build**

Run: `cd forge-portal && npm run build`

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/error-boundary.tsx forge-portal/app/layout.tsx
git commit -m "feat: add React Error Boundary and global toast notifications"
```

---

## Task 10: 前端错误码处理增强

**Files:**
- Modify: `forge-portal/lib/api.ts` — 解析结构化错误码

- [ ] **Step 1: Update ApiError and request function**

```ts
// Update ApiError class
class ApiError extends Error {
  constructor(
    public code: number,
    message: string,
    public httpStatus?: number
  ) {
    super(message);
  }

  get isAuth() {
    return this.code >= 2000 && this.code < 3000;
  }

  get isValidation() {
    return this.code >= 1000 && this.code < 2000;
  }

  get isNotFound() {
    return this.code >= 4000 && this.code < 5000;
  }
}
```

Update the `request` function to pass `res.status` to `ApiError`:

```ts
if (json.code !== 0) {
  throw new ApiError(json.code, json.message, res.status);
}
```

- [ ] **Step 2: Build**

Run: `cd forge-portal && npm run build`

- [ ] **Step 3: Commit**

```bash
git add forge-portal/lib/api.ts
git commit -m "feat: structured error code handling in frontend API layer"
```

---

## Task 11: 配置启动校验

**Files:**
- Modify: `forge-core/internal/config/config.go` — 添加 `Validate()` 方法
- Modify: `forge-core/cmd/forge-core/main.go` — 启动时调用

- [ ] **Step 1: Add Validate method**

```go
import (
	"log/slog"
	"os"
)

// Validate logs warnings for insecure defaults. In production, these should be fatal.
func (c *Config) Validate() {
	if c.JWTSecret == "forge-dev-secret-key-change-in-production" {
		slog.Warn("JWT_SECRET is using insecure default — set JWT_SECRET env var in production")
	}
	if c.EncryptionKey == "" {
		slog.Warn("ENCRYPTION_KEY not set — GitHub tokens will be stored unencrypted")
	}
	if c.GitHubClientID == "" {
		slog.Warn("GITHUB_CLIENT_ID not set — GitHub OAuth will not work")
	}
}
```

- [ ] **Step 2: Call Validate in main.go**

After `cfg := config.Load()`, add `cfg.Validate()`.

- [ ] **Step 3: Build and verify**

Run: `cd forge-core && go build ./cmd/forge-core`

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/config/config.go forge-core/cmd/forge-core/main.go
git commit -m "feat: add startup config validation with security warnings"
```

---

## 完成后验证清单

- [ ] `cd forge-core && go build ./cmd/forge-core` — 编译通过
- [ ] `cd forge-core && go test ./...` — 全部测试通过
- [ ] `cd forge-portal && npm run build` — 前端构建通过
- [ ] `cd forge-portal && npm test` — 前端测试通过
- [ ] 启动 forge-core 后检查日志中的安全警告
- [ ] 用 curl 验证响应头包含 `X-Request-ID`、`X-Frame-Options` 等

---

## 本次加固未覆盖（记录为后续 TODO）

以下问题在审视报告中提及，但本次加固 sprint 不处理（降低范围，保持 1.5-2 天工期）：

| 延后项 | 原因 | 建议处理时间 |
|--------|------|-------------|
| **前端 token → HttpOnly Cookie** | 涉及 auth 流程重构 + CSRF token 联动，改动面大 | S5 或独立 sprint |
| **admin/admin123 首次登录强制修改** | 需要前端引导流程 + 后端密码策略 | S5 |
| **审计日志** | 需要设计审计表 + 中间件，属于 P1 | Phase 2 |
| **Prometheus 指标** | 属于 P2 可观测性，不阻塞功能开发 | Phase 2 |
| **前端 Sentry** | 依赖外部服务选型 | Phase 2 |
| **数据库 migration 工具** | golang-migrate 引入需要重构现有 migration 流程 | S5 |
| **CI pipeline** | 需要 GitHub Actions 配置，独立任务 | S5 完成后 |
