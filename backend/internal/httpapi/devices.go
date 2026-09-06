package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"iotwong/backend/internal/store"
)

const second = time.Second

func after(d time.Duration) <-chan time.Time { return time.After(d) }

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	f := store.DeviceFilter{
		Source:    q.Get("source"),
		ProjectID: q.Get("project_id"),
		Q:         q.Get("q"),
		Status:    q.Get("status"),
		Limit:     limit,
		Cursor:    q.Get("cursor"),
	}
	switch f.Status {
	case "", "online", "offline", "unknown":
	default:
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "status 取值无效", nil)
		return
	}
	page, err := s.st.ListDevices(r.Context(), tenantIDOf(r), f)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询设备失败", nil)
		return
	}
	ok(w, r, map[string]any{
		"items": page.Items,
		"page":  map[string]any{"next_cursor": page.NextCursor, "total": nil},
	})
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "缺少设备 id", nil)
		return
	}
	// Single-device view enforced within the session tenant: 404 for other
	// tenants' devices (contract: cross-tenant returns 404, never leaks).
	page, err := s.st.ListDevices(r.Context(), tenantIDOf(r), store.DeviceFilter{})
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询设备失败", nil)
		return
	}
	for _, it := range page.Items {
		if it.ID == id {
			ok(w, r, it)
			return
		}
	}
	fail(w, r, http.StatusNotFound, codeNotFound, "设备不存在", nil)
}

func (s *Server) handleDevicePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == nil {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "缺少 name", nil)
		return
	}
	name := strings.TrimSpace(*req.Name)
	if name == "" || len(name) > 200 {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "name 需为 1-200 字符", nil)
		return
	}
	err := s.st.UpdateDeviceAlias(r.Context(), tenantIDOf(r), id, name, userIDOf(r))
	if errors.Is(err, store.ErrNoRows) {
		fail(w, r, http.StatusNotFound, codeNotFound, "设备不存在", nil)
		return
	}
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "重命名失败", nil)
		return
	}
	ok(w, r, map[string]any{"ok": true})
}

// SSE device events (docs/04-contracts.md).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.st == nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "store unavailable", nil)
		return
	}
	flusher, okFlush := w.(http.Flusher)
	if !okFlush {
		fail(w, r, http.StatusInternalServerError, codeServiceUnhealthy, "streaming unsupported", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	tenantID := tenantIDOf(r)
	var cursor int64
	if le := r.Header.Get("Last-Event-ID"); le != "" {
		if v, err := strconv.ParseInt(le, 10, 64); err == nil {
			cursor = v
		}
	}
	heartbeat := 0
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		rows, err := s.st.ReadOutbox(r.Context(), tenantID, cursor, 200)
		if err != nil {
			// transient DB error: keep stream open, retry after a short pause
			select {
			case <-r.Context().Done():
				return
			case <-after(2 * second):
			}
			continue
		}
		for _, o := range rows {
			payload, _ := json.Marshal(o.Payload)
			if _, err := w.Write([]byte("id: " + strconv.FormatInt(o.ID, 10) + "\n")); err != nil {
				return
			}
			if _, err := w.Write([]byte("event: " + o.Type + "\n")); err != nil {
				return
			}
			if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
				return
			}
			cursor = o.ID
			heartbeat = 0
		}
		flusher.Flush()
		heartbeat++
		if heartbeat >= 15 { // ~15s comment heartbeat
			if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
				return
			}
			flusher.Flush()
			heartbeat = 0
		}
		select {
		case <-r.Context().Done():
			return
		case <-after(second):
		}
	}
}
