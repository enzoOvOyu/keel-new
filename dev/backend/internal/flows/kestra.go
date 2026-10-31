package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v3"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Kestra struct {
	Base, Tenant, User, Password string
	HTTP                         *http.Client
}

func NewKestra() *Kestra {
	return &Kestra{Base: strings.TrimRight(os.Getenv("KESTRA_URL"), "/"), Tenant: "main", User: os.Getenv("KESTRA_USERNAME"), Password: os.Getenv("KESTRA_PASSWORD"), HTTP: &http.Client{Timeout: 30 * time.Second}}
}
func (k *Kestra) Request(ctx context.Context, method, path, content string, body io.Reader) (*http.Response, error) {
	if k.Base == "" {
		return nil, problem(503, "KESTRA_NOT_CONFIGURED")
	}
	r, e := http.NewRequestWithContext(ctx, method, k.Base+"/api/v1/"+k.Tenant+path, body)
	if e != nil {
		return nil, e
	}
	if content != "" {
		r.Header.Set("Content-Type", content)
	}
	if k.User != "" {
		r.SetBasicAuth(k.User, k.Password)
	}
	res, e := k.HTTP.Do(r)
	if e != nil {
		return nil, problem(502, "KESTRA_UNAVAILABLE")
	}
	if res.StatusCode >= 400 {
		io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		res.Body.Close()
		status := 502
		if res.StatusCode == 404 || res.StatusCode == 409 || res.StatusCode == 422 {
			status = res.StatusCode
		}
		if res.StatusCode == 401 || res.StatusCode == 403 {
			status = 503
		}
		return nil, problem(status, fmt.Sprintf("KESTRA_HTTP_%d", res.StatusCode))
	}
	return res, nil
}
func (k *Kestra) JSON(ctx context.Context, method, path string, out any) error {
	r, e := k.Request(ctx, method, path, "", nil)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(out)
}
func (k *Kestra) Save(ctx context.Context, source string, update bool, ns, id string, drafts ...bool) (map[string]any, error) {
	method, path := "POST", "/flows"
	if update {
		method, path = "PUT", "/flows/"+url.PathEscape(ns)+"/"+url.PathEscape(id)
	}
	if len(drafts) > 0 && drafts[0] {
		path += "?draft=true"
	}
	r, e := k.Request(ctx, method, path, "application/x-yaml", strings.NewReader(source))
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	var out map[string]any
	e = json.NewDecoder(r.Body).Decode(&out)
	return out, e
}
func (k *Kestra) Validate(ctx context.Context, source string) (any, error) {
	r, e := k.Request(ctx, "POST", "/flows/validate", "application/x-yaml", strings.NewReader(source))
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	var out any
	e = json.NewDecoder(r.Body).Decode(&out)
	return out, e
}
func (k *Kestra) Execute(ctx context.Context, ns, id string, revision int, inputs map[string]string, bindingIDs ...string) (map[string]any, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, value := range inputs {
		w.WriteField(key, value)
	}
	w.Close()
	q := ""
	if revision > 0 {
		q = fmt.Sprintf("?revision=%d", revision)
	}
	if len(bindingIDs) > 0 {
		sep := "?"
		if q != "" {
			sep = "&"
		}
		q += sep + "labels=" + url.QueryEscape("keel_binding:"+bindingIDs[0])
	}
	r, e := k.Request(ctx, "POST", "/executions/"+url.PathEscape(ns)+"/"+url.PathEscape(id)+q, w.FormDataContentType(), &b)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	var out map[string]any
	e = json.NewDecoder(r.Body).Decode(&out)
	return out, e
}
func FlowDocument(source string) (map[string]any, error) {
	var d map[string]any
	if yaml.Unmarshal([]byte(source), &d) != nil || d["id"] == nil || d["namespace"] == nil {
		return nil, problem(422, "FLOW_YAML_INVALID")
	}
	tasks, ok := d["tasks"].([]any)
	if !ok || len(tasks) == 0 {
		return nil, problem(422, "FLOW_TASKS_REQUIRED")
	}
	return d, nil
}

// All target commands run through the Keel boundary. No credentials or task-side plugins are invented.
func bridgeSource(ns, id, base string, r RunRequest) string {
	request := func(id, transport string) map[string]any {
		return map[string]any{"id": id, "type": "io.kestra.plugin.core.http.Request", "uri": base + "/api/v1/flows/internal/dispatch", "method": "POST", "contentType": "application/json", "headers": map[string]any{"Authorization": "Bearer {{ inputs.ticket }}"}, "body": "{\"asset_id\":{{ fromJson(item.value).asset_id | toJson }},\"required_transport\":\"" + transport + "\"}", "timeout": fmt.Sprintf("PT%dS", r.Timeout+10)}
	}
	doc := map[string]any{"id": id, "namespace": ns, "labels": []any{map[string]any{"key": "keel_job_contract", "value": "v2"}}, "inputs": []any{map[string]any{"id": "targets", "type": "JSON"}, map[string]any{"id": "ticket", "type": "SECRET"}}, "tasks": []any{
		map[string]any{"id": "frozen_targets", "type": "io.kestra.plugin.core.debug.Return", "format": "{{ inputs.targets | toJson }}"},
		map[string]any{"id": "targets", "type": "io.kestra.plugin.core.flow.Loop", "values": "{{ inputs.targets }}", "concurrencyLimit": r.Concurrency, "transmitFailed": false, "tasks": []any{map[string]any{"id": "transport", "type": "io.kestra.plugin.core.flow.If", "condition": "{{ fromJson(item.value).transport == 'AGENT' }}", "then": []any{request("agent_task", "AGENT")}, "else": []any{request("ssh_task", "SSH")}}}},
	}, "finally": []any{map[string]any{"id": "collect_results", "type": "io.kestra.plugin.core.http.Request", "uri": base + "/api/v1/flows/internal/results", "method": "GET", "headers": map[string]any{"Authorization": "Bearer {{ inputs.ticket }}"}, "timeout": "PT30S"}}}
	// The graph consumes frozen preflight decisions; native branches cannot change the selected transport.

	b, _ := yaml.Marshal(doc)
	return string(b)
}
