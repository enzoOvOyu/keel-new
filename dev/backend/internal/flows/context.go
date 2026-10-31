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

// ExecutionDefinition resolves the exact engine revision, including system Quick Runs.
// It never substitutes the current Flow revision for an unavailable historical one.
type ExecutionDefinition struct {
	Namespace string `json:"namespace"`
	FlowID    string `json:"flow_id"`
	Revision  int    `json:"revision"`
	Source    string `json:"source"`
}

func (s *Service) ExecutionDefinition(ctx context.Context, p Scope, id string) (ExecutionDefinition, error) {
	var out ExecutionDefinition
	var executionID *string
	if err := s.DB.QueryRow(ctx, `SELECT namespace,engine_flow_id,flow_revision,kestra_id FROM flow_execution_binding WHERE id::text=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&out.Namespace, &out.FlowID, &out.Revision, &executionID); err != nil {
		return out, problem(404, "EXECUTION_NOT_FOUND")
	}
	if out.Revision < 1 {
		if executionID == nil {
			return out, problem(409, "EXECUTION_DEFINITION_NOT_FROZEN")
		}
		var execution map[string]any
		if err := s.Engine.JSON(ctx, "GET", "/executions/"+url.PathEscape(*executionID), &execution); err != nil {
			return out, err
		}
		if execution["namespace"] != out.Namespace || execution["flowId"] != out.FlowID {
			return out, problem(409, "EXECUTION_DEFINITION_IDENTITY_MISMATCH")
		}
		revision, err := engineRevision(map[string]any{"revision": execution["flowRevision"]})
		if err != nil {
			return out, problem(409, "EXECUTION_DEFINITION_NOT_FROZEN")
		}
		out.Revision = revision
		if _, err = s.DB.Exec(ctx, `UPDATE flow_execution_binding SET flow_revision=$2 WHERE id::text=$1 AND flow_revision=0`, id, revision); err != nil {
			return out, err
		}
	}
	var versions []map[string]any
	if err := s.Engine.JSON(ctx, "GET", "/flows/"+url.PathEscape(out.Namespace)+"/"+url.PathEscape(out.FlowID)+"/revisions", &versions); err != nil {
		return out, err
	}
	for _, version := range versions {
		revision, ok := version["revision"].(float64)
		if !ok || int(revision) != out.Revision {
			continue
		}
		out.Source, _ = version["source"].(string)
		if out.Source != "" {
			return out, nil
		}
	}
	return out, problem(404, "EXECUTION_DEFINITION_UNAVAILABLE")
}
