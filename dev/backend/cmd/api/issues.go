package main

import (
	"encoding/csv"
	"encoding/json"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

func (s *Server) issues(w http.ResponseWriter, r *http.Request, id string, rev int) bool {
	prefix := "/api/v1/bom-imports/" + id + "/issues"
	if r.URL.Path != prefix && r.URL.Path != prefix+"/export" {
		return false
	}
	if r.Method != "GET" {
		return false
	}
	var raw []byte
	if s.db.QueryRow(r.Context(), "SELECT issues FROM bom_import_batch WHERE id=$1", id).Scan(&raw) != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	var issues []bom.Issue
	json.Unmarshal(raw, &issues)
	out := []any{}
	if strings.HasSuffix(r.URL.Path, "/export") {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="bom-issues.csv"`)
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "severity", "rule_id", "item_uid", "field", "reason"})
		for _, i := range issues {
			reason := i.Reason
			if len(reason) > 0 && strings.ContainsAny(reason[:1], "=+-@") {
				reason = "'" + reason
			}
			cw.Write([]string{i.ID, i.Severity, i.Rule, i.Item, i.Field, reason})
		}
		cw.Flush()
		return true
	}
	for _, i := range issues {
		sev := "BLOCK"
		if i.Severity == "WARNING" {
			sev = "WARN"
		} else if i.Severity == "INFO" {
			sev = "INFO"
		}
		filter := r.URL.Query().Get("severity")
		if filter != "" && filter != sev {
			continue
		}
		out = append(out, map[string]any{"id": i.ID, "rule_id": i.Rule, "rule_version": i.Version, "severity": sev, "code": i.Rule, "status": "open", "message": i.Reason, "entity_ref": i.Item, "field_path": i.Field})
	}
	listResponse(w, r, out)
	return true
}
