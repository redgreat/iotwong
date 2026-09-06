// Package httpapi implements the self-hosted HTTP v1 API surface
// (docs/04-contracts.md, docs/api/openapi.yaml).
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Success payloads follow  {data:...,request_id:...}.
// Failure payloads follow {error:{code,message,details?},request_id}.
// Normalized error codes are documented in docs/api/openapi.yaml.

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type envelope struct {
	Data      any        `json:"data,omitempty"`
	Error     *errorBody `json:"error,omitempty"`
	RequestID string     `json:"request_id"`
}

type ctxKey int

const (
	requestIDKey ctxKey = iota
	ctxKeyUser
	ctxKeyTenantID
	ctxKeyRole
)

// withRequestID returns a context carrying the request id.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// requestIDFrom extracts the request id set by the requestID middleware.
func requestIDFrom(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey).(string)
	return id
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any, details any, errCode, message string) {
	e := envelope{Data: data, RequestID: requestIDFrom(r)}
	if errCode != "" {
		e.Data = nil
		e.Error = &errorBody{Code: errCode, Message: message, Details: details}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(e); err != nil {
		slog.Error("write json response", "err", err)
	}
}

// ok writes a 200 data envelope.
func ok(w http.ResponseWriter, r *http.Request, data any) {
	writeJSON(w, r, http.StatusOK, data, nil, "", "")
}

// fail writes an error envelope with a normalized code.
func fail(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	writeJSON(w, r, status, nil, details, code, message)
}

const (
	codeInvalidRequest   = "invalid_request"
	codeUnauthorized     = "unauthorized"
	codeForbidden        = "forbidden"
	codeNotFound         = "not_found"
	codeConflict         = "conflict"
	codeRateLimited      = "rate_limited"
	codeUpstreamDown     = "upstream_unavailable"
	codeServiceUnhealthy = "service_unavailable"
)
