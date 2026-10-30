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

func (s *Server) jobs(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "GET" || !strings.HasPrefix(r.URL.Path, "/api/v1/jobs/") {
		return false
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	var kind, status, importID string
	var rev int
	var created time.Time
	var result []byte
	var errCode *string
	e := s.db.QueryRow(r.Context(), "SELECT kind,status,import_id,revision,result,error_code,created_at FROM bom_job WHERE id::text=$1 AND project_id=$2", id, s.project).Scan(&kind, &status, &importID, &rev, &result, &errCode, &created)
	if e != nil {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	respond(w, 200, map[string]any{"created_at": created, "id": id, "kind": kind, "status": status, "resource_id": importID, "revision": rev, "result": json.RawMessage(result), "error_code": errCode})
	return true
}
func (s *Server) queueValidation(w http.ResponseWriter, r *http.Request, id string, rev int) {
	ctx := r.Context()
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	defer tx.Rollback(ctx)
	job := uuid.NewString()
	tag, e := tx.Exec(ctx, "UPDATE bom_import_batch SET status='VALIDATING',reviewed_revision=NULL WHERE id=$1 AND revision=$2 AND status<>'COMMITTED'", id, rev)
	if e != nil || tag.RowsAffected() != 1 {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	_, e = tx.Exec(ctx, "INSERT INTO bom_job(id,import_id,project_id,kind,revision) VALUES($1,$2,$3,'validate',$4)", job, id, s.project, rev)
	if e != nil || tx.Commit(ctx) != nil {
		fail(w, 500, "QUEUE_FAILED")
		return
	}
	respond(w, 202, map[string]any{"id": job, "kind": "validate", "status": "pending", "resource_id": id, "created_at": time.Now().UTC()})
}
func (s *Server) runJob(ctx context.Context) {
	var job, id, kind, project string
	var parameters []byte
	var rev int
	e := s.db.QueryRow(ctx, "UPDATE bom_job SET status='running',updated_at=now() WHERE id=(SELECT id FROM bom_job WHERE (status='pending' OR (status='running' AND updated_at<now()-interval '5 minutes')) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,import_id,revision,kind,parameters,project_id").Scan(&job, &id, &rev, &kind, &parameters, &project)
	if e != nil {
		return
	}
	if kind == "recognize" {
		scoped := *s
		scoped.project = project
		scoped.runRecognition(ctx, job, id, rev, parameters)
		return
	}
	var raw []byte
	var version int
	var status string
	e = s.db.QueryRow(ctx, "SELECT draft,revision,status FROM bom_import_batch WHERE id=$1", id).Scan(&raw, &version, &status)
	if e != nil || version != rev || status == "COMMITTED" {
		s.db.Exec(ctx, "UPDATE bom_job SET status='failed',error_code='REVISION_CONFLICT',updated_at=now() WHERE id=$1", job)
		return
	}
	var items []bom.Item
	json.Unmarshal(raw, &items)
	issues := s.validateDraft(ctx, id, items)
	next := "READY_TO_COMMIT"
	if hasBlockers(issues) {
		next = "REVIEW_REQUIRED"
	}
	result, _ := json.Marshal(issues)
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE bom_import_batch SET issues=$2,status=$3 WHERE id=$1 AND revision=$4 AND status<>'COMMITTED'", id, result, next, rev)
	if e != nil || tag.RowsAffected() != 1 {
		tx.Rollback(ctx)
		s.db.Exec(ctx, "UPDATE bom_job SET status='failed',error_code='REVISION_CONFLICT' WHERE id=$1", job)
		return
	}
	_, e = tx.Exec(ctx, "UPDATE bom_job SET status='succeeded',result=$2,updated_at=now() WHERE id=$1", job, result)
	if e == nil {
		tx.Commit(ctx)
	}
}
