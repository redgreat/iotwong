// Command server runs the iotwong HTTP v1 API.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iotwong/backend/internal/buildinfo"
	"iotwong/backend/internal/httpapi"
	"iotwong/backend/internal/store"
)

func main() {
	addr := flag.String("addr", envOr("HTTP_ADDR", ":8080"), "listen address (HTTP_ADDR)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.NewPool(ctx)
	if err != nil {
		slog.Error("db pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	st := store.NewStore(pool)

	cfg := httpapi.Config{SecureCookies: os.Getenv("APP_ENV") == "production", DisableLoginLimit: os.Getenv("APP_ENV") != "production"}
	srv := httpapi.NewAPIServer(st, cfg)

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", *addr, "version", buildinfo.Version, "commit", buildinfo.Commit)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
