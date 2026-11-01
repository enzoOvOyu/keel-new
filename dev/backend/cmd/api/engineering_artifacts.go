package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/ai"
)

// Stream through a private temporary file: neither JSON serialization nor S3
// multipart upload retains an entire large analysis artifact in RAM.
func (s *Server) saveAnalysisArtifact(ctx context.Context, job, claim, kind, state string, value any) error {
	if s.objects == nil {
		return errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	f, err := os.CreateTemp("", "keel-analysis-*.json.gz")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	hash := sha256.New()
	compressed := gzip.NewWriter(io.MultiWriter(f, hash))
	if err = json.NewEncoder(compressed).Encode(value); err != nil {
		return err
	}
	if err = compressed.Close(); err != nil {
		return err
	}
	size, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	id := uuid.NewString()
	key := s.project + "/engineering-analysis/" + job + "/" + id + ".json.gz"
	if _, err = s.objects.PutObject(ctx, s.bucket, key, f, size, minio.PutObjectOptions{ContentType: "application/gzip"}); err != nil {
		return errors.New("ANALYSIS_STORAGE_UNAVAILABLE")
	}
	tag, err := s.db.Exec(ctx, `INSERT INTO eng_analysis_artifact(id,job_id,kind,state,object_key,content_sha256,byte_size) SELECT $1,$2,$3,$4,$5,$6,$7 WHERE EXISTS(SELECT 1 FROM eng_job WHERE id=$2 AND claim_id=$8 AND state='running')`, id, job, kind, state, key, hex.EncodeToString(hash.Sum(nil)), size, claim)
	if err != nil || tag.RowsAffected() != 1 {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.objects.RemoveObject(cleanup, s.bucket, key, minio.RemoveObjectOptions{})
		if err != nil {
			return err
		}
		return errors.New("ENGINEERING_JOB_CLAIM_LOST")
	}
	return nil
}

func (s *Server) resumeEngineeringAnalysis(ctx context.Context, source string, expected int) (*ai.EngineeringCheckpoint, error) {
	var key, digest string
	err := s.db.QueryRow(ctx, `SELECT a.object_key,a.content_sha256 FROM eng_analysis_artifact a JOIN eng_job j ON j.id=a.job_id WHERE j.source_id=$1 AND j.project_id=$2 AND j.expected_revision=$3 AND a.kind='checkpoint' AND a.state='succeeded' ORDER BY a.created_at DESC LIMIT 1`, source, s.project, expected).Scan(&key, &digest)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	object, err := s.objects.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	hash := sha256.New()
	compressed, err := gzip.NewReader(io.TeeReader(object, hash))
	if err != nil {
		return nil, err
	}
	defer compressed.Close()
	var checkpoint ai.EngineeringCheckpoint
	if err = json.NewDecoder(compressed).Decode(&checkpoint); err != nil {
		return nil, err
	}
	if _, err = io.Copy(io.Discard, compressed); err != nil {
		return nil, err
	}
	if _, err = io.Copy(io.Discard, io.TeeReader(object, hash)); err != nil {
		return nil, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return nil, errors.New("ANALYSIS_ARTIFACT_HASH_MISMATCH")
	}
	return &checkpoint, nil
}
