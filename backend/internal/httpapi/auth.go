package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"iotwong/backend/internal/auth"
)

const cookieName = "iotwong_session"

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// loginRate tracks naive per-IP attempts (5/min) for /auth/login.
type loginRate struct {
	mu      sync.Mutex
	byIP    map[string][]time.Time
	blocked map[string]time.Time
}

var rate = loginRate{byIP: map[string][]time.Time{}, blocked: map[string]time.Time{}}

func rateLimited(ip string) bool {
	rate.mu.Lock()
	defer rate.mu.Unlock()
	if until, ok := rate.blocked[ip]; ok && time.Now().Before(until) {
		return true
	}
	delete(rate.blocked, ip)
	now := time.Now()
	cut := now.Add(-time.Minute)
	keep := rate.byIP[ip][:0]
	for _, t := range rate.byIP[ip] {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	rate.byIP[ip] = keep
	if len(keep) >= 5 {
		rate.blocked[ip] = now.Add(5 * time.Minute)
		return true
	}
	rate.byIP[ip] = append(rate.byIP[ip], now)
	return false
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 && strings.Count(ip, ":") == 1 {
		ip = ip[:idx]
	}
	return ip
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.st == nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "store unavailable", nil)
		return
	}
	if !s.cfg.DisableLoginLimit && rateLimited(clientIP(r)) {
		fail(w, r, http.StatusTooManyRequests, codeRateLimited, "尝试过于频繁，请稍后再试", nil)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Login == "" || req.Password == "" {
		fail(w, r, http.StatusBadRequest, codeInvalidRequest, "缺少登录名或密码", nil)
		return
	}
	user, err := s.st.UserByLogin(r.Context(), req.Login)
	if err != nil || !auth.VerifyPassword(user.PasswordHash, req.Password) {
		fail(w, r, http.StatusUnauthorized, codeUnauthorized, "用户名或密码错误", nil)
		return
	}
	token, err := s.st.CreateSession(r.Context(), user.ID)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "无法建立会话", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((12 * time.Hour).Seconds()),
	})
	ok(w, r, map[string]any{"login": user.Login})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.st == nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "store unavailable", nil)
		return
	}
	c, err := r.Cookie(cookieName)
	if err == nil {
		_ = s.st.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	ok(w, r, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, _ := r.Context().Value(ctxKeyUser).(string)
	user, err := s.st.UserByID(r.Context(), uid)
	if err != nil {
		fail(w, r, http.StatusUnauthorized, codeUnauthorized, "会话无效", nil)
		return
	}
	memberships, err := s.st.MembershipsOf(r.Context(), uid)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "读取成员关系失败", nil)
		return
	}
	ok(w, r, map[string]any{
		"user":     map[string]any{"id": user.ID, "login": user.Login},
		"tenants":  memberships,
		"timezone": "Asia/Shanghai",
	})
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	projects, err := s.st.ListProjects(r.Context(), tenantIDOf(r), source)
	if err != nil {
		fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "读取项目失败", nil)
		return
	}
	ok(w, r, map[string]any{"items": projects, "page": map[string]any{"next_cursor": nil, "total": nil}})
}
