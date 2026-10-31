package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
	"time"
)

type submitScope struct {
	IDs        []string
	WarningAck bool
}
type submitScopeKey struct{}
type workbenchRequest struct {
	SourceMappings map[string]string `json:"source_mappings"`
	Mode           string            `json:"import_mode"`
	Revision       int               `json:"revision"`
	IDs            []string          `json:"record_ids"`
	Action         string            `json:"action"`
	Fields         map[string]string `json:"fields"`
	Reason         string            `json:"reason"`
	WarningAck     bool              `json:"acknowledge_warnings"`
}

func (s *Server) loadWorkbench(ctx context.Context, id string) (bom.Workbench, string, error) {
	var raw []byte
	var mode string
	e := s.db.QueryRow(ctx, "SELECT workbench,import_mode FROM bom_import_batch WHERE id=$1", id).Scan(&raw, &mode)
	var b bom.Workbench
	if e == nil {
		e = json.Unmarshal(raw, &b)
	}
	if e == nil && b.State == "recognizing" {
		var active bool
		var code string
		e = s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM bom_job WHERE import_id=$1 AND kind='recognize' AND status IN ('pending','running')),COALESCE((SELECT error_code FROM bom_job WHERE import_id=$1 AND kind='recognize' ORDER BY created_at DESC LIMIT 1),'RECOGNITION_INTERRUPTED')", id).Scan(&active, &code)
		if e == nil && !active {
			b.State, b.Phase, b.Error = "pending_review", "review", code
		}
	}
	return b, mode, e
}
func (s *Server) workbenchIssues(ctx context.Context, id string, b *bom.Workbench) []bom.Issue {
	items := s.existingBOMDependencies(ctx, bom.WorkbenchItems(*b, s.project))
	byID := map[string]bom.Item{}
	for _, i := range items {
		byID[i.UID] = i
	}
	for j := range b.Records {
		r := &b.Records[j]
		if i, ok := byID[r.ID]; ok {
			r.Item.Parent = i.Parent
			r.Item.A = i.A
			r.Item.B = i.B
			for _, f := range []string{"parent_uid", "endpoint_a_uid", "endpoint_b_uid", "rack_uid"} {
				if v := i.Fields[f]; v != "" {
					r.Item.Fields[f] = v
				}
			}
		}
	}
	issues := s.validateDraft(ctx, id, items)
	existing := map[string]bom.Item{}
	existingRows, e := s.db.Query(ctx, "SELECT b.identity_key,b.payload FROM bom_item b JOIN project_asset pa ON pa.item_uid=b.item_uid WHERE pa.project_id=$1", s.project)
	if e == nil {
		for existingRows.Next() {
			var key string
			var raw []byte
			existingRows.Scan(&key, &raw)
			var old bom.Item
			json.Unmarshal(raw, &old)
			existing[key] = old
		}
		existingRows.Close()
	}
	for _, i := range items {
		if old, ok := existing[bom.Identity(i)]; ok && materialFactsDiffer(old, i) {
			issues = append(issues, bom.RecordIssue(i.UID, "existing_asset", "已有资产事实存在差异；请查看来源并走变更审核，不能通过导入覆盖"))
		}
	}

	var mode string
	s.db.QueryRow(ctx, "SELECT import_mode FROM bom_import_batch WHERE id=$1", id).Scan(&mode)
	if mode == "fixed" {
		for _, rec := range b.Records {
			if rec.Ignored || rec.Imported {
				continue
			}
			for _, cell := range rec.Unmapped {
				if rec.Dispositions[cell.Ref] == "" {
					issues = append(issues, bom.RecordIssue(rec.ID, cell.Ref, "此列没有标准字段映射：请在修正面板中指定字段，或注明作为原始参考保留"))
				}
			}
		}
	}
	bom.ValidateWorkbench(b, issues)
	return issues
}
func (s *Server) workbenchAPI(w http.ResponseWriter, r *http.Request, id string, rev int, status string) bool {
	prefix := "/api/v1/bom-imports/" + id + "/workbench"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	action := strings.TrimPrefix(r.URL.Path, prefix)
	ctx := r.Context()
	if r.Method == "GET" && action == "/progress" {
		// Poll persisted task metadata only. Never decode candidates, Workbook IR,
		// or rerun engineering validation to render a progress screen.
		var progress []byte
		err := s.db.QueryRow(ctx, `WITH snapshot AS MATERIALIZED (
			SELECT import_mode,revision,
    EXISTS(SELECT 1 FROM bom_job j WHERE j.import_id=bom_import_batch.id AND j.kind='recognize' AND j.status IN ('pending','running')) active,
    COALESCE((SELECT j.error_code FROM bom_job j WHERE j.import_id=bom_import_batch.id AND j.kind='recognize' ORDER BY created_at DESC LIMIT 1),'RECOGNITION_INTERRUPTED') job_error,
				workbench-ARRAY['records','classifications'] AS workbench,
				COALESCE(jsonb_array_length(NULLIF(workbench->'records','null'::jsonb)),0) AS record_count
			FROM bom_import_batch WHERE id=$1
		) SELECT jsonb_build_object(
			'mode',import_mode,'revision',revision,
			'state',CASE WHEN workbench->>'state'='recognizing' AND NOT active THEN 'pending_review' ELSE COALESCE(workbench->>'state','pending_review') END,
			'phase',CASE WHEN workbench->>'state'='recognizing' AND NOT active THEN 'review' ELSE COALESCE(workbench->>'phase','') END,
			'error',CASE WHEN workbench->>'state'='recognizing' AND NOT active THEN job_error ELSE COALESCE(workbench->>'error','') END,
			'analysis',workbench->'analysis','coverage',workbench->'coverage',
			'record_count',record_count,
			'steps',COALESCE((SELECT jsonb_agg(event ORDER BY position)
				FROM jsonb_array_elements(COALESCE(workbench->'steps','[]'::jsonb)) WITH ORDINALITY AS e(event,position)
				WHERE position > COALESCE(jsonb_array_length(workbench->'steps'),0)-16),'[]'::jsonb),
			'sheets',COALESCE((SELECT jsonb_agg(sheet-'plan')
				FROM jsonb_array_elements(COALESCE(workbench->'sheets','[]'::jsonb)) AS sheet),'[]'::jsonb)
		) FROM snapshot`, id).Scan(&progress)
		if err != nil {
			fail(w, 500, "DATABASE_ERROR")
		} else {
			respond(w, 200, json.RawMessage(progress))
		}
		return true
	}
	b, mode, e := s.loadWorkbench(ctx, id)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return true
	}
	if r.Method == "GET" {
		if action != "" {
			fail(w, 404, "NOT_FOUND")
			return true
		}
		if mode == "legacy" {
			b = s.legacyWorkbench(ctx, id, status)
		} else {
			s.workbenchIssues(ctx, id, &b)
		}
		submissions := []any{}
		rows, err := s.db.Query(ctx, "SELECT revision_id::text,results,created_at FROM bom_submission WHERE import_id=$1 ORDER BY created_at DESC", id)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var revision string
				var results []byte
				var at time.Time
				rows.Scan(&revision, &results, &at)
				submissions = append(submissions, map[string]any{"bom_revision": revision, "results": json.RawMessage(results), "created_at": at})
			}
		}
		if mode == "legacy" && status == "COMMITTED" {
			var revision string
			var at time.Time
			if s.db.QueryRow(ctx, "SELECT r.id::text,r.created_at FROM bom_revision r WHERE import_id=$1 ORDER BY created_at DESC LIMIT 1", id).Scan(&revision, &at) == nil {
				results := []any{}
				for _, rec := range b.Records {
					results = append(results, map[string]any{"record_id": rec.ID, "asset_id": rec.AssetID, "status": "imported", "sheet_name": rec.Sheet, "source_row": rec.Row})
				}
				submissions = append(submissions, map[string]any{"bom_revision": revision, "created_at": at, "results": results})
			}
		}
		if recordID := r.URL.Query().Get("record_id"); recordID != "" {
			found := []bom.ImportRecord{}
			for _, rec := range b.Records {
				if rec.ID == recordID {
					found = append(found, rec)
				}
			}
			// Detail resolves immutable workbook sources, including full formula and cached value.
			var wb bom.Workbook
			var sourceJSON []byte
			if s.db.QueryRow(ctx, "SELECT workbook FROM bom_import_batch WHERE id=$1", id).Scan(&sourceJSON) == nil && json.Unmarshal(sourceJSON, &wb) == nil {
				cells := map[string]bom.Cell{}
				for _, sh := range wb.Sheets {
					for _, c := range sh.Cells {
						cells[sh.Name+":"+c.Ref] = c
					}
				}
				for j := range found {
					for k, c := range found[j].Raw {
						if src, ok := cells[c.Sheet+":"+c.Ref]; ok {
							found[j].Raw[k] = src
						}
					}
				}
			}
			b.Records = found
		}
		if r.URL.Query().Get("compact") == "true" {
			for j := range b.Records {
				if len([]rune(b.Records[j].Original)) > 320 {
					b.Records[j].Original = string([]rune(b.Records[j].Original)[:320]) + "…"
				}
				b.Records[j].Raw = nil
				b.Records[j].HeaderContext = nil
				b.Records[j].Item.Sources = nil
				b.Records[j].Unmapped = nil
			}
		}
		respond(w, 200, map[string]any{"mode": mode, "revision": rev, "state": b.State, "error": b.Error, "records": b.Records, "sheets": b.Sheets, "phase": b.Phase, "analysis": b.Analysis, "coverage": b.Coverage, "steps": b.Steps, "entity_types": bom.ImportTypes, "submissions": submissions})
		return true
	}
	if r.Method != "POST" {
		fail(w, 405, "METHOD_NOT_ALLOWED")
		return true
	}
	if status == "UPLOADED" || status == "PARSING" {
		fail(w, 409, "WORKBENCH_NOT_READY")
		return true
	}
	var req workbenchRequest
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil {
		fail(w, 400, "INVALID_REQUEST")
		return true
	}
	if action == "/initialize" {
		if mode != "legacy" || status == "COMMITTED" || req.Revision != rev || req.Mode != "ai" {
			fail(w, 409, "MODE_SELECTION_NOT_AVAILABLE")
			return true
		}
		b = s.legacyWorkbench(ctx, id, status)
		raw, _ := json.Marshal(b)
		items, _ := json.Marshal(bom.WorkbenchItems(b, s.project))
		e = s.mutate(ctx, id, "import.mode.selected", req.Mode, "UPDATE bom_import_batch SET import_mode=$2,workbench=$3,draft=$4,revision=revision+1,reviewed_revision=NULL,mapping_confirmed=true,status='MAPPING' WHERE id=$1 AND revision=$5 AND import_mode='legacy' AND status<>'COMMITTED'", id, req.Mode, raw, items, rev)
		if e != nil {
			fail(w, 409, "REVISION_CONFLICT")
			return true
		}
		respond(w, 200, map[string]any{"revision": rev + 1})
		return true
	}
	if mode == "legacy" {
		fail(w, 422, "IMPORT_MODE_SELECTION_REQUIRED")
		return true
	}
	if action == "/submit" {
		if !subjectOf(ctx).Can("bom:review", s.project, "bom_import", id) || !subjectOf(ctx).Can("bom:execute", s.project, "bom_import", id) {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		if s.replaySubmission(w, r, id, req.IDs) {
			return true
		}
		if req.Revision != rev {
			fail(w, 409, "REVISION_CONFLICT")
			return true
		}
		s.commit(w, r.WithContext(context.WithValue(ctx, submitScopeKey{}, submitScope{req.IDs, req.WarningAck})), id, rev)
		return true
	}
	if req.Revision != rev {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	if status == "COMMITTED" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	if b.State == "recognizing" {
		fail(w, 409, "RECOGNITION_RUNNING")
		return true
	}
	selected := map[string]bool{}
	for _, v := range req.IDs {
		selected[v] = true
	}
	found := map[string]bool{}
	for _, v := range b.Records {
		if selected[v.ID] {
			found[v.ID] = true
		}
	}
	if len(found) != len(selected) {
		fail(w, 422, "RECORD_NOT_FOUND")
		return true
	}
	if action == "/recognize" {
		if mode != "ai" {
			fail(w, 422, "FIXED_MODE_NO_AI")
			return true
		}
		if _, _, err := s.workbenchProvider(ctx, id); err != nil {
			fail(w, 422, err.Error())
			return true
		}
		ids := []string{}
		// Empty is the persisted whole-workbook/resume scope. Expanding it
		// into candidate IDs loses that distinction and misses absent rows.
		for _, v := range b.Records {
			if selected[v.ID] {
				if v.Imported {
					fail(w, 409, "IMPORTED_RECORD_IMMUTABLE")
					return true
				}
				ids = append(ids, v.ID)
			}
		}
		params, _ := json.Marshal(ids)
		b.State = "recognizing"
		b.Phase = "queued"
		if len(req.IDs) == 0 && len(b.Sheets) > 0 {
			b.Phase = "resuming"
		}
		b.Error = ""
		raw, _ := json.Marshal(b)
		tx, err := s.db.Begin(ctx)
		if err != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		defer tx.Rollback(ctx)
		tag, err := tx.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2,revision=revision+1,reviewed_revision=NULL WHERE id=$1 AND revision=$3", id, raw, rev)
		if err != nil || tag.RowsAffected() != 1 {
			fail(w, 409, "REVISION_CONFLICT")
			return true
		}
		_, err = tx.Exec(ctx, "INSERT INTO bom_job(id,import_id,project_id,kind,revision,parameters) VALUES($1,$2,$3,'recognize',$4,$5)", uuid.NewString(), id, s.project, rev+1, params)
		if err != nil || tx.Commit(ctx) != nil {
			fail(w, 500, "QUEUE_FAILED")
			return true
		}
		respond(w, 202, map[string]any{"state": "recognizing", "revision": rev + 1})
		return true
	}
	s.workbenchIssues(ctx, id, &b)
	if action == "/records" {
		if len(selected) == 0 {
			fail(w, 422, "SELECTION_REQUIRED")
			return true
		}
		if req.Action == "confirm" && !subjectOf(ctx).Can("bom:review", s.project, "bom_import", id) {
			fail(w, 403, "FORBIDDEN")
			return true
		}
		// Type changes across unlike known types require individual review, never blind reclassification.
		types := map[string]bool{}
		for _, v := range b.Records {
			if selected[v.ID] && v.Item.Category != "" && v.Item.Category != "other" {
				types[v.Item.Category] = true
			}
		}
		if _, changesType := req.Fields["category"]; changesType && len(types) > 1 {
			fail(w, 422, "MIXED_ENTITY_TYPES_REQUIRE_INDIVIDUAL_REVIEW")
			return true
		}
		if len(req.SourceMappings) > 0 && len(selected) != 1 {
			fail(w, 422, "SOURCE_MAPPING_REQUIRES_SINGLE_RECORD")
			return true
		}
		for j := range b.Records {
			v := &b.Records[j]
			if !selected[v.ID] {
				continue
			}
			if v.Imported {
				fail(w, 409, "IMPORTED_RECORD_IMMUTABLE")
				return true
			}
			switch req.Action {
			case "confirm":
				if v.Ignored || v.Blocked() {
					fail(w, 422, "SELECTED_RECORDS_REQUIRE_CORRECTION")
					return true
				}
				v.Confirmed = true
			case "ignore":
				if strings.TrimSpace(req.Reason) == "" {
					fail(w, 422, "IGNORE_REASON_REQUIRED")
					return true
				}
				if mode == "fixed" && v.Item.Category != "" && v.Item.Category != "other" {
					fail(w, 422, "FIXED_MATERIAL_CANNOT_BYPASS_VALIDATION")
					return true
				}
				v.Ignored = true
				v.Confirmed = false
				v.Reason = req.Reason
			case "restore":
				v.Ignored = false
				v.Confirmed = false
			case "edit":
				if strings.TrimSpace(req.Reason) == "" {
					fail(w, 422, "CORRECTION_REASON_REQUIRED")
					return true
				}
				if v.Item.Fields == nil {
					v.Item.Fields = map[string]string{}
				}
				if v.Before == nil {
					v.Before = map[string]string{}
				}
				if v.Item.Corrections == nil {
					v.Item.Corrections = map[string]bom.ManualCorrection{}
				}
				if req.Fields == nil {
					req.Fields = map[string]string{}
				}
				if v.Dispositions == nil {
					v.Dispositions = map[string]string{}
				}
				for cell, field := range req.SourceMappings {
					found := false
					for _, src := range v.Unmapped {
						if src.Ref != cell {
							continue
						}
						found = true
						if field != "__reference__" {
							if field == "" || bom.Field(field) != field || field == "category" {
								fail(w, 422, "UNKNOWN_FIELD")
								return true
							}
							if strings.TrimSpace(req.Fields[field]) == "" {
								req.Fields[field] = src.Value
							}
							v.Item.Sources[field] = src
						}
						v.Dispositions[cell] = field
					}
					if !found {
						fail(w, 422, "SOURCE_REFERENCE_INVALID")
						return true
					}
				}
				for _, side := range []string{"a", "b"} {
					uid := "endpoint_" + side + "_uid"
					changed := false
					for _, f := range []string{"endpoint_" + side + "_device", "endpoint_" + side + "_port"} {
						if value, ok := req.Fields[f]; ok && value != v.Item.Fields[f] {
							changed = true
						}
					}
					if changed && (req.Fields[uid] == "" || req.Fields[uid] == v.Item.Fields[uid]) {
						req.Fields[uid] = ""
					}
				}
				for f, value := range req.Fields {
					if bom.Field(f) == "" {
						fail(w, 422, "UNKNOWN_FIELD")
						return true
					}
					if f == "category" && !bom.KnownImportType(value) {
						fail(w, 422, "UNKNOWN_ENTITY_TYPE")
						return true
					}
					before := v.Item.Fields[f]
					if _, ok := v.Before[f]; !ok {
						v.Before[f] = before
					}
					v.Item.Fields[f] = strings.TrimSpace(value)
					v.Item.Corrections[f] = bom.ManualCorrection{SourceFile: id, Actor: actorName(ctx), Reason: req.Reason, Before: before, After: value, At: time.Now().UTC().Format(time.RFC3339), Kind: "manual"}
				}
				// Clear top-level mirrors before hydration so removing a value really removes it.
				v.Item.Category = v.Item.Fields["category"]
				v.Item.Serial = v.Item.Fields["serial"]
				v.Item.Model = v.Item.Fields["model"]
				v.Item.Hostname = v.Item.Fields["hostname"]
				v.Item.Parent = v.Item.Fields["parent_uid"]
				v.Item.Port = v.Item.Fields["port_index"]
				v.Item.A = v.Item.Fields["endpoint_a_uid"]
				v.Item.B = v.Item.Fields["endpoint_b_uid"]
				v.Recognition = "recognized"
				if v.Item.Category == "" || v.Item.Category == "other" {
					v.Recognition = "unrecognized"
				}
				v.Method = "manual"
				v.Ambiguity = false
				v.Confirmed = false
				v.Ignored = false
			default:
				fail(w, 422, "UNKNOWN_ACTION")
				return true
			}
		}
	} else if action != "/validate" {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	issues := s.workbenchIssues(ctx, id, &b)
	bom.RefreshCoverageReview(&b)
	items := bom.WorkbenchItems(b, s.project)
	bom.CompactWorkbench(&b)
	raw, _ := json.Marshal(b)
	draft, _ := json.Marshal(items)
	ii, _ := json.Marshal(issues)
	e = s.mutate(ctx, id, "import.workbench."+req.Action, req.Reason, "UPDATE bom_import_batch SET workbench=$2,draft=$3,issues=$4,revision=revision+1,reviewed_revision=NULL,status='MAPPING',mapping_confirmed=true WHERE id=$1 AND revision=$5 AND status NOT IN ('COMMITTED','PARSING')", id, raw, draft, ii, rev)
	if e != nil {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	respond(w, 200, map[string]any{"revision": rev + 1})
	return true
}
func (s *Server) initializeWorkbench(ctx context.Context, id string, wb bom.Workbook) {
	var mode, project string
	s.db.QueryRow(ctx, "SELECT import_mode,project_id FROM bom_import_batch WHERE id=$1", id).Scan(&mode, &project)
	if mode == "legacy" {
		return
	}
	b := bom.WorkbenchRows(wb)
	if mode == "ai" {
		b = bom.PlanningSourceRows(wb)
	}
	items := bom.WorkbenchItems(b, project)
	bom.CompactWorkbench(&b)
	raw, _ := json.Marshal(b)
	draft, _ := json.Marshal(items)
	s.db.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2,draft=$3,mapping_confirmed=true WHERE id=$1", id, raw, draft)
	if mode == "ai" {
		if _, _, e := s.workbenchProvider(ctx, id); e == nil {
			b.State = "recognizing"
			raw, _ = json.Marshal(b)
			s.db.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2 WHERE id=$1", id, raw)
			s.db.Exec(ctx, "INSERT INTO bom_job(id,import_id,project_id,kind,revision) SELECT $2,id,project_id,'recognize',revision FROM bom_import_batch WHERE id=$1", id, uuid.NewString())
		} else {
			b.Error = e.Error()
			raw, _ = json.Marshal(b)
			s.db.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2 WHERE id=$1", id, raw)
		}
	}
}

func (s *Server) replaySubmission(w http.ResponseWriter, r *http.Request, id string, ids []string) bool {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		return false
	}
	var oldImport, revision string
	var raw []byte
	if s.db.QueryRow(r.Context(), "SELECT import_id::text,revision_id::text,results FROM bom_submission WHERE project_id=$1 AND idempotency_key=$2", s.project, key).Scan(&oldImport, &revision, &raw) != nil {
		return false
	}
	var results []struct {
		ID string `json:"record_id"`
	}
	json.Unmarshal(raw, &results)
	selected := map[string]bool{}
	for _, v := range ids {
		selected[v] = true
	}
	same := oldImport == id && len(results) == len(ids) && len(selected) == len(ids)
	for _, v := range results {
		if !selected[v.ID] {
			same = false
		}
	}
	if !same {
		fail(w, 409, "IDEMPOTENCY_KEY_REUSED")
		return true
	}
	var pending bool
	s.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM outbox_event WHERE payload->>'revision_id'=$1 AND published_at IS NULL)", revision).Scan(&pending)
	state := "published"
	if pending {
		state = "pending"
	}
	respond(w, 201, map[string]any{"id": revision, "bom_revision": revision, "project_id": s.project, "bom_import_batch_id": id, "cmdb_status": state, "status": "committed"})
	return true
}

func (s *Server) legacyWorkbench(ctx context.Context, id, status string) bom.Workbench {
	var raw, workbook []byte
	s.db.QueryRow(ctx, "SELECT draft,workbook FROM bom_import_batch WHERE id=$1", id).Scan(&raw, &workbook)
	var items []bom.Item
	json.Unmarshal(raw, &items)
	var source bom.Workbook
	json.Unmarshal(workbook, &source)
	b := bom.WorkbenchRows(source)
	existing := map[string]bom.Item{}
	for _, i := range items {
		existing[i.UID] = i
	}
	if status == "COMMITTED" {
		b.Records = nil
		for _, i := range items {
			row := 1
			cells := []bom.Cell{}
			for _, c := range i.Sources {
				cells = append(cells, c)
				row = c.Row
			}
			b.Records = append(b.Records, bom.ImportRecord{ID: i.UID, Sheet: i.Sheet, Row: row, Raw: cells, Original: i.Hostname + " " + i.Model, Item: i, Recognition: "recognized", Method: "rules", Confirmed: true, Imported: true, AssetID: i.UID, Issues: []bom.Issue{}})
		}
		b.State = "imported"
		return b
	}
	for j := range b.Records {
		if old, ok := existing[b.Records[j].ID]; ok {
			b.Records[j].Item = old
			if old.Category != "" && old.Category != "other" {
				b.Records[j].Recognition = "recognized"
			}
			delete(existing, old.UID)
		}
	}
	for _, i := range existing {
		if len(i.Sources) == 0 {
			continue
		}
		row := 1
		cells := []bom.Cell{}
		for _, c := range i.Sources {
			cells = append(cells, c)
			row = c.Row
		}
		b.Records = append(b.Records, bom.ImportRecord{ID: i.UID, Sheet: i.Sheet, Row: row, Raw: cells, Original: i.Hostname + " " + i.Model, Item: i, Recognition: "recognized", Method: "rules", Issues: []bom.Issue{}})
	}
	return b
}

// Only published BOM assets already associated with this project can satisfy external references.
// This reuses the existing identity; it never constructs assets from a name or another project's inventory.
func (s *Server) existingBOMDependencies(ctx context.Context, items []bom.Item) []bom.Item {
	out := append([]bom.Item{}, items...)
	present := map[string]bool{}
	for _, i := range out {
		present[i.UID] = true
	}
	aliases := map[string]string{}
	for pos := 0; pos < len(out) && len(out) < 10000; pos++ {
		for _, ref := range []struct {
			key   string
			value string
		}{{"parent_uid", out[pos].Parent}, {"endpoint_a_uid", out[pos].A}, {"endpoint_b_uid", out[pos].B}, {"port_uid", out[pos].Fields["port_uid"]}, {"rack_uid", out[pos].Fields["rack_uid"]}} {
			old := ref.value
			if old == "" || present[old] {
				continue
			}
			canonical := aliases[old]
			if canonical == "" {
				var raw []byte
				e := s.db.QueryRow(ctx, `SELECT b.payload FROM bom_item b WHERE (b.item_uid::text=$1 OR b.item_uid=(SELECT bom_item_uid FROM cmdb_entity e WHERE e.item_uid=$1 AND EXISTS(SELECT 1 FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND sc.project_id=$2))) AND EXISTS(SELECT 1 FROM project_asset pa WHERE pa.item_uid=b.item_uid AND pa.project_id=$2)`, old, s.project).Scan(&raw)
				if e != nil {
					continue
				}
				var prior bom.Item
				if json.Unmarshal(raw, &prior) != nil {
					continue
				}
				prior.Category = bom.AssetType(prior)
				canonical = prior.UID
				aliases[old] = canonical
				if !present[canonical] {
					out = append(out, prior)
					present[canonical] = true
				}
			}
			// append can reallocate; update by position, not an earlier pointer into the slice.
			switch ref.key {
			case "parent_uid":
				out[pos].Parent = canonical
			case "endpoint_a_uid":
				out[pos].A = canonical
			case "endpoint_b_uid":
				out[pos].B = canonical
			}
			out[pos].Fields[ref.key] = canonical
		}
	}
	return out
}

func materialFactsDiffer(a, b bom.Item) bool {
	if bom.AssetType(a) != bom.AssetType(b) || a.Serial != b.Serial || a.Model != b.Model {
		return true
	}
	for k, v := range b.Fields {
		switch k {
		case "model_version_ref", "category", "item_uid", "parent_uid", "endpoint_a_uid", "endpoint_b_uid", "rack_uid":
			continue
		}
		if v != "" && a.Fields[k] != v {
			return true
		}
	}
	return false
}

// A later subset references candidate IDs, while committed dependencies live
// under their stable asset IDs. Resolve that boundary before dependency loading.
func resolveImportedReferences(items []bom.Item, records []bom.ImportRecord) {
	aliases := map[string]string{}
	for _, r := range records {
		if r.Imported && r.AssetID != "" {
			aliases[r.ID] = r.AssetID
		}
	}
	for j := range items {
		item := &items[j]
		for _, ref := range []struct {
			field string
			value *string
		}{{"parent_uid", &item.Parent}, {"endpoint_a_uid", &item.A}, {"endpoint_b_uid", &item.B}} {
			if asset := aliases[*ref.value]; asset != "" {
				*ref.value = asset
				item.Fields[ref.field] = asset
			}
		}
		for _, field := range []string{"rack_uid", "port_uid"} {
			if asset := aliases[item.Fields[field]]; asset != "" {
				item.Fields[field] = asset
			}
		}
	}
}
