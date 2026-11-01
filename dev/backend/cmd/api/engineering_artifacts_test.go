package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
)

func TestEngineeringAnalysisArtifactsResumeAndScope(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") {
		t.Skip("explicit isolated database required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	objects := map[string][]byte{}
	var lock sync.Mutex
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()
		switch r.Method {
		case "PUT":
			var body io.Reader = r.Body
			if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
				body = httputil.NewChunkedReader(body)
			}
			objects[r.URL.Path], _ = io.ReadAll(body)
			w.Header().Set("ETag", `"synthetic"`)
		case "GET", "HEAD":
			b, ok := objects[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Last-Modified", "Sun, 01 Nov 2026 00:00:00 GMT")
			w.Header().Set("ETag", `"synthetic"`)
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			w.Header().Set("Content-Type", "application/gzip")
			if r.Method == "GET" {
				_, _ = w.Write(b)
			}
		case "DELETE":
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected S3 request %s", r.Method)
		}
	}))
	defer endpoint.Close()
	client, err := minio.New(strings.TrimPrefix(endpoint.URL, "http://"), &minio.Options{Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	project := "analysis-" + uuid.NewString()
	batch, source, job, claim := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO project(id,name,code) VALUES($1,'Synthetic analysis artifact',$1)`, []any{project}},
		{`INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,import_mode) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic','MAPPING','engineering')`, []any{batch, project, uuid.NewString(), project + "/source"}},
		{`INSERT INTO eng_source(id,project_id,import_id) VALUES($1,$2,$3)`, []any{source, project, batch}},
		{`INSERT INTO eng_job(id,source_id,project_id,expected_revision,state,claim_id) VALUES($1,$2,$3,1,'running',$4)`, []any{job, source, project, claim}},
	} {
		if _, err = db.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		db.Exec(ctx, `DELETE FROM eng_source WHERE project_id=$1`, project)
		db.Exec(ctx, `DELETE FROM bom_import_batch WHERE project_id=$1`, project)
		db.Exec(ctx, `DELETE FROM project WHERE id=$1`, project)
	}()
	s := Server{db: db, project: project, objects: client, bucket: "synthetic-bucket"}
	checkpoint := ai.EngineeringCheckpoint{Proposal: engineering.EngineeringProposal{Summary: "synthetic checkpoint"}, CompletedSheets: []string{"renamed sheet"}}
	if err = s.saveAnalysisArtifact(ctx, job, claim, "checkpoint", "succeeded", checkpoint); err != nil {
		t.Fatal(err)
	}
	// A new service instance reads durable state, not the previous process memory.
	restarted := Server{db: db, project: project, objects: client, bucket: "synthetic-bucket"}
	got, err := restarted.resumeEngineeringAnalysis(ctx, source, 1)
	if err != nil || got == nil || got.Proposal.Summary != checkpoint.Proposal.Summary || len(got.CompletedSheets) != 1 {
		t.Fatalf("checkpoint not recovered: %v %+v", err, got)
	}
	restarted.project = "different-project"
	if got, err = restarted.resumeEngineeringAnalysis(ctx, source, 1); err != nil || got != nil {
		t.Fatal("cross-project artifact exposed")
	}
	restarted.project = project
	if got, err = restarted.resumeEngineeringAnalysis(ctx, source, 2); err != nil || got != nil {
		t.Fatal("stale revision reused")
	}
	if err = s.saveAnalysisArtifact(ctx, job, uuid.NewString(), "checkpoint", "succeeded", checkpoint); err == nil {
		t.Fatal("lost claim accepted")
	}
	lock.Lock()
	if len(objects) != 1 {
		t.Errorf("orphan upload retained: %d", len(objects))
	}
	for key := range objects {
		if !strings.HasPrefix(key, "/synthetic-bucket/"+project+"/engineering-analysis/"+job+"/") {
			t.Error("wrong project prefix")
		}
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		gz.Write([]byte(`{"proposal":{"summary":"altered"},"completed_sheets":[]}`))
		gz.Close()
		objects[key] = b.Bytes()
	}
	lock.Unlock()
	if _, err = s.resumeEngineeringAnalysis(ctx, source, 1); err == nil || err.Error() != "ANALYSIS_ARTIFACT_HASH_MISMATCH" {
		t.Fatalf("tampered artifact accepted: %v", err)
	}
	if _, err = db.Exec(ctx, `DELETE FROM eng_source WHERE project_id=$1`, project); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM eng_analysis_artifact WHERE job_id=$1`, job).Scan(&count); err != nil || count != 0 {
		t.Fatal("artifact index not cascaded")
	}
}
