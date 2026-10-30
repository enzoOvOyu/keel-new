package identity

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"sort"
	"strings"
	"time"
)

type Service struct{ DB *pgxpool.Pool }
type Grant struct {
	ID           string   `json:"id"`
	Project      string   `json:"project_id"`
	Role         string   `json:"role_id"`
	ResourceType string   `json:"resource_type"`
	ResourceID   string   `json:"resource_id"`
	Permissions  []string `json:"permissions"`
}
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	TemplateID string `json:"template_id"`
}
type Subject struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"display_name"`
	Version           int64     `json:"permission_version"`
	GlobalRoles       []string  `json:"global_roles"`
	GlobalPermissions []string  `json:"global_permissions"`
	Grants            []Grant   `json:"grants"`
	Projects          []Project `json:"authorized_projects"`
	SessionID         string    `json:"-"`
}

func (s Service) Initialize(ctx context.Context, password string) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1123199001)"); e != nil {
		return e
	}
	for _, p := range Catalog() {
		if _, e = tx.Exec(ctx, "INSERT INTO permission(id,module,action,description) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET description=excluded.description", p.ID, p.Module, p.Action, p.Description); e != nil {
			return e
		}
	}
	for _, r := range Roles() {
		if _, e = tx.Exec(ctx, "INSERT INTO role(id,name,description) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description", r.ID, r.Name, r.Description); e != nil {
			return e
		}
		for _, p := range r.Permissions {
			if _, e = tx.Exec(ctx, "INSERT INTO role_permission(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING", r.ID, p); e != nil {
				return e
			}
		}
	}
	var exists bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM app_user)").Scan(&exists); e != nil {
		return e
	}
	if !exists {
		hash, err := HashPassword(password)
		if err != nil {
			return err
		}
		id := uuid.NewString()
		if _, e = tx.Exec(ctx, "INSERT INTO app_user(id,username,display_name,password_hash) VALUES($1,'admin','平台管理员',$2)", id, hash); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO user_global_role(id,user_id,role_id) VALUES($1,$2,$3)", uuid.NewString(), id, Admin); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s Service) Load(ctx context.Context, userID string) (Subject, error) {
	p := Subject{ID: userID, GlobalRoles: []string{}, GlobalPermissions: []string{"identity:roles:read"}, Grants: []Grant{}, Projects: []Project{}}
	e := s.DB.QueryRow(ctx, "SELECT username,display_name,permission_version FROM app_user WHERE id::text=$1 AND status='active'", userID).Scan(&p.Username, &p.DisplayName, &p.Version)
	if e != nil {
		return p, e
	}
	rows, e := s.DB.Query(ctx, "SELECT g.role_id,rp.permission_id FROM user_global_role g JOIN role_permission rp ON rp.role_id=g.role_id WHERE g.user_id=$1 AND g.role_id=$2 AND g.revoked_at IS NULL AND g.valid_from<=now() AND (g.expires_at IS NULL OR g.expires_at>now())", userID, Admin)
	if e != nil {
		return p, e
	}
	roles := map[string]bool{}
	perms := map[string]bool{}
	for rows.Next() {
		var role, perm string
		rows.Scan(&role, &perm)
		roles[role] = true
		perms[perm] = true
	}
	rows.Close()
	for r := range roles {
		p.GlobalRoles = append(p.GlobalRoles, r)
	}
	for v := range perms {
		p.GlobalPermissions = append(p.GlobalPermissions, v)
	}
	rows, e = s.DB.Query(ctx, "SELECT m.id,m.project_id,m.role_id,m.resource_type,m.resource_id,array_agg(rp.permission_id ORDER BY rp.permission_id) FROM project_membership m JOIN role_permission rp ON rp.role_id=m.role_id WHERE m.user_id=$1 AND m.revoked_at IS NULL AND m.valid_from<=now() AND (m.expires_at IS NULL OR m.expires_at>now()) GROUP BY m.id ORDER BY m.project_id,m.id", userID)
	if e != nil {
		return p, e
	}
	for rows.Next() {
		var g Grant
		if e = rows.Scan(&g.ID, &g.Project, &g.Role, &g.ResourceType, &g.ResourceID, &g.Permissions); e != nil {
			rows.Close()
			return p, e
		}
		p.Grants = append(p.Grants, g)
	}
	rows.Close()
	rows, e = s.DB.Query(ctx, "SELECT p.id,p.name,p.code,p.status,p.template_id FROM project p WHERE $2 OR EXISTS(SELECT 1 FROM project_membership m WHERE m.project_id=p.id AND m.user_id=$1 AND m.revoked_at IS NULL AND m.valid_from<=now() AND (m.expires_at IS NULL OR m.expires_at>now())) ORDER BY p.id", userID, len(p.GlobalRoles) > 0)
	if e != nil {
		return p, e
	}
	for rows.Next() {
		var project Project
		rows.Scan(&project.ID, &project.Name, &project.Code, &project.Status, &project.TemplateID)
		p.Projects = append(p.Projects, project)
	}
	rows.Close()
	sort.Strings(p.GlobalRoles)
	sort.Strings(p.GlobalPermissions)
	return p, nil
}
func (p Subject) Can(permission, project, kind, id string) bool {
	permission = Canonical(permission)
	for _, v := range p.GlobalPermissions {
		if v == permission {
			return true
		}
	}
	if project == "" {
		return false
	}
	for _, g := range p.Grants {
		if g.Project != project {
			continue
		}
		if g.ResourceID != "" && (g.ResourceID != id || g.ResourceType != kind) {
			continue
		}
		for _, v := range g.Permissions {
			if v == permission {
				return true
			}
		}
	}
	return false
}
func (p Subject) Permissions(project string) []string {
	m := map[string]bool{}
	for _, v := range p.GlobalPermissions {
		m[v] = true
	}
	for _, g := range p.Grants {
		if g.Project == project {
			for _, v := range g.Permissions {
				m[v] = true
			}
		}
	}
	out := []string{}
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func (p Subject) HasRole(role, project, kind, id string) bool {
	for _, g := range p.Grants {
		if g.Role == role && g.Project == project && (g.ResourceID == "" || (g.ResourceID == id && g.ResourceType == kind)) {
			return true
		}
	}
	return false
}
func (p Subject) RoleLabel() string {
	if len(p.GlobalRoles) > 0 {
		return strings.Join(p.GlobalRoles, ",")
	}
	roles := map[string]bool{}
	for _, g := range p.Grants {
		roles[g.Role] = true
	}
	out := []string{}
	for r := range roles {
		out = append(out, r)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
func ValidExpiry(v *time.Time) bool { return v == nil || v.After(time.Now()) }
