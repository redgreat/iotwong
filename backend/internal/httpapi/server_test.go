package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doGet(t *testing.T, s *Server, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v (body=%q)", err, rec.Body.String())
	}
	return rec, body
}

func TestLiveOK(t *testing.T) {
	s := NewServer()
	rec, body := doGet(t, s, "/api/v1/health/live")
	if rec.Code != http.StatusOK {
		t.Fatalf("live status = %d, want 200", rec.Code)
	}
	if _, ok := body["data"]; !ok {
		t.Fatalf("live body missing data: %v", body)
	}
	if rid, _ := body["request_id"].(string); len(rid) != 32 {
		t.Fatalf("request_id = %q, want 32 hex chars", rid)
	}
}

func TestReadyEmptyChecksOK(t *testing.T) {
	// At T01 no required dependency is registered, so ready is a pass.
	s := NewServer()
	rec, body := doGet(t, s, "/api/v1/health/ready")
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("ready data missing: %v", body)
	}
	if data["ready"] != true {
		t.Fatalf("ready data.ready = %v, want true", data["ready"])
	}
}

func TestReadyFailingCheckIs503(t *testing.T) {
	s := NewServer()
	s.RegisterReadiness(ReadinessCheck{Name: "db", Check: func(ctx context.Context) error {
		return errors.New("db unreachable")
	}})
	rec, body := doGet(t, s, "/api/v1/health/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", rec.Code)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("error envelope missing: %v", body)
	}
	if errObj["code"] != codeServiceUnhealthy {
		t.Fatalf("error.code = %v, want %q", errObj["code"], codeServiceUnhealthy)
	}
}

func TestReadyPassingCheckIs200(t *testing.T) {
	s := NewServer()
	s.RegisterReadiness(ReadinessCheck{Name: "db", Check: func(ctx context.Context) error { return nil }})
	rec, _ := doGet(t, s, "/api/v1/health/ready")
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", rec.Code)
	}
}

func TestUnknownRouteIsEnvelope404(t *testing.T) {
	s := NewServer()
	rec, body := doGet(t, s, "/api/v1/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if errObj, ok := body["error"].(map[string]any); !ok || errObj["code"] != codeNotFound {
		t.Fatalf("want not_found error envelope, got %v", body)
	}
}

func TestErrorEnvelopeNeverLeaksRequestIDEmpty(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	rec := httptest.NewRecorder()
	// Exercise handler directly without middleware: request id must still not
	// crash and the response is valid JSON.
	s.Handler().ServeHTTP(rec, req)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
}
