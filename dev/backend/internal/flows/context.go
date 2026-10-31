package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"strings"
)

type NamespaceFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (s *Service) File(ctx context.Context, p Scope, id, method string, in NamespaceFile) (any, error) {
	f, e := s.Flow(ctx, p, id)
	if e != nil {
		return nil, e
	}
	if in.Path == "" || strings.Contains(in.Path, "..") || strings.Contains(in.Path, "\\") {
		return nil, problem(422, "NAMESPACE_PATH_INVALID")
	}
	in.Path = "/" + strings.TrimPrefix(path.Clean(in.Path), "/")
	endpoint := "/namespaces/" + url.PathEscape(f.Namespace) + "/files?path=" + url.QueryEscape(in.Path)
	if method == "GET" {
		r, e := s.Engine.Request(ctx, "GET", endpoint, "", nil)
		if e != nil {
			return nil, e
		}
		defer r.Body.Close()
		b, e := io.ReadAll(io.LimitReader(r.Body, (128<<10)+1))
		if len(b) > 128<<10 {
			return nil, problem(422, "NAMESPACE_FILE_TOO_LARGE")
		}
		return NamespaceFile{Path: in.Path, Content: string(b)}, e
	}
	if strings.Contains(in.Content, "PRIVATE KEY-----") {
		return nil, problem(422, "NAMESPACE_FILE_MUST_NOT_CONTAIN_PRIVATE_KEY")
	}
	if method != "PUT" || len(in.Content) > 128<<10 {
		return nil, problem(422, "NAMESPACE_CONTENT_INVALID")
	}
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, e := writer.CreateFormFile("fileContent", path.Base(in.Path))
	if e != nil {
		return nil, e
	}
	part.Write([]byte(in.Content))
	writer.Close()
	r, e := s.Engine.Request(ctx, "POST", endpoint, writer.FormDataContentType(), &b)
	if e != nil {
		return nil, e
	}
	r.Body.Close()
	s.audit(ctx, p, f.Project, "namespace.file.save", f.ID, map[string]any{"path": in.Path, "sha256": hash(in.Content)})
	return in, nil
}
func (s *Service) Revisions(ctx context.Context, p Scope, id string) ([]map[string]any, error) {
	f, e := s.Flow(ctx, p, id)
	if e != nil {
		return nil, e
	}
	var values []map[string]any
	e = s.Engine.JSON(ctx, "GET", "/flows/"+url.PathEscape(f.Namespace)+"/"+url.PathEscape(f.FlowID)+"/revisions", &values)
	return values, e
}
func (s *Service) Template(project string) (map[string]any, error) {
	base := strings.TrimRight(os.Getenv("KEEL_WORKER_CALLBACK_URL"), "/")
	if base == "" {
		return nil, problem(503, "CALLBACK_NOT_CONFIGURED")
	}
	r := RunRequest{Concurrency: 8, Timeout: 600}
	source := bridgeSource(Namespace(project), "cmdb-target-job", base, r)
	var doc map[string]any // The canonical definition remains Kestra YAML; no invented custom plugin.
	json.Unmarshal([]byte(`{"name":"CMDB Batch Execution","kind":"controlled-target-flow"}`), &doc)
	doc["source"] = source
	return doc, nil
}
