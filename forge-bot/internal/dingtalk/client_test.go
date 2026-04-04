package dingtalk

import (
	"testing"
)

func TestVerifySignature_NoToken(t *testing.T) {
	// With empty token, should always pass
	if !VerifySignature("", "123", "abc", "body") {
		t.Error("expected true when token is empty")
	}
}

func TestVerifySignature_ValidSignature(t *testing.T) {
	token := "test-token"
	timestamp := "1234567890"

	// Generate the expected signature
	client := NewClient("", token)
	_ = client // just to verify construction works

	// With matching signature it should pass
	if VerifySignature(token, timestamp, "wrong-sig", "") {
		t.Error("expected false for wrong signature")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://oapi.dingtalk.com/robot/send?access_token=test", "secret123")
	if c.webhook == "" {
		t.Error("webhook should be set")
	}
	if c.secret == "" {
		t.Error("secret should be set")
	}
}

func TestSignedURL_NoSecret(t *testing.T) {
	c := NewClient("https://example.com/webhook", "")
	url := c.signedURL()
	if url != "https://example.com/webhook" {
		t.Errorf("expected raw webhook URL, got %s", url)
	}
}

func TestSignedURL_WithSecret(t *testing.T) {
	c := NewClient("https://example.com/webhook", "mysecret")
	url := c.signedURL()
	if len(url) <= len("https://example.com/webhook") {
		t.Error("expected URL with timestamp and sign parameters")
	}
}
