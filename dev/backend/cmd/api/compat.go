package main

import (
	"encoding/json"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

// Existing per-field mapping contract delegates to the same profile and draft pipeline.
func (s *Server) mappingPatch(w http.ResponseWriter, r *http.Request, id string, rev int, status string, workbook []byte) bool {
	prefix := "/api/v1/bom-imports/" + id + "/mappings/"
	if r.Method != "PATCH" || !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	if status == "COMMITTED" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	mappingID := strings.TrimPrefix(r.URL.Path, prefix)
	var q struct {
		Target   string `json:"target_field"`
		Status   string `json:"status"`
		Reason   string `json:"reason"`
		Revision int    `json:"mapping_revision"`
		Value    any    `json:"normalized_value"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Reason == "" {
		fail(w, 422, "REASON_REQUIRED")
		return true
	}
	if q.Revision != rev {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	if q.Target != "" && bom.Field(q.Target) == "" {
		fail(w, 422, "UNKNOWN_STANDARD_FIELD")
		return true
	}
	var raw []byte
	if s.db.QueryRow(r.Context(), "SELECT profiles FROM bom_import_batch WHERE id=$1", id).Scan(&raw) != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	var profiles []bom.Profile
	var wb bom.Workbook
	json.Unmarshal(raw, &profiles)
	json.Unmarshal(workbook, &wb)
	found := false
	for j := range profiles {
		p := &profiles[j]
		for _, sh := range wb.Sheets {
			if sh.Name != p.Sheet {
				continue
			}
			for _, c := range sh.Cells {
				if sh.Name+":"+c.Ref != mappingID {
					continue
				}
				for k := range p.Columns {
					if p.Columns[k].Index == c.Column {
						p.Columns[k].Field = q.Target
						p.Columns[k].Ignored = q.Status == "rejected"
						p.Columns[k].Reason = q.Reason
						found = true
					}
				}
			}
		}
	}
	if !found {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	items, issues := bom.Generate(wb, profiles)
	p, _ := json.Marshal(profiles)
	d, _ := json.Marshal(items)
	i, _ := json.Marshal(issues)
	tag, e := s.db.Exec(r.Context(), "UPDATE bom_import_batch SET profiles=$2,draft=$3,issues=$4,revision=revision+1,mapping_confirmed=false,reviewed_revision=NULL,status='MAPPING' WHERE id=$1 AND revision=$5", id, p, d, i, rev)
	if e != nil || tag.RowsAffected() != 1 {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	s.audit(r.Context(), id, "mapping.corrected", q.Reason)
	respond(w, 200, map[string]any{"id": mappingID, "source": map[string]any{"document_id": id, "sheet": strings.Split(mappingID, ":")[0], "cell": strings.Split(mappingID, ":")[len(strings.Split(mappingID, ":"))-1]}, "method": "manual", "status": q.Status, "mapping_revision": rev + 1, "target_field": q.Target, "reason": q.Reason})
	return true
}
