// Package dbmigrate applies versioned SQL migrations from db/migrations
// (docs/03-data-design.md: MUST versioned SQL migration, verified on a
// scratch database first, executed by the project owner role).
package dbmigrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// Migration is one ordered, versioned SQL file.
type Migration struct {
	Version  int64
	Name     string
	Path     string
	Body     string
	Checksum string
}

var fileRe = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.sql$`)

// Load reads *.sql migrations from dir in numeric order. Non-matching files
// are ignored so editors can keep scratch SQL next to the folder.
func Load(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var out []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := fileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse version %q: %w", m[1], err)
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(body)
		out = append(out, Migration{
			Version:  v,
			Name:     m[2],
			Path:     e.Name(),
			Body:     string(body),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Applied queries which versions already ran.
func Applied(ctx context.Context, conn *pgx.Conn) (map[int64]bool, error) {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    bigint PRIMARY KEY,
		name       text NOT NULL,
		checksum   text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return nil, fmt.Errorf("ensure schema_migrations: %w", err)
	}
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied: %w", err)
	}
	defer rows.Close()
	done := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		done[v] = true
	}
	return done, rows.Err()
}

// Apply runs pending migrations in order inside a session advisory lock.
// Each migration executes in its own transaction; a failure aborts the run
// and leaves previously applied files untouched.
func Apply(ctx context.Context, conn *pgx.Conn, migrations []Migration) ([]string, error) {
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('iotwong_migrate'))`); err != nil {
		return nil, fmt.Errorf("acquire advisory lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtext('iotwong_migrate'))`)

	done, err := Applied(ctx, conn)
	if err != nil {
		return nil, err
	}

	var applied []string
	for _, m := range migrations {
		if done[m.Version] {
			continue
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("begin %s: %w", m.Path, err)
		}
		rollback := true
		defer func() {
			if rollback {
				_ = tx.Rollback(context.WithoutCancel(ctx))
			}
		}()

		if _, err := tx.Exec(ctx, m.Body); err != nil {
			return applied, fmt.Errorf("apply %s: %w", m.Path, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
			m.Version, m.Name, m.Checksum); err != nil {
			return applied, fmt.Errorf("record %s: %w", m.Path, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("commit %s: %w", m.Path, err)
		}
		rollback = false
		applied = append(applied, m.Path)
	}
	return applied, nil
}
