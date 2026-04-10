package workspace

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// GenerateKeyPair creates a fresh ed25519 keypair and serialises it in
// OpenSSH formats that git, ssh, and GitHub accept directly.
//
// The comment argument goes on the public key's third field (the one
// ssh shows in log messages) and should identify this key for human
// debugging, e.g. "forge-deploy-{tenant}-{project}-{epoch}".
//
// Returns:
//   - publicKey: single-line "ssh-ed25519 <base64> <comment>"
//   - privateKey: multi-line OpenSSH PEM bytes (what `ssh -i` reads)
func GenerateKeyPair(comment string) (string, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, fmt.Errorf("ed25519.GenerateKey: %w", err)
	}

	// Public key in OpenSSH authorized_keys format
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", nil, fmt.Errorf("ssh.NewPublicKey: %w", err)
	}
	pubLine := string(ssh.MarshalAuthorizedKey(sshPub))
	// MarshalAuthorizedKey adds a trailing newline; strip it and append comment
	if len(pubLine) > 0 && pubLine[len(pubLine)-1] == '\n' {
		pubLine = pubLine[:len(pubLine)-1]
	}
	pubLine = fmt.Sprintf("%s %s", pubLine, comment)

	// Private key in OpenSSH PEM format (what `ssh -i` consumes)
	pemBlock, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return "", nil, fmt.Errorf("ssh.MarshalPrivateKey: %w", err)
	}
	privPEM := pem.EncodeToMemory(pemBlock)

	return pubLine, privPEM, nil
}

// RealCryptoService implements CryptoService (defined in keys.go) using
// AES-256-GCM with a 32-byte master key from FORGE_SECRETS_MASTER_KEY.
//
// Storage format: nonce(12) || ciphertext || tag(16). The AEAD's Seal
// method already appends the tag to the ciphertext, so we only need to
// prepend the nonce and the concatenation falls out naturally.
type RealCryptoService struct {
	aead cipher.AEAD
}

// NewCryptoService constructs a CryptoService from a base64-encoded
// 32-byte master key. Typical source is the FORGE_SECRETS_MASTER_KEY
// env var populated by the deployment.
//
// Rejects invalid inputs loudly rather than falling back to any default
// -- we never want to accidentally encrypt deploy keys under a zero key.
func NewCryptoService(masterKeyBase64 string) (*RealCryptoService, error) {
	if masterKeyBase64 == "" {
		return nil, errors.New("crypto: master key is empty")
	}
	key, err := base64.StdEncoding.DecodeString(masterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("crypto: master key base64 decode: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: master key must be 32 bytes (got %d) — AES-256-GCM", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: cipher.NewGCM: %w", err)
	}

	return &RealCryptoService{aead: aead}, nil
}

// Encrypt wraps plaintext in AES-256-GCM using a fresh random nonce.
// The returned blob layout is:
//
//	[ nonce(12) | ciphertext(len(plaintext)) | tag(16) ]
//
// which is what Decrypt expects. This is the single format stored in
// engine.project_deploy_keys.private_key_enc.
func (r *RealCryptoService) Encrypt(plaintext []byte) ([]byte, error) {
	nonceSize := r.aead.NonceSize()
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	// Seal appends ciphertext+tag to the first arg. Pre-allocate the
	// output starting with the nonce so the caller gets a single slice.
	out := make([]byte, 0, nonceSize+len(plaintext)+r.aead.Overhead())
	out = append(out, nonce...)
	out = r.aead.Seal(out, nonce, plaintext, nil)
	return out, nil
}

// Decrypt reverses Encrypt. Fails loudly if:
//   - blob is shorter than nonceSize + tagSize (malformed)
//   - the tag verification fails (tampered ciphertext or wrong key)
func (r *RealCryptoService) Decrypt(blob []byte) ([]byte, error) {
	nonceSize := r.aead.NonceSize()
	if len(blob) < nonceSize+r.aead.Overhead() {
		return nil, fmt.Errorf("crypto: ciphertext too short: %d bytes", len(blob))
	}
	nonce := blob[:nonceSize]
	ct := blob[nonceSize:]
	pt, err := r.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return pt, nil
}
