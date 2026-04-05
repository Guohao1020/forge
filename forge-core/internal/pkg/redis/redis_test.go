package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestNewClient_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	client, err := NewClient(context.Background(), mr.Addr(), "")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	// Verify client works
	if err := client.Set(context.Background(), "test-key", "test-value", 0).Err(); err != nil {
		t.Fatalf("SET failed: %v", err)
	}
	val, err := client.Get(context.Background(), "test-key").Result()
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if val != "test-value" {
		t.Errorf("expected test-value, got %s", val)
	}
}

func TestNewClient_WithPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("test-pass")

	client, err := NewClient(context.Background(), mr.Addr(), "test-pass")
	if err != nil {
		t.Fatalf("NewClient with password failed: %v", err)
	}
	defer client.Close()

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestNewClient_WrongPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("correct-pass")

	_, err := NewClient(context.Background(), mr.Addr(), "wrong-pass")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestNewClient_Unreachable(t *testing.T) {
	_, err := NewClient(context.Background(), "localhost:19999", "")
	if err == nil {
		t.Error("expected error for unreachable address")
	}
}
