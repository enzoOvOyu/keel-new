package flows

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/google/uuid"
	"mime/multipart"
	"net/url"
	"path"
	"strings"
	"time"
)

type FileRevision struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Revision  int64     `json:"revision"`
	SHA256    string    `json:"sha256"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}
type FileUpload struct {
	Path     string `json:"path"`
	Data     string `json:"data_base64"`
	Expected int64  `json:"expected_revision"`
}

func (s *Service) FileVersions(ctx context.Context, p Scope, flowID string) ([]FileRevision, error) {
	f, e := s.Flow(ctx, p, flowID)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT id::text,path,revision,sha256,size,created_at FROM flow_file_revision WHERE project_id=$1 AND namespace=$2 ORDER BY path,revision DESC LIMIT 500`, f.Project, f.Namespace)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []FileRevision{}
	for rows.Next() {
		var v FileRevision
		if e = rows.Scan(&v.ID, &v.Path, &v.Revision, &v.SHA256, &v.Size, &v.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) UploadFile(ctx context.Context, p Scope, flowID string, in FileUpload) (FileRevision, error) {
	f, e := s.Flow(ctx, p, flowID)
	if e != nil {
		return FileRevision{}, e
	}
	b, e := base64.StdEncoding.DecodeString(in.Data)
	if e != nil || len(b) > 1<<20 || in.Path == "" || strings.Contains(in.Path, "..") || strings.Contains(in.Path, "\\") || strings.Contains(string(b), "PRIVATE KEY-----") {
		return FileRevision{}, problem(422, "NAMESPACE_BINARY_INVALID")
	}
	name := "/" + strings.TrimPrefix(path.Clean(in.Path), "/")
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return FileRevision{}, e
	}
	defer tx.Rollback(ctx)
	tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, f.Namespace+name)
	var revision int64
	tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0) FROM flow_file_revision WHERE namespace=$1 AND path=$2`, f.Namespace, name).Scan(&revision)
	if revision != in.Expected {
		return FileRevision{}, problem(409, "FILE_REVISION_CONFLICT")
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, e := w.CreateFormFile("fileContent", path.Base(name))
	if e != nil {
		return FileRevision{}, e
	}
	part.Write(b)
	w.Close()
	res, e := s.Engine.Request(ctx, "POST", "/namespaces/"+url.PathEscape(f.Namespace)+"/files?path="+url.QueryEscape(name), w.FormDataContentType(), &body)
	if e != nil {
		return FileRevision{}, e
	}
	res.Body.Close()
	out := FileRevision{ID: uuid.NewString(), Path: name, Revision: revision + 1, SHA256: hash(string(b)), Size: int64(len(b)), CreatedAt: time.Now()}
	_, e = tx.Exec(ctx, `INSERT INTO flow_file_revision(id,project_id,namespace,path,revision,sha256,size,content,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, out.ID, f.Project, f.Namespace, name, out.Revision, out.SHA256, out.Size, b, p.User)
	if e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	s.audit(ctx, p, f.Project, "namespace.binary.upload", flowID, map[string]any{"path": name, "revision": out.Revision, "sha256": out.SHA256})
	return out, nil
}
func (s *Service) FileContent(ctx context.Context, p Scope, flowID, revisionID string) (FileRevision, []byte, error) {
	f, e := s.Flow(ctx, p, flowID)
	if e != nil {
		return FileRevision{}, nil, e
	}
	var out FileRevision
	var b []byte
	e = s.DB.QueryRow(ctx, `SELECT id::text,path,revision,sha256,size,created_at,content FROM flow_file_revision WHERE id::text=$1 AND project_id=$2 AND namespace=$3`, revisionID, f.Project, f.Namespace).Scan(&out.ID, &out.Path, &out.Revision, &out.SHA256, &out.Size, &out.CreatedAt, &b)
	if e != nil {
		return out, nil, problem(404, "FILE_REVISION_NOT_FOUND")
	}
	return out, b, nil
}
func (s *Service) DistributeFile(ctx context.Context, p Scope, flowID, revisionID string, r RunRequest, key string) (Execution, error) {
	meta, b, e := s.FileContent(ctx, p, flowID, revisionID)
	if e != nil {
		return Execution{}, e
	}
	f, e := s.Flow(ctx, p, flowID)
	if e != nil {
		return Execution{}, e
	}
	r.Flow = ""
	r.Project = f.Project
	r.Kind = "file.upload"
	r.Policy = "agent_only"
	r.Source = "namespace-file"
	r.Steps = nil
	r.Command = string(raw(map[string]string{"path": "uploads/" + path.Base(meta.Path), "data_base64": base64.StdEncoding.EncodeToString(b), "sha256": meta.SHA256}))
	return s.Start(ctx, p, r, key)
}
