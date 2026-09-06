package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Project lists one project visible to a tenant.
type Project struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	ExternalID *string `json:"external_id"`
	Name       string  `json:"name"`
}

// ListProjects returns projects of a tenant (optional source filter).
func (s *Store) ListProjects(ctx context.Context, tenantID, source string) ([]Project, error) {
	q := `SELECT id::text, source, external_id, name FROM projects WHERE tenant_id=$1`
	args := []any{tenantID}
	if source != "" {
		q += ` AND source=$2`
		args = append(args, source)
	}
	q += ` ORDER BY source, name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Source, &p.ExternalID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeviceListItem is the API device DTO (docs/04-contracts.md).
type DeviceListItem struct {
	ID                  string    `json:"id"`
	Source              string    `json:"source"`
	ExternalID          string    `json:"external_id"`
	Name                string    `json:"name"`
	ProjectID           string    `json:"project_id"`
	CommunicationStatus string    `json:"communication_status"`
	LastSeenAt          *string   `json:"last_seen_at"`
	LastPositionAt      *string   `json:"last_position_at"`
	Position            *GeoPoint `json:"position"`
	LocationSource      *string   `json:"location_source"`
	AccuracyM           *float64  `json:"accuracy_m"`
	BatteryPct          *float64  `json:"battery_pct"`
	SpeedMPS            *float64  `json:"speed_mps"`
	IsDemo              bool      `json:"is_demo"`
	SyncStatus          *string   `json:"sync_status"`
}

// GeoPoint is a WGS84 coordinate.
type GeoPoint struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	CRS       string  `json:"crs"`
}

// CommunicationStatus decides online/offline/unknown for self-hosted
// devices: online when last_seen is within max(3*interval_s,180s).
func CommunicationStatus(expectedIntervalS *int, lastSeen *time.Time, now time.Time) string {
	if lastSeen == nil {
		return "unknown"
	}
	interval := 180 * time.Second
	if expectedIntervalS != nil && *expectedIntervalS > 0 {
		if iv := time.Duration(3**expectedIntervalS) * time.Second; iv > interval {
			interval = iv
		}
	}
	if now.Sub(*lastSeen) <= interval {
		return "online"
	}
	return "offline"
}

// DeviceFilter narrows device listing.
type DeviceFilter struct {
	Source    string
	ProjectID string
	Q         string
	Status    string // online|offline|unknown|""
	Limit     int    // default 50, max 200
	Cursor    string // opaque base64 "name|uuid"
}

// DevicePage is one page with a next cursor.
type DevicePage struct {
	Items      []DeviceListItem `json:"items"`
	NextCursor *string          `json:"next_cursor,omitempty"`
}

// Cursor holds a decoded position in the (name,id) order.
type Cursor struct{ Name, ID string }

// EncodeCursor and DecodeCursor implement the contract cursor scheme
// (排序值+指纹防篡改在 T04 简化版：仅排序值，文档注明).
func EncodeCursor(name, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name + "|" + id))
}
func DecodeCursor(c string) (Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return Cursor{}, fmt.Errorf("bad cursor")
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return Cursor{}, fmt.Errorf("bad cursor")
	}
	return Cursor{Name: parts[0], ID: parts[1]}, nil
}

// ListDevices lists tenant devices with optional filters; latest state comes
// from device_latest; status computed with CommunicationStatus.
func (s *Store) ListDevices(ctx context.Context, tenantID string, f DeviceFilter) (*DevicePage, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	limit := f.Limit + 1 // detect next page

	q := `
		SELECT d.id::text, d.source, d.external_id, d.name, d.project_id::text,
		       COALESCE(dl.last_seen_at,NULL), COALESCE(dl.last_position_at,NULL),
		       ST_X(dl.point), ST_Y(dl.point), dl.location_source,
		       dl.speed_mps, dl.accuracy_m, dl.battery_pct, d.expected_report_interval_s
		FROM devices d
		LEFT JOIN device_latest dl ON dl.tenant_id = d.tenant_id AND dl.device_id = d.id
		WHERE d.tenant_id=$1`
	args := []any{tenantID}
	i := 2
	if f.Source != "" {
		q += fmt.Sprintf(" AND d.source=$%d", i)
		args = append(args, f.Source)
		i++
	}
	if f.ProjectID != "" {
		q += fmt.Sprintf(" AND d.project_id=$%d::uuid", i)
		args = append(args, f.ProjectID)
		i++
	}
	if f.Q != "" {
		q += fmt.Sprintf(` AND (d.name ILIKE $%d OR d.external_id ILIKE $%d)`, i, i)
		args = append(args, "%"+f.Q+"%")
		i++
	}
	if f.Cursor != "" {
		cur, err := DecodeCursor(f.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor")
		}
		q += fmt.Sprintf(` AND (d.name, d.id::text) > ($%d, $%d)`, i, i+1)
		args = append(args, cur.Name, cur.ID)
		i += 2
	}
	q += ` ORDER BY d.name, d.id LIMIT ` + fmt.Sprintf("%d", limit)

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var items []DeviceListItem
	for rows.Next() {
		var it DeviceListItem
		var lastSeen, lastPos *time.Time
		var lng, lat *float64
		var locSource *string
		var sp, acc, batt *float64
		var interval *int
		if err := rows.Scan(&it.ID, &it.Source, &it.ExternalID, &it.Name, &it.ProjectID,
			&lastSeen, &lastPos, &lng, &lat, &locSource,
			&sp, &acc, &batt, &interval); err != nil {
			return nil, err
		}
		it.IsDemo = it.Source == "racebox_demo"
		if it.IsDemo {
			demoSync := "historical"
			it.SyncStatus = &demoSync
		}
		it.CommunicationStatus = CommunicationStatus(interval, lastSeen, now)
		if lastSeen != nil {
			v := lastSeen.UTC().Format(time.RFC3339Nano)
			it.LastSeenAt = &v
		}
		if lastPos != nil {
			v := lastPos.UTC().Format(time.RFC3339Nano)
			it.LastPositionAt = &v
		}
		if lng != nil && lat != nil && lastPos != nil {
			it.Position = &GeoPoint{Longitude: *lng, Latitude: *lat, CRS: "WGS84"}
		}
		it.LocationSource = locSource
		it.SpeedMPS = sp
		it.AccuracyM = acc
		it.BatteryPct = batt
		// status filter applied in Go (online computed after scan)
		if f.Status != "" && it.CommunicationStatus != f.Status {
			continue
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	more := len(items) > f.Limit
	if more {
		items = items[:f.Limit]
	}
	page := &DevicePage{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		c := EncodeCursor(last.Name, last.ID)
		page.NextCursor = &c
	}
	return page, nil
}
