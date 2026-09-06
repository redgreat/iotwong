// Package racebox imports a bounded, read-only sample from the legacy
// eadm.public.lc_racebox table into the iotwong project.
//
// Contract (docs/03-data-design.md 导入规范):
//   - 原库只读限量：连接用只读角色（CONNECT+SELECT），默认 dry-run，
//     批量窗口、statement_timeout、显式列、只读事务；
//   - 默认最多一个批次、每窗口 5000 行、总行数上限 --max-rows；
//   - WGS84 十进制度坐标直接入库（本地实测确认样本为十进制 WGS84）；
//   - 时区必须显式给出（默认拒绝）；速度/精度单位未经上游协议核验前一律
//     NULL 存储（位置与航向同规则：未知即 NULL，不猜测换算）；
//   - event_id = racebox:<batch短号>:<源id>，position_id 为该值的 UUIDv5，
//     recorded_at 由源列还原 → 重跑天然幂等（ON CONFLICT DO NOTHING）。
package racebox

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SourceBatch is one imp_stamp batch in lc_racebox.
type SourceBatch struct {
	Stamp string `json:"stamp"`
	Rows  int64  `json:"rows"`
}

// ListBatches returns imp_stamp batches with row counts (read-only).
func ListBatches(ctx context.Context, src *pgx.Conn) ([]SourceBatch, error) {
	rows, err := src.Query(ctx, `
		SELECT imp_stamp::text, count(*)::bigint
		FROM lc_racebox GROUP BY 1 ORDER BY 2, 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceBatch
	for rows.Next() {
		var b SourceBatch
		if err := rows.Scan(&b.Stamp, &b.Rows); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BatchStats is the units/validity audit of one batch (docs/03 单位核验报告)。
// 只输出统计与结论性标志；不把未核验单位当作已确认。
type BatchStats struct {
	Stamp       string        `json:"stamp"`
	Rows        int64         `json:"rows"`
	MinYear     int           `json:"min_year"`
	MaxYear     int           `json:"max_year"`
	MinNs       int64         `json:"min_nanoseconds"`
	MaxNs       int64         `json:"max_nanoseconds"`
	MinLng      *float64      `json:"min_longitude"`
	MaxLng      *float64      `json:"max_longitude"`
	MinLat      *float64      `json:"min_latitude"`
	MaxLat      *float64      `json:"max_latitude"`
	SpeedMin    *float64      `json:"raw_speed_min"`
	SpeedMax    *float64      `json:"raw_speed_max"`
	AccuracyMin *float64      `json:"raw_accuracy_min"`
	AccuracyMax *float64      `json:"raw_accuracy_max"`
	FixStatuses map[int]int64 `json:"fix_status_counts"`
	Assumptions []string      `json:"assumptions"`
}

// InspectBatch computes BatchStats (read-only, one aggregate query +
// fix_status counts). Does not conclude units.
func InspectBatch(ctx context.Context, src *pgx.Conn, stamp string) (*BatchStats, error) {
	s := &BatchStats{Stamp: stamp, FixStatuses: map[int]int64{}}
	row := src.QueryRow(ctx, `
		SELECT count(*)::bigint,
		       min(year), max(year), min(nanoseconds)::bigint, max(nanoseconds)::bigint,
		       min(longitude), max(longitude), min(latitude), max(latitude),
		       min(speed), max(speed), min(horizontal_accuracy), max(horizontal_accuracy)
		FROM lc_racebox WHERE imp_stamp = $1`, stamp)
	var minYear, maxYear int
	if err := row.Scan(&s.Rows, &minYear, &maxYear, &s.MinNs, &s.MaxNs,
		&s.MinLng, &s.MaxLng, &s.MinLat, &s.MaxLat,
		&s.SpeedMin, &s.SpeedMax, &s.AccuracyMin, &s.AccuracyMax); err != nil {
		return nil, err
	}
	s.MinYear, s.MaxYear = minYear, maxYear
	rows, err := src.Query(ctx, `SELECT fix_status, count(*)::bigint
		FROM lc_racebox WHERE imp_stamp=$1 GROUP BY 1`, stamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k int
		var c int64
		if err := rows.Scan(&k, &c); err != nil {
			return nil, err
		}
		s.FixStatuses[k] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.Assumptions = []string{
		"coordinates: 十进制度 WGS84（本地样本实测）直接入库",
		"speed/heading/accuracy 单位未经上游处理程序核验 → 目标库存 NULL，不换算",
		"fix_status 单值无法区分有效/无效，不作有效性门禁，仅记录分布",
		"recorded_at 由源 year..second+nanoseconds 还原；时区来自 --source-timezone（必须显式）",
	}
	return s, nil
}

// FixedOffset parses --source-timezone. Asia/Shanghai is UTC+8 without DST;
// the minimal alpine runtime has no tzdata, so only fixed offsets are allowed.
func FixedOffset(name string) (*time.Location, error) {
	switch name {
	case "Asia/Shanghai", "UTC+8", "+08:00", "CST":
		return time.FixedZone("Asia/Shanghai", 8*3600), nil
	case "UTC", "Etc/UTC", "Z":
		return time.UTC, nil
	}
	// Allow generic +HH:MM / -HH:MM
	if len(name) == 6 && (name[0] == '+' || name[0] == '-') && name[3] == ':' {
		var h, m int
		if _, err := fmt.Sscanf(name[1:], "%02d:%02d", &h, &m); err != nil {
			return nil, fmt.Errorf("invalid tz %q", name)
		}
		sec := h*3600 + m*60
		if name[0] == '-' {
			sec = -sec
		}
		return time.FixedZone(name, sec), nil
	}
	return nil, fmt.Errorf("unsupported source-timezone %q (use Asia/Shanghai or ±HH:MM)", name)
}

// UUIDv5 derives a deterministic UUIDv5 (RFC 4122, SHA-1 namespace variant)
// for idempotency keys.
func UUIDv5(namespace string, name string) string {
	h := sha1.New()
	h.Write([]byte(namespace))
	h.Write([]byte{0})
	h.Write([]byte(name))
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x50 // version 5
	sum[8] = (sum[8] & 0x3f) | 0x80 // RFC 4122 variant
	s := hex.EncodeToString(sum[:16])
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
}
