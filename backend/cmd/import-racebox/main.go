// Command import-racebox imports a bounded read-only sample from the legacy
// eadm.public.lc_racebox into iotwong (docs/03-data-design.md 导入规范).
//
// Modes:
//
//	--list-batches              list imp_stamp batches (read-only)
//	--stats --batch <stamp>     units/validity audit of one batch
//	--batch <stamp> ...         import; default dry-run, requires --apply
//
// Source env: EADM_HOST/PORT/DATABASE/USER/PASSWORD (read-only role).
// Target env: PG* or IOTWONG_APP_USER/PASSWORD (runtime app role; owner only
// runs migrations, never this importer's writes).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5"
	"iotwong/backend/internal/racebox"
)

func connFromEnv(host, port, db, user, password string) (*pgx.Conn, error) {
	if user == "" {
		return nil, fmt.Errorf("missing user for %s", db)
	}
	u := url.URL{Scheme: "postgres", Host: host + ":" + port, Path: "/" + db,
		User: url.UserPassword(user, password)}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return pgx.Connect(context.Background(), u.String())
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	listBatches := flag.Bool("list-batches", false, "list source batches (read-only)")
	stats := flag.Bool("stats", false, "print units/validity audit for --batch")
	batch := flag.String("batch", "", "imp_stamp uuid of the batch to import/inspect")
	apply := flag.Bool("apply", false, "write to iotwong (default dry-run)")
	maxRows := flag.Int64("max-rows", 100000, "total row cap for one run")
	window := flag.Int("window", 5000, "rows per source window / target transaction")
	tzName := flag.String("source-timezone", "", "required: Asia/Shanghai or +HH:MM")
	speedUnit := flag.String("speed-unit", "unknown", "only 'unknown' accepted until units verified")
	tenant := flag.String("tenant", "local", "target tenant name")
	flag.Parse()

	ctx := context.Background()

	srcHost := env("EADM_HOST", env("PGHOST", "127.0.0.1"))
	srcPort := env("EADM_PORT", env("PGPORT", "5432"))
	srcDB := env("EADM_DATABASE", "eadm")
	src, err := connFromEnv(srcHost, srcPort, srcDB, env("EADM_USER", ""), env("EADM_PASSWORD", ""))
	if err != nil {
		log.Fatalf("source connect (eadm): %v", err)
	}
	defer src.Close(ctx)

	if *listBatches {
		batches, err := racebox.ListBatches(ctx, src)
		if err != nil {
			log.Fatalf("list batches: %v", err)
		}
		for _, b := range batches {
			fmt.Printf("%s\t%d\n", b.Stamp, b.Rows)
		}
		return
	}
	if *stats {
		if *batch == "" {
			log.Fatal("--batch is required with --stats")
		}
		st, err := racebox.InspectBatch(ctx, src, *batch)
		if err != nil {
			log.Fatalf("inspect batch: %v", err)
		}
		fmt.Printf("batch=%s rows=%d years=%d..%d ns=[%d,%d] lng=[%v,%v] lat=[%v,%v] speed=[%v,%v] accuracy=[%v,%v] fix=%v\n",
			st.Stamp, st.Rows, st.MinYear, st.MaxYear, st.MinNs, st.MaxNs,
			floats(st.MinLng), floats(st.MaxLng), floats(st.MinLat), floats(st.MaxLat),
			floats(st.SpeedMin), floats(st.SpeedMax), floats(st.AccuracyMin), floats(st.AccuracyMax),
			st.FixStatuses)
		for _, a := range st.Assumptions {
			fmt.Println("assumption:", a)
		}
		return
	}

	loc, err := racebox.FixedOffset(*tzName)
	if err != nil {
		log.Fatalf("timezone: %v", err)
	}

	// Target connection: prefer the runtime app role credentials, fall back to PG*.
	dstUser := env("IOTWONG_APP_USER", env("PGUSER", ""))
	dstPass := env("IOTWONG_APP_PASSWORD", env("PGPASSWORD", ""))
	dst, err := connFromEnv(env("PGHOST", "127.0.0.1"), env("PGPORT", "5432"),
		env("PGDATABASE", "iotwong"), dstUser, dstPass)
	if err != nil {
		log.Fatalf("target connect (iotwong): %v", err)
	}
	defer dst.Close(ctx)

	opts := &racebox.Options{
		Stamp:      *batch,
		MaxRows:    *maxRows,
		Window:     *window,
		Timezone:   loc,
		DryRun:     !*apply,
		TenantName: *tenant,
		SpeedUnit:  *speedUnit,
	}
	if _, err := racebox.Import(ctx, src, dst, opts); err != nil {
		log.Fatalf("import: %v", err)
	}
}

func floats(f *float64) string {
	if f == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", *f)
}
