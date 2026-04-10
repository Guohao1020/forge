package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DeployKey holds a decrypted project deploy key in memory. The private
// key is raw bytes (OpenSSH PEM-formatted -- see crypto.go) suitable for
// writing to a tempfile for GIT_SSH_COMMAND.
//
// The PrivateKey field carries plaintext bytes because callers
// (RealGitRunner in git.go) need to write it to a tempfile
// immediately. Lifetime is scoped to the single git operation: the
// caller fetches a DeployKey, uses it, and lets it go out of scope.
type DeployKey struct {
	ProjectID   int64
	TenantID    int64
	PublicKey   string    // OpenSSH single-line authorized_keys format
	PrivateKey  []byte    // OpenSSH PEM bytes (DECRYPTED)
	KeyType     string    // "ed25519"
	GitHubKeyID *int64    // GitHub-assigned key ID after upload; nil until uploaded
	CreatedAt   time.Time
}

// CryptoService is the minimal interface DeployKeyRepo needs from the
// crypto layer. The production implementation lives in crypto.go
// (Task 1b.2). Having it as an interface lets the DAO tests use a
// stub that pass-through encryption without pulling in AES-GCM.
type CryptoService interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(blob []byte) ([]byte, error)
}

// DeployKeyRepo is the DAO for engine.project_deploy_keys. It holds a
// reference to the CryptoService so callers see decrypted private keys
// transparently.
type DeployKeyRepo struct {
	db     *sql.DB
	crypto CryptoService
}

// NewDeployKeyRepo constructs a DeployKeyRepo. The CryptoService must
// be initialised with a valid master key -- see crypto.go.
func NewDeployKeyRepo(db *sql.DB, crypto CryptoService) *DeployKeyRepo {
	return &DeployKeyRepo{db: db, crypto: crypto}
}

// GetByProject loads the deploy key row for (tenantID, projectID). The
// tenantID is passed explicitly for multi-tenant isolation: we match on
// both columns so a cross-tenant query returns nil even if the
// project_id happens to exist in a different tenant.
//
// Returns (nil, nil) if no row exists -- same pattern as StateRepo.
//
// Returns an error if the row exists but decryption fails -- that's a
// programmer error (wrong master key, corrupted bytes) and the caller
// should halt loudly rather than silently fall back.
func (r *DeployKeyRepo) GetByProject(ctx context.Context, tenantID, projectID int64) (*DeployKey, error) {
	const q = `
		SELECT project_id, tenant_id, public_key, private_key_enc,
		       key_type, github_key_id, created_at
		FROM engine.project_deploy_keys
		WHERE project_id = $1 AND tenant_id = $2
	`
	dk := &DeployKey{}
	var ct []byte
	var ghID sql.NullInt64

	err := r.db.QueryRowContext(ctx, q, projectID, tenantID).Scan(
		&dk.ProjectID, &dk.TenantID, &dk.PublicKey, &ct,
		&dk.KeyType, &ghID, &dk.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deploy_key: GetByProject: %w", err)
	}

	priv, err := r.crypto.Decrypt(ct)
	if err != nil {
		return nil, fmt.Errorf("deploy_key: decrypt project %d: %w", projectID, err)
	}
	dk.PrivateKey = priv

	if ghID.Valid {
		id := ghID.Int64
		dk.GitHubKeyID = &id
	}
	return dk, nil
}

// UpsertKey writes or replaces the deploy key row for (tenantID, projectID).
// The private key is encrypted via the CryptoService before storage.
//
// githubKeyID may be 0 to mean "not uploaded yet" (stored as NULL).
// When called by generateAndUploadDeployKey, the upload has already
// succeeded and githubKeyID is the non-zero value from GitHub.
//
// Upsert semantics via ON CONFLICT allow the same caller to replace a
// stale row -- used by the RotateKey stub (Task 1b.6) and by error-
// recovery paths that regenerate keys.
func (r *DeployKeyRepo) UpsertKey(
	ctx context.Context,
	tenantID, projectID int64,
	publicKey string,
	privateKey []byte,
	githubKeyID int64,
) error {
	ct, err := r.crypto.Encrypt(privateKey)
	if err != nil {
		return fmt.Errorf("deploy_key: encrypt: %w", err)
	}

	var ghID sql.NullInt64
	if githubKeyID != 0 {
		ghID = sql.NullInt64{Int64: githubKeyID, Valid: true}
	}

	const q = `
		INSERT INTO engine.project_deploy_keys
			(project_id, tenant_id, public_key, private_key_enc, key_type, github_key_id)
		VALUES ($1, $2, $3, $4, 'ed25519', $5)
		ON CONFLICT (project_id) DO UPDATE SET
			tenant_id       = EXCLUDED.tenant_id,
			public_key      = EXCLUDED.public_key,
			private_key_enc = EXCLUDED.private_key_enc,
			key_type        = EXCLUDED.key_type,
			github_key_id   = EXCLUDED.github_key_id
	`
	if _, err := r.db.ExecContext(ctx, q, projectID, tenantID, publicKey, ct, ghID); err != nil {
		return fmt.Errorf("deploy_key: upsert: %w", err)
	}
	return nil
}

// RotateKey is a stub reserved for future key rotation. Round 2
// explicitly does not implement rotation -- keys are generated once and
// reused. See spec "Key rotation: Not implemented in this release".
//
// This stub exists so the method surface is discoverable in code review
// and so Task 1b.6 has a single well-named place to document the
// follow-up work. When rotation is implemented, this method should:
//  1. Generate a new keypair
//  2. Call GitHub API to upload the new key and capture the new ID
//  3. Call GitHub API to delete the old key by GitHubKeyID
//  4. UpsertKey to replace the row
//
// Any of steps 1-3 failing should leave the old row intact.
func (r *DeployKeyRepo) RotateKey(ctx context.Context, tenantID, projectID int64) error {
	return errors.New("deploy_key: key rotation not implemented in Round 2 — future project")
}
