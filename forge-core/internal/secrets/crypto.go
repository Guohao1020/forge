// Package secrets provides symmetric crypto for the forge-core process.
//
// A single Service instance owns an HKDF-derived AES-256-GCM key
// material keyed from FORGE_SECRETS_MASTER_KEY (a base64-encoded
// 32-byte master key). Callers use Encrypt/Decrypt to seal arbitrary
// byte slices. Storage format: nonce(12) || ciphertext || tag(16).
//
// When the master key source changes (Vault, KMS, etc.), only
// NewService needs to change.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	masterKeyBytes = 32 // AES-256
	nonceBytes     = 12 // GCM standard nonce size
	tagBytes       = 16 // GCM tag
	hkdfSalt       = "forge-secrets-v1"
	hkdfInfo       = "forge-deploy-key-encryption"
)

// Service is the single entry point for symmetric encryption in forge-core.
type Service struct {
	aead cipher.AEAD
}

// NewService constructs a Service from a base64-encoded 32-byte master key.
func NewService(masterKeyB64 string) (*Service, error) {
	master, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("secrets: master key is not valid base64: %w", err)
	}
	if len(master) != masterKeyBytes {
		return nil, fmt.Errorf("secrets: master key must decode to %d bytes, got %d",
			masterKeyBytes, len(master))
	}

	derivedKey := make([]byte, 32)
	kdf := hkdf.New(func() hash.Hash { return sha256.New() }, master, []byte(hkdfSalt), []byte(hkdfInfo))
	if _, err := io.ReadFull(kdf, derivedKey); err != nil {
		return nil, fmt.Errorf("secrets: HKDF expand failed: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: cipher.NewGCM: %w", err)
	}

	return &Service{aead: aead}, nil
}

// Encrypt seals plaintext with AES-256-GCM using a fresh random nonce.
// Output: nonce(12) || ciphertext || tag(16). Length = len(plaintext) + 28.
func (s *Service) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secrets: rand.Read: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens a blob produced by Encrypt.
func (s *Service) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) < nonceBytes+tagBytes {
		return nil, errors.New("secrets: ciphertext too short")
	}
	nonce := blob[:nonceBytes]
	ct := blob[nonceBytes:]
	pt, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt: %w", err)
	}
	return pt, nil
}
