// Command seed idempotently creates the local admin account and a local
// source device used by T04 end-to-end verification.
//
// Usage: SEED_PASSWORD=<password> go run ./cmd/seed [--login admin]
// The password is read from the environment only; never passed on argv.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"iotwong/backend/internal/auth"
	"iotwong/backend/internal/store"
)

func main() {
	login := flag.String("login", "admin", "login of the local user")
	tenant := flag.String("tenant", "local", "tenant name for the seed")
	role := flag.String("role", "admin", "membership role: admin | viewer")
	reset := flag.Bool("reset", false, "reset the password even if the user exists")
	flag.Parse()

	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		log.Fatal("SEED_PASSWORD is required (environment only)")
	}

	ctx := context.Background()
	pool, err := store.NewPool(ctx)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	st := store.NewStore(pool)

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	created, err := st.EnsureAdmin(ctx, *tenant, *login, hash, *role, *reset)
	if err != nil {
		log.Fatalf("ensure admin: %v", err)
	}
	deviceID, err := st.EnsureLocalDevice(ctx, *tenant, "dev-1", "本地演示设备", 30)
	if err != nil {
		log.Fatalf("ensure local device: %v", err)
	}
	state := "updated"
	if created {
		state = "created"
	}
	fmt.Printf("seed: user %q role=%s (%s), tenant %q, local device dev-1 id=%s\n", *login, *role, state, *tenant, deviceID)
}
