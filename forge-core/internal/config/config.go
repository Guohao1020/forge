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

	// Temporal
	TemporalAddress string

	// Encryption
	EncryptionKey string
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
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
