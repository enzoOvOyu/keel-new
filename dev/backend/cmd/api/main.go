package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"keel.local/dev/internal/bom"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Server struct {
	db                     *pgxpool.Pool
	objects                *minio.Client
	bucket, token, project string
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
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(auth), []byte(s.token)) != 1 {
		fail(w, 401, "UNAUTHENTICATED")
		return
	}
	if r.Method != "GET" && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != env("DEV_ORIGIN", "http://localhost:5174") {
		fail(w, 403, "ORIGIN_DENIED")
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
			rows, e := s.db.Query(ctx, "SELECT id,status,filename,revision,created_at FROM bom_import_batch WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100", s.project)
			if e != nil {
				fail(w, 500, "DATABASE_ERROR")
				return
			}
			defer rows.Close()
			out := []any{}
			for rows.Next() {
				var id, status, name string
				var rev int
				var at time.Time
				rows.Scan(&id, &status, &name, &rev, &at)
				out = append(out, map[string]any{"id": id, "status": status, "filename": name, "revision": rev, "created_at": at})
			}
			respond(w, 200, out)
			return
		}
	}
	if len(p) >= 4 && p[2] == "bom-imports" {
		id := p[3]
		var project, status string
		var rev int
		var workbook, draft, issues []byte
		e := s.db.QueryRow(ctx, "SELECT project_id,status,revision,workbook,draft,issues FROM bom_import_batch WHERE id::text=$1", id).Scan(&project, &status, &rev, &workbook, &draft, &issues)
		if e != nil || project != s.project {
			fail(w, 404, "NOT_FOUND")
			return
		}
		if len(p) == 4 && r.Method == "GET" {
			respond(w, 200, map[string]any{"id": id, "status": status, "revision": rev, "workbook": json.RawMessage(workbook), "draft": json.RawMessage(draft), "issues": json.RawMessage(issues)})
			return
		}
		if len(p) == 5 && s.action(w, r, id, p[4], rev, status, workbook, draft) {
			return
		}
		if len(p) == 5 && p[4] == "sheets" && r.Method == "GET" {
			respond(w, 200, json.RawMessage(workbook))
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
	if !strings.HasSuffix(strings.ToLower(h.Filename), ".xlsx") {
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
	id := uuid.NewString()
	doc := uuid.NewString()
	hash := sha256.Sum256(data)
	key := s.project + "/sources/" + doc + ".xlsx"
	_, e = s.objects.PutObject(r.Context(), s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"})
	if e != nil {
		fail(w, 503, "OBJECT_STORAGE_UNAVAILABLE")
		return
	}
	_, e = s.db.Exec(r.Context(), "INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status) VALUES($1,$2,$3,$4,$5,$6,'UPLOADED')", id, s.project, doc, key, h.Filename, hex.EncodeToString(hash[:]))
	if e != nil {
		fail(w, 500, "DATABASE_ERROR")
		return
	}
	respond(w, 202, map[string]any{"id": id, "status": "UPLOADED", "source_document_id": doc, "content_sha256": hex.EncodeToString(hash[:])})
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
			var id, key string
			e := s.db.QueryRow(ctx, "UPDATE bom_import_batch SET status='PARSING',updated_at=now() WHERE id=(SELECT id FROM bom_import_batch WHERE status='UPLOADED' OR (status='PARSING' AND updated_at<now()-interval '5 minutes') ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,object_key").Scan(&id, &key)
			if e == nil {
				obj, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
				var wb bom.Workbook
				if err == nil {
					var b []byte
					b, err = io.ReadAll(io.LimitReader(obj, 20<<20+1))
					obj.Close()
					if err == nil {
						wb, err = bom.Parse(b)
					}
				}
				if err != nil {
					s.db.Exec(ctx, "UPDATE bom_import_batch SET status='FAILED',error_code='PARSE_FAILED',updated_at=now() WHERE id=$1", id)
				} else {
					b, _ := json.Marshal(wb)
					draft, _ := json.Marshal(bom.Draft(wb))
					s.db.Exec(ctx, "UPDATE bom_import_batch SET status='MAPPING',workbook=$2,draft=$3,updated_at=now() WHERE id=$1", id, b, draft)
				}
			}
		}
	}
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	token := os.Getenv("KEEL_API_TOKEN")
	if len(token) < 32 {
		log.Fatal("KEEL_API_TOKEN must be configured, minimum 32 characters")
	}
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal("database configuration invalid")
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		log.Fatal("database unavailable")
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
	s := &Server{db, objects, bucket, token, env("KEEL_PROJECT_ID", "proj-b300-demo")}
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
