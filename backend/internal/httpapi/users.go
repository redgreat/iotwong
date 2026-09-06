package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"iotwong/backend/internal/auth"
	"iotwong/backend/internal/store"
)

// 用户管理（admin）：GET /users、POST /users、POST /users/{id}/reset-password。
// 写接口均有 csrf + requireAdmin；viewer/无会话一律 403/401。
var (
	loginPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	uuidPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListTenantUsers(r.Context(), tenantIDOf(r))
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "查询用户失败", nil)
		return
	}
	ok(w, r, map[string]any{"items": users, "page": map[string]any{"next_cursor": nil, "total": nil}})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "请求格式无效", nil)
		return
	}
	login := strings.TrimSpace(req.Login)
	if !loginPattern.MatchString(login) {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest,
			"登录名需以字母/数字开头，1-64 位字母数字 _ . -", nil)
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 128 {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "密码需为 6-128 位", nil)
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "角色需为 admin 或 viewer", nil)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(w, r, http.StatusInternalServerError, codeServiceUnhealthy, "无法处理密码", nil)
		return
	}
	id, err := s.st.CreateTenantUser(r.Context(), tenantIDOf(r), login, hash, req.Role)
	if errors.Is(err, store.ErrLoginTaken) {
		fail(w, r, http.StatusConflict, codeConflict, "登录名已存在", nil)
		return
	}
	if errors.Is(err, store.ErrNoRows) {
		fail(w, r, http.StatusNotFound, codeNotFound, "租户不存在", nil)
		return
	}
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "创建用户失败", nil)
		return
	}
	ok(w, r, map[string]any{"id": id})
}

func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "用户 id 无效", nil)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "请求格式无效", nil)
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 128 {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "密码需为 6-128 位", nil)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(w, r, http.StatusInternalServerError, codeServiceUnhealthy, "无法处理密码", nil)
		return
	}
	err = s.st.ResetUserPassword(r.Context(), tenantIDOf(r), id, hash)
	if errors.Is(err, store.ErrNoRows) {
		fail(w, r, http.StatusNotFound, codeNotFound, "用户不存在或不在当前租户", nil)
		return
	}
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "重置密码失败", nil)
		return
	}
	ok(w, r, map[string]any{"ok": true})
}
