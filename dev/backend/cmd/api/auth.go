package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type principalKey struct{}
type principal struct{ Actor, Role, Project string }

var loginMu sync.Mutex
var failures = map[string]struct {
	N     int
	Until time.Time
}{}

func secureID() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic("secure randomness unavailable")
	}
	return hex.EncodeToString(b)
}
func digestToken(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (s *Server) authRoute(w http.ResponseWriter, r *http.Request) bool {
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
	loginMu.Lock()
	f := failures[ip]
	limited := f.N >= 10 && time.Now().Before(f.Until)
	loginMu.Unlock()
	if limited {
		fail(w, 429, "RATE_LIMITED")
		return true
	}
	var q struct {
		User     string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		fail(w, 400, "INVALID_REQUEST")
		return true
	}
	role := "admin"
	expected := os.Getenv("KEEL_LOGIN_PASSWORD")
	validUser := q.User == env("KEEL_LOGIN_USER", "developer")
	if q.User == "viewer" {
		role = "viewer"
		expected = os.Getenv("KEEL_VIEWER_PASSWORD")
		validUser = expected != ""
	}
	a, b := sha256.Sum256([]byte(q.Password)), sha256.Sum256([]byte(expected))
	if !validUser || len(expected) < 16 || subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
		loginMu.Lock()
		f.N++
		f.Until = time.Now().Add(10 * time.Minute)
		failures[ip] = f
		loginMu.Unlock()
		fail(w, 401, "UNAUTHENTICATED")
		return true
	}
	token, csrf := secureID(), secureID()
	_, e := s.db.Exec(r.Context(), "INSERT INTO api_session(token_hash,csrf_token,actor,role,project_id,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '8 hours')", digestToken(token), csrf, q.User, role, s.project)
	if e != nil {
		fail(w, 503, "SESSION_STORE_UNAVAILABLE")
		return true
	}
	http.SetCookie(w, &http.Cookie{Name: "keel_session", Value: token, Path: "/api/v1", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 8 * 3600})
	respond(w, 200, map[string]any{"actor": q.User, "role": role, "csrf_token": csrf, "project_id": s.project, "permissions": authPermissions(role)})
	return true
}
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " && subtle.ConstantTimeCompare([]byte(auth[7:]), []byte(s.token)) == 1 {
		return r.WithContext(context.WithValue(r.Context(), principalKey{}, principal{"local-developer", env("KEEL_API_ROLE", "admin"), s.project})), true
	}
	cookie, e := r.Cookie("keel_session")
	if e != nil {
		fail(w, 401, "UNAUTHENTICATED")
		return r, false
	}
	var p principal
	var csrf string
	e = s.db.QueryRow(r.Context(), "SELECT actor,role,project_id,csrf_token FROM api_session WHERE token_hash=$1 AND expires_at>now()", digestToken(cookie.Value)).Scan(&p.Actor, &p.Role, &p.Project, &csrf)
	if e != nil || p.Project != s.project {
		fail(w, 401, "UNAUTHENTICATED")
		return r, false
	}
	if r.Method != "GET" && subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
		fail(w, 403, "CSRF_DENIED")
		return r, false
	}
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, p)), true
}

func allowedOrigin(r *http.Request, origin string) bool {
	if origin == env("DEV_ORIGIN", "http://localhost:5174") {
		return true
	}
	u, e := url.Parse(origin)
	return e == nil && u.User == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host == r.Host && u.Path == ""
}
func authPermissions(role string) []string {
	if role == "viewer" {
		return []string{"bom:read", "cmdb:read"}
	}
	return []string{"bom:read", "bom:write", "bom:review", "bom:commit", "model:approve", "cmdb:read"}
}
