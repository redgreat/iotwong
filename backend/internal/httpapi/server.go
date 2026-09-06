package httpapi

import (
	"context"
	"net/http"
	"time"

	"iotwong/backend/internal/store"
)

// ReadinessCheck reports whether one required subsystem is ready.
type ReadinessCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

// Config carries API runtime options.
type Config struct {
	SecureCookies     bool // true when served over HTTPS (production)
	DisableLoginLimit bool // brute-force limiter disabled outside production (dev/tests)
}

// Server holds dependencies of the HTTP v1 API and its routes.
type Server struct {
	st        *store.Store
	cfg       Config
	readiness []ReadinessCheck
}

// NewServer creates an API server without store/config (health-only, tests).
func NewServer() *Server { return &Server{} }

// NewAPIServer creates the full API server wired to the store.
func NewAPIServer(st *store.Store, cfg Config) *Server {
	s := &Server{st: st, cfg: cfg}
	s.RegisterReadiness(ReadinessCheck{Name: "db", Check: func(ctx context.Context) error {
		return st.Pool.Ping(ctx)
	}})
	return s
}

// RegisterReadiness adds a required dependency check for /health/ready.
func (s *Server) RegisterReadiness(c ReadinessCheck) { s.readiness = append(s.readiness, c) }

// Handler returns the fully-wrapped route handler for the /api/v1 tree.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health/live", s.handleLive)
	mux.HandleFunc("GET /api/v1/health/ready", s.handleReady)

	// auth + session-scoped resources
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.csrf(s.requireAuth(s.handleLogout)))
	mux.HandleFunc("GET /api/v1/auth/me", s.requireAuth(s.handleMe))

	mux.HandleFunc("GET /api/v1/projects", s.requireAuth(s.handleProjects))
	mux.HandleFunc("GET /api/v1/devices", s.requireAuth(s.handleDevices))
	mux.HandleFunc("GET /api/v1/devices/{id}", s.requireAuth(s.handleDevice))
	mux.HandleFunc("PATCH /api/v1/devices/{id}", s.csrf(s.requireAdmin(s.handleDevicePatch)))
	mux.HandleFunc("GET /api/v1/devices/{id}/positions", s.requireAuth(s.handlePositions))
	mux.HandleFunc("GET /api/v1/devices/{id}/track", s.requireAuth(s.handleTrack))
	mux.HandleFunc("GET /api/v1/events", s.requireAuth(s.handleEvents))

	mux.HandleFunc("GET /api/v1/fences", s.requireAuth(s.handleListFences))
	mux.HandleFunc("POST /api/v1/fences", s.csrf(s.requireAdmin(s.handleCreateFence)))
	mux.HandleFunc("PATCH /api/v1/fences/{id}", s.csrf(s.requireAdmin(s.handleUpdateFence)))
	mux.HandleFunc("DELETE /api/v1/fences/{id}", s.csrf(s.requireAdmin(s.handleDeleteFence)))
	mux.HandleFunc("GET /api/v1/alarms", s.requireAuth(s.handleListAlarms))
	mux.HandleFunc("POST /api/v1/alarms/{id}/ack", s.csrf(s.requireAdmin(s.handleAckAlarm)))

	// 用户管理（admin；docs/04-contracts.md）
	mux.HandleFunc("GET /api/v1/users", s.requireAdmin(s.handleListUsers))
	mux.HandleFunc("POST /api/v1/users", s.csrf(s.requireAdmin(s.handleCreateUser)))
	mux.HandleFunc("POST /api/v1/users/{id}/reset-password", s.csrf(s.requireAdmin(s.handleResetUserPassword)))

	// Every unmounted path must answer with the error envelope, not the SPA.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, r, http.StatusNotFound, codeNotFound, "not found", nil)
	})

	return recoverMiddleware(accessLogMiddleware(requestIDMiddleware(mux)))
}

// readinessTimeout bounds each /health/ready evaluation.
const readinessTimeout = 3 * time.Second

type readyComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if len(s.readiness) == 0 {
		// Nothing required yet: the process is up and no mandatory
		// dependency has failed. Database readiness joins at T02.
		ok(w, r, map[string]any{"ready": true, "components": []readyComponent{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	components := make([]readyComponent, 0, len(s.readiness))
	ready := true
	for _, rc := range s.readiness {
		c := readyComponent{Name: rc.Name, Status: "ok"}
		if err := rc.Check(ctx); err != nil {
			c.Status = "error"
			c.Error = err.Error()
			ready = false
		}
		components = append(components, c)
	}
	if !ready {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy,
			"service not ready", map[string]any{"components": components})
		return
	}
	ok(w, r, map[string]any{"ready": true, "components": components})
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	// Process liveness only; never leaks internal connection state.
	ok(w, r, map[string]any{"status": "ok"})
}
