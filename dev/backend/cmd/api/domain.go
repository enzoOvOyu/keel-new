package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
)

func (s *Server) action(w http.ResponseWriter, r *http.Request, id, action string, rev int, status string, workbook, draft []byte) bool {
	ctx := r.Context()
	if r.Method == "GET" && (action == "preview" || action == "mappings" || action == "relations") {
		respond(w, 200, json.RawMessage(draft))
		return true
	}
	if r.Method != "POST" && r.Method != "PATCH" {
		return false
	}
	if status == "COMMITTED" {
		if action == "commit" {
			s.receipt(w, r, id)
			return true
		}
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	var req struct {
		Revision int        `json:"revision"`
		Items    []bom.Item `json:"items"`
		Reason   string     `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil {
		fail(w, 400, "INVALID_REQUEST")
		return true
	}
	if req.Revision != rev {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	var items []bom.Item
	json.Unmarshal(draft, &items)
	switch action {
	case "mappings", "relations":
		if req.Reason == "" {
			fail(w, 422, "REASON_REQUIRED")
			return true
		}
		var wb bom.Workbook
		json.Unmarshal(workbook, &wb)
		sources := map[string]bool{}
		for _, sh := range wb.Sheets {
			for _, c := range sh.Cells {
				if !c.Sensitive {
					sources[sh.Name+":"+c.Ref] = true
				}
			}
		}
		for i := range req.Items {
			item := &req.Items[i]
			for _, src := range item.Sources {
				if !sources[item.Sheet+":"+src.Ref] {
					fail(w, 422, "SOURCE_REFERENCE_INVALID")
					return true
				}
			}
			if len(item.Sources) == 0 {
				fail(w, 422, "SOURCE_REQUIRED")
				return true
			}
			item.UID = bom.StableID(bom.Identity(*item))
		}
		b, _ := json.Marshal(req.Items)
		tag, e := s.db.Exec(ctx, "UPDATE bom_import_batch SET draft=$2,revision=revision+1,reviewed_revision=NULL,status='MAPPING' WHERE id=$1 AND revision=$3", id, b, rev)
		if e != nil || tag.RowsAffected() != 1 {
			fail(w, 409, "REVISION_CONFLICT")
			return true
		}
		s.audit(ctx, id, "mapping.corrected", req.Reason)
		respond(w, 200, map[string]any{"revision": rev + 1})
		return true
	case "validate":
		issues := bom.Validate(items)
		b, _ := json.Marshal(issues)
		next := "READY_TO_COMMIT"
		if len(issues) > 0 {
			next = "REVIEW_REQUIRED"
		}
		s.db.Exec(ctx, "UPDATE bom_import_batch SET issues=$2,status=$3 WHERE id=$1 AND revision=$4", id, b, next, rev)
		respond(w, 200, issues)
		return true
	case "review":
		if len(bom.Validate(items)) > 0 {
			fail(w, 422, "BLOCKING_ISSUES_PRESENT")
			return true
		}
		s.db.Exec(ctx, "UPDATE bom_import_batch SET reviewed_revision=revision WHERE id=$1 AND revision=$2", id, rev)
		s.audit(ctx, id, "review.confirmed", req.Reason)
		respond(w, 200, map[string]any{"reviewed_revision": rev})
		return true
	case "commit":
		s.commit(w, r, id, rev)
		return true
	}
	return false
}
func (s *Server) audit(ctx context.Context, id, action, reason string) {
	b, _ := json.Marshal(map[string]string{"reason": reason})
	s.db.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,'local-developer',$3,$4,$5)", uuid.NewString(), s.project, action, id, b)
}
func (s *Server) receipt(w http.ResponseWriter, r *http.Request, id string) {
	var revision, publish string
	s.db.QueryRow(r.Context(), "SELECT committed_revision::text,CASE WHEN EXISTS(SELECT 1 FROM outbox_event WHERE payload->>'import_id'=$1 AND published_at IS NULL) THEN 'pending' ELSE 'published' END FROM bom_import_batch WHERE id=$1", id).Scan(&revision, &publish)
	respond(w, 200, map[string]string{"id": id, "bom_revision": revision, "cmdb_status": publish})
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
	var raw []byte
	e = tx.QueryRow(ctx, "SELECT status,revision,reviewed_revision,draft,content_sha256 FROM bom_import_batch WHERE id=$1 AND project_id=$2 FOR UPDATE", id, s.project).Scan(&status, &version, &reviewed, &raw, &hash)
	if e != nil {
		fail(w, 404, "NOT_FOUND")
		return
	}
	if status == "COMMITTED" {
		tx.Rollback(ctx)
		s.receipt(w, r, id)
		return
	}
	var items []bom.Item
	json.Unmarshal(raw, &items)
	if version != rev || reviewed == nil || *reviewed != version || len(bom.Validate(items)) > 0 {
		fail(w, 422, "REVIEW_OR_VALIDATION_REQUIRED")
		return
	}
	revision := uuid.NewString()
	_, e = tx.Exec(ctx, "INSERT INTO bom_revision(id,import_id,project_id,digest) VALUES($1,$2,$3,$4)", revision, id, s.project, hash)
	if e != nil {
		fail(w, 409, "STATE_CONFLICT")
		return
	}
	// Material identity is global; stable IDs are derived from the confirmed identity, not row order.
	for _, item := range items {
		item.UID = bom.StableID(bom.Identity(item))
		b, _ := json.Marshal(item)
		_, e = tx.Exec(ctx, "INSERT INTO bom_item(item_uid,identity_key,category,serial,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT(identity_key) DO NOTHING", item.UID, bom.Identity(item), item.Category, item.Serial, b)
		if e != nil {
			fail(w, 409, "IDENTITY_CONFLICT")
			return
		}
		_, e = tx.Exec(ctx, "INSERT INTO project_asset(project_id,item_uid,revision_id) VALUES($1,$2,$3)", s.project, item.UID, revision)
		if e != nil {
			fail(w, 409, "IDENTITY_CONFLICT")
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"import_id": id, "revision_id": revision, "items": items})
	_, e = tx.Exec(ctx, "INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES($1,$2,'bom.committed',$3)", uuid.NewString(), s.project, payload)
	if e == nil {
		_, e = tx.Exec(ctx, "UPDATE bom_import_batch SET status='COMMITTED',committed_revision=$2 WHERE id=$1", id, revision)
	}
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id) VALUES($1,$2,'local-developer','bom.committed',$3)", uuid.NewString(), s.project, id)
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
	e = tx.QueryRow(ctx, "SELECT id,payload FROM outbox_event WHERE published_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1").Scan(&id, &raw)
	if e != nil {
		return
	}
	var event struct {
		Items []bom.Item `json:"items"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	for _, i := range event.Items {
		b, _ := json.Marshal(i)
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_entity(item_uid,payload) VALUES($1,$2) ON CONFLICT(item_uid) DO UPDATE SET payload=excluded.payload,updated_at=now()", i.UID, b); e != nil {
			return
		}
	}
	if _, e = tx.Exec(ctx, "UPDATE outbox_event SET published_at=now() WHERE id=$1", id); e == nil {
		tx.Commit(ctx)
	}
}

var _ = strings.TrimSpace
