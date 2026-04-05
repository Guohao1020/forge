package crypto

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 64 hex chars = 32 bytes
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
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	plaintext := "same-input"

	c1, _ := Encrypt(key, plaintext)
	c2, _ := Encrypt(key, plaintext)

	if c1 == c2 {
		t.Fatal("two encryptions of same plaintext should differ (unique nonce)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	key2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

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

func TestEncryptEmptyString(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	encrypted, err := Encrypt(key, "")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("expected empty, got %q", decrypted)
	}
}

func TestEncryptLargeData(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// 10KB of data
	data := make([]byte, 10240)
	for i := range data {
		data[i] = byte(i % 256)
	}
	plaintext := string(data)

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt large: %v", err)
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt large: %v", err)
	}
	if decrypted != plaintext {
		t.Fatal("large data round-trip failed")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	_, err := Decrypt(key, "not-valid-base64!!!")
	if err == nil {
		t.Fatal("invalid base64 should fail")
	}
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	_, err := Decrypt(key, "YWJj") // "abc" in base64, too short for nonce
	if err == nil {
		t.Fatal("truncated ciphertext should fail")
	}
}
