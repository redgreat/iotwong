// Package store centralizes PostgreSQL access shared by the API server and
// the MQTT ingestor (docs/02-architecture.md ADR-003). All data access goes
// through the runtime app role; tenant isolation is enforced in SQL.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool builds a pgxpool from PG* / IOTWONG_APP_* environment variables.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	host := envOr("PGHOST", "127.0.0.1")
	port := envOr("PGPORT", "5432")
	db := envOr("PGDATABASE", "iotwong")
	user := envOr("IOTWONG_APP_USER", envOr("PGUSER", ""))
	pass := envOr("IOTWONG_APP_PASSWORD", envOr("PGPASSWORD", ""))
	if user == "" {
		return nil, errors.New("no DB user configured (PGUSER or IOTWONG_APP_USER)")
	}
	u := url.URL{Scheme: "postgres", Host: host + ":" + port, Path: "/" + db,
		User: url.UserPassword(user, pass)}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return pgxpool.New(ctx, u.String())
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Store provides typed queries.
type Store struct {
	Pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

// ---- sessions -------------------------------------------------------------

const sessionTTL = 12 * time.Hour

// Session is one logged-in browser session.
type Session struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
}

// CreateSession stores a new random session and returns its plain token.
func (s *Store) CreateSession(ctx context.Context, userID string) (token string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	_, err = s.Pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1,$2,$3)`, hex.EncodeToString(sum[:]), userID, time.Now().UTC().Add(sessionTTL))
	if err != nil {
		return "", err
	}
	return token, nil
}

// SessionUser returns the user id for a live session token (hash lookup).
func (s *Store) SessionUser(ctx context.Context, token string) (string, error) {
	sum := sha256.Sum256([]byte(token))
	var uid string
	err := s.Pool.QueryRow(ctx, `
		SELECT user_id FROM sessions
		WHERE token_hash=$1 AND expires_at > now()`,
		hex.EncodeToString(sum[:])).Scan(&uid)
	return uid, err
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hex.EncodeToString(sum[:]))
	return err
}

// Membership is a tenant binding with a role.
type Membership struct {
	TenantID string `json:"tenant_id"`
	Tenant   string `json:"tenant_name"`
	Role     string `json:"role"`
}

// MembershipsOf returns the user's tenant memberships (server determines the
// tenant; request bodies never carry tenant_id).
func (s *Store) MembershipsOf(ctx context.Context, userID string) ([]Membership, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT t.id, t.name, m.role
		FROM memberships m JOIN tenants t ON t.id = m.tenant_id
		WHERE m.user_id = $1 ORDER BY t.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.TenantID, &m.Tenant, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

var (
	ErrNoRows        = pgx.ErrNoRows
	ErrLoginMismatch = errors.New("login mismatch")
	ErrDeviceUnknown = errors.New("device not registered")
)

// ---- ingest ------------------------------------------------------------------

// IngestResult classifies one MQTT message outcome.
type IngestResult string

const (
	IngestInserted  IngestResult = "inserted"
	IngestDuplicate IngestResult = "duplicate"
	IngestConflict  IngestResult = "conflict"
)

// IngestInput carries the validated message and classification windows.
type IngestInput struct {
	DeviceExternalID string
	RawPayloadHash   string // sha256 hex of raw bytes
	MessageID        string // event_id
	RecordedAt       time.Time
	ReceivedAt       time.Time
	HasPosition      bool
	Longitude        float64
	Latitude         float64
	LocationSource   string
	SpeedMPS         *float64
	HeadingDeg       *float64
	AccuracyM        *float64
	BatteryPct       *float64
	StatusReason     string // "" accepted; poison reason otherwise
}

// Ingest writes one telemetry event transactionally
// (docs/03-data-design.md 写入事务与生命周期).
func (s *Store) Ingest(ctx context.Context, in IngestInput) (IngestResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var deviceID, tenantID string
	err = tx.QueryRow(ctx, `
		SELECT d.id, d.tenant_id FROM devices d
		WHERE d.source='local' AND d.external_id=$1`, in.DeviceExternalID).
		Scan(&deviceID, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDeviceUnknown
	}
	if err != nil {
		return "", fmt.Errorf("device lookup: %w", err)
	}

	// idempotency ledger with payload-hash conflict detection
	tag, err := tx.Exec(ctx, `
		INSERT INTO ingest_events (tenant_id, device_id, event_id, payload_hash, received_at, status)
		VALUES ($1,$2,$3,$4,$5,'ok')
		ON CONFLICT (tenant_id, device_id, event_id) DO NOTHING`,
		tenantID, deviceID, in.MessageID, in.RawPayloadHash, in.ReceivedAt)
	if err != nil {
		return "", fmt.Errorf("ledger insert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var prevHash string
		var prevStatus string
		if err := tx.QueryRow(ctx, `
			SELECT payload_hash, status FROM ingest_events
			WHERE tenant_id=$1 AND device_id=$2 AND event_id=$3`,
			tenantID, deviceID, in.MessageID).Scan(&prevHash, &prevStatus); err != nil {
			return "", fmt.Errorf("ledger read: %w", err)
		}
		if prevHash == in.RawPayloadHash {
			return IngestDuplicate, tx.Commit(ctx) // legal replay: success but no new business event
		}
		if _, err := tx.Exec(ctx, `
			UPDATE ingest_events SET status='conflict' WHERE tenant_id=$1 AND device_id=$2 AND event_id=$3`,
			tenantID, deviceID, in.MessageID); err != nil {
			return "", fmt.Errorf("ledger conflict: %w", err)
		}
		return IngestConflict, tx.Commit(ctx)
	}

	// latest communication time moves forward only
	if _, err := tx.Exec(ctx, `
		INSERT INTO device_latest (tenant_id, device_id, last_seen_at)
		VALUES ($1,$2,$3)
		ON CONFLICT (tenant_id, device_id) DO UPDATE
		  SET last_seen_at = CASE WHEN device_latest.last_seen_at IS NULL
		       OR excluded.last_seen_at > device_latest.last_seen_at
		       THEN excluded.last_seen_at ELSE device_latest.last_seen_at END
		  , updated_at = now()`,
		tenantID, deviceID, in.ReceivedAt); err != nil {
		return "", fmt.Errorf("latest seen: %w", err)
	}

	if in.HasPosition {
		if _, err := tx.Exec(ctx, `
			INSERT INTO positions
			  (tenant_id, device_id, recorded_at, position_id, event_id, received_at,
			   point, speed_mps, heading_deg, accuracy_m, location_source)
			VALUES ($1,$2,$3,$4,$5,$6,
			        ST_SetSRID(ST_MakePoint($7,$8),4326),$9,$10,$11,$12)`,
			tenantID, deviceID, in.RecordedAt, uuidv5(in.MessageID), in.MessageID, in.ReceivedAt,
			in.Longitude, in.Latitude, in.SpeedMPS, in.HeadingDeg, in.AccuracyM,
			in.LocationSource); err != nil {
			return "", fmt.Errorf("position insert: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO device_latest
			  (tenant_id, device_id, last_position_at, point, location_source, speed_mps, accuracy_m, battery_pct)
			VALUES ($1,$2,$3,ST_SetSRID(ST_MakePoint($4,$5),4326),$6,$7,$8,$9)
			ON CONFLICT (tenant_id, device_id) DO UPDATE
			  SET last_position_at=excluded.last_position_at,
			      point=excluded.point, location_source=excluded.location_source,
			      speed_mps=excluded.speed_mps, accuracy_m=excluded.accuracy_m,
			      battery_pct=excluded.battery_pct, updated_at=now()
			  WHERE device_latest.last_position_at IS NULL
			     OR excluded.last_position_at > device_latest.last_position_at`,
			tenantID, deviceID, in.RecordedAt, in.Longitude, in.Latitude, in.LocationSource,
			in.SpeedMPS, in.AccuracyM, in.BatteryPct); err != nil {
			return "", err
		}
	}

	// outbox: minimal notification DTO
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (tenant_id, type, device_id, payload)
		VALUES ($1,'device.updated',$2,
		        jsonb_build_object('device_id',$5::text,'recorded_at',$3::text,'has_position',$4::boolean))`,
		tenantID, deviceID, in.RecordedAt.Format(time.RFC3339Nano), in.HasPosition,
		in.DeviceExternalID); err != nil {
		return "", fmt.Errorf("outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return IngestInserted, nil
}

// DeviceExternalInfo resolves a local-source device external id to its
// tenant and internal device id (ingestor registration check + fence eval).
// ErrDeviceUnknown when absent.
func (s *Store) DeviceExternalInfo(ctx context.Context, external string) (tenantID, deviceID string, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT d.tenant_id, d.id FROM devices d
		WHERE d.source='local' AND d.external_id=$1`, external).Scan(&tenantID, &deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrDeviceUnknown
	}
	return tenantID, deviceID, err
}

// Poison records a rejected message category with a bounded/redacted digest
// (docs/03-data-design.md: 毒消息持久化错误类别及受限长度/脱敏摘要).
func (s *Store) Poison(ctx context.Context, tenantID *string, deviceExternalID, reason, digest string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO ingest_poison (tenant_id, device_external_id, reason, payload_digest)
		VALUES ($1,$2,$3,$4)`, tenantID, deviceExternalID, reason, digest)
	return err
}

// OutboxRow is one notification for SSE.
type OutboxRow struct {
	ID        int64          `json:"id"`
	TenantID  string         `json:"-"`
	Type      string         `json:"type"`
	DeviceID  string         `json:"device_id"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

// ReadOutbox returns rows after a cursor for a tenant (bounded).
func (s *Store) ReadOutbox(ctx context.Context, tenantID string, after int64, limit int) ([]OutboxRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, type, COALESCE(device_id::text,''), payload, created_at
		FROM outbox_events WHERE tenant_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		tenantID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxRow
	for rows.Next() {
		var o OutboxRow
		var payload []byte
		if err := rows.Scan(&o.ID, &o.Type, &o.DeviceID, &payload, &o.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &o.Payload); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// uuidv5 minimal deterministic uuid for position identity (same scheme as
// racebox importer).
func uuidv5(name string) string {
	h := sha256.Sum256([]byte("iotwong:local:" + name))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
}
