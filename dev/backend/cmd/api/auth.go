package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/identity"
	"net"
	"net/http"
	"net/url"
	"time"
)

type principalKey struct{}
type principal struct {
	Actor, Role, Project string
	Subject              identity.Subject
	SessionHash          string
}

func secureID() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digestToken(s string) string                { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func identityService(s *Server) identity.Service { return identity.Service{DB: s.db} }
func (s *Server) authRoute(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/api/v1/auth/providers" && r.Method == "GET" {
		respond(w, 200, map[string]any{"methods": []string{"local"}, "environment": env("APP_ENV", "development"), "status": "ready"})
		return true
	}
	if r.URL.Path != "/api/v1/auth/login" {
		return false
	}
	if r.Method != "POST" {
		fail(w, 405, "METHOD_NOT_ALLOWED")
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" && !allowedOrigin(r, origin) {
		fail(w, 403, "ORIGIN_DENIED")
		return true
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	var q struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Method   string `json:"method"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Username == "" {
		fail(w, 400, "INVALID_REQUEST")
		return true
	}
	if q.Method != "" && q.Method != "local" {
		fail(w, 422, "AUTH_PROVIDER_NOT_CONFIGURED")
		return true
	}
	var attempts int
	s.db.QueryRow(r.Context(), "SELECT count(*) FROM login_event WHERE ip=$1 AND outcome IN('failed','rate_limited') AND created_at>now()-interval '10 minutes'", ip).Scan(&attempts)
	if attempts >= 20 {
		w.Header().Set("Retry-After", "600")
		fail(w, 429, "RATE_LIMITED")
		return true
	}
	var id, hash, status string
	var locked *time.Time
	e := s.db.QueryRow(r.Context(), "SELECT id,password_hash,status,locked_until FROM app_user WHERE username=$1", q.Username).Scan(&id, &hash, &status, &locked)
	if e == nil && locked != nil && time.Now().Before(*locked) {
		s.loginEvent(r, id, q.Username, "locked", ip)
		w.Header().Set("Retry-After", "600")
		fail(w, 429, "ACCOUNT_LOCKED")
		return true
	}
	if e != nil || status != "active" || !identity.VerifyPassword(hash, q.Password) {
		if id != "" {
			s.db.Exec(r.Context(), "UPDATE app_user SET failed_attempts=failed_attempts+1,locked_until=CASE WHEN failed_attempts+1>=5 THEN now()+interval '10 minutes' ELSE locked_until END WHERE id=$1", id)
		}
		s.loginEvent(r, id, q.Username, "failed", ip)
		fail(w, 401, "UNAUTHENTICATED")
		return true
	}
	token, csrf := secureID(), secureID()
	_, e = s.db.Exec(r.Context(), "INSERT INTO api_session(token_hash,csrf_token,actor,role,project_id,user_id,expires_at,user_agent,ip) VALUES($1,$2,$3,'db','',$4,now()+interval '8 hours',$5,$6)", digestToken(token), csrf, q.Username, id, r.UserAgent(), ip)
	if e != nil {
		fail(w, 503, "SESSION_STORE_UNAVAILABLE")
		return true
	}
	s.db.Exec(r.Context(), "UPDATE app_user SET failed_attempts=0,locked_until=NULL WHERE id=$1", id)
	s.loginEvent(r, id, q.Username, "success", ip)
	http.SetCookie(w, &http.Cookie{Name: "keel_session", Value: token, Path: "/api/v1", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: env("APP_ENV", "development") == "production" || r.TLS != nil, MaxAge: 8 * 3600})
	subject, e := identityService(s).Load(r.Context(), id)
	if e != nil {
		fail(w, 503, "IDENTITY_UNAVAILABLE")
		return true
	}
	s.sessionResponse(w, r, subject, csrf)
	return true
}
func (s *Server) loginEvent(r *http.Request, id, user, outcome, ip string) {
	s.db.Exec(r.Context(), "INSERT INTO login_event(id,user_id,username,outcome,ip) VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5)", uuid.NewString(), id, user, outcome, ip)
}
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	cookie, e := r.Cookie("keel_session")
	if e != nil {
		fail(w, 401, "UNAUTHENTICATED")
		return r, false
	}
	var uid, csrf string
	e = s.db.QueryRow(r.Context(), "SELECT s.user_id::text,s.csrf_token FROM api_session s JOIN app_user u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND s.revoked_at IS NULL AND u.status='active'", digestToken(cookie.Value)).Scan(&uid, &csrf)
	if e != nil {
		fail(w, 401, "SESSION_EXPIRED")
		return r, false
	}
	if r.Method != "GET" && r.Header.Get("X-CSRF-Token") != csrf {
		fail(w, 403, "CSRF_DENIED")
		return r, false
	}
	subject, e := identityService(s).Load(r.Context(), uid)
	if e != nil {
		fail(w, 401, "UNAUTHENTICATED")
		return r, false
	}
	subject.SessionID = digestToken(cookie.Value)
	s.db.Exec(r.Context(), "UPDATE api_session SET last_seen_at=now() WHERE token_hash=$1", subject.SessionID)
	p := principal{Actor: subject.Username, Role: subject.RoleLabel(), Subject: subject, SessionHash: subject.SessionID}
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, p)), true
}
func (s *Server) sessionResponse(w http.ResponseWriter, r *http.Request, p identity.Subject, csrf string) {
	projectPermissions := map[string][]string{}
	projectRoles := map[string][]string{}
	for _, project := range p.Projects {
		projectPermissions[project.ID] = p.Permissions(project.ID)
	}
	for _, g := range p.Grants {
		projectRoles[g.Project] = append(projectRoles[g.Project], g.Role)
	}
	selected := ""
	if len(p.Projects) > 0 {
		selected = p.Projects[0].ID
	}
	respond(w, 200, map[string]any{"actor": p.Username, "user": map[string]any{"id": p.ID, "username": p.Username, "display_name": p.DisplayName}, "role": p.RoleLabel(), "global_roles": p.GlobalRoles, "global_permissions": p.GlobalPermissions, "project_permissions": projectPermissions, "project_roles": projectRoles, "grants": p.Grants, "authorized_projects": p.Projects, "permissions": p.Permissions(selected), "project_id": selected, "permission_revision": p.Version, "csrf_token": csrf, "ai_configured": s.aiConfigured(r.Context())})
}
func allowedOrigin(r *http.Request, origin string) bool {
	if origin == env("DEV_ORIGIN", "http://localhost:5174") {
		return true
	}
	u, e := url.Parse(origin)
	return e == nil && u.User == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host == r.Host && u.Path == ""
}
