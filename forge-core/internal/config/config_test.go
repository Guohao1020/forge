package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Ensure no env vars interfere
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("DATABASE_URL")

	cfg := Load()

	if cfg.ServerPort != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.ServerPort)
	}
	if cfg.JWTExpireHours != 8 {
		t.Errorf("expected 8 hour JWT expiry, got %d", cfg.JWTExpireHours)
	}
	if cfg.RedisPassword == "" {
		t.Error("expected default redis password")
	}
	if cfg.TemporalAddress != "localhost:7233" {
		t.Errorf("expected default temporal address, got %s", cfg.TemporalAddress)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("SERVER_PORT")
	defer os.Unsetenv("JWT_SECRET")

	cfg := Load()

	if cfg.ServerPort != "9090" {
		t.Errorf("expected port 9090 from env, got %s", cfg.ServerPort)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("expected JWT secret from env, got %s", cfg.JWTSecret)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY_12345", "test_value")
	defer os.Unsetenv("TEST_KEY_12345")

	if got := getEnv("TEST_KEY_12345", "default"); got != "test_value" {
		t.Errorf("expected test_value, got %s", got)
	}
	if got := getEnv("NONEXISTENT_KEY_12345", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %s", got)
	}
}

func TestValidate(t *testing.T) {
	cfg := Load()
	// Validate should not panic
	cfg.Validate()
}

func TestConfigFields(t *testing.T) {
	cfg := Load()

	// Verify all fields have reasonable defaults
	fields := map[string]string{
		"ServerPort":      cfg.ServerPort,
		"DatabaseURL":     cfg.DatabaseURL,
		"RedisAddr":       cfg.RedisAddr,
		"JWTSecret":       cfg.JWTSecret,
		"TemporalAddress": cfg.TemporalAddress,
		"WorkspaceRoot":   cfg.WorkspaceRoot,
	}

	for name, value := range fields {
		if value == "" {
			t.Errorf("config field %s should not be empty", name)
		}
	}
}
