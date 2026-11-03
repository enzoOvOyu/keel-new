package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/engineering"
)

// A replaceable read artifact, not a source of truth. Every hit is authorized by
// the route and matched to database versions in the request's read transaction.
// Keep one object per project; an older concurrent writer can only cause a miss.
type kickoffWorkspaceArtifact struct {
	Key  string          `json:"key"`
	Data json.RawMessage `json:"data"`
}

func (s *Server) kickoffWorkspaceKey(ctx context.Context, q engineeringQuery, revision int, name string) (string, error) {
	var heads, receipts, sources string
	err := q.QueryRow(ctx, `SELECT
 COALESCE((SELECT e.head_revision_id::text FROM kickoff_workspace w JOIN eng_source e ON e.id=w.source_id WHERE w.project_id=$1),''),
 COALESCE((SELECT string_agg(id::text||':'||state||':'||COALESCE(error_code,''),',' ORDER BY id) FROM kickoff_publication WHERE project_id=$1),''),
 COALESCE((SELECT string_agg(k.id::text||':'||k.state||':'||COALESCE(e.head_revision_id::text,''),',' ORDER BY k.id) FROM kickoff_source k LEFT JOIN eng_source e ON e.id=k.id WHERE k.project_id=$1 AND k.detached_at IS NULL),'')`, s.project).Scan(&heads, &receipts, &sources)
	if err != nil {
		return "", err
	}
	return engineering.Digest([]any{"workspace-read-v3", s.project, revision, name, heads, receipts, sources}), nil
}
func (s *Server) readKickoffWorkspaceArtifact(ctx context.Context, key string) json.RawMessage {
	if s.objects == nil || key == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 350*time.Millisecond)
	defer cancel()
	object, err := s.objects.GetObject(ctx, s.bucket, s.project+"/kickoff/derived/workspace-v3.json.gz", minio.GetObjectOptions{})
	if err != nil {
		return nil
	}
	defer object.Close()
	zipped, err := gzip.NewReader(object)
	if err != nil {
		return nil
	}
	defer zipped.Close()
	raw, err := io.ReadAll(zipped)
	if err != nil {
		return nil
	}
	var artifact kickoffWorkspaceArtifact
	if json.Unmarshal(raw, &artifact) != nil || artifact.Key != key || !json.Valid(artifact.Data) {
		return nil
	}
	return artifact.Data
}
func (s *Server) saveKickoffWorkspaceArtifact(ctx context.Context, key string, payload any) {
	if s.objects == nil || key == "" {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var data bytes.Buffer
	zipped, _ := gzip.NewWriterLevel(&data, gzip.BestSpeed)
	if json.NewEncoder(zipped).Encode(kickoffWorkspaceArtifact{Key: key, Data: raw}) != nil {
		return
	}
	if zipped.Close() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 350*time.Millisecond)
	defer cancel()
	_, _ = s.objects.PutObject(ctx, s.bucket, s.project+"/kickoff/derived/workspace-v3.json.gz", bytes.NewReader(data.Bytes()), int64(data.Len()), minio.PutObjectOptions{ContentType: "application/gzip"})
}

// Only the large workspace response uses compression; the JSON contract and
// errors are unchanged. Browsers transparently decode the response.
type kickoffGzipWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (w kickoffGzipWriter) Write(p []byte) (int, error) { return w.writer.Write(p) }
func respondKickoffWorkspace(w http.ResponseWriter, r *http.Request, payload any) {
	w.Header().Add("Vary", "Accept-Encoding")
	for _, encoding := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		fields := strings.Split(strings.TrimSpace(encoding), ";")
		if strings.TrimSpace(fields[0]) != "gzip" {
			continue
		}
		quality := 1.0
		for _, param := range fields[1:] {
			if k, v, ok := strings.Cut(strings.TrimSpace(param), "="); ok && k == "q" {
				quality, _ = strconv.ParseFloat(v, 64)
			}
		}
		if quality <= 0 {
			continue
		}
		w.Header().Set("Content-Encoding", "gzip")
		zipped, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
		defer zipped.Close()
		respond(kickoffGzipWriter{w, zipped}, 200, payload)
		return
	}
	respond(w, 200, payload)
}
