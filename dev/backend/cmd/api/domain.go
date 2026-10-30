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
	if version != rev || reviewed == nil || *reviewed != version || hasBlockers(s.validateDraft(ctx, id, items)) {
		fail(w, 422, "REVIEW_OR_VALIDATION_REQUIRED")
		return
	}
	normalized, normalizeErr := bom.Normalize(items)
	if normalizeErr != nil {
		fail(w, 422, "REFERENCE_UNRESOLVED")
		return
	}
	items = normalized
	for j := range items {
		item := &items[j]
		if item.Model != "" {
			var modelRef string
			e = tx.QueryRow(ctx, "SELECT id::text FROM device_model_revision WHERE model=$1 AND category=$2 AND ($3='' OR id::text=$3)", item.Model, item.Category, item.Fields["model_version_ref"]).Scan(&modelRef)
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
	var id string
	var raw []byte
	e = tx.QueryRow(ctx, "SELECT id,payload FROM outbox_event WHERE project_id=$1 AND published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1", s.project).Scan(&id, &raw)
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
	var event struct {
		Items    []bom.Item `json:"items"`
		Revision string     `json:"revision_id"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	for _, i := range event.Items {
		b, _ := json.Marshal(i)
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_entity(item_uid,payload,revision_id) VALUES($1,$2,$3) ON CONFLICT(item_uid) DO UPDATE SET payload=excluded.payload,revision_id=excluded.revision_id,updated_at=now() WHERE cmdb_entity.revision_id IS NULL OR (SELECT created_at FROM bom_revision WHERE id=cmdb_entity.revision_id)<=(SELECT created_at FROM bom_revision WHERE id=excluded.revision_id)", i.UID, b, event.Revision); e != nil {
			return
		}
	}
	for _, i := range event.Items {
		edges := map[string]string{}
		if rack := i.Fields["rack_uid"]; rack != "" {
			edges[rack] = "located_in"
		}
		if i.Parent != "" {
			edges[i.Parent] = "contained_by"
		}
		if i.A != "" {
			edges[i.A] = "cable_endpoint_a"
		}
		if i.B != "" {
			edges[i.B] = "cable_endpoint_b"
		}
		for target, kind := range edges {
			if _, e = tx.Exec(ctx, "INSERT INTO cmdb_relationship(from_uid,to_uid,kind,revision_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", i.UID, target, kind, event.Revision); e != nil {
				return
			}
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id) VALUES($1,$2,'cmdb-projector','cmdb.published',$3)", uuid.NewString(), s.project, id); e != nil {
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
