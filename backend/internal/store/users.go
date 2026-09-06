package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// UserRow is a local login account (no secrets beyond password_hash).
type UserRow struct {
	ID           string
	Login        string
	PasswordHash string
}

// UserByLogin looks up a local user by login.
func (s *Store) UserByLogin(ctx context.Context, login string) (*UserRow, error) {
	var u UserRow
	err := s.Pool.QueryRow(ctx,
		`SELECT id::text, login, password_hash FROM users WHERE login=$1`, login).
		Scan(&u.ID, &u.Login, &u.PasswordHash)
	return &u, err
}

// UserByID looks up a local user by id.
func (s *Store) UserByID(ctx context.Context, id string) (*UserRow, error) {
	var u UserRow
	err := s.Pool.QueryRow(ctx,
		`SELECT id::text, login, password_hash FROM users WHERE id=$1::uuid`, id).
		Scan(&u.ID, &u.Login, &u.PasswordHash)
	return &u, err
}

// EnsureAdmin idempotently creates (tenant, user, admin membership) used by
// the seed CLI. Idempotent: re-running with the same login does not reset the
// password unless reset=true.
func (s *Store) EnsureAdmin(ctx context.Context, tenantName, login, passwordHash, role string, reset bool) (created bool, err error) {
	if role != "viewer" && role != "admin" {
		return false, fmt.Errorf("role must be viewer or admin")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var tenantID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM tenants WHERE name=$1`, tenantName).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx,
			`INSERT INTO tenants (name) VALUES ($1) RETURNING id::text`, tenantName).Scan(&tenantID); err != nil {
			return false, fmt.Errorf("create tenant: %w", err)
		}
	} else if err != nil {
		return false, err
	}

	var userID string
	exists := false
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE login=$1`, login).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (login, password_hash) VALUES ($1,$2) RETURNING id::text`,
			login, passwordHash).Scan(&userID); err != nil {
			return false, fmt.Errorf("create user: %w", err)
		}
	} else if err != nil {
		return false, err
	} else {
		exists = true
		if reset {
			if _, err := tx.Exec(ctx,
				`UPDATE users SET password_hash=$1 WHERE id=$2::uuid`, passwordHash, userID); err != nil {
				return false, err
			}
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1,$2,$3)
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role=excluded.role`,
		tenantID, userID, role); err != nil {
		return false, fmt.Errorf("upsert membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return !exists, nil
}

// EnsureLocalDevice registers a local-source device under the tenant's local
// project (idempotent by (tenant_id, source, external_id)).
func (s *Store) EnsureLocalDevice(ctx context.Context, tenantName, externalID, name string, expectedIntervalS int) (string, error) {
	var tenantID, projectID, deviceID string
	if err := s.Pool.QueryRow(ctx,
		`SELECT id::text FROM tenants WHERE name=$1`, tenantName).Scan(&tenantID); err != nil {
		return "", fmt.Errorf("tenant %q: %w", tenantName, err)
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO projects (tenant_id, source, external_id, name)
		VALUES ($1,'local',NULL,'本地设备')
		ON CONFLICT (tenant_id, source) WHERE external_id IS NULL DO NOTHING
		RETURNING id::text`, tenantID).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := s.Pool.QueryRow(ctx,
			`SELECT id::text FROM projects WHERE tenant_id=$1 AND source='local' AND external_id IS NULL`,
			tenantID).Scan(&projectID); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	err = s.Pool.QueryRow(ctx, `
		INSERT INTO devices (tenant_id, project_id, source, external_id, name, expected_report_interval_s)
		VALUES ($1,$2,'local',$3,$4,$5)
		ON CONFLICT (tenant_id, source, external_id) DO NOTHING
		RETURNING id::text`, tenantID, projectID, externalID, name, expectedIntervalS).Scan(&deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := s.Pool.QueryRow(ctx,
			`SELECT id::text FROM devices WHERE tenant_id=$1 AND source='local' AND external_id=$2`,
			tenantID, externalID).Scan(&deviceID); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return deviceID, nil
}

// ---- 用户管理（admin；docs/04-contracts.md）----
// 会话按租户隔离：列表/新建/重置密码都限定在调用者所在租户的成员关系内，
// 绝不跨租户操作。users.login 全局唯一，冲突映射为 ErrLoginTaken。

// ErrLoginTaken reports a duplicate local login on user creation.
var ErrLoginTaken = errors.New("login already exists")

// TenantUser is a local account bound to one tenant.
type TenantUser struct {
	ID         string    `json:"id"`
	Login      string    `json:"login"`
	TenantName string    `json:"tenant_name"`
	Role       string    `json:"role"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListTenantUsers returns users having a membership in the tenant.
func (s *Store) ListTenantUsers(ctx context.Context, tenantID string) ([]TenantUser, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT u.id::text, u.login, t.name, m.role, u.created_at
		FROM users u
		JOIN memberships m ON m.user_id = u.id
		JOIN tenants t ON t.id = m.tenant_id
		WHERE m.tenant_id = $1
		ORDER BY u.login`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TenantUser{}
	for rows.Next() {
		var u TenantUser
		if err := rows.Scan(&u.ID, &u.Login, &u.TenantName, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateTenantUser creates a login + membership (role admin|viewer) in tenant.
func (s *Store) CreateTenantUser(ctx context.Context, tenantID, login, passwordHash, role string) (string, error) {
	if role != "viewer" && role != "admin" {
		return "", fmt.Errorf("role must be viewer or admin")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var tenantName string
	if err := tx.QueryRow(ctx, `SELECT name FROM tenants WHERE id=$1`, tenantID).Scan(&tenantName); err != nil {
		return "", fmt.Errorf("tenant: %w", err)
	}
	var userID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1,$2) RETURNING id::text`,
		login, passwordHash).Scan(&userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrLoginTaken
		}
		return "", fmt.Errorf("insert user: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1,$2,$3)`,
		tenantID, userID, role); err != nil {
		return "", fmt.Errorf("insert membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

// ResetUserPassword updates the password hash when the user belongs to the
// tenant; other tenants' users are never visible (ErrNoRows -> 404).
func (s *Store) ResetUserPassword(ctx context.Context, tenantID, userID, passwordHash string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE users u SET password_hash=$1
		WHERE u.id=$2::uuid
		  AND EXISTS (SELECT 1 FROM memberships m WHERE m.user_id=u.id AND m.tenant_id=$3)`,
		passwordHash, userID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}
