package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// requestID generates a random hex request id used for traceability.
func requestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is effectively unrecoverable; a fixed suffix
		// keeps requests traceable while the error is surfaced in logs.
		slog.Error("crypto/rand unavailable for request id", "err", err)
		return "rand-fail-" + time.Now().UTC().Format("20060102150405")
	}
	return hex.EncodeToString(b[:])
}

// requestIDMiddleware assigns a request id, echoed in every response.
// It never logs authorization headers or query tokens.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), requestID())))
	})
}

// accessLogMiddleware logs one line per request without secrets
// (method/path/status/duration/request_id only).
func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestIDFrom(r),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer so SSE streaming works through
// the logging wrapper.
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// recoverMiddleware converts handler panics into a 500 error envelope so a
// single bad request cannot take the process down.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("handler panic", "panic", rec, "request_id", requestIDFrom(r))
				fail(w, r, http.StatusInternalServerError, codeServiceUnhealthy,
					"internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
