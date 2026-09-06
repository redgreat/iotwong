// Command migrate applies versioned SQL migrations (db/migrations) to a
// PostgreSQL database. Connection comes from standard PG* environment
// variables (PGHOST/PGPORT/PGDATABASE/PGUSER/PGPASSWORD).
//
// Usage: migrate --dir db/migrations [--dry-run]
// Migrations are executed by the role in PGUSER (project owner for DDL);
// the app runtime role must never run this binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"iotwong/backend/internal/dbmigrate"
)

func main() {
	dir := flag.String("dir", "db/migrations", "directory containing <NNN>_<name>.sql files")
	dryRun := flag.Bool("dry-run", false, "list pending migrations without applying")
	flag.Parse()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "") // config from PG* env
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	migrations, err := dbmigrate.Load(*dir)
	if err != nil {
		log.Fatalf("load migrations: %v", err)
	}

	done, err := dbmigrate.Applied(ctx, conn)
	if err != nil {
		log.Fatalf("applied: %v", err)
	}

	var pending []string
	for _, m := range migrations {
		if !done[m.Version] {
			pending = append(pending, fmt.Sprintf("%04d_%s.sql", m.Version, m.Name))
		}
	}
	fmt.Printf("migrate: %d known, %d pending\n", len(migrations), len(pending))
	if *dryRun {
		for _, p := range pending {
			fmt.Println("  pending:", p)
		}
		return
	}

	applied, err := dbmigrate.Apply(ctx, conn, migrations)
	if err != nil {
		log.Fatalf("apply failed: %v", err)
	}
	if len(applied) == 0 {
		fmt.Println("migrate: nothing to apply")
		return
	}
	for _, a := range applied {
		fmt.Println("  applied:", a)
	}
}
