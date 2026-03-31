package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByUsername(ctx context.Context, tenantID int64, username string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, username, email, password_hash, display_name, avatar_url, status, last_login_at, created_at
		 FROM auth.users WHERE tenant_id = $1 AND username = $2`,
		tenantID, username,
	).Scan(&user.ID, &user.TenantID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.AvatarURL, &user.Status, &user.LastLoginAt, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (r *Repository) FindUserByID(ctx context.Context, userID int64) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, username, email, password_hash, display_name, avatar_url, status, last_login_at, created_at
		 FROM auth.users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.TenantID, &user.Username, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.AvatarURL, &user.Status, &user.LastLoginAt, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserRoles(ctx context.Context, userID int64) ([]Role, error) {
	rows, err := r.db.Query(ctx,
		`SELECT r.id, r.code, r.name
		 FROM auth.roles r
		 JOIN auth.user_roles ur ON r.id = ur.role_id
		 WHERE ur.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Code, &role.Name); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *Repository) UpdateLastLogin(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, "UPDATE auth.users SET last_login_at = NOW() WHERE id = $1", userID)
	return err
}

func (r *Repository) SaveActiveToken(ctx context.Context, tenantID, userID int64, jti, tokenType, deviceInfo string, ipAddr string, expiresAt interface{}) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO auth.active_tokens (tenant_id, user_id, token_jti, token_type, device_info, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6::inet, $7)`,
		tenantID, userID, jti, tokenType, deviceInfo, ipAddr, expiresAt,
	)
	return err
}

func (r *Repository) DeleteActiveToken(ctx context.Context, jti string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM auth.active_tokens WHERE token_jti = $1", jti)
	return err
}

func (r *Repository) IsTokenActive(ctx context.Context, jti string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM auth.active_tokens WHERE token_jti = $1 AND expires_at > NOW())",
		jti,
	).Scan(&exists)
	return exists, err
}
