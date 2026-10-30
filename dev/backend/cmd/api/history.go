package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) history(w http.ResponseWriter, r *http.Request, id string) bool {
	if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/history") {
		return false
	}
	ctx := r.Context()
	rows, e := s.db.Query(ctx, "SELECT revision,reason,actor,created_at FROM import_draft_revision WHERE import_id=$1 ORDER BY revision DESC LIMIT 100", id)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var rev int
		var reason, actor string
		var at any
		rows.Scan(&rev, &reason, &actor, &at)
		out = append(out, map[string]any{"revision": rev, "reason": reason, "actor": actor, "created_at": at})
	}
	audit, e := s.db.Query(ctx, "SELECT action,details,created_at FROM audit_event WHERE resource_id=$1 AND project_id=$2 ORDER BY created_at DESC LIMIT 100", id, s.project)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	defer audit.Close()
	events := []any{}
	for audit.Next() {
		var action string
		var raw []byte
		var at any
		audit.Scan(&action, &raw, &at)
		events = append(events, map[string]any{"action": action, "details": json.RawMessage(raw), "created_at": at})
	}
	respond(w, 200, map[string]any{"revisions": out, "events": events})
	return true
}
