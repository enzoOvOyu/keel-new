package main

import (
	"encoding/json"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

func (s *Server) applyAI(w http.ResponseWriter, r *http.Request, id string, rev int, status string, workbook []byte) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) != 7 || p[4] != "ai-candidates" || p[6] != "apply" || r.Method != "POST" {
		return false
	}
	if status == "COMMITTED" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	var q struct {
		Revision int    `json:"revision"`
		Indices  []int  `json:"indices"`
		Reason   string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Revision != rev || q.Reason == "" || len(q.Indices) == 0 {
		fail(w, 409, "REVISION_OR_REVIEW_REQUIRED")
		return true
	}
	var raw []byte
	if s.db.QueryRow(r.Context(), "SELECT diagnostic FROM ai_task WHERE id::text=$1 AND import_id=$2 AND status='SUCCEEDED'", p[5], id).Scan(&raw) != nil {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	var diag struct {
		Candidates []ai.Candidate `json:"candidates"`
	}
	json.Unmarshal(raw, &diag)
	s.db.QueryRow(r.Context(), "SELECT profiles FROM bom_import_batch WHERE id=$1", id).Scan(&raw)
	var profiles []bom.Profile
	var wb bom.Workbook
	json.Unmarshal(raw, &profiles)
	json.Unmarshal(workbook, &wb)
	changed := map[string]string{}
	for _, index := range q.Indices {
		if index < 0 || index >= len(diag.Candidates) {
			fail(w, 422, "CANDIDATE_INDEX_INVALID")
			return true
		}
		c := diag.Candidates[index]
		if c.SourceFile != id || (bom.Field(c.Field) == "" && c.Field != "sheet_purpose") {
			fail(w, 422, "CANDIDATE_SOURCE_OR_FIELD_INVALID")
			return true
		}
		key := c.Sheet + ":" + c.Cell
		if field := changed[key]; field != "" && field != c.Field {
			fail(w, 422, "AMBIGUOUS_CANDIDATES")
			return true
		}
		changed[key] = c.Field
		valid := false
		for j := range profiles {
			profile := &profiles[j]
			if profile.Sheet != c.Sheet {
				continue
			}
			for _, sh := range wb.Sheets {
				if sh.Name != c.Sheet {
					continue
				}
				for _, cell := range sh.Cells {
					if cell.Ref == c.Cell && cell.Row == c.Row && cell.Column == c.Column && cell.Value == c.Original && !cell.Sensitive {
						for k := range profile.Columns {
							if profile.Columns[k].Index == cell.Column {
								if c.Field == "sheet_purpose" {
									if c.Normalized != "assets" && c.Normalized != "cabling" && c.Normalized != "unclassified" && c.Normalized != "unknown" {
										fail(w, 422, "INVALID_SHEET_PURPOSE")
										return true
									}
									profile.Purpose = c.Normalized
									if c.Normalized == "unknown" {
										profile.Purpose = "unclassified"
									}
									profile.Confirmed = false
								} else {
									profile.Columns[k].Field = c.Field
								}
								profile.Columns[k].Reason = q.Reason
								valid = true
							}
						}
					}
				}
			}
		}
		if !valid {
			fail(w, 422, "CANDIDATE_PROVENANCE_INVALID")
			return true
		}
	}
	items, issues := bom.Generate(wb, profiles)
	pr, _ := json.Marshal(profiles)
	d, _ := json.Marshal(items)
	i, _ := json.Marshal(issues)
	e := s.mutate(r.Context(), id, "ai.candidate_human_confirmed", q.Reason, "UPDATE bom_import_batch SET profiles=$2,draft=$3,issues=$4,revision=revision+1,mapping_confirmed=false,reviewed_revision=NULL,status='MAPPING' WHERE id=$1 AND revision=$5", id, pr, d, i, rev)
	if e != nil {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	respond(w, 200, map[string]int{"revision": rev + 1})
	return true
}
