package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/identity"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func (s *Server) identityRoutes(w http.ResponseWriter, r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	p := strings.Split(path, "/")
	u := subjectOf(r.Context())
	ctx := r.Context()
	adminRead := u.Can("admin:read", "", "", "")
	adminWrite := u.Can("admin:edit", "", "", "")
	if path == "auth/session" && r.Method == "GET" {
		var csrf string
		s.db.QueryRow(ctx, "SELECT csrf_token FROM api_session WHERE token_hash=$1", r.Context().Value(principalKey{}).(principal).SessionHash).Scan(&csrf)
		s.sessionResponse(w, r, u, csrf)
		return true
	}
	if path == "auth/logout" && r.Method == "POST" {
		s.db.Exec(ctx, "UPDATE api_session SET revoked_at=now() WHERE token_hash=$1", r.Context().Value(principalKey{}).(principal).SessionHash)
		http.SetCookie(w, &http.Cookie{Name: "keel_session", Value: "", Path: "/api/v1", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: env("APP_ENV", "development") == "production", MaxAge: -1})
		respond(w, 200, map[string]string{"status": "signed_out"})
		return true
	}
	if path == "auth/password" && r.Method == "POST" {
		var q struct {
			Current string `json:"current_password"`
			Next    string `json:"new_password"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		var hash string
		s.db.QueryRow(ctx, "SELECT password_hash FROM app_user WHERE id=$1", u.ID).Scan(&hash)
		if !identity.VerifyPassword(hash, q.Current) {
			fail(w, 401, "INVALID_CURRENT_PASSWORD")
			return true
		}
		newHash, e := identity.HashPassword(q.Next)
		if e != nil {
			fail(w, 422, "PASSWORD_LENGTH")
			return true
		}
		s.identityMutation(w, r, "password.changed", u.ID, func(tx identityTx) error {
			if _, e := tx.Exec(ctx, "UPDATE app_user SET password_hash=$2,password_changed_at=now(),permission_version=permission_version+1 WHERE id=$1", u.ID, newHash); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, "UPDATE api_session SET revoked_at=now() WHERE user_id=$1", u.ID)
			return e
		})
		return true
	}
	if path == "roles" && r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT r.id,r.name,r.description,array_agg(p.permission_id ORDER BY p.permission_id) FROM role r JOIN role_permission p ON p.role_id=r.id GROUP BY r.id ORDER BY r.id")
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, name, desc string
			var permissions []string
			rows.Scan(&id, &name, &desc, &permissions)
			out = append(out, map[string]any{"id": id, "name": name, "description": desc, "permissions": permissions, "assignable_to_project": identity.ProjectRole(id)})
		}
		respond(w, 200, out)
		return true
	}
	if path == "permissions" && r.Method == "GET" {
		respond(w, 200, identity.Catalog())
		return true
	}
	if path == "projects" && r.Method == "GET" {
		respond(w, 200, u.Projects)
		return true
	}
	if path == "projects" && r.Method == "POST" {
		if !u.Can("project:create", "", "", "") {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		var q struct {
			Name       string `json:"name"`
			Code       string `json:"code"`
			TemplateID string `json:"template_id"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		q.Name = strings.TrimSpace(q.Name)
		q.Code = strings.TrimSpace(q.Code)
		if len([]rune(q.Name)) < 2 || len([]rune(q.Name)) > 240 || !regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`).MatchString(q.Code) {
			fail(w, 422, "PROJECT_FIELDS_REQUIRED")
			return true
		}
		if q.TemplateID != "" {
			fail(w, 422, "PROJECT_TEMPLATE_NOT_AVAILABLE")
			return true
		}
		id := "proj-" + uuid.NewString()
		s.identityMutation(w, r, "project.created", id, func(tx identityTx) error {
			_, e := tx.Exec(ctx, "INSERT INTO project(id,name,code,template_id,status) VALUES($1,$2,$3,$4,'planned')", id, q.Name, q.Code, q.TemplateID)
			if e != nil {
				return e
			}
			for _, role := range []string{identity.DeliveryEngineer, identity.DeliveryLead} {
				if _, e = tx.Exec(ctx, "INSERT INTO project_membership(id,user_id,project_id,role_id,created_by) VALUES($1,$2,$3,$4,$2)", uuid.NewString(), u.ID, id, role); e != nil {
					return e
				}
			}
			_, e = tx.Exec(ctx, "UPDATE app_user SET permission_version=permission_version+1 WHERE id=$1", u.ID)
			return e
		})
		return true
	}
	if len(p) == 3 && p[0] == "projects" && p[2] == "members" {
		project := p[1]
		if !u.Can("project:members", project, "", "") {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		if r.Method == "GET" {
			rows, e := s.db.Query(ctx, "SELECT m.id,m.user_id,u.username,u.display_name,m.role_id,m.resource_type,m.resource_id,m.valid_from,m.expires_at,m.revoked_at FROM project_membership m JOIN app_user u ON u.id=m.user_id WHERE m.project_id=$1 ORDER BY m.valid_from DESC", project)
			if e != nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, uid, name, display, role, kind, resource string
				var at time.Time
				var expires, revoked *time.Time
				rows.Scan(&id, &uid, &name, &display, &role, &kind, &resource, &at, &expires, &revoked)
				out = append(out, map[string]any{"id": id, "user_id": uid, "username": name, "display_name": display, "role_id": role, "resource_type": kind, "resource_id": resource, "valid_from": at, "expires_at": expires, "revoked_at": revoked})
			}
			respond(w, 200, out)
			return true
		}
		if r.Method == "POST" {
			var q struct {
				User         string     `json:"user_id"`
				Username     string     `json:"username"`
				Role         string     `json:"role_id"`
				ResourceType string     `json:"resource_type"`
				ResourceID   string     `json:"resource_id"`
				Expires      *time.Time `json:"expires_at"`
				Reason       string     `json:"reason"`
			}
			if !decodeIdentity(w, r, &q) {
				return true
			}
			if q.User == "" {
				s.db.QueryRow(ctx, "SELECT id FROM app_user WHERE username=$1", q.Username).Scan(&q.User)
			}
			if q.User == "" || !identity.ProjectRole(q.Role) || q.Reason == "" || !identity.ValidExpiry(q.Expires) || (q.ResourceType == "") != (q.ResourceID == "") {
				fail(w, 422, "INVALID_PROJECT_GRANT")
				return true
			}
			s.identityMutation(w, r, "membership.granted", q.User, func(tx identityTx) error {
				if _, e := tx.Exec(ctx, "INSERT INTO project_membership(id,user_id,project_id,role_id,resource_type,resource_id,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(user_id,project_id,role_id,resource_type,resource_id) WHERE revoked_at IS NULL DO UPDATE SET expires_at=excluded.expires_at,valid_from=now()", uuid.NewString(), q.User, project, q.Role, q.ResourceType, q.ResourceID, q.Expires, u.ID); e != nil {
					return e
				}
				_, e := tx.Exec(ctx, "UPDATE app_user SET permission_version=permission_version+1 WHERE id=$1", q.User)
				return e
			})
			return true
		}
	}
	if len(p) == 4 && p[0] == "projects" && p[2] == "members" && r.Method == "DELETE" {
		if !u.Can("project:members", p[1], "", "") {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		var target string
		if s.db.QueryRow(ctx, "SELECT user_id FROM project_membership WHERE id::text=$1 AND project_id=$2", p[3], p[1]).Scan(&target) != nil {
			fail(w, 404, "NOT_FOUND")
			return true
		}
		s.identityMutation(w, r, "membership.revoked", target, func(tx identityTx) error {
			if _, e := tx.Exec(ctx, "UPDATE project_membership SET revoked_at=now() WHERE id::text=$1", p[3]); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, "UPDATE app_user SET permission_version=permission_version+1 WHERE id=$1", target)
			return e
		})
		return true
	}
	if path == "users" {
		if r.Method == "GET" {
			if !adminRead {
				fail(w, 403, "FORBIDDEN")
				return true
			}
			rows, e := s.db.Query(ctx, "SELECT id,username,display_name,status,locked_until,permission_version,created_at FROM app_user ORDER BY username LIMIT 200")
			if e != nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, user, name, status string
				var locked *time.Time
				var version int64
				var at time.Time
				rows.Scan(&id, &user, &name, &status, &locked, &version, &at)
				out = append(out, map[string]any{"id": id, "username": user, "display_name": name, "status": status, "locked_until": locked, "permission_version": version, "created_at": at})
			}
			respond(w, 200, out)
			return true
		}
		if r.Method == "POST" {
			if !adminWrite {
				fail(w, 403, "FORBIDDEN")
				return true
			}
			var q struct {
				Username    string `json:"username"`
				DisplayName string `json:"display_name"`
				Password    string `json:"password"`
			}
			if !decodeIdentity(w, r, &q) {
				return true
			}
			if len(q.Username) < 3 || len(q.Username) > 64 || q.DisplayName == "" {
				fail(w, 422, "USER_FIELDS_REQUIRED")
				return true
			}
			hash, e := identity.HashPassword(q.Password)
			if e != nil {
				fail(w, 422, "PASSWORD_LENGTH")
				return true
			}
			id := uuid.NewString()
			s.identityMutation(w, r, "user.created", id, func(tx identityTx) error {
				_, e := tx.Exec(ctx, "INSERT INTO app_user(id,username,display_name,password_hash) VALUES($1,$2,$3,$4)", id, q.Username, q.DisplayName, hash)
				return e
			})
			return true
		}
	}
	if len(p) >= 2 && p[0] == "users" {
		if !adminRead {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		target := p[1]
		if len(p) == 2 && r.Method == "GET" {
			var user, name, status string
			var locked *time.Time
			if s.db.QueryRow(ctx, "SELECT username,display_name,status,locked_until FROM app_user WHERE id::text=$1", target).Scan(&user, &name, &status, &locked) != nil {
				fail(w, 404, "NOT_FOUND")
				return true
			}
			respond(w, 200, map[string]any{"id": target, "username": user, "display_name": name, "status": status, "locked_until": locked})
			return true
		}
		if !adminWrite {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		if len(p) == 2 && r.Method == "PATCH" {
			var q struct {
				Display string `json:"display_name"`
				Status  string `json:"status"`
			}
			if !decodeIdentity(w, r, &q) {
				return true
			}
			if q.Status != "" && q.Status != "active" && q.Status != "disabled" {
				fail(w, 422, "INVALID_STATUS")
				return true
			}
			if target == u.ID && q.Status == "disabled" {
				fail(w, 409, "SELF_DISABLE_DENIED")
				return true
			}
			s.identityMutation(w, r, "user.updated", target, func(tx identityTx) error {
				if _, e := tx.Exec(ctx, "UPDATE app_user SET display_name=COALESCE(NULLIF($2,''),display_name),status=COALESCE(NULLIF($3,''),status),permission_version=permission_version+1 WHERE id::text=$1", target, q.Display, q.Status); e != nil {
					return e
				}
				if q.Status == "disabled" {
					_, e := tx.Exec(ctx, "UPDATE api_session SET revoked_at=now() WHERE user_id::text=$1", target)
					return e
				}
				return nil
			})
			return true
		}
		if len(p) == 3 && r.Method == "POST" {
			switch p[2] {
			case "reset-password":
				var q struct {
					Password string `json:"new_password"`
				}
				if !decodeIdentity(w, r, &q) {
					return true
				}
				hash, e := identity.HashPassword(q.Password)
				if e != nil {
					fail(w, 422, "PASSWORD_LENGTH")
					return true
				}
				s.identityMutation(w, r, "password.admin_reset", target, func(tx identityTx) error {
					if _, e := tx.Exec(ctx, "UPDATE app_user SET password_hash=$2,password_changed_at=now(),failed_attempts=0,locked_until=NULL,permission_version=permission_version+1 WHERE id::text=$1", target, hash); e != nil {
						return e
					}
					_, e := tx.Exec(ctx, "UPDATE api_session SET revoked_at=now() WHERE user_id::text=$1", target)
					return e
				})
				return true
			case "unlock":
				s.identityMutation(w, r, "user.unlocked", target, func(tx identityTx) error {
					_, e := tx.Exec(ctx, "UPDATE app_user SET failed_attempts=0,locked_until=NULL WHERE id::text=$1", target)
					return e
				})
				return true
			}
		}
		if len(p) == 3 && p[2] == "global-roles" && r.Method == "POST" {
			var q struct {
				Role    string     `json:"role_id"`
				Revoke  bool       `json:"revoke"`
				Expires *time.Time `json:"expires_at"`
			}
			if !decodeIdentity(w, r, &q) {
				return true
			}
			if q.Role != identity.Admin || !identity.ValidExpiry(q.Expires) {
				fail(w, 422, "PLATFORM_ADMIN_ROLE_REQUIRED")
				return true
			}
			if q.Revoke && target == u.ID {
				fail(w, 409, "SELF_ADMIN_REVOKE_DENIED")
				return true
			}
			s.identityMutation(w, r, "global_role.changed", target, func(tx identityTx) error {
				if q.Revoke {
					if _, e := tx.Exec(ctx, "UPDATE user_global_role SET revoked_at=now() WHERE user_id::text=$1 AND role_id=$2 AND revoked_at IS NULL", target, q.Role); e != nil {
						return e
					}
				} else {
					if _, e := tx.Exec(ctx, "INSERT INTO user_global_role(id,user_id,role_id,expires_at,created_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,role_id) WHERE revoked_at IS NULL DO UPDATE SET expires_at=excluded.expires_at,valid_from=now()", uuid.NewString(), target, q.Role, q.Expires, u.ID); e != nil {
						return e
					}
				}
				_, e := tx.Exec(ctx, "UPDATE app_user SET permission_version=permission_version+1 WHERE id::text=$1", target)
				return e
			})
			return true
		}
	}
	if path == "sessions" && r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT session_id,user_id,actor,ip,user_agent,created_at,expires_at,revoked_at,last_seen_at FROM api_session WHERE user_id=$1 OR $2 ORDER BY created_at DESC LIMIT 200", u.ID, adminRead)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, uid, actor, ip, agent string
			var at, expires, seen time.Time
			var revoked *time.Time
			rows.Scan(&id, &uid, &actor, &ip, &agent, &at, &expires, &revoked, &seen)
			out = append(out, map[string]any{"id": id, "user_id": uid, "username": actor, "ip": ip, "user_agent": agent, "created_at": at, "expires_at": expires, "revoked_at": revoked, "last_seen_at": seen})
		}
		respond(w, 200, out)
		return true
	}
	if len(p) == 2 && p[0] == "sessions" && r.Method == "DELETE" {
		var owner string
		if s.db.QueryRow(ctx, "SELECT user_id FROM api_session WHERE session_id::text=$1", p[1]).Scan(&owner) != nil {
			fail(w, 404, "NOT_FOUND")
			return true
		}
		if owner != u.ID && !adminWrite {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		s.identityMutation(w, r, "session.revoked", owner, func(tx identityTx) error {
			_, e := tx.Exec(ctx, "UPDATE api_session SET revoked_at=now() WHERE session_id::text=$1", p[1])
			return e
		})
		return true
	}
	if (path == "login-events" || path == "identity-audits") && r.Method == "GET" {
		if !adminRead {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		if path == "login-events" {
			rows, _ := s.db.Query(ctx, "SELECT id,username,outcome,ip,created_at FROM login_event ORDER BY created_at DESC LIMIT 200")
			if rows == nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, name, outcome, ip string
				var at time.Time
				rows.Scan(&id, &name, &outcome, &ip, &at)
				out = append(out, map[string]any{"id": id, "username": name, "outcome": outcome, "ip": ip, "created_at": at})
			}
			respond(w, 200, out)
		} else {
			rows, _ := s.db.Query(ctx, "SELECT id,actor,action,resource_id,details,created_at FROM audit_event WHERE action LIKE 'identity.%' ORDER BY created_at DESC LIMIT 200")
			if rows == nil {
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, actor, action, resource string
				var details []byte
				var at time.Time
				rows.Scan(&id, &actor, &action, &resource, &details, &at)
				out = append(out, map[string]any{"id": id, "actor": actor, "action": action, "resource_id": resource, "details": json.RawMessage(details), "created_at": at})
			}
			respond(w, 200, out)
		}
		return true
	}
	if path == "authorization/check" && r.Method == "POST" {
		var q struct {
			Permission string `json:"permission"`
			Project    string `json:"project_id"`
			Kind       string `json:"resource_type"`
			ID         string `json:"resource_id"`
		}
		if !decodeIdentity(w, r, &q) {
			return true
		}
		allowed := u.Can(q.Permission, q.Project, q.Kind, q.ID)
		if !allowed {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		respond(w, 200, map[string]bool{"allowed": true})
		return true
	}
	return s.approvalRoutes(w, r)
}
