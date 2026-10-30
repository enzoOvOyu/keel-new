package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/bom"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Server) action(w http.ResponseWriter, r *http.Request, id, action string, rev int, status string, workbook, draft []byte) bool {
	ctx := r.Context()
	if r.Method == "GET" && action == "mappings" {
		var wb bom.Workbook
		json.Unmarshal(workbook, &wb)
		rows := []any{}
		for _, m := range bom.Rules(wb) {
			rows = append(rows, map[string]any{"id": m.ID, "source": map[string]any{"document_id": id, "sheet": m.Sheet, "cell": m.Cell, "header": m.Header}, "method": "deterministic", "status": "candidate", "mapping_revision": rev, "target_field": m.Field, "confidence": m.Confidence, "reason": m.Reason})
		}
		respond(w, 200, rows)
		return true
	}
	if r.Method == "GET" && (action == "preview" || action == "draft") {
		var items []bom.Item
		json.Unmarshal(draft, &items)
		if action == "preview" {
			counts := map[string]int{}
			for _, i := range items {
				counts[i.Category]++
			}
			respond(w, 200, map[string]any{"import_id": id, "eligible_to_commit": !hasBlockers(s.validateDraft(r.Context(), id, items)), "diff_summary": map[string]int{"added": len(items), "changed": 0, "unchanged": 0, "removed": 0}, "items": items, "counts": counts, "revision": rev, "formal": false})
			return true
		}
		respond(w, 200, json.RawMessage(draft))
		return true
	}
	if r.Method != "POST" && r.Method != "PATCH" {
		return false
	}
	if status == "COMMITTED" && action != "commit" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	var req struct {
		WarningAck       bool       `json:"acknowledge_warnings"`
		Revision         int        `json:"revision"`
		ExpectedRevision int        `json:"expected_revision"`
		Confirm          bool       `json:"confirm"`
		ChangeReason     string     `json:"change_reason"`
		Items            []bom.Item `json:"items"`
		Reason           string     `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil {
		fail(w, 400, "INVALID_REQUEST")
		return true
	}
	if req.ExpectedRevision != 0 {
		req.Revision = req.ExpectedRevision
	}
	if req.ChangeReason != "" {
		req.Reason = req.ChangeReason
	}
	if action == "commit" && !req.Confirm {
		fail(w, 422, "CONFIRM_REQUIRED")
		return true
	}
	if req.Revision != rev {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	var items []bom.Item
	json.Unmarshal(draft, &items)
	switch action {
	case "draft":
		if req.Reason == "" {
			fail(w, 422, "REASON_REQUIRED")
			return true
		}
		var wb bom.Workbook
		json.Unmarshal(workbook, &wb)
		sources := map[string]bom.Cell{}
		for _, sh := range wb.Sheets {
			for _, c := range sh.Cells {
				if !c.Sensitive {
					sources[sh.Name+":"+c.Ref] = c
				}
			}
		}
		oldItems := map[string]bom.Item{}
		for _, old := range items {
			oldItems[old.UID] = old
		}
		for i := range req.Items {
			item := &req.Items[i]
			old := oldItems[item.UID]
			item.Corrections = map[string]bom.ManualCorrection{}
			for k, c := range old.Corrections {
				item.Corrections[k] = c
			}
			before := itemValues(old)
			after := itemValues(*item)
			for field, value := range after {
				if value != before[field] {
					item.Corrections[field] = bom.ManualCorrection{SourceFile: id, Actor: actorName(ctx), Reason: req.Reason, Before: before[field], After: value, At: time.Now().UTC().Format(time.RFC3339Nano), Kind: "manual"}
				}
			}
			item.Status = "planned"
			for field, src := range item.Sources {
				sourceSheet := src.Sheet
				if sourceSheet == "" {
					sourceSheet = item.Sheet
				}
				canonical, ok := sources[sourceSheet+":"+src.Ref]
				if !ok {
					fail(w, 422, "SOURCE_REFERENCE_INVALID")
					return true
				}
				item.Sources[field] = canonical
			}
			if len(item.Sources) == 0 {
				fail(w, 422, "SOURCE_REQUIRED")
				return true
			}
			// Candidate identities remain stable throughout review; normalize at commit.
		}
		b, _ := json.Marshal(req.Items)
		e := s.mutate(ctx, id, "mapping.corrected", req.Reason, "UPDATE bom_import_batch SET draft=$2,revision=revision+1,mapping_confirmed=true,reviewed_revision=NULL,status='MAPPING' WHERE id=$1 AND revision=$3", id, b, rev)
		if e != nil {
			fail(w, 409, "REVISION_CONFLICT")
			return true
		}
		respond(w, 200, map[string]any{"revision": rev + 1})
		return true
	case "validate":
		s.queueValidation(w, r, id, rev)
		return true
	case "review":
		for _, i := range s.validateDraft(ctx, id, items) {
			if i.Severity == "WARNING" && !req.WarningAck {
				fail(w, 422, "WARNING_CONFIRMATION_REQUIRED")
				return true
			}
		}
		if status != "READY_TO_COMMIT" {
			fail(w, 409, "VALIDATION_REQUIRED")
			return true
		}
		if strings.TrimSpace(req.Reason) == "" {
			fail(w, 422, "REASON_REQUIRED")
			return true
		}
		if hasBlockers(s.validateDraft(ctx, id, items)) {
			fail(w, 422, "BLOCKING_ISSUES_PRESENT")
			return true
		}
		s.review(w, r, id, rev, req.Reason, items)
		return true
	case "commit":
		s.commit(w, r, id, rev)
		return true
	}
	return false
}
func (s *Server) audit(ctx context.Context, id, action, reason string) {
	b, _ := json.Marshal(map[string]string{"reason": reason})
	s.db.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)", uuid.NewString(), s.project, actorName(ctx), action, id, b)
}
func (s *Server) receipt(w http.ResponseWriter, r *http.Request, id string) {
	var revision, hash string
	var at time.Time
	e := s.db.QueryRow(r.Context(), "SELECT i.committed_revision::text,r.digest,r.created_at FROM bom_import_batch i JOIN bom_revision r ON r.id=i.committed_revision WHERE i.id=$1 AND i.project_id=$2", id, s.project).Scan(&revision, &hash, &at)
	if e != nil {
		respond(w, 200, map[string]any{"id": id, "cmdb_status": "not_committed"})
		return
	}
	var pending bool
	s.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM outbox_event WHERE payload->>'import_id'=$1 AND published_at IS NULL)", id).Scan(&pending)
	var retries int
	var lastError *string
	s.db.QueryRow(r.Context(), "SELECT COALESCE(MAX(retry_count),0),MAX(last_error_code) FROM outbox_event WHERE payload->>'import_id'=$1", id).Scan(&retries, &lastError)
	publish := "published"
	if pending {
		publish = "pending"
		if retries > 0 {
			publish = "retrying"
		}
	}
	code := 201
	if r.Method == "GET" {
		code = 200
	}
	respond(w, code, map[string]any{"id": revision, "project_id": s.project, "bom_import_batch_id": id, "content_sha256": hash, "status": "committed", "created_at": at, "bom_revision": revision, "cmdb_status": publish, "cmdb_retry_count": retries, "cmdb_error_code": lastError})
}
func (s *Server) commit(w http.ResponseWriter, r *http.Request, id string, rev int) {
	ctx := r.Context()
	if r.Header.Get("Idempotency-Key") == "" {
		fail(w, 400, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	defer tx.Rollback(ctx)
	var status, hash string
	var version int
	var reviewed *int
	var raw, reviewBindings []byte
	e = tx.QueryRow(ctx, "SELECT status,revision,reviewed_revision,draft,content_sha256,review_model_bindings FROM bom_import_batch WHERE id=$1 AND project_id=$2 FOR UPDATE", id, s.project).Scan(&status, &version, &reviewed, &raw, &hash, &reviewBindings)
	if e != nil {
		fail(w, 404, "NOT_FOUND")
		return
	}
	if status == "COMMITTED" {
		if _, newSubmit := ctx.Value(submitScopeKey{}).(submitScope); newSubmit {
			fail(w, 409, "IMPORT_IMMUTABLE")
			return
		}
		var existingImport string
		keyErr := tx.QueryRow(ctx, "SELECT import_id::text FROM idempotency_record WHERE project_id=$1 AND key=$2", s.project, r.Header.Get("Idempotency-Key")).Scan(&existingImport)
		if keyErr == nil && existingImport != id {
			fail(w, 409, "IDEMPOTENCY_KEY_REUSED")
			return
		}
		if rev != version {
			fail(w, 409, "REVISION_CONFLICT")
			return
		}
		tx.Rollback(ctx)
		s.receipt(w, r, id)
		return
	}
	var items []bom.Item
	json.Unmarshal(raw, &items)
	scope, workbenchSubmit := ctx.Value(submitScopeKey{}).(submitScope)
	var wb bom.Workbench
	var mode string
	if e = tx.QueryRow(ctx, "SELECT workbench,import_mode FROM bom_import_batch WHERE id=$1", id).Scan(&raw, &mode); e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	if mode != "legacy" && !workbenchSubmit {
		fail(w, 422, "WORKBENCH_CONFIRMATION_REQUIRED")
		return
	}
	selected := map[string]bool{}
	selectedAssets := map[string]string{}
	existingAssets := map[string]bool{}
	if workbenchSubmit {
		if len(scope.IDs) == 0 {
			fail(w, 422, "SELECTION_REQUIRED")
			return
		}
		for _, v := range scope.IDs {
			if selected[v] {
				fail(w, 422, "DUPLICATE_SELECTION")
				return
			}
			selected[v] = true
		}
		var previous []byte
		if tx.QueryRow(ctx, "SELECT results FROM bom_submission WHERE project_id=$1 AND idempotency_key=$2 AND import_id=$3", s.project, r.Header.Get("Idempotency-Key"), id).Scan(&previous) == nil {
			tx.Rollback(ctx)
			s.receipt(w, r, id)
			return
		}
		json.Unmarshal(raw, &wb)
		s.workbenchIssues(ctx, id, &wb)
		found := 0
		items = nil
		for _, rec := range wb.Records {
			if mode == "fixed" && !rec.Ignored && rec.Blocked() {
				fail(w, 422, "FIXED_IMPORT_HAS_BLOCKERS")
				return
			}
			if selected[rec.ID] {
				found++
				if rec.Imported || rec.Ignored || !rec.Confirmed || rec.Blocked() {
					fail(w, 422, "SELECTED_RECORDS_NOT_CONFIRMED_OR_VALID")
					return
				}
				for _, issue := range rec.Issues {
					if issue.Severity == "WARNING" && !scope.WarningAck {
						fail(w, 422, "WARNING_CONFIRMATION_REQUIRED")
						return
					}
				}
				items = append(items, bom.WorkbenchItem(rec, s.project))
			}
			if mode == "fixed" && !rec.Ignored && !rec.Imported && !selected[rec.ID] {
				fail(w, 422, "FIXED_IMPORT_REQUIRES_ALL_RECORDS")
				return
			}
		}
		if found != len(selected) {
			fail(w, 422, "RECORD_NOT_FOUND")
			return
		}
		// Previously imported dependencies retain their identity and may support later subsets.
		imported := map[string]bom.Item{}
		for _, rec := range wb.Records {
			if rec.Imported {
				imported[rec.ID] = bom.WorkbenchItem(rec, s.project)
			}
		}
		items = bom.ResolveWorkbench(bom.EnsureRacks(bom.ExpandPorts(bom.ResolveWorkbench(items))))
		items = s.existingBOMDependencies(ctx, items)
		included := map[string]bool{}
		for _, i := range items {
			included[i.UID] = true
		}
		for pos := 0; pos < len(items); pos++ {
			for _, dep := range []string{items[pos].Parent, items[pos].A, items[pos].B, items[pos].Fields["rack_uid"], items[pos].Fields["port_uid"]} {
				if dep == "" || included[dep] {
					continue
				}
				if prior, ok := imported[dep]; ok {
					items = append(items, prior)
					included[dep] = true
				} else {
					fail(w, 422, "SELECT_RELATED_DEPENDENCIES")
					return
				}
			}
		}
		items = bom.ResolveWorkbench(items)

	}

	var bindings map[string]string
	json.Unmarshal(reviewBindings, &bindings)
	for j := range items {
		if ref := bindings[items[j].UID]; ref != "" {
			if items[j].Fields == nil {
				items[j].Fields = map[string]string{}
			}
			items[j].Fields["model_version_ref"] = ref
		}
	}
	if version != rev || (!workbenchSubmit && (reviewed == nil || *reviewed != version)) || hasBlockers(s.validateDraft(ctx, id, items)) {
		fail(w, 422, "REVIEW_OR_VALIDATION_REQUIRED")
		return
	}
	normalized, normalizeErr := bom.Normalize(items)
	if normalizeErr != nil {
		fail(w, 422, "REFERENCE_UNRESOLVED")
		return
	}
	for j, i := range items {
		if selected[i.UID] {
			selectedAssets[i.UID] = normalized[j].UID
		}
	}
	items = normalized
	for j := range items {
		item := &items[j]
		if item.Model != "" && bom.RequiresModel(bom.AssetType(*item)) {
			var modelRef string
			e = tx.QueryRow(ctx, "SELECT id::text FROM device_model_revision WHERE model=$1 AND category=$2 AND ($3='' OR id::text=$3)", item.Model, bom.AssetType(*item), item.Fields["model_version_ref"]).Scan(&modelRef)
			if e != nil {
				fail(w, 422, "MODEL_REVISION_REQUIRED")
				return
			}
			item.Fields["model_version_ref"] = modelRef
		}
	}
	canonicalPayload, _ := json.Marshal(items)
	digestBytes := sha256.Sum256(canonicalPayload)
	digest := hex.EncodeToString(digestBytes[:])
	key := r.Header.Get("Idempotency-Key")
	var previousDigest, previousImport string
	e = tx.QueryRow(ctx, "SELECT request_digest,import_id::text FROM idempotency_record WHERE project_id=$1 AND key=$2", s.project, key).Scan(&previousDigest, &previousImport)
	if e == nil && (previousDigest != digest || previousImport != id) {
		fail(w, 409, "IDEMPOTENCY_KEY_REUSED")
		return
	}
	if _, e = tx.Exec(ctx, "INSERT INTO idempotency_record(project_id,key,request_digest,import_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", s.project, key, digest, id); e != nil {
		fail(w, 409, "IDEMPOTENCY_CONFLICT")
		return
	}
	lockedIDs := make([]string, 0, len(items))
	for _, i := range items {
		lockedIDs = append(lockedIDs, i.UID)
	}
	sort.Strings(lockedIDs)
	for _, uid := range lockedIDs {
		if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", uid); e != nil {
			fail(w, 409, "ASSET_LOCK_FAILED")
			return
		}
	}
	revision := uuid.NewString()
	_, e = tx.Exec(ctx, "INSERT INTO bom_revision(id,import_id,project_id,digest,created_at) VALUES($1,$2,$3,$4,clock_timestamp())", revision, id, s.project, digest)
	if e != nil {
		fail(w, 409, "STATE_CONFLICT")
		return
	}
	// Material identity is global; stable IDs are derived from the confirmed identity, not row order.
	for _, item := range items {
		// Already normalized with references in one graph pass.
		if workbenchSubmit {
			var oldRaw []byte
			if tx.QueryRow(ctx, "SELECT payload FROM bom_item WHERE identity_key=$1", bom.Identity(item)).Scan(&oldRaw) == nil {
				var old bom.Item
				json.Unmarshal(oldRaw, &old)
				existingAssets[item.UID] = true
				if !sameMaterial(old, item) {
					fail(w, 409, "EXISTING_ASSET_CHANGE_REQUIRES_REVIEW")
					return
				}
			}
		}
		b, _ := json.Marshal(item)
		_, e = tx.Exec(ctx, "INSERT INTO bom_item(item_uid,identity_key,category,serial,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT(identity_key) DO UPDATE SET payload=excluded.payload", item.UID, bom.Identity(item), item.Category, item.Serial, b)
		if e != nil {
			fail(w, 409, "IDENTITY_CONFLICT")
			return
		}
		_, e = tx.Exec(ctx, "INSERT INTO bom_item_revision(revision_id,item_uid,payload) VALUES($1,$2,$3)", revision, item.UID, b)
		if e != nil {
			fail(w, 409, "REVISION_CONFLICT")
			return
		}
		_, e = tx.Exec(ctx, "INSERT INTO project_asset(project_id,item_uid,revision_id) VALUES($1,$2,$3)", s.project, item.UID, revision)
		if e != nil {
			fail(w, 409, "IDENTITY_CONFLICT")
			return
		}
	}
	for _, item := range items {
		if item.Parent != "" {
			if _, e = tx.Exec(ctx, "UPDATE bom_item SET parent_uid=$2 WHERE item_uid=$1", item.UID, item.Parent); e != nil {
				fail(w, 422, "PARENT_REFERENCE_INVALID")
				return
			}
		}
	}
	payload, _ := json.Marshal(map[string]any{"import_id": id, "revision_id": revision, "items": items})
	_, e = tx.Exec(ctx, "INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES($1,$2,'bom.committed',$3)", uuid.NewString(), s.project, payload)
	if e == nil {
		_, e = tx.Exec(ctx, "UPDATE bom_import_batch SET status='COMMITTED',committed_revision=$2 WHERE id=$1", id, revision)
	}
	if e == nil && workbenchSubmit {
		results := []any{}
		index := 0
		remaining := 0
		for j := range wb.Records {
			rec := &wb.Records[j]
			if selected[rec.ID] {
				rec.Imported = true
				rec.AssetID = selectedAssets[rec.ID]
				index++
				resultStatus := "imported"
				if existingAssets[rec.AssetID] {
					resultStatus = "linked_existing"
				}
				results = append(results, map[string]any{"record_id": rec.ID, "asset_id": rec.AssetID, "status": resultStatus, "sheet_name": rec.Sheet, "source_row": rec.Row})
			}
			if !rec.Imported && !rec.Ignored {
				remaining++
			}
		}
		wb.State = "imported"
		next := "COMMITTED"
		if remaining > 0 {
			wb.State = "partially_imported"
			next = "PARTIALLY_IMPORTED"
		}
		workspace, _ := json.Marshal(wb)
		result, _ := json.Marshal(results)
		_, e = tx.Exec(ctx, "UPDATE bom_import_batch SET workbench=$2,status=$3,revision=revision+1 WHERE id=$1", id, workspace, next)
		if e == nil {
			_, e = tx.Exec(ctx, "INSERT INTO bom_submission(id,import_id,project_id,idempotency_key,request_digest,revision_id,results) VALUES($1,$2,$3,$4,$5,$6,$7)", uuid.NewString(), id, s.project, key, digest, revision, result)
		}
	}
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id) VALUES($1,$2,$3,'bom.committed',$4)", uuid.NewString(), s.project, actorName(ctx), id)
	}
	if e != nil || tx.Commit(ctx) != nil {
		fail(w, 500, "COMMIT_FAILED")
		return
	}
	s.receipt(w, r, id)
}
func (s *Server) publish(ctx context.Context) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return
	}
	defer tx.Rollback(ctx)
	var id, project, eventType string
	var raw []byte
	e = tx.QueryRow(ctx, "SELECT id,project_id,event_type,payload FROM outbox_event WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1").Scan(&id, &project, &eventType, &raw)
	if e != nil {
		return
	}
	completed := false
	defer func() {
		if !completed {
			tx.Rollback(ctx)
			recovery, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s.db.Exec(recovery, "UPDATE outbox_event SET retry_count=retry_count+1,last_error_code='CMDB_PROJECTION_FAILED',next_attempt_at=now()+make_interval(secs=>LEAST(60,POWER(2,LEAST(retry_count,6)))::int) WHERE id=$1 AND published_at IS NULL", id)
		}
	}()
	if eventType == "cmdb.changeset.published" {
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_ingest_event(event_id,source_type,source_ref) VALUES($1,'manual',$1) ON CONFLICT DO NOTHING", id); e != nil {
			return
		}
		if _, e = tx.Exec(ctx, "UPDATE outbox_event SET published_at=now(),last_error_code=NULL WHERE id=$1", id); e == nil {
			completed = tx.Commit(ctx) == nil
		}
		return
	}
	var event struct {
		Items    []bom.Item `json:"items"`
		Revision string     `json:"revision_id"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	if e = cmdbService(s).ProjectBOM(ctx, tx, id, project, event.Revision, event.Items); e != nil {
		return
	}

	if _, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id) VALUES($1,$2,'cmdb-projector','cmdb.published',$3)", uuid.NewString(), project, id); e != nil {
		return
	}
	if _, e = tx.Exec(ctx, "UPDATE outbox_event SET published_at=now(),last_error_code=NULL WHERE id=$1", id); e == nil {
		completed = tx.Commit(ctx) == nil
	}
}

var _ = strings.TrimSpace

func itemValues(i bom.Item) map[string]string {
	m := map[string]string{}
	for k, v := range i.Fields {
		m[k] = v
	}
	for k, v := range map[string]string{"category": i.Category, "serial": i.Serial, "hostname": i.Hostname, "model": i.Model, "parent_uid": i.Parent, "port_index": i.Port, "endpoint_a_uid": i.A, "endpoint_b_uid": i.B} {
		m[k] = v
	}
	return m
}

// Importing the same asset may add provenance, but cannot silently replace its material facts.
func sameMaterial(a, b bom.Item) bool {
	if bom.AssetType(a) != bom.AssetType(b) || a.Serial != b.Serial || a.Model != b.Model || a.Parent != b.Parent || a.A != b.A || a.B != b.B {
		return false
	}
	for k, v := range b.Fields {
		if k == "model_version_ref" || k == "category" || k == "item_uid" {
			continue
		}
		if old := a.Fields[k]; old != v && v != "" {
			return false
		}
	}
	return true
}
