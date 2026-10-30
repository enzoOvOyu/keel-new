package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/identity"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Server struct {
	db              *pgxpool.Pool
	objects         *minio.Client
	bucket, project string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"data": v, "request_id": uuid.NewString()})
}
func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": msg, "message": msg, "retryable": false}, "request_id": uuid.NewString()})
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		respond(w, 200, map[string]string{"status": "ok"})
		return
	}
	if s.authRoute(w, r) {
		return
	}
	authenticated, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	r = authenticated
	if r.Method != "GET" && r.Header.Get("Origin") != "" && !allowedOrigin(r, r.Header.Get("Origin")) {
		fail(w, 403, "ORIGIN_DENIED")
		return
	}
	if s.identityRoutes(w, r) {
		return
	}
	if s.aiProviderRoutes(w, r) {
		return
	}
	if s.cmdbRoutes(w, r) {
		return
	}
	project, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}
	scoped := *s
	scoped.project = project
	s = &scoped
	if r.URL.Path == "/api/v1/bom-rules" && r.Method == "GET" {
		respond(w, 200, bom.RuleCatalog())
		return
	}
	if s.jobs(w, r) {
		return
	}
	if s.templates(w, r) {
		return
	}
	if s.models(w, r) {
		return
	}
	if s.published(w, r) {
		return
	}

	ctx := r.Context()
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) == 5 && p[0] == "api" && p[1] == "v1" && p[2] == "projects" && p[4] == "bom-imports" {
		if p[3] != s.project {
			fail(w, 403, "FORBIDDEN")
			return
		}
		if r.Method == "POST" {
			s.upload(w, r)
			return
		}
		if r.Method == "GET" {
			rows, e := s.db.Query(ctx, "SELECT id,status,filename,revision,created_at,content_sha256 FROM bom_import_batch WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100", s.project)
			if e != nil {
				fail(w, 500, "DATABASE_ERROR")
				return
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, status, name, hash string
				var rev int
				var at time.Time
				rows.Scan(&id, &status, &name, &rev, &at, &hash)
				out = append(out, map[string]any{"id": id, "project_id": s.project, "content_sha256": hash, "status": strings.ToLower(status), "filename": name, "revision": rev, "created_at": at})
			}
			respond(w, 200, out)
			return
		}
	}
	if len(p) >= 4 && p[0] == "api" && p[1] == "v1" && p[2] == "bom-imports" {
		id := p[3]
		var project, status string
		var rev int
		var workbook, draft, issues []byte
		e := s.db.QueryRow(ctx, "SELECT project_id,status,revision FROM bom_import_batch WHERE id::text=$1", id).Scan(&project, &status, &rev)
		if e != nil || project != s.project {
			fail(w, 404, "NOT_FOUND")
			return
		}
		if s.workbenchAPI(w, r, id, rev, status) {
			return
		}
		if len(p) == 4 && r.Method == "GET" && r.URL.Query().Get("summary") == "true" {
			workbook, draft, issues = []byte("null"), []byte("[]"), []byte("[]")
		} else if s.db.QueryRow(ctx, "SELECT workbook,draft,issues FROM bom_import_batch WHERE id=$1", id).Scan(&workbook, &draft, &issues) != nil {
			fail(w, 500, "DATABASE_ERROR")
			return
		}
		if r.Method != "GET" {
			var mode string
			s.db.QueryRow(ctx, "SELECT import_mode FROM bom_import_batch WHERE id=$1", id).Scan(&mode)
			if mode != "legacy" {
				fail(w, 422, "USE_WORKBENCH_ACTION")
				return
			}
		}
		if s.placements(w, r, id, rev, status, workbook, draft) {
			return
		}
		if s.applyAI(w, r, id, rev, status, workbook) {
			return
		}
		if s.mappingPatch(w, r, id, rev, status, workbook) {
			return
		}
		if s.issues(w, r, id, rev) {
			return
		}
		if s.packageAPI(w, r, id, rev, draft) {
			return
		}
		if s.history(w, r, id) {
			return
		}
		if s.aiAPI(w, r, id, rev, status, workbook) {
			return
		}
		if s.profiles(w, r, id, rev, status, workbook) {
			return
		}
		if len(p) == 4 && r.Method == "GET" {
			var hash, sourceDocument, filename string
			var reviewed *int
			var errorCode *string
			var created time.Time
			s.db.QueryRow(ctx, "SELECT content_sha256,created_at,source_document_id,filename,reviewed_revision,error_code FROM bom_import_batch WHERE id=$1", id).Scan(&hash, &created, &sourceDocument, &filename, &reviewed, &errorCode)
			respond(w, 200, map[string]any{"error_code": errorCode, "reviewed_revision": reviewed, "source_document_id": sourceDocument, "filename": filename, "project_id": project, "content_sha256": hash, "created_at": created, "mapping_revision": rev, "id": id, "status": strings.ToLower(status), "internal_status": status, "revision": rev, "workbook": json.RawMessage(workbook), "draft": json.RawMessage(draft), "issues": json.RawMessage(issues)})
			return
		}
		if len(p) == 5 && s.action(w, r, id, p[4], rev, status, workbook, draft) {
			return
		}
		if len(p) == 5 && p[4] == "sheets" && r.Method == "GET" {
			var wb bom.Workbook
			json.Unmarshal(workbook, &wb)
			rows := []any{}
			for _, sh := range wb.Sheets {
				rows = append(rows, map[string]any{"sheet_name": sh.Name, "index": sh.Order, "hidden": sh.Hidden, "detected_regions": []any{map[string]any{"region_id": sh.Name, "range": sh.Dimension}}, "merged_cells": sh.Merges})
			}
			respond(w, 200, rows)
			return
		}
	}
	fail(w, 404, "NOT_FOUND")
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 21<<20)
	if r.ParseMultipartForm(1<<20) != nil {
		fail(w, 413, "FILE_TOO_LARGE")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, h, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "FILE_REQUIRED")
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(h.Filename), ".xlsx") && !strings.HasSuffix(strings.ToLower(h.Filename), ".xlsm") {
		fail(w, 400, "UNSUPPORTED_FILE_TYPE")
		return
	}
	data, e := io.ReadAll(io.LimitReader(file, 20<<20+1))
	if e != nil || len(data) > 20<<20 {
		fail(w, 413, "FILE_TOO_LARGE")
		return
	}
	if len(data) < 4 || string(data[:2]) != "PK" {
		fail(w, 400, "INVALID_WORKBOOK")
		return
	}
	mode := r.FormValue("import_mode")
	if mode == "" {
		mode = "ai"
	}
	if mode != "ai" {
		fail(w, 400, "INVALID_IMPORT_MODE")
		return
	}
	provider := r.FormValue("provider_id")
	if provider != "" {
		var available bool
		if s.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM ai_provider_config WHERE id::text=$1 AND enabled=true)", provider).Scan(&available) != nil || !available {
			fail(w, 422, "PROVIDER_NOT_AVAILABLE")
			return
		}
	}
	id := uuid.NewString()
	doc := uuid.NewString()
	hash := sha256.Sum256(data)
	declared := strings.TrimSpace(r.FormValue("declared_batch_id"))
	template := strings.TrimSpace(r.FormValue("template_revision_id"))
	if template == "" {
		template = "manual-v1"
	}
	if len(declared) > 128 {
		fail(w, 400, "INVALID_BATCH_ID")
		return
	}
	if template != "manual-v1" {
		var exists bool
		if s.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM bom_mapping_template WHERE project_id=$1 AND name||'@'||revision::text=$2)", s.project, template).Scan(&exists) != nil || !exists {
			fail(w, 422, "TEMPLATE_NOT_FOUND")
			return
		}
	}
	if declared != "" {
		var existing, oldHash, source, oldStatus string
		var created time.Time
		e = s.db.QueryRow(r.Context(), "SELECT id,content_sha256,source_document_id,status,created_at FROM bom_import_batch WHERE project_id=$1 AND declared_batch_id=$2", s.project, declared).Scan(&existing, &oldHash, &source, &oldStatus, &created)
		if e == nil {
			if oldHash != hex.EncodeToString(hash[:]) {
				fail(w, 409, "BATCH_DIGEST_CONFLICT")
				return
			}
			respond(w, 202, map[string]any{"id": existing, "project_id": s.project, "source_document_id": source, "content_sha256": oldHash, "status": strings.ToLower(oldStatus), "created_at": created})
			return
		}
	}
	key := s.project + "/sources/" + doc + ".xlsx"
	_, e = s.objects.PutObject(r.Context(), s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"})
	if e != nil {
		fail(w, 503, "OBJECT_STORAGE_UNAVAILABLE")
		return
	}
	e = s.mutate(r.Context(), id, "import.uploaded", hex.EncodeToString(hash[:]), "INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,declared_batch_id,template_revision_id,import_mode,provider_id) VALUES($1,$2,$3,$4,$5,$6,'UPLOADED',NULLIF($7,''),$8,$9,NULLIF($10,'')::uuid)", id, s.project, doc, key, h.Filename, hex.EncodeToString(hash[:]), declared, template, mode, provider)
	if e != nil {
		if declared != "" {
			var existing, oldHash, oldStatus string
			var created time.Time
			if s.db.QueryRow(r.Context(), "SELECT id,content_sha256,status,created_at FROM bom_import_batch WHERE project_id=$1 AND declared_batch_id=$2", s.project, declared).Scan(&existing, &oldHash, &oldStatus, &created) == nil {
				if oldHash != hex.EncodeToString(hash[:]) {
					fail(w, 409, "BATCH_DIGEST_CONFLICT")
					return
				}
				respond(w, 202, map[string]any{"id": existing, "project_id": s.project, "content_sha256": oldHash, "status": strings.ToLower(oldStatus), "created_at": created})
				return
			}
		}
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	respond(w, 202, map[string]any{"id": id, "status": "uploaded", "project_id": s.project, "created_at": time.Now().UTC(), "source_document_id": doc, "content_sha256": hex.EncodeToString(hash[:])})
}
func (s *Server) worker(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.publish(ctx)
			s.runJob(ctx)
			var id, key, expectedHash string
			e := s.db.QueryRow(ctx, "UPDATE bom_import_batch SET status='PARSING',updated_at=now() WHERE id=(SELECT id FROM bom_import_batch WHERE (status='UPLOADED' OR (status='PARSING' AND updated_at<now()-interval '5 minutes')) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,object_key,content_sha256").Scan(&id, &key, &expectedHash)
			if e == nil {
				obj, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
				var wb bom.Workbook
				if err == nil {
					var b []byte
					b, err = io.ReadAll(io.LimitReader(obj, 20<<20+1))
					obj.Close()
					if err == nil {
						digest := sha256.Sum256(b)
						if hex.EncodeToString(digest[:]) != expectedHash {
							err = errors.New("SOURCE_HASH_MISMATCH")
						} else {
							wb, err = bom.Parse(b)
						}
					}
				}
				if err != nil {
					code := "PARSE_FAILED"
					for _, allowed := range []string{"SOURCE_HASH_MISMATCH", "UNSAFE_ARCHIVE", "ROW_LIMIT", "COLUMN_LIMIT", "SHEET_LIMIT", "INVALID_WORKBOOK"} {
						if err.Error() == allowed {
							code = allowed
						}
					}
					s.db.Exec(ctx, "UPDATE bom_import_batch SET status='FAILED',error_code=$2,updated_at=now() WHERE id=$1", id, code)
				} else {
					b, _ := json.Marshal(wb)
					profileList := bom.Profiles(wb)
					var template, project, importMode string
					s.db.QueryRow(ctx, "SELECT template_revision_id,project_id,import_mode FROM bom_import_batch WHERE id=$1", id).Scan(&template, &project, &importMode)
					var templateRaw []byte
					if template != "manual-v1" && s.db.QueryRow(ctx, "SELECT profiles FROM bom_mapping_template WHERE project_id=$1 AND name||'@'||revision::text=$2", project, template).Scan(&templateRaw) == nil {
						var approved []bom.Profile
						json.Unmarshal(templateRaw, &approved)
						profileList = bom.ApplyTemplate(wb, profileList, approved, template)
					}
					profiles, _ := json.Marshal(profileList)
					initialDraft, initialIssues := []bom.Item{}, []bom.Issue{}
					if importMode != "ai" {
						initialDraft, initialIssues = bom.Generate(wb, profileList)
					}
					issueBytes, _ := json.Marshal(initialIssues)
					draft, _ := json.Marshal(initialDraft)
					s.db.Exec(ctx, "UPDATE bom_import_batch SET status='MAPPING',workbook=$2,draft=$3,profiles=$4,issues=$5,updated_at=now() WHERE id=$1", id, b, draft, profiles, issueBytes)
					s.initializeWorkbench(ctx, id, wb)
				}
			}
		}
	}
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal("database configuration invalid")
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		log.Fatal("database unavailable")
	}
	if e = (identity.Service{DB: db}).Initialize(ctx, os.Getenv("KEEL_BOOTSTRAP_PASSWORD")); e != nil {
		log.Fatal("identity initialization failed: ", e)
	}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		log.Fatal("CMDB schema initialization failed: ", e)
	}
	objects, e := minio.New(env("S3_ENDPOINT", "127.0.0.1:9000"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), Secure: false})
	if e != nil {
		log.Fatal("object storage configuration invalid")
	}
	bucket := env("S3_BUCKET", "keel-dev")
	exists, e := objects.BucketExists(ctx, bucket)
	if e != nil {
		log.Fatal("object storage unavailable")
	}
	if !exists {
		if e = objects.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); e != nil {
			log.Fatal("bucket initialization failed")
		}
	}
	s := &Server{db, objects, bucket, env("KEEL_PROJECT_ID", "proj-b300-demo")}
	go s.worker(ctx)
	srv := &http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8081"), Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		srv.Shutdown(shutdown)
	}()
	fmt.Println("Keel real BOM API listening", srv.Addr)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal("HTTP server stopped")
	}
}
