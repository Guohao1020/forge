package workspace

import (
	"bytes"
	"context"
	"testing"
)

// stubCrypto is a CryptoService stand-in for DAO tests: it identifies
// encrypt(x) = x and decrypt(x) = x. The real CryptoService lives in
// crypto.go (Task 1b.2) and is tested independently there.
type stubCrypto struct{}

func (stubCrypto) Encrypt(plaintext []byte) ([]byte, error) { return plaintext, nil }
func (stubCrypto) Decrypt(blob []byte) ([]byte, error)      { return blob, nil }

func TestDeployKeyRepo_GetByProject_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, stubCrypto{})
	ctx := context.Background()

	got, err := repo.GetByProject(ctx, 1, 888888)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing row, got %+v", got)
	}
}

func TestDeployKeyRepo_UpsertAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, stubCrypto{})
	ctx := context.Background()
	defer func() {
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, 1001)
	}()

	pub := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITEST forge-test"
	priv := []byte("fake-private-bytes-for-dao-test")
	if err := repo.UpsertKey(ctx, 1, 1001, pub, priv, 42); err != nil {
		t.Fatalf("UpsertKey: %v", err)
	}

	got, err := repo.GetByProject(ctx, 1, 1001)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if got == nil {
		t.Fatal("GetByProject returned nil for existing row")
	}
	if got.ProjectID != 1001 {
		t.Errorf("ProjectID: got %d, want 1001", got.ProjectID)
	}
	if got.TenantID != 1 {
		t.Errorf("TenantID: got %d, want 1", got.TenantID)
	}
	if got.PublicKey != pub {
		t.Errorf("PublicKey mismatch")
	}
	if !bytes.Equal(got.PrivateKey, priv) {
		t.Errorf("PrivateKey roundtrip failed")
	}
	if got.GitHubKeyID == nil || *got.GitHubKeyID != 42 {
		t.Errorf("GitHubKeyID: got %v, want 42", got.GitHubKeyID)
	}
	if got.KeyType != "ed25519" {
		t.Errorf("KeyType: got %s, want ed25519", got.KeyType)
	}
}

func TestDeployKeyRepo_UpsertIdempotent(t *testing.T) {
	// A second UpsertKey on the same project updates the row (overwrites)
	// rather than erroring on primary-key conflict.
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, stubCrypto{})
	ctx := context.Background()
	defer func() {
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, 1002)
	}()

	if err := repo.UpsertKey(ctx, 1, 1002, "pub1", []byte("priv1"), 10); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertKey(ctx, 1, 1002, "pub2", []byte("priv2"), 20); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.GetByProject(ctx, 1, 1002)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if got.PublicKey != "pub2" {
		t.Errorf("PublicKey: got %s, want pub2 (second upsert should win)", got.PublicKey)
	}
	if got.GitHubKeyID == nil || *got.GitHubKeyID != 20 {
		t.Errorf("GitHubKeyID: got %v, want 20", got.GitHubKeyID)
	}
}

func TestDeployKeyRepo_UpsertNilGitHubID(t *testing.T) {
	// githubKeyID=0 means "not set yet" and stores NULL.
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, stubCrypto{})
	ctx := context.Background()
	defer func() {
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, 1003)
	}()

	if err := repo.UpsertKey(ctx, 1, 1003, "pub", []byte("priv"), 0); err != nil {
		t.Fatalf("UpsertKey: %v", err)
	}

	got, err := repo.GetByProject(ctx, 1, 1003)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if got.GitHubKeyID != nil {
		t.Errorf("GitHubKeyID: got %v, want nil", got.GitHubKeyID)
	}
}

func TestDeployKeyRepo_GetByProject_TenantMismatch(t *testing.T) {
	// Multi-tenant isolation: tenant A cannot read tenant B's row.
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, stubCrypto{})
	ctx := context.Background()
	defer func() {
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, 1004)
	}()

	if err := repo.UpsertKey(ctx, 5, 1004, "pub", []byte("priv"), 1); err != nil {
		t.Fatalf("UpsertKey: %v", err)
	}

	// Wrong tenant queries -- should return nil
	got, err := repo.GetByProject(ctx, 6, 1004)
	if err != nil {
		t.Fatalf("GetByProject wrong tenant: %v", err)
	}
	if got != nil {
		t.Fatalf("tenant isolation violated: got row for wrong tenant: %+v", got)
	}

	// Correct tenant -- should return the row
	got2, err := repo.GetByProject(ctx, 5, 1004)
	if err != nil || got2 == nil {
		t.Fatalf("correct tenant query failed: got=%v err=%v", got2, err)
	}
}
