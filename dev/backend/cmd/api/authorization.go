package main

import (
	"context"
	"keel.local/dev/internal/identity"
	"net/http"
	"strings"
)

func subjectOf(ctx context.Context) identity.Subject {
	return ctx.Value(principalKey{}).(principal).Subject
}
func (s *Server) authorizeRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	parts := strings.Split(path, "/")
	write := r.Method != "GET"
	project := r.URL.Query().Get("project_id")
	permission := ""
	kind, id := "", ""
	if len(parts) >= 2 && parts[0] == "bom-imports" {
		id = parts[1]
		kind = "bom_import"
		if s.db.QueryRow(r.Context(), "SELECT project_id FROM bom_import_batch WHERE id::text=$1", id).Scan(&project) != nil {
			fail(w, 404, "NOT_FOUND")
			return "", false
		}
		permission = "bom:read"
		if write {
			permission = "bom:edit"
			if strings.HasSuffix(path, "/review") {
				permission = "bom:review"
			}
			if strings.HasSuffix(path, "/commit") || strings.HasSuffix(path, "/workbench/submit") {
				permission = "bom:execute"
			}
		}
	} else if len(parts) >= 2 && parts[0] == "bom-batches" {
		id = parts[1]
		kind = "bom_revision"
		s.db.QueryRow(r.Context(), "SELECT project_id FROM bom_revision WHERE id::text=$1", id).Scan(&project)
		permission = "bom:read"
	} else if len(parts) >= 3 && parts[0] == "projects" {
		project = parts[1]
		module := parts[2]
		if module == "bom-imports" {
			permission = "bom:read"
			if write {
				permission = "bom:import"
			}
		} else if module == "bom-batches" {
			permission = "bom:read"
		} else {
			permission = module + ":read"
			if write {
				permission = module + ":execute"
			}
		}
	} else {
		switch parts[0] {
		case "bom-rules":
			permission = "bom:read"
		case "bom-models":
			permission = "model:read"
			if write {
				permission = "model:approve"
			}
		case "bom-mapping-templates":
			permission = "bom:read"
			if write {
				permission = "bom:approve"
			}
		case "jobs":
			permission = "bom:read"
			if len(parts) > 1 {
				s.db.QueryRow(r.Context(), "SELECT project_id,import_id::text FROM bom_job WHERE id::text=$1", parts[1]).Scan(&project, &id)
				kind = "bom_import"
			}
		case "cmdb":
			permission = "cmdb:read"
		}
	}
	if requested := r.URL.Query().Get("project_id"); requested != "" && project != "" && requested != project {
		fail(w, 403, "PROJECT_SCOPE_MISMATCH")
		return "", false
	}
	p := subjectOf(r.Context())
	if permission == "" {
		fail(w, 403, "FORBIDDEN")
		return "", false
	}
	if project == "" && len(p.Projects) > 0 {
		project = p.Projects[0].ID
	}
	if !p.Can(permission, project, kind, id) {
		fail(w, 403, "FORBIDDEN")
		return "", false
	}
	return project, true
}
