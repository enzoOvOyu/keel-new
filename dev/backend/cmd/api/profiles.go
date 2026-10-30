package main

import (
	"context"
	"encoding/json"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

func (s *Server) profiles(w http.ResponseWriter, r *http.Request, id string, rev int, status string, workbook []byte) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) != 5 {
		return false
	}
	action := p[4]
	if action != "profiles" && action != "relations" && action != "publication" && action != "retry" {
		return false
	}
	var raw []byte
	if s.db.QueryRow(r.Context(), "SELECT profiles FROM bom_import_batch WHERE id=$1", id).Scan(&raw) != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	if r.Method == "GET" {
		if action == "profiles" {
			respond(w, 200, json.RawMessage(raw))
			return true
		}
		if action == "publication" {
			s.receipt(w, r, id)
			return true
		}
		if action == "relations" {
			var d []byte
			s.db.QueryRow(r.Context(), "SELECT draft FROM bom_import_batch WHERE id=$1", id).Scan(&d)
			var items []bom.Item
			json.Unmarshal(d, &items)
			respond(w, 200, bom.Relations(items))
			return true
		}
		return false
	}
	if r.Method != "PATCH" && r.Method != "POST" {
		return false
	}
	if status == "COMMITTED" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	var req struct {
		Revision int           `json:"revision"`
		Profiles []bom.Profile `json:"profiles"`
		Reason   string        `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil || req.Revision != rev || strings.TrimSpace(req.Reason) == "" {
		fail(w, 409, "REVISION_OR_REASON_REQUIRED")
		return true
	}
	if action == "retry" {
		if status != "FAILED" {
			fail(w, 409, "STATE_CONFLICT")
			return true
		}
		if e := s.mutate(r.Context(), id, "parse.retry_requested", req.Reason, "UPDATE bom_import_batch SET status='UPLOADED',error_code=NULL WHERE id=$1 AND status='FAILED'", id); e != nil {
			fail(w, 409, "STATE_CONFLICT")
			return true
		}
		respond(w, 202, map[string]string{"id": id, "status": "UPLOADED"})
		return true
	}
	if action != "profiles" {
		return false
	}
	var wb bom.Workbook
	json.Unmarshal(workbook, &wb)
	if len(req.Profiles) != len(wb.Sheets) {
		fail(w, 422, "ALL_SHEETS_REQUIRED")
		return true
	}
	seen := map[string]bool{}
	for _, p := range req.Profiles {
		if seen[p.Sheet] {
			fail(w, 422, "DUPLICATE_PROFILE")
			return true
		}
		seen[p.Sheet] = true
	}
	for _, sh := range wb.Sheets {
		if !seen[sh.Name] {
			fail(w, 422, "ALL_SHEETS_REQUIRED")
			return true
		}
	}
	items, issues := bom.Generate(wb, req.Profiles)
	a, _ := json.Marshal(req.Profiles)
	b, _ := json.Marshal(items)
	c, _ := json.Marshal(issues)
	tx, e := s.db.Begin(r.Context())
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	defer tx.Rollback(r.Context())
	tag, e := tx.Exec(r.Context(), "UPDATE bom_import_batch SET profiles=$2,draft=$3,issues=$4,revision=revision+1,mapping_confirmed=false,reviewed_revision=NULL,status='MAPPING' WHERE id=$1 AND revision=$5", id, a, b, c, rev)
	if e != nil || tag.RowsAffected() != 1 {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	_, e = tx.Exec(r.Context(), "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES(gen_random_uuid(),$1,$4,'profiles.corrected',$2,jsonb_build_object('reason',$3::text))", s.project, id, req.Reason, actorName(r.Context()))
	if e != nil || tx.Commit(r.Context()) != nil {
		fail(w, 500, "SAVE_FAILED")
		return true
	}
	respond(w, 200, map[string]int{"revision": rev + 1, "candidates": len(items)})
	return true
}
func (s *Server) validateDraft(ctx context.Context, id string, items []bom.Item) []bom.Issue {
	out := bom.Validate(items)
	for _, item := range items {
		for field, c := range item.Sources {
			if c.Formula != "" || c.FormulaBacked {
				out = append(out, bom.Issue{ID: "formula-" + item.UID, Severity: "WARNING", Rule: "FORMULA-CACHE", Version: "1.0", Item: item.UID, Field: field, Reason: "Cached formula result is preserved without execution; confirm its applicability during review"})
				break
			}
		}
	}
	capabilities := map[string]map[string]any{}
	modelVersions := map[string][]string{}
	rows, e := s.db.Query(ctx, "SELECT id::text,category,model,capabilities FROM device_model_revision")
	if e == nil {
		for rows.Next() {
			var revisionID, kind, model string
			var raw []byte
			rows.Scan(&revisionID, &kind, &model, &raw)
			modelVersions[kind+":"+model] = append(modelVersions[kind+":"+model], revisionID)
			var cap map[string]any
			json.Unmarshal(raw, &cap)
			capabilities[kind+":"+model] = cap
		}
		rows.Close()
	}
	out = append(out, bom.Engineering(items, capabilities)...)
	for _, item := range items {
		if item.Model != "" && bom.RequiresModel(item.Category) {
			count := 0
			for _, revisionID := range modelVersions[item.Category+":"+item.Model] {
				if item.Fields["model_version_ref"] == "" || item.Fields["model_version_ref"] == revisionID {
					count++
				}
			}
			if count != 1 {
				out = append(out, bom.Issue{ID: "model-" + item.UID, Severity: "BLOCKER", Rule: "MODEL-APPROVED-REVISION", Version: "1.0", Item: item.UID, Field: "model", Reason: "Model must resolve to exactly one approved catalog revision"})
			}
		}
	}
	var mode string
	if s.db.QueryRow(ctx, "SELECT import_mode FROM bom_import_batch WHERE id=$1", id).Scan(&mode) == nil && mode != "legacy" {
		return out
	}
	var raw, workbook []byte
	var confirmed bool
	if s.db.QueryRow(ctx, "SELECT profiles,workbook,mapping_confirmed FROM bom_import_batch WHERE id=$1", id).Scan(&raw, &workbook, &confirmed) != nil {
		return append(out, bom.Issue{ID: "database", Severity: "BLOCKER", Rule: "STATE", Version: "1.0", Reason: "State unavailable"})
	}
	var profiles []bom.Profile
	var wb bom.Workbook
	json.Unmarshal(raw, &profiles)
	json.Unmarshal(workbook, &wb)
	sensitive := false
	for _, sh := range wb.Sheets {
		for _, c := range sh.Cells {
			if c.Sensitive {
				sensitive = true
				break
			}
		}
	}
	if sensitive {
		out = append(out, bom.Issue{ID: "sensitive-excluded", Severity: "INFO", Rule: "SENSITIVE-EXCLUDED", Version: "1.0", Reason: "Sensitive source cells were isolated and are never mapped into official assets or sent to AI"})
	}
	expected, pi := bom.Build(wb, profiles)
	expected = bom.EnsureRacks(bom.ExpandPorts(expected))
	expected, _ = bom.Reconcile(expected)
	present := map[string]bool{}
	for _, i := range items {
		present[i.UID] = true
	}
	for _, i := range expected {
		if !present[i.UID] {
			out = append(out, bom.Issue{ID: "coverage-" + i.UID, Severity: "BLOCKER", Rule: "SOURCE-ROW-COVERAGE", Version: "1.0", Item: i.UID, Field: i.Sheet, Reason: "Mapped source record disappeared; restore it or explicitly exclude its source row with a reason"})
		}
	}
	out = append(out, pi...)
	if !confirmed {
		out = append(out, bom.Issue{ID: "mapping-confirmation", Severity: "BLOCKER", Rule: "MAPPING-REVIEW", Version: "1.0", Reason: "Confirm candidate mappings before review"})
	}
	return out
}

func hasBlockers(issues []bom.Issue) bool {
	for _, i := range issues {
		if i.Severity == "BLOCKER" {
			return true
		}
	}
	return false
}
