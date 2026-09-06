package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"iotwong/backend/internal/store"
)

// requireAdmin enforces the admin role (viewer read-only per RBAC).
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if roleOf(r) != "admin" {
			fail(w, r, http.StatusForbidden, codeForbidden, "需要管理员权限", nil)
			return
		}
		next(w, r)
	})
}

func (s *Server) handleListFences(w http.ResponseWriter, r *http.Request) {
	fences, err := s.st.ListFences(r.Context(), tenantIDOf(r))
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询围栏失败", nil)
		return
	}
	ok(w, r, map[string]any{"items": fences, "page": map[string]any{"next_cursor": nil, "total": nil}})
}

func (s *Server) handleCreateFence(w http.ResponseWriter, r *http.Request) {
	var in store.FenceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "请求格式无效", nil)
		return
	}
	id, err := s.st.CreateFence(r.Context(), tenantIDOf(r), in)
	if err != nil {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, err.Error(), nil)
		return
	}
	ok(w, r, map[string]any{"id": id})
}

func (s *Server) handleUpdateFence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in store.FenceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || id == "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "请求格式无效", nil)
		return
	}
	err := s.st.UpdateFence(r.Context(), tenantIDOf(r), id, in)
	if errors.Is(err, store.ErrNoRows) {
		fail(w, r, http.StatusNotFound, codeNotFound, "围栏不存在", nil)
		return
	}
	if err != nil {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, err.Error(), nil)
		return
	}
	ok(w, r, map[string]any{"ok": true})
}

func (s *Server) handleDeleteFence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.st.DeleteFence(r.Context(), tenantIDOf(r), id)
	if errors.Is(err, store.ErrNoRows) {
		fail(w, r, http.StatusNotFound, codeNotFound, "围栏不存在", nil)
		return
	}
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "删除围栏失败", nil)
		return
	}
	ok(w, r, map[string]any{"ok": true})
}

func (s *Server) handleListAlarms(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	alarms, err := s.st.ListAlarms(r.Context(), tenantIDOf(r), limit)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询报警失败", nil)
		return
	}
	ok(w, r, map[string]any{"items": alarms, "page": map[string]any{"next_cursor": nil, "total": nil}})
}

func (s *Server) handleAckAlarm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.st.AckAlarm(r.Context(), tenantIDOf(r), id)
	if errors.Is(err, store.ErrNoRows) {
		fail(w, r, http.StatusNotFound, codeNotFound, "报警不存在或已确认", nil)
		return
	}
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "确认报警失败", nil)
		return
	}
	ok(w, r, map[string]any{"ok": true})
}
