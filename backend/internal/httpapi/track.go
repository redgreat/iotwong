package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"iotwong/backend/internal/store"
)

// parseTimeRange reads required from/to (RFC3339 UTC, [from,to), <= 7 days).
func parseTimeRange(r *http.Request) (time.Time, time.Time, string) {
	q := r.URL.Query()
	from, err1 := time.Parse(time.RFC3339, q.Get("from"))
	to, err2 := time.Parse(time.RFC3339, q.Get("to"))
	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, "from/to 必须为 ISO8601 UTC 时间"
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, "to 必须晚于 from"
	}
	if to.Sub(from) > 7*24*time.Hour {
		return time.Time{}, time.Time{}, "时间窗口最长 7 天"
	}
	return from, to, ""
}

// deviceIDOf validates the :id path value exists for this tenant (404 other
// tenants); returns internal device row resolved via listing (devices are few).
func (s *Server) deviceIDOf(w http.ResponseWriter, r *http.Request, id string) string {
	if id == "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "缺少设备 id", nil)
		return ""
	}
	page, err := s.st.ListDevices(r.Context(), tenantIDOf(r), store.DeviceFilter{Limit: 200})
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询设备失败", nil)
		return ""
	}
	for _, it := range page.Items {
		if it.ID == id {
			return id
		}
	}
	fail(w, r, http.StatusNotFound, codeNotFound, "设备不存在", nil)
	return ""
}

func (s *Server) handlePositions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.deviceIDOf(w, r, id) == "" {
		return
	}
	from, to, msg := parseTimeRange(r)
	if msg != "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, msg, nil)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 5000
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, next, err := s.st.ListPositions(r.Context(), tenantIDOf(r), id, from, to,
		r.URL.Query().Get("cursor"), limit)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询位置失败", nil)
		return
	}
	ok(w, r, map[string]any{
		"items": rows,
		"page":  map[string]any{"next_cursor": next, "total": nil},
	})
}

func (s *Server) handleTrack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.deviceIDOf(w, r, id) == "" {
		return
	}
	from, to, msg := parseTimeRange(r)
	if msg != "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, msg, nil)
		return
	}
	q := r.URL.Query()
	maxPoints, _ := strconv.Atoi(q.Get("max_points"))
	gapSec := 300.0 // default gap: 5 minutes (docs/03)
	if g := q.Get("gap_sec"); g != "" {
		if v, err := strconv.ParseFloat(g, 64); err == nil && v > 0 {
			gapSec = v
		}
	}
	res, err := s.st.Track(r.Context(), tenantIDOf(r), id, from, to, maxPoints, gapSec)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询轨迹失败", nil)
		return
	}
	ok(w, r, res)
}
