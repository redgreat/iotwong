package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// requireAuth resolves the session cookie to a user and the tenant scoping
// used by every subsequent SQL query (never trusts request-body tenant_id).
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.st == nil {
			fail(w, r, http.StatusServiceUnavailable, codeServiceUnhealthy, "store unavailable", nil)
			return
		}
		c, err := r.Cookie(cookieName)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, codeUnauthorized, "未登录或会话已过期", nil)
			return
		}
		uid, err := s.st.SessionUser(r.Context(), c.Value)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, codeUnauthorized, "未登录或会话已过期", nil)
			return
		}
		memberships, err := s.st.MembershipsOf(r.Context(), uid)
		if err != nil || len(memberships) == 0 {
			fail(w, r, http.StatusForbidden, codeForbidden, "无租户访问权限", nil)
			return
		}
		// optional ?tenant= must belong to the session
		tenantID := memberships[0].TenantID
		role := memberships[0].Role
		if want := r.URL.Query().Get("tenant"); want != "" {
			found := false
			for _, m := range memberships {
				if m.TenantID == want {
					tenantID, role = m.TenantID, m.Role
					found = true
					break
				}
			}
			if !found {
				fail(w, r, http.StatusNotFound, codeNotFound, "租户不存在", nil)
				return
			}
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxKeyUser, uid)
		ctx = context.WithValue(ctx, ctxKeyTenantID, tenantID)
		ctx = context.WithValue(ctx, ctxKeyRole, role)
		next(w, r.WithContext(ctx))
	}
}

// csrf rejects cross-origin state-changing requests when an Origin header is
// present (same-origin SPA + API; Origin missing is tolerated for CLI tools).
func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				fail(w, r, http.StatusForbidden, codeForbidden, "跨源请求被拒绝", nil)
				return
			}
		}
		next(w, r)
	}
}

func userIDOf(r *http.Request) string {
	v, _ := r.Context().Value(ctxKeyUser).(string)
	return v
}
func tenantIDOf(r *http.Request) string {
	v, _ := r.Context().Value(ctxKeyTenantID).(string)
	return v
}
func roleOf(r *http.Request) string {
	v, _ := r.Context().Value(ctxKeyRole).(string)
	return v
}
