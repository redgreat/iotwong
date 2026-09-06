package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Fence is a circle or polygon geofence bound to devices of one tenant.
type Fence struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Kind      string       `json:"kind"` // circle | polygon
	Enabled   bool         `json:"enabled"`
	CenterLng *float64     `json:"center_longitude,omitempty"`
	CenterLat *float64     `json:"center_latitude,omitempty"`
	RadiusM   *float64     `json:"radius_m,omitempty"`
	Polygon   [][2]float64 `json:"polygon,omitempty"` // closed [lng,lat] ring
	DeviceIDs []string     `json:"device_ids"`
}

// FenceInput is the write shape.
type FenceInput struct {
	Name      string       `json:"name"`
	Kind      string       `json:"kind"`
	Enabled   bool         `json:"enabled"`
	CenterLng *float64     `json:"center_longitude"`
	CenterLat *float64     `json:"center_latitude"`
	RadiusM   *float64     `json:"radius_m"`
	Polygon   [][2]float64 `json:"polygon"`
	DeviceIDs []string     `json:"device_ids"`
}

func (s *Store) ListFences(ctx context.Context, tenantID string) ([]Fence, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT f.id::text, f.name, f.kind, f.enabled, ST_X(f.center), ST_Y(f.center),
		       f.radius_m, ST_AsGeoJSON(f.geometry),
		       COALESCE((SELECT jsonb_agg(device_id::text) FROM fence_devices fd
		                  WHERE fd.fence_id=f.id AND fd.tenant_id=f.tenant_id), '[]'::jsonb)
		FROM fences f WHERE f.tenant_id=$1 ORDER BY f.name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fence
	for rows.Next() {
		var f Fence
		var cx, cy *float64
		var geo *string
		var devs []byte
		if err := rows.Scan(&f.ID, &f.Name, &f.Kind, &f.Enabled, &cx, &cy, &f.RadiusM, &geo, &devs); err != nil {
			return nil, err
		}
		if f.Kind == "circle" && cx != nil && cy != nil {
			f.CenterLng, f.CenterLat = cx, cy
		}
		if geo != nil {
			f.Polygon = decodeRing(*geo)
		}
		if len(devs) > 0 {
			_ = json.Unmarshal(devs, &f.DeviceIDs)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func decodeRing(geo string) [][2]float64 {
	var g struct {
		Coordinates [][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(geo), &g); err != nil || len(g.Coordinates) == 0 {
		return nil
	}
	return g.Coordinates[0]
}

func (s *Store) CreateFence(ctx context.Context, tenantID string, in FenceInput) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var fenceID string
	switch in.Kind {
	case "circle":
		if in.CenterLng == nil || in.CenterLat == nil || in.RadiusM == nil || *in.RadiusM <= 0 {
			return "", fmt.Errorf("circle fence requires center + radius_m>0")
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO fences (tenant_id, name, kind, enabled, center, radius_m)
			VALUES ($1,$2,'circle',$3, ST_SetSRID(ST_MakePoint($4,$5),4326), $6)
			RETURNING id::text`,
			tenantID, in.Name, in.Enabled, *in.CenterLng, *in.CenterLat, *in.RadiusM).Scan(&fenceID)
	case "polygon":
		ring := in.Polygon
		if len(ring) < 4 {
			return "", fmt.Errorf("polygon fence requires a closed ring of >=4 points")
		}
		first, last := ring[0], ring[len(ring)-1]
		if first[0] != last[0] || first[1] != last[1] {
			ring = append(append([][2]float64{}, ring...), first)
		}
		poly := map[string]any{"type": "Polygon", "coordinates": [][][2]float64{ring}}
		b, _ := json.Marshal(poly)
		err = tx.QueryRow(ctx, `
			INSERT INTO fences (tenant_id, name, kind, enabled, geometry)
			VALUES ($1,$2,'polygon',$3, ST_GeomFromGeoJSON($4))
			RETURNING id::text`,
			tenantID, in.Name, in.Enabled, string(b)).Scan(&fenceID)
	default:
		return "", fmt.Errorf("kind must be circle or polygon")
	}
	if err != nil {
		return "", err
	}
	if err := bindFenceDevices(ctx, tx, tenantID, fenceID, in.DeviceIDs); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return fenceID, nil
}

func (s *Store) UpdateFence(ctx context.Context, tenantID, fenceID string, in FenceInput) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE fences SET name=$1, enabled=$2
		WHERE id=$3::uuid AND tenant_id=$4`, in.Name, in.Enabled, fenceID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	if _, err := tx.Exec(ctx, `DELETE FROM fence_devices WHERE fence_id=$1::uuid AND tenant_id=$2`,
		fenceID, tenantID); err != nil {
		return err
	}
	if err := bindFenceDevices(ctx, tx, tenantID, fenceID, in.DeviceIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func bindFenceDevices(ctx context.Context, tx pgx.Tx, tenantID, fenceID string, ids []string) error {
	for _, d := range ids {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fence_devices (fence_id, tenant_id, device_id)
			VALUES ($1::uuid,$2,$3::uuid) ON CONFLICT DO NOTHING`,
			fenceID, tenantID, d); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteFence(ctx context.Context, tenantID, fenceID string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM fences WHERE id=$1::uuid AND tenant_id=$2`, fenceID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

// ---- alarms ----------------------------------------------------------------

type Alarm struct {
	ID         string    `json:"id"`
	FenceID    string    `json:"fence_id"`
	FenceName  string    `json:"fence_name"`
	DeviceID   string    `json:"device_id"`
	Transition string    `json:"transition"`
	OccurredAt time.Time `json:"occurred_at"`
	Acked      bool      `json:"acked"`
}

func (s *Store) ListAlarms(ctx context.Context, tenantID string, limit int) ([]Alarm, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT a.id::text, a.fence_id::text, f.name, a.device_id::text,
		       a.transition, a.occurred_at, (a.acked_at IS NOT NULL)
		FROM alarms a JOIN fences f ON f.id=a.fence_id AND f.tenant_id=a.tenant_id
		WHERE a.tenant_id=$1 ORDER BY a.occurred_at DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alarm
	for rows.Next() {
		var a Alarm
		if err := rows.Scan(&a.ID, &a.FenceID, &a.FenceName, &a.DeviceID, &a.Transition,
			&a.OccurredAt, &a.Acked); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) AckAlarm(ctx context.Context, tenantID, alarmID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE alarms SET acked_at=now()
		WHERE id=$1::uuid AND tenant_id=$2 AND acked_at IS NULL`, alarmID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

// EvalFences runs inside the ingest transaction (server-side, independent of
// any open webpage). The first contact only establishes the baseline state;
// real inside/outside transitions raise alarms. Repeated same-side messages
// do not duplicate alarms.
func EvalFences(ctx context.Context, tx pgx.Tx, tenantID, deviceID, recTS string, lng, lat float64) error {
	rows, err := tx.Query(ctx, `
		SELECT f.id::text, f.kind, ST_X(f.center), ST_Y(f.center), f.radius_m,
		       ST_AsText(f.geometry)
		FROM fences f JOIN fence_devices fd ON fd.fence_id = f.id
		WHERE f.tenant_id=$1 AND fd.device_id=$2::uuid AND f.enabled`, tenantID, deviceID)
	if err != nil {
		return fmt.Errorf("fence list: %w", err)
	}
	type hit struct {
		id     string
		inside bool
	}
	defer rows.Close()
	var hits []hit
	for rows.Next() {
		var id, kind string
		var cx, cy, rad *float64
		var geom *string
		if err := rows.Scan(&id, &kind, &cx, &cy, &rad, &geom); err != nil {
			return fmt.Errorf("fence row: %w", err)
		}
		var inside bool
		switch {
		case kind == "circle" && cx != nil && rad != nil:
			err = tx.QueryRow(ctx, `
				SELECT ST_DWithin($1::geography,
				       ST_SetSRID(ST_MakePoint($2,$3),4326)::geography, $4)`,
				pointGeom(*cx, *cy), lng, lat, *rad).Scan(&inside)
		case kind == "polygon" && geom != nil:
			err = tx.QueryRow(ctx, `
				SELECT ST_Contains($1::geometry,
				       ST_SetSRID(ST_MakePoint($2,$3),4326))`,
				*geom, lng, lat).Scan(&inside)
		default:
			continue
		}
		if err != nil {
			return fmt.Errorf("fence hit(%s): %w", kind, err)
		}
		hits = append(hits, hit{id: id, inside: inside})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("fence rows: %w", err)
	}
	rows.Close() // release the tx before per-fence queries
	for _, h := range hits {
		want := "outside"
		if h.inside {
			want = "inside"
		}
		var prev *string
		err := tx.QueryRow(ctx, `SELECT last_state FROM fence_states
			WHERE tenant_id=$1 AND fence_id=$2::uuid AND device_id=$3::uuid`,
			tenantID, h.id, deviceID).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("state read: %w", err)
		}
		if prev == nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO fence_states (tenant_id, fence_id, device_id, last_state, last_event_time)
				VALUES ($1,$2::uuid,$3::uuid,$4,$5::timestamptz)`,
				tenantID, h.id, deviceID, want, recTS); err != nil {
				return err
			}
			continue
		}
		if *prev == want {
			continue
		}
		transition := "exit"
		if want == "inside" {
			transition = "enter"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE fence_states SET last_state=$1, last_event_time=$2::timestamptz
			WHERE tenant_id=$3 AND fence_id=$4::uuid AND device_id=$5::uuid`,
			want, recTS, tenantID, h.id, deviceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO alarms (tenant_id, fence_id, device_id, transition, occurred_at)
			VALUES ($1,$2::uuid,$3::uuid,$4,$5::timestamptz)
			ON CONFLICT (tenant_id, fence_id, device_id, occurred_at, transition) DO NOTHING`,
			tenantID, h.id, deviceID, transition, recTS); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox_events (tenant_id, type, device_id, payload)
			VALUES ($1,'alarm.created',$2::uuid,
			        jsonb_build_object('fence_id',$3::text,'transition',$4::text))`,
			tenantID, deviceID, h.id, transition); err != nil {
			return err
		}
	}
	return nil
}

func pointGeom(lng, lat float64) string {
	return fmt.Sprintf("SRID=4326;POINT(%v %v)", lng, lat)
}

// EvalFencesPost runs fence evaluation after the ingest commit on its own
// pool connections (no shared-tx query interleaving). Position durability is
// already guaranteed by the ingest transaction; an evaluation failure here
// only logs a warning and never blocks/retries message delivery. Historical
// (racebox_demo) ingest does not call this method.
func (s *Store) EvalFencesPost(ctx context.Context, tenantID, deviceID, recTS string, lng, lat float64) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT f.id::text, f.kind, ST_X(f.center), ST_Y(f.center), f.radius_m,
		       ST_AsText(f.geometry)
		FROM fences f JOIN fence_devices fd ON fd.fence_id = f.id
		WHERE f.tenant_id=$1 AND fd.device_id=$2::uuid AND f.enabled`, tenantID, deviceID)
	if err != nil {
		return err
	}
	type hit struct {
		id     string
		inside bool
	}
	var hits []hit
	for rows.Next() {
		var id, kind string
		var cx, cy, rad *float64
		var geom *string
		if err := rows.Scan(&id, &kind, &cx, &cy, &rad, &geom); err != nil {
			rows.Close()
			return err
		}
		var inside bool
		switch {
		case kind == "circle" && cx != nil && rad != nil:
			err = s.Pool.QueryRow(ctx, `
				SELECT ST_DWithin($1::geography,
				       ST_SetSRID(ST_MakePoint($2,$3),4326)::geography, $4)`,
				pointGeom(*cx, *cy), lng, lat, *rad).Scan(&inside)
		case kind == "polygon" && geom != nil:
			err = s.Pool.QueryRow(ctx, `
				SELECT ST_Contains($1::geometry,
				       ST_SetSRID(ST_MakePoint($2,$3),4326))`,
				*geom, lng, lat).Scan(&inside)
		}
		if err != nil {
			rows.Close()
			return err
		}
		hits = append(hits, hit{id: id, inside: inside})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, h := range hits {
		want := "outside"
		if h.inside {
			want = "inside"
		}
		var prev *string
		err := s.Pool.QueryRow(ctx, `SELECT last_state FROM fence_states
			WHERE tenant_id=$1 AND fence_id=$2::uuid AND device_id=$3::uuid`,
			tenantID, h.id, deviceID).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if prev == nil {
			if _, err := s.Pool.Exec(ctx, `
				INSERT INTO fence_states (tenant_id, fence_id, device_id, last_state, last_event_time)
				VALUES ($1,$2::uuid,$3::uuid,$4,$5::timestamptz)`,
				tenantID, h.id, deviceID, want, recTS); err != nil {
				return err
			}
			continue
		}
		if *prev == want {
			continue
		}
		transition := "exit"
		if want == "inside" {
			transition = "enter"
		}
		if _, err := s.Pool.Exec(ctx, `
			UPDATE fence_states SET last_state=$1, last_event_time=$2::timestamptz
			WHERE tenant_id=$3 AND fence_id=$4::uuid AND device_id=$5::uuid`,
			want, recTS, tenantID, h.id, deviceID); err != nil {
			return err
		}
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO alarms (tenant_id, fence_id, device_id, transition, occurred_at)
			VALUES ($1,$2::uuid,$3::uuid,$4,$5::timestamptz)
			ON CONFLICT (tenant_id, fence_id, device_id, occurred_at, transition) DO NOTHING`,
			tenantID, h.id, deviceID, transition, recTS); err != nil {
			return err
		}
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO outbox_events (tenant_id, type, device_id, payload)
			VALUES ($1,'alarm.created',$2::uuid,
			        jsonb_build_object('fence_id',$3::text,'transition',$4::text))`,
			tenantID, deviceID, h.id, transition); err != nil {
			return err
		}
	}
	return nil
}
