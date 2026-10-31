package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"net/http"
	"strings"
)

// Project deletion is restricted to project administrators. Shared identities and
// catalog definitions are retained; source objects are removed by the Outbox.
func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request, id string) {
	if !subjectOf(r.Context()).Can("project:delete", id, "", "") {
		fail(w, 403, "FORBIDDEN")
		return
	}
	var q struct {
		ConfirmName string `json:"confirm_name"`
	}
	if !decodeIdentity(w, r, &q) {
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	defer tx.Rollback(ctx)
	// Serialize against import workers, submissions and project-scoped writers.
	_, err = tx.Exec(ctx, `LOCK TABLE project, bom_import_batch, bom_job, outbox_event, project_asset, cmdb_asset_scope, cmdb_entity, cmdb_relationship, flow_binding, flow_execution_binding, flow_agent IN SHARE ROW EXCLUSIVE MODE`)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	var name string
	err = tx.QueryRow(ctx, "SELECT name FROM project WHERE id=$1 FOR UPDATE", id).Scan(&name)
	if err == pgx.ErrNoRows {
		fail(w, 404, "NOT_FOUND")
		return
	}
	if err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	if q.ConfirmName != name {
		fail(w, 422, "PROJECT_NAME_MISMATCH")
		return
	}
	var busy bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bom_job WHERE project_id=$1 AND status IN ('pending','running')) OR EXISTS(SELECT 1 FROM bom_import_batch WHERE project_id=$1 AND status IN ('UPLOADED','PARSING'))`, id).Scan(&busy)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	if busy {
		fail(w, 409, "PROJECT_TASKS_ACTIVE")
		return
	}
	// External engine definitions and enrolled agents require their own retirement
	// lifecycle; this endpoint must not silently orphan them.
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_binding WHERE project_id=$1) OR EXISTS(SELECT 1 FROM flow_execution_binding WHERE project_id=$1) OR EXISTS(SELECT 1 FROM flow_agent WHERE project_id=$1)`, id).Scan(&busy)
	if err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	if busy {
		fail(w, 409, "PROJECT_FLOWS_REQUIRE_RETIREMENT")
		return
	}
	statements := projectDeletionStatements()
	for _, sql := range statements {
		args := []any{id}
		if !strings.Contains(sql, "$1") {
			args = nil
		}
		if _, err = tx.Exec(ctx, sql, args...); err != nil {
			fail(w, 409, "PROJECT_DELETE_CONFLICT")
			return
		}
	}
	event := uuid.NewString()
	payload, _ := json.Marshal(map[string]string{"prefix": id + "/"})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES($1,$2,'project.objects.delete',$3)`, event, id, payload); err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	details, _ := json.Marshal(map[string]string{"name": name, "storage_event_id": event, "shared_catalog": "retained"})
	if _, err = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'project.deleted',$2,$4)`, uuid.NewString(), id, subjectOf(ctx).Username, details); err != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	if tx.Commit(ctx) != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	respond(w, 202, map[string]string{"id": id, "status": "deleted", "storage_status": "pending", "storage_event_id": event})
}

func projectDeletionStatements() []string {
	return []string{
		`CREATE TEMP TABLE deletion_assets ON COMMIT DROP AS SELECT asset_id FROM cmdb_asset_scope WHERE project_id=$1 AND NOT EXISTS(SELECT 1 FROM cmdb_asset_scope other WHERE other.asset_id=cmdb_asset_scope.asset_id AND other.project_id<>$1)`,
		`CREATE TEMP TABLE deletion_items ON COMMIT DROP AS SELECT DISTINCT item_uid FROM project_asset WHERE project_id=$1 AND NOT EXISTS(SELECT 1 FROM project_asset other WHERE other.item_uid=project_asset.item_uid AND other.project_id<>$1)`,
		`DELETE FROM cmdb_relationship_revision WHERE relationship_id IN (SELECT id FROM cmdb_relationship WHERE project_id=$1)`,
		`DELETE FROM cmdb_relationship WHERE project_id=$1`,
		`DELETE FROM cmdb_lifecycle_event WHERE project_id=$1`,
		`DELETE FROM cmdb_field_source WHERE project_id=$1`,
		`DELETE FROM cmdb_revision WHERE project_id=$1`,
		`DELETE FROM cmdb_config_reference WHERE project_id=$1`,
		`DELETE FROM cmdb_external_reference WHERE project_id=$1`,
		`DELETE FROM cmdb_changeset WHERE project_id=$1`,
		`DELETE FROM cmdb_asset_scope WHERE project_id=$1`,
		`DELETE FROM cmdb_unique_attribute WHERE asset_id IN (SELECT asset_id FROM deletion_assets) AND NOT EXISTS(SELECT 1 FROM cmdb_relationship WHERE from_uid=asset_id OR to_uid=asset_id)`,
		`DELETE FROM cmdb_field_source WHERE asset_id IN (SELECT asset_id FROM deletion_assets) AND NOT EXISTS(SELECT 1 FROM cmdb_relationship WHERE from_uid=asset_id OR to_uid=asset_id)`,
		`DELETE FROM cmdb_revision WHERE asset_id IN (SELECT asset_id FROM deletion_assets) AND NOT EXISTS(SELECT 1 FROM cmdb_relationship WHERE from_uid=asset_id OR to_uid=asset_id)`,
		`DELETE FROM flow_enrollment WHERE project_id=$1`,
		`DELETE FROM flow_ssh_trust WHERE project_id=$1`,
		`DELETE FROM cmdb_entity e WHERE item_uid IN (SELECT asset_id FROM deletion_assets) AND NOT EXISTS(SELECT 1 FROM cmdb_relationship WHERE from_uid=e.item_uid OR to_uid=e.item_uid)`,
		`DELETE FROM cmdb_ingest_event WHERE event_id IN (SELECT id::text FROM outbox_event WHERE project_id=$1)`,
		`DELETE FROM outbox_event WHERE project_id=$1`,
		`DELETE FROM bom_submission WHERE project_id=$1`,
		`DELETE FROM project_asset WHERE project_id=$1`,
		`DELETE FROM bom_item_revision WHERE revision_id IN (SELECT id FROM bom_revision WHERE project_id=$1)`,
		// Shared assets retain their payload and source reference; the optional
		// live revision pointer cannot reference a project being removed.
		`UPDATE cmdb_entity e SET revision_id=(SELECT p.revision_id FROM project_asset p JOIN bom_revision r ON r.id=p.revision_id WHERE p.item_uid::text=e.item_uid AND p.project_id<>$1 AND r.project_id<>$1 ORDER BY r.created_at DESC LIMIT 1) WHERE revision_id IN (SELECT id FROM bom_revision WHERE project_id=$1)`,
		`UPDATE cmdb_relationship SET revision_id=NULL WHERE revision_id IN (SELECT id FROM bom_revision WHERE project_id=$1)`,
		`DELETE FROM bom_revision WHERE project_id=$1`,
		`UPDATE bom_item SET parent_uid=NULL WHERE item_uid IN (SELECT item_uid FROM deletion_items) AND parent_uid IN (SELECT item_uid FROM deletion_items)`,
		`DELETE FROM bom_item i WHERE item_uid IN (SELECT item_uid FROM deletion_items) AND NOT EXISTS(SELECT 1 FROM cmdb_entity WHERE bom_item_uid=i.item_uid) AND NOT EXISTS(SELECT 1 FROM bom_item WHERE parent_uid=i.item_uid) AND NOT EXISTS(SELECT 1 FROM bom_item_revision WHERE item_uid=i.item_uid)`,
		`DELETE FROM ai_task WHERE import_id IN (SELECT id FROM bom_import_batch WHERE project_id=$1)`,
		`DELETE FROM import_draft_revision WHERE import_id IN (SELECT id FROM bom_import_batch WHERE project_id=$1)`,
		`DELETE FROM bom_job WHERE project_id=$1`,
		`DELETE FROM idempotency_record WHERE project_id=$1`,
		`DELETE FROM bom_mapping_template WHERE project_id=$1`,
		`DELETE FROM bom_import_batch WHERE project_id=$1`,
		`DELETE FROM flow_file_revision WHERE project_id=$1`,
		`UPDATE app_user SET permission_version=permission_version+1 WHERE id IN (SELECT user_id FROM project_membership WHERE project_id=$1)`,
		`DELETE FROM project_membership WHERE project_id=$1`,
		`DELETE FROM project WHERE id=$1`,
	}
}

func (s *Server) removeProjectObjects(ctx context.Context, project string) error {
	for object := range s.objects.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: project + "/", Recursive: true}) {
		if object.Err != nil {
			return object.Err
		}
		if err := s.objects.RemoveObject(ctx, s.bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	return ctx.Err()
}
