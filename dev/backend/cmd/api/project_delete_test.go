package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/identity"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestProjectDeletePermission(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/api/v1/projects/example", strings.NewReader(`{"confirm_name":"example"}`))
	r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal{Subject: identity.Subject{}}))
	w := httptest.NewRecorder()
	(&Server{}).deleteProject(w, r, "example")
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}
func TestProjectDeleteRollback(t *testing.T) {
	project := os.Getenv("KEEL_DELETE_TEST_PROJECT")
	if project == "" {
		t.Skip("requires explicit existing project; all changes rolled back")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	migration, e := os.ReadFile("../../migrations/025_project_delete.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, string(migration)); e != nil {
		t.Fatal(e)
	}
	// One asset is shared with another project; deleting this project must retain it.
	var shared string
	if e = tx.QueryRow(ctx, "SELECT asset_id FROM cmdb_asset_scope WHERE project_id=$1 LIMIT 1", project).Scan(&shared); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO project(id,name,code) VALUES('synthetic-delete-neighbor','Synthetic neighbor','SYNTHETIC-DELETE')"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES('synthetic-delete-neighbor',$1,'synthetic')", shared); e != nil {
		t.Fatal(e)
	}
	var models, audits int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM device_model_revision").Scan(&models); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM audit_event").Scan(&audits); e != nil {
		t.Fatal(e)
	}
	for i, sql := range projectDeletionStatements() {
		args := []any{project}
		if !strings.Contains(sql, "$1") {
			args = nil
		}
		if _, e = tx.Exec(ctx, sql, args...); e != nil {
			t.Fatalf("statement %d: %v", i, e)
		}
	}
	var remaining int
	if e = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM project WHERE id=$1)+(SELECT count(*) FROM bom_import_batch WHERE project_id=$1)+(SELECT count(*) FROM project_asset WHERE project_id=$1)+(SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1)`, project).Scan(&remaining); e != nil || remaining != 0 {
		t.Fatalf("remaining=%d err=%v", remaining, e)
	}
	var retained int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM cmdb_entity WHERE item_uid=$1", shared).Scan(&retained); e != nil || retained != 1 {
		t.Fatalf("shared asset lost: %v", e)
	}
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM cmdb_asset_scope WHERE project_id='synthetic-delete-neighbor'").Scan(&retained); e != nil || retained != 1 {
		t.Fatalf("neighbor scope lost: %v", e)
	}
	var afterModels, afterAudits int
	tx.QueryRow(ctx, "SELECT count(*) FROM device_model_revision").Scan(&afterModels)
	tx.QueryRow(ctx, "SELECT count(*) FROM audit_event").Scan(&afterAudits)
	if models != afterModels || audits != afterAudits {
		t.Fatal("deleted shared model or audit")
	}
}

func TestProjectObjectCleanupScopeAndRetry(t *testing.T) {
	failDelete := true
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if r.URL.Query().Get("prefix") != "synthetic-project/" {
				t.Error("incorrect prefix")
			}
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(`<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>synthetic-project/sources/example.xlsx</Key><Size>1</Size></Contents></ListBucketResult>`))
			return
		}
		if r.Method != "DELETE" || r.URL.Path != "/test-bucket/synthetic-project/sources/example.xlsx" {
			t.Error("incorrect delete scope")
		}
		deletes++
		if failDelete {
			w.WriteHeader(503)
			w.Write([]byte(`<Error><Code>CleanupUnavailable</Code></Error>`))
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	client, err := minio.New(strings.TrimPrefix(server.URL, "http://"), &minio.Options{Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	s := Server{objects: client, bucket: "test-bucket"}
	if s.removeProjectObjects(context.Background(), "synthetic-project") == nil {
		t.Fatal("storage failure hidden")
	}
	failDelete = false
	if err = s.removeProjectObjects(context.Background(), "synthetic-project"); err != nil {
		t.Fatal(err)
	}
	if deletes < 2 {
		t.Fatal("retry did not delete")
	}
}
