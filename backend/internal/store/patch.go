package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpdateDeviceAlias renames a device (local alias only, R10) and records an
// audit log entry with the actor id. Remote devices are never modified here.
func (s *Store) UpdateDeviceAlias(ctx context.Context, tenantID, deviceID, name, actorID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE devices SET name=$1
		WHERE id=$2::uuid AND tenant_id=$3`, name, deviceID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (tenant_id, actor, action, target, redacted_diff)
		VALUES ($1,$2::uuid,'device.rename',$3::uuid,
		        jsonb_build_object('name',$4::text))`,
		tenantID, actorID, deviceID, name); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return tx.Commit(ctx)
}

var _ = pgx.ErrNoRows
var _ = errors.Is
