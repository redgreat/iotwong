package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

func parseCursor(c string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("bad cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return ts, parts[1], nil
}

// PositionRow is one stored position (SI fields, WGS84 point).
type PositionRow struct {
	RecordedAt time.Time `json:"recorded_at"`
	PositionID string    `json:"-"`
	Longitude  float64   `json:"longitude"`
	Latitude   float64   `json:"latitude"`
	SpeedMPS   *float64  `json:"speed_mps"`
	HeadingDeg *float64  `json:"heading_deg"`
	AccuracyM  *float64  `json:"accuracy_m"`
}

// ListPositions returns device positions in [from,to) UTC ordered by
// (recorded_at, position_id) with cursor support. Cursor format:
// base64url("<recorded_at RFC3339Nano>|<position_id>").
func (s *Store) ListPositions(ctx context.Context, tenantID, deviceID string,
	from, to time.Time, cursor string, limit int) ([]PositionRow, string, error) {
	if limit <= 0 {
		limit = 5000
	}
	if limit > 5000 {
		limit = 5000
	}
	q := `
		SELECT recorded_at, position_id::text, ST_X(point), ST_Y(point),
		       speed_mps, heading_deg, accuracy_m
		FROM positions
		WHERE tenant_id=$1 AND device_id=$2 AND recorded_at >= $3 AND recorded_at < $4`
	args := []any{tenantID, deviceID, from, to}
	n := 5
	if cursor != "" {
		ts, id, err := parseCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		q += fmt.Sprintf(" AND (recorded_at, position_id) > ($%d, $%d::uuid)", n, n+1)
		args = append(args, ts, id)
		n += 2
	}
	q += fmt.Sprintf(" ORDER BY recorded_at, position_id LIMIT %d", limit+1)
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []PositionRow
	for rows.Next() {
		var r PositionRow
		var pid string
		var sp, hd, acc *float64
		if err := rows.Scan(&r.RecordedAt, &pid, &r.Longitude, &r.Latitude, &sp, &hd, &acc); err != nil {
			return nil, "", err
		}
		r.PositionID = pid
		r.SpeedMPS, r.HeadingDeg, r.AccuracyM = sp, hd, acc
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > limit {
		out = out[:limit]
		last := out[len(out)-1]
		next = EncodeCursor(last.RecordedAt.Format(time.RFC3339Nano), last.PositionID)
	}
	return out, next, nil
}

// TrackPoint is a trajectory vertex with time.
type TrackPoint struct {
	RecordedAt  time.Time `json:"recorded_at"`
	Coordinates []float64 `json:"coordinates"` // [lng, lat]
}

// TrackSegment groups consecutive points; gaps longer than gapSec break
// segments (docs/03-data-design.md 跨5分钟空洞默认断段).
type TrackSegment struct {
	Points []TrackPoint `json:"points"`
}

// TrackResult is the /track response body.
type TrackResult struct {
	Segments      []TrackSegment `json:"segments"`
	RawCount      int            `json:"raw_count"`
	ReturnedCount int            `json:"returned_count"`
	Simplified    bool           `json:"simplified"`
}

// Track loads device positions in [from,to), applies time-gap segmentation
// and optional bounded downsampling (maxPoints, keeps first/last per segment).
func (s *Store) Track(ctx context.Context, tenantID, deviceID string,
	from, to time.Time, maxPoints int, gapSec float64) (*TrackResult, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT recorded_at, ST_X(point), ST_Y(point)
		FROM positions
		WHERE tenant_id=$1 AND device_id=$2 AND recorded_at >= $3 AND recorded_at < $4
		ORDER BY recorded_at`, tenantID, deviceID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &TrackResult{Segments: []TrackSegment{}} // empty window must be [], not null
	var cur *TrackSegment
	flush := func() {
		if cur != nil && len(cur.Points) > 0 {
			res.Segments = append(res.Segments, *cur)
		}
		cur = nil
	}
	for rows.Next() {
		var t time.Time
		var lng, lat float64
		if err := rows.Scan(&t, &lng, &lat); err != nil {
			return nil, err
		}
		res.RawCount++
		if cur != nil {
			last := cur.Points[len(cur.Points)-1]
			if t.Sub(last.RecordedAt) > time.Duration(gapSec*float64(time.Second)) {
				flush()
			}
		}
		if cur == nil {
			cur = &TrackSegment{}
		}
		cur.Points = append(cur.Points, TrackPoint{RecordedAt: t, Coordinates: []float64{lng, lat}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	flush()

	res.ReturnedCount = res.RawCount
	if maxPoints <= 0 {
		maxPoints = 5000
	}
	if res.RawCount > maxPoints {
		res.Simplified = true
		step := float64(res.RawCount) / float64(maxPoints)
		var kept []TrackSegment
		for _, seg := range res.Segments {
			if len(seg.Points) <= 2 {
				kept = append(kept, seg)
				continue
			}
			ns := TrackSegment{}
			pos := 0.0
			for i := range seg.Points {
				if i == 0 || i == len(seg.Points)-1 || float64(i) >= pos {
					ns.Points = append(ns.Points, seg.Points[i])
					pos += step
				}
			}
			kept = append(kept, ns)
		}
		res.Segments = kept
		res.ReturnedCount = 0
		for _, seg := range kept {
			res.ReturnedCount += len(seg.Points)
		}
	}
	return res, nil
}
