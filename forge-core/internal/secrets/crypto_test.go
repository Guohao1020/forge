package secrets

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// testMasterKey is a fixed 32-byte key for deterministic testing.
// Production uses FORGE_SECRETS_MASTER_KEY env var.
var testMasterKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, 32))

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(testMasterKey)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	svc := newTestService(t)
	plaintext := []byte("hello world")
	ct, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := svc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("roundtrip mismatch: want %q got %q", plaintext, pt)
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	svc := newTestService(t)
	plaintext := []byte("same plaintext every time")
	ct1, _ := svc.Encrypt(plaintext)
	ct2, _ := svc.Encrypt(plaintext)
	if bytes.Equal(ct1, ct2) {
		t.Fatal("ciphertexts are identical — nonce is not random, GCM is broken")
	}
}

func TestCiphertextDoesNotContainPlaintext(t *testing.T) {
	svc := newTestService(t)
	plaintext := []byte("FORGE_UNIQUE_MARKER_STRING_12345")
	ct, _ := svc.Encrypt(plaintext)
	if bytes.Contains(ct, plaintext) {
		t.Fatal("ciphertext contains plaintext as substring — not encrypted")
	}
	if strings.Contains(string(ct), "FORGE_UNIQUE_MARKER") {
		t.Fatal("ciphertext leaks plaintext partially")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	svc := newTestService(t)
	ct, _ := svc.Encrypt([]byte("secret data"))
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[15] ^= 0x01
	_, err := svc.Decrypt(tampered)
	if err == nil {
		t.Fatal("tampered ciphertext decrypted without error — GCM auth failed")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	svc1 := newTestService(t)
	otherKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xCD}, 32))
	svc2, err := NewService(otherKey)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ct, _ := svc1.Encrypt([]byte("written with svc1"))
	_, err = svc2.Decrypt(ct)
	if err == nil {
		t.Fatal("decryption with wrong key succeeded — should fail auth check")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	svc := newTestService(t)
	ct, err := svc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt(empty): %v", err)
	}
	if len(ct) != 12+16 {
		t.Fatalf("want 28-byte ciphertext for empty plaintext, got %d", len(ct))
	}
	pt, err := svc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt(empty ct): %v", err)
	}
	if len(pt) != 0 {
		t.Fatalf("want empty plaintext, got %d bytes", len(pt))
	}
}

func TestNewServiceRejectsShortKey(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err := NewService(short)
	if err == nil {
		t.Fatal("NewService accepted a short master key")
	}
}

func TestNewServiceRejectsBadBase64(t *testing.T) {
	_, err := NewService("not-valid-base64!!!")
	if err == nil {
		t.Fatal("NewService accepted invalid base64")
	}
}
