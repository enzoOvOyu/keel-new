package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) published(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "GET" {
		return false
	}
	ctx := r.Context()
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) < 4 || p[0] != "api" || p[1] != "v1" {
		return false
	}
	if len(p) == 5 && p[2] == "projects" && p[4] == "bom-batches" {
		if p[3] != s.project {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		rows, e := s.db.Query(ctx, "SELECT id,import_id,digest,created_at,project_id FROM bom_revision WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100", s.project)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, imp, digest, project string
			var at any
			rows.Scan(&id, &imp, &digest, &at, &project)
			out = append(out, map[string]any{"id": id, "project_id": project, "bom_import_batch_id": imp, "content_sha256": digest, "status": "committed", "created_at": at})
		}
		respond(w, 200, out)
		return true
	}
	if len(p) == 5 && p[2] == "bom-batches" && p[4] == "items" {
		limit, offset, ok := pageBounds(r)
		if !ok {
			fail(w, 400, "INVALID_PAGE")
			return true
		}
		var project string
		if s.db.QueryRow(ctx, "SELECT project_id FROM bom_revision WHERE id::text=$1", p[3]).Scan(&project) != nil || project != s.project {
			fail(w, 404, "NOT_FOUND")
			return true
		}
		rows, e := s.db.Query(ctx, "SELECT payload FROM bom_item_revision WHERE revision_id::text=$1 ORDER BY item_uid LIMIT $2 OFFSET $3", p[3], limit+1, offset)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			rows.Scan(&raw)
			out = append(out, json.RawMessage(raw))
		}
		more := len(out) > limit
		if more {
			out = out[:limit]
		}
		pageResponse(w, out, limit, offset, len(out), more)
		return true
	}
	if len(p) == 4 && p[2] == "cmdb" && p[3] == "published-assets" {
		rows, e := s.db.Query(ctx, "SELECT DISTINCT e.item_uid,e.payload FROM cmdb_entity e JOIN project_asset p ON p.item_uid=e.item_uid WHERE p.project_id=$1 ORDER BY e.item_uid LIMIT 200", s.project)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id string
			var raw []byte
			rows.Scan(&id, &raw)
			out = append(out, map[string]any{"id": id, "asset": json.RawMessage(raw), "source": "confirmed_bom"})
		}
		respond(w, 200, out)
		return true
	}
	return false
}
