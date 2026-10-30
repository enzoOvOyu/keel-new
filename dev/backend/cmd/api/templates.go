package main

import (
	"encoding/json"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

func (s *Server) templates(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/v1/bom-mapping-templates" {
		return false
	}
	ctx := r.Context()
	if r.Method == "GET" {
		rows, e := s.db.Query(ctx, "SELECT name,revision,profiles,approved_by FROM bom_mapping_template WHERE project_id=$1 ORDER BY name,revision DESC LIMIT 100", s.project)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var name, actor string
			var rev int
			var raw []byte
			rows.Scan(&name, &rev, &raw, &actor)
			out = append(out, map[string]any{"name": name, "revision": rev, "profiles": json.RawMessage(raw), "approved_by": actor})
		}
		respond(w, 200, out)
		return true
	}
	if r.Method != "POST" {
		return false
	}
	var q struct {
		Import   string `json:"import_id"`
		Name     string `json:"name"`
		Reason   string `json:"reason"`
		Revision int    `json:"revision"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.Name) == "" || strings.TrimSpace(q.Reason) == "" {
		fail(w, 422, "TEMPLATE_APPROVAL_REQUIRED")
		return true
	}
	var raw []byte
	var project string
	var rev int
	if s.db.QueryRow(ctx, "SELECT project_id,revision,profiles FROM bom_import_batch WHERE id::text=$1", q.Import).Scan(&project, &rev, &raw) != nil || project != s.project {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	if q.Revision != rev {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	var profiles []bom.Profile
	json.Unmarshal(raw, &profiles)
	for _, p := range profiles {
		if !p.Confirmed {
			fail(w, 422, "ALL_SHEETS_MUST_BE_CONFIRMED")
			return true
		}
	}
	var next int
	e := s.db.QueryRow(ctx, "INSERT INTO bom_mapping_template(project_id,name,revision,profiles,approved_by) SELECT $1,$2,COALESCE(MAX(revision),0)+1,$3,'local-developer' FROM bom_mapping_template WHERE project_id=$1 AND name=$2 RETURNING revision", s.project, q.Name, raw).Scan(&next)
	if e != nil {
		fail(w, 409, "TEMPLATE_REVISION_CONFLICT")
		return true
	}
	s.audit(ctx, q.Import, "template.approved", q.Reason)
	respond(w, 201, map[string]any{"name": q.Name, "revision": next})
	return true
}
