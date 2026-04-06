package config

import (
	"log/slog"
	"os"
)

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

	// Temporal
	TemporalAddress string

	// Encryption
	EncryptionKey string

	// Kubernetes
	KubeconfigPath string

	// Workspace
	WorkspaceRoot string

	// ACR (Alibaba Cloud Container Registry)
	ACRRegistry string // e.g., registry.cn-hangzhou.aliyuncs.com/forge
	ACRUsername string
	ACRPassword string

	// K8s Node IP (for NodePort preview URLs)
	K8sNodeIP string
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

		TemporalAddress: getEnv("TEMPORAL_ADDRESS", "localhost:7233"),

		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),

		KubeconfigPath: getEnv("KUBECONFIG_PATH", "k8s/kubeconfig"),

		WorkspaceRoot: getEnv("FORGE_WORKSPACE_ROOT", "./workspaces"),

		ACRRegistry: getEnv("ACR_REGISTRY", "repo-voc-registry-vpc.cn-hangzhou.cr.aliyuncs.com/voc-repo"),
		ACRUsername: getEnv("ACR_USERNAME", "1652058863700531@shulex"),
		ACRPassword: getEnv("ACR_PASSWORD", "shulex123123"),

		K8sNodeIP: getEnv("K8S_NODE_IP", "47.97.49.242"),
	}
}

// Validate logs warnings for insecure defaults.
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
