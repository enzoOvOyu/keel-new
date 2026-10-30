package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"keel.local/dev/internal/bom"
	"net/http"
)

func (s *Server) review(w http.ResponseWriter, r *http.Request, id string, rev int, reason string, items []bom.Item) {
	ctx := r.Context()
	bindings := map[string]string{}
	for _, item := range items {
		if item.Model == "" {
			continue
		}
		var modelID string
		e := s.db.QueryRow(ctx, "SELECT CASE WHEN count(*)=1 THEN min(id::text) ELSE '' END FROM device_model_revision WHERE model=$1 AND category=$2 AND ($3='' OR id::text=$3)", item.Model, item.Category, item.Fields["model_version_ref"]).Scan(&modelID)
		if e != nil || modelID == "" {
			fail(w, 422, "MODEL_REVISION_REQUIRED")
			return
		}
		bindings[item.UID] = modelID
	}
	b, _ := json.Marshal(bindings)
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE bom_import_batch SET reviewed_revision=revision,review_model_bindings=$3 WHERE id=$1 AND revision=$2 AND status='READY_TO_COMMIT'", id, rev, b)
	if e != nil || tag.RowsAffected() != 1 {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	_, e = tx.Exec(ctx, "INSERT INTO bom_review(id,import_id,revision,actor,model_bindings,reason) VALUES($1,$2,$3,$4,$5,$6)", uuid.NewString(), id, rev, actorName(ctx), b, reason)
	details, _ := json.Marshal(map[string]any{"reason": reason, "model_bindings": bindings})
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'review.confirmed',$4,$5)", uuid.NewString(), s.project, actorName(ctx), id, details)
	}
	if e != nil || tx.Commit(ctx) != nil {
		fail(w, 500, "REVIEW_FAILED")
		return
	}
	respond(w, 200, map[string]any{"reviewed_revision": rev, "model_bindings": bindings})
}
