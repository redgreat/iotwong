package racebox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Options configures one import pass.
type Options struct {
	Stamp      string // imp_stamp batch uuid string
	MaxRows    int64
	Window     int
	Timezone   *time.Location
	DryRun     bool
	TenantName string // defaults "local"
	SpeedUnit  string // only "unknown" accepted until units are verified
}

// Summary reports one pass; printed as JSON by the CLI (no secrets).
type Summary struct {
	Batch            string           `json:"batch"`
	DryRun           bool             `json:"dry_run"`
	Timezone         string           `json:"source_timezone"`
	SpeedUnit        string           `json:"speed_unit"`
	MaxRows          int64            `json:"max_rows"`
	SourceRows       int64            `json:"source_rows_read"`
	NewWritten       int64            `json:"rows_new_written"`
	SkippedDupes     int64            `json:"rows_already_present"`
	Rejected         int64            `json:"rows_rejected"`
	RejectionReasons map[string]int64 `json:"rejection_reasons"`
	Assumptions      []string         `json:"assumptions"`
	Project          string           `json:"project_external_id,omitempty"`
	Device           string           `json:"device_external_id,omitempty"`
	RunID            string           `json:"import_run_id,omitempty"`
	PriorRunDone     bool             `json:"prior_run_done"`
}

type rejectedRow struct {
	sourceID int64
	reason   string
}

// ValidateOptions sanity-checks documented assumptions before any I/O.
func ValidateOptions(o *Options) error {
	if o.Stamp == "" {
		return errors.New("--batch is required")
	}
	if strings.ToLower(o.SpeedUnit) != "unknown" {
		return fmt.Errorf("speed unit %q unverified: only \"unknown\" is accepted until source units are confirmed (docs/03)", o.SpeedUnit)
	}
	if o.Timezone == nil {
		return errors.New("--source-timezone is required (never assume server tz)")
	}
	if o.Window <= 0 {
		o.Window = 5000
	}
	if o.MaxRows <= 0 {
		o.MaxRows = 100000
	}
	if o.TenantName == "" {
		o.TenantName = "local"
	}
	return nil
}

type srcRow struct {
	id                     int64
	rec                    time.Time
	lng, lat               float64
	rawYear, rawMo, rawDay int
	rawNs                  int64
}

func (r srcRow) eventID(stamp8 string) string {
	return fmt.Sprintf("racebox:%s:%d", stamp8, r.id)
}

func (r srcRow) positionID(stamp8 string) string {
	return UUIDv5("iotwong:racebox", r.eventID(stamp8))
}

func (r srcRow) validate() string {
	if r.rawYear < 2000 || r.rawYear > 2100 {
		return "invalid_year"
	}
	if r.rawMo < 1 || r.rawMo > 12 || r.rawDay < 1 || r.rawDay > 31 {
		return "invalid_date"
	}
	if r.rawNs < 0 || r.rawNs >= 1_000_000_000 {
		return "invalid_nanoseconds"
	}
	if r.lng < -180 || r.lng > 180 || r.lat < -90 || r.lat > 90 {
		return "invalid_coordinates"
	}
	return ""
}

// readWindow reads up to limit rows strictly after cursor (windowed, explicit
// columns, bounded). The caller supplies an already read-only transaction.
func readWindow(ctx context.Context, q pgxQuerier, stamp string, cursor, limit int64) ([]srcRow, error) {
	rows, err := q.Query(ctx, `
		SELECT id, year, month, day, hour, minute, second, nanoseconds::bigint,
		       longitude::float8, latitude::float8
		FROM lc_racebox
		WHERE imp_stamp = $1 AND id > $2
		ORDER BY id
		LIMIT $3`, stamp, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []srcRow
	for rows.Next() {
		var r srcRow
		var y, mo, d, h, mi, s int
		if err := rows.Scan(&r.id, &y, &mo, &d, &h, &mi, &s, &r.rawNs, &r.lng, &r.lat); err != nil {
			return nil, err
		}
		r.rawYear, r.rawMo, r.rawDay = y, mo, d
		r.rec = time.Date(y, time.Month(mo), d, h, mi, s, int(r.rawNs), time.UTC)
		out = append(out, r)
	}
	return out, rows.Err()
}

type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Import runs the whole pass and returns a Summary (already printed).
func Import(ctx context.Context, src, dst *pgx.Conn, o *Options) (*Summary, error) {
	if err := ValidateOptions(o); err != nil {
		return nil, err
	}

	// Source connection: one read-only transaction with a bounded statement
	// timeout. The original database is never written.
	stx, err := src.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin read-only source tx: %w", err)
	}
	defer stx.Rollback(ctx) //nolint:errcheck
	if _, err := stx.Exec(ctx, `SET LOCAL statement_timeout = '60s'`); err != nil {
		return nil, err
	}

	sum := &Summary{
		Batch:            o.Stamp,
		DryRun:           o.DryRun,
		Timezone:         o.Timezone.String(),
		SpeedUnit:        "unknown",
		MaxRows:          o.MaxRows,
		RejectionReasons: map[string]int64{},
		Assumptions: []string{
			"source coordinates: decimal degrees WGS84 (local sample verified)",
			"speed/heading/accuracy units unverified by source handler -> stored NULL, no conversion",
			"fix_status is not used as a validity gate (uniform value per sample batch)",
		},
	}

	// ---- target identifiers + import_run resume point (skipped on dry-run) ----
	var tenantID, deviceID, deviceTag string
	var runID string
	var priorDone bool
	var resumeFrom int64
	stamp8 := safeTag(o.Stamp)

	if !o.DryRun {
		tx, err := dst.Begin(ctx)
		if err != nil {
			return nil, err
		}
		ok := false
		defer func() {
			if !ok {
				_ = tx.Rollback(ctx)
			}
		}()

		if err := tx.QueryRow(ctx,
			`SELECT id FROM tenants WHERE name=$1 LIMIT 1`, o.TenantName).Scan(&tenantID); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("lookup tenant: %w", err)
			}
			if err := tx.QueryRow(ctx,
				`INSERT INTO tenants (name) VALUES ($1) RETURNING id`, o.TenantName).Scan(&tenantID); err != nil {
				return nil, fmt.Errorf("create tenant: %w", err)
			}
		}
		var projectID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO projects (tenant_id, source, external_id, name)
			VALUES ($1,'racebox_demo',$2,$2)
			ON CONFLICT (tenant_id, source, external_id) DO UPDATE SET name=EXCLUDED.name
			RETURNING id`, tenantID, o.Stamp).Scan(&projectID); err != nil {
			return nil, fmt.Errorf("ensure project: %w", err)
		}
		deviceTag = "racebox-" + safeTag(o.Stamp)
		err = tx.QueryRow(ctx, `
			INSERT INTO devices (tenant_id, project_id, source, external_id, name)
			VALUES ($1,$2,'racebox_demo',$3,$3)
			ON CONFLICT (tenant_id, source, external_id) DO NOTHING
			RETURNING id`, tenantID, projectID, deviceTag).Scan(&deviceID)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(ctx,
				`SELECT id FROM devices WHERE tenant_id=$1 AND source='racebox_demo' AND external_id=$2`,
				tenantID, deviceTag).Scan(&deviceID); err != nil {
				return nil, fmt.Errorf("lookup device: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("ensure device: %w", err)
		}

		var status string
		var lastID int64
		err = tx.QueryRow(ctx, `
			SELECT id, status, COALESCE(last_source_id::bigint,0)
			FROM import_runs WHERE source='racebox_demo' AND source_batch_id=$1
			ORDER BY started_at DESC LIMIT 1`, o.Stamp).Scan(&runID, &status, &lastID)
		switch {
		case err == nil && status == "done":
			priorDone = true
		case err == nil:
			resumeFrom = lastID // running/interrupted: resume (断点)
		case errors.Is(err, pgx.ErrNoRows):
			cfg := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", o.Stamp, o.SpeedUnit, o.Timezone)))
			if err := tx.QueryRow(ctx, `
				INSERT INTO import_runs (source, source_batch_id, status, config_hash)
				VALUES ('racebox_demo',$1,'running',$2) RETURNING id`,
				o.Stamp, hex.EncodeToString(cfg[:])).Scan(&runID); err != nil {
				return nil, fmt.Errorf("create import_run: %w", err)
			}
		default:
			return nil, fmt.Errorf("read import_run: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit setup: %w", err)
		}
		ok = true
		sum.RunID = runID
		sum.PriorRunDone = priorDone
		sum.Project = o.Stamp
		sum.Device = deviceTag
	}

	// ---- windowed source read -> validate -> optional write ----
	cursor := resumeFrom
	var written, dup int64
	for sum.SourceRows < o.MaxRows {
		want := int64(o.Window)
		if remain := o.MaxRows - sum.SourceRows; remain < want {
			want = remain
		}
		rows, err := readWindow(ctx, stx, o.Stamp, cursor, want)
		if err != nil {
			return nil, fmt.Errorf("read source window: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		cursor = rows[len(rows)-1].id
		sum.SourceRows += int64(len(rows))

		var valid []srcRow
		var windowRej []rejectedRow
		for _, r := range rows {
			if why := r.validate(); why != "" {
				windowRej = append(windowRej, rejectedRow{sourceID: r.id, reason: why})
				sum.RejectionReasons[why]++
				sum.Rejected++
				continue
			}
			valid = append(valid, r)
		}

		if o.DryRun {
			written += int64(len(valid))
			if int64(len(rows)) < want {
				break
			}
			continue
		}

		tx, err := dst.Begin(ctx)
		if err != nil {
			return nil, err
		}
		var winWritten, winDup int64
		winErr := func() error {
			for _, r := range valid {
				rec := r.rec.In(o.Timezone).UTC()
				tag, err := tx.Exec(ctx, `
					INSERT INTO positions
					  (tenant_id, device_id, recorded_at, position_id, event_id, received_at,
					   point, location_source, raw_ref)
					VALUES ($1,$2,$3,$4,$5,now(),
					        ST_SetSRID(ST_MakePoint($6,$7),4326),'unknown',$8)
					ON CONFLICT (recorded_at, position_id) DO NOTHING`,
					tenantID, deviceID, rec, r.positionID(stamp8), r.eventID(stamp8),
					r.lng, r.lat, r.eventID(stamp8))
				if err != nil {
					return fmt.Errorf("insert position id=%d: %w", r.id, err)
				}
				if tag.RowsAffected() > 0 {
					winWritten++
				} else {
					winDup++
				}
			}
			for _, rj := range windowRej {
				if _, err := tx.Exec(ctx, `
					INSERT INTO import_rejections (run_id, source_id, reason)
					VALUES ($1,$2,$3)`, runID, rj.sourceID, rj.reason); err != nil {
					return fmt.Errorf("record rejection: %w", err)
				}
			}
			if !priorDone {
				if _, err := tx.Exec(ctx, `
					UPDATE import_runs
					SET status='running', last_source_id=$2,
					    rows_read=$3, rows_rejected=$4, rows_written=rows_written+$5
					WHERE id=$1`,
					runID, fmt.Sprintf("%d", cursor), sum.SourceRows, sum.Rejected, winWritten); err != nil {
					return fmt.Errorf("update import_run progress: %w", err)
				}
			}
			return nil
		}()
		if winErr != nil {
			_ = tx.Rollback(ctx)
			return nil, winErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit window: %w", err)
		}
		written += winWritten
		dup += winDup
		if int64(len(rows)) < want {
			break
		}
	}
	sum.NewWritten = written
	sum.SkippedDupes = dup

	if o.DryRun {
		printSummary(sum)
		return sum, nil
	}

	// ---- finalize: import_run done + device_latest forward-only ----
	ftx, err := dst.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if !priorDone {
		if _, err := ftx.Exec(ctx, `
			UPDATE import_runs SET status='done', finished_at=now(), last_source_id=$2
			WHERE id=$1`, runID, fmt.Sprintf("%d", cursor)); err != nil {
			_ = ftx.Rollback(ctx)
			return nil, fmt.Errorf("finalize run: %w", err)
		}
	}
	if _, err := ftx.Exec(ctx, `
		INSERT INTO device_latest (tenant_id, device_id, last_position_at, point, location_source)
		SELECT d.tenant_id, d.id, mx.m, mx.p, 'unknown'
		FROM devices d
		JOIN LATERAL (
			SELECT max(recorded_at) AS m,
			       (array_agg(point ORDER BY recorded_at DESC))[1] AS p
			FROM positions WHERE tenant_id=$1 AND device_id=$2
		) mx ON true
		WHERE d.tenant_id=$1 AND d.id=$2
		ON CONFLICT (tenant_id, device_id) DO UPDATE
		  SET last_position_at=EXCLUDED.last_position_at, point=EXCLUDED.point,
		      location_source='unknown', updated_at=now()
		  WHERE device_latest.last_position_at IS NULL
		     OR EXCLUDED.last_position_at > device_latest.last_position_at`,
		tenantID, deviceID); err != nil {
		_ = ftx.Rollback(ctx)
		return nil, fmt.Errorf("update device_latest: %w", err)
	}
	if err := ftx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("finalize commit: %w", err)
	}
	printSummary(sum)
	return sum, nil
}

func printSummary(s *Summary) {
	b, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(b))
}

func safeTag(stamp string) string {
	compact := strings.ReplaceAll(stamp, "-", "")
	if len(compact) >= 8 {
		return compact[:8]
	}
	return compact
}
