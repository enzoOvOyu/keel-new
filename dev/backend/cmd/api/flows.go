package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"keel.local/dev/internal/flows"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func flowsFail(w http.ResponseWriter, e error) {
	var f *flows.Error
	if errors.As(e, &f) {
		fail(w, f.Status, f.Code)
	} else {
		fail(w, 500, "FLOWS_STORAGE_ERROR")
	}
}
func flowScope(r *http.Request, permission string) flows.Scope {
	u := subjectOf(r.Context())
	scope := flows.Scope{User: u.ID, Actor: u.Username, Projects: []string{}}
	project := r.URL.Query().Get("project_id")
	for _, p := range u.Projects {
		if (project == "" || p.ID == project) && u.Can(permission, p.ID, "", "") {
			scope.Projects = append(scope.Projects, p.ID)
		}
	}
	return scope
}
func readFlowInput(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, 256<<10))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		fail(w, 400, "FLOWS_INPUT_INVALID")
		return false
	}
	return true
}
func (s *Server) flowsInternal(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/api/v1/flows/agents/register" && r.Method == "POST" {
		var input flows.EnrollInput
		if !readFlowInput(w, r, &input) {
			return true
		}
		out, e := s.flows.Enroll(r.Context(), input)
		if e != nil {
			flowsFail(w, e)
		} else {
			respond(w, 201, out)
		}
		return true
	}
	if r.URL.Path == "/api/v1/flows/internal/results" && r.Method == "GET" {
		var id, project string
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		digest := sha256.Sum256([]byte(token))
		if e := s.db.QueryRow(r.Context(), `SELECT b.id::text,b.project_id FROM flow_dispatch_ticket t JOIN flow_execution_binding b ON b.id=t.execution_id WHERE t.hash=$1 AND t.expires_at>now()`, hex.EncodeToString(digest[:])).Scan(&id, &project); e != nil {
			fail(w, 403, "DISPATCH_TICKET_INVALID")
			return true
		}
		ex, e := s.flows.Execution(r.Context(), flows.Scope{Projects: []string{project}}, id)
		if e != nil {
			flowsFail(w, e)
		} else {
			respond(w, 200, ex.Targets)
		}
		return true
	}
	if r.URL.Path != "/api/v1/flows/internal/dispatch" {
		return false
	}
	if r.Method != "POST" {
		fail(w, 405, "METHOD_NOT_ALLOWED")
		return true
	}
	var input struct {
		Asset    string `json:"asset_id"`
		Required string `json:"required_transport"`
	}
	if !readFlowInput(w, r, &input) {
		return true
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	result, e := s.flows.Dispatch(r.Context(), token, input.Asset, input.Required)
	if e != nil {
		flowsFail(w, e)
	} else if result.State != "SUCCESS" {
		respond(w, 422, result)
	} else {
		respond(w, 200, result)
	}
	return true
}
func (s *Server) flowsRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/v1/flows" && !strings.HasPrefix(r.URL.Path, "/api/v1/flows/") {
		return false
	}
	permission := "flows:read"
	if r.Method != "GET" {
		permission = "flows:edit"
		if strings.HasSuffix(r.URL.Path, "/execute") || strings.Contains(r.URL.Path, "/executions/") || strings.HasSuffix(r.URL.Path, "/preflight") || strings.HasSuffix(r.URL.Path, "/quick-run") {
			permission = "flows:execute"
		}
		if r.URL.Path == "/api/v1/flows" {
			permission = "flows:create"
		}
		if strings.Contains(r.URL.Path, "/agents/") || strings.HasSuffix(r.URL.Path, "/ssh-trust") {
			permission = "flows:edit"
		}
	}
	scope := flowScope(r, permission)
	if len(scope.Projects) == 0 {
		fail(w, 403, "FLOWS_SCOPE_DENIED")
		return true
	}
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/flows")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	service := s.flows
	var out any
	var e error
	code := 200
	switch {
	case path == "/capabilities" && r.Method == "GET":
		out = map[string]any{"namespace": flows.Namespace(r.URL.Query().Get("project_id")), "engine": "Kestra", "version": "2.0.7", "configured": service.Engine.Base != "", "agent_gateway": service.Gateway != nil, "scheduling": "Kestra triggers", "native_plugins": "validated by configured Kestra"}
	case path == "/templates" && r.Method == "GET":
		project := r.URL.Query().Get("project_id")
		found := false
		for _, v := range scope.Projects {
			if v == project {
				found = true
			}
		}
		if !found {
			fail(w, 403, "PROJECT_REQUIRED")
			return true
		}
		out, e = service.Template(project)
	case path == "/targets" && r.Method == "GET":
		out, e = service.Targets(ctx, scope, r.URL.Query().Get("q"))
	case path == "/preflight" && r.Method == "POST":
		var input flows.RunRequest
		if !readFlowInput(w, r, &input) {
			return true
		}
		out, e = service.Preflight(ctx, scope, input)
	case path == "/quick-run" && r.Method == "POST":
		var input flows.RunRequest
		if !readFlowInput(w, r, &input) {
			return true
		}
		input.Flow = ""
		input.Source = "quick-run"
		out, e = service.Start(ctx, scope, input, r.Header.Get("Idempotency-Key"))
		code = 201
	case path == "" && r.Method == "GET":
		out, e = service.Flows(ctx, scope)
	case path == "" && r.Method == "POST":
		var input flows.SaveFlow
		if !readFlowInput(w, r, &input) {
			return true
		}
		out, e = service.Save(ctx, scope, "", input)
		code = 201
	case path == "/agents" && r.Method == "GET":
		out, e = service.Agents(ctx, scope)
	case path == "/agents/enrollment-tokens" && r.Method == "POST":
		var input struct {
			Project string `json:"project_id"`
			Asset   string `json:"asset_id"`
		}
		if !readFlowInput(w, r, &input) {
			return true
		}
		out, e = service.Token(ctx, scope, input.Project, input.Asset)
	case path == "/agents/bootstrap" && r.Method == "POST":
		var input flows.BootstrapRequest
		if !readFlowInput(w, r, &input) {
			return true
		}
		out, e = service.Bootstrap(ctx, scope, input)
	case len(parts) >= 2 && parts[0] == "agents":
		agents, err := service.Agents(ctx, scope)
		if err != nil {
			e = err
			break
		}
		var agent *flows.Agent
		for _, a := range agents {
			if a.ID == parts[1] {
				copy := a
				agent = &copy
			}
		}
		if agent == nil {
			fail(w, 404, "AGENT_NOT_FOUND")
			return true
		}
		if len(parts) == 2 && r.Method == "GET" {
			out = agent
		} else if len(parts) == 3 && r.Method == "POST" && (parts[2] == "approve" || parts[2] == "quarantine" || parts[2] == "revoke") {
			var input struct {
				Reason string `json:"reason"`
			}
			if !readFlowInput(w, r, &input) {
				return true
			}
			if len(input.Reason) < 5 {
				fail(w, 422, "REASON_REQUIRED")
				return true
			}
			state := map[string]string{"approve": "enrolled", "quarantine": "quarantined", "revoke": "revoked"}[parts[2]]
			_, e = s.db.Exec(ctx, `WITH changed AS(UPDATE flow_agent SET state=$2 WHERE id::text=$1 AND state<>'revoked' RETURNING project_id,id) INSERT INTO flow_audit(id,project_id,actor,action,resource_id,details) SELECT gen_random_uuid(),project_id,$3,$4,id::text,$5 FROM changed`, agent.ID, state, scope.Actor, "agent."+parts[2], jsonBytes(map[string]string{"reason": input.Reason}))
			out = map[string]string{"id": agent.ID, "state": state}
		} else if len(parts) == 3 && (parts[2] == "tasks" || parts[2] == "audit") && r.Method == "GET" {
			out, e = service.AgentHistory(ctx, scope, agent.ID, parts[2])
		} else {
			fail(w, 422, "AGENT_ACTION_UNAVAILABLE")
			return true
		}
	case path == "/executions" && r.Method == "GET":
		out, e = service.ExecutionPage(ctx, scope, r.URL.Query())
	case len(parts) >= 2 && parts[0] == "executions":
		ex, err := service.Execution(ctx, scope, parts[1])
		if err != nil {
			e = err
			break
		}
		if len(parts) == 2 && r.Method == "GET" {
			out = ex
			break
		}
		if len(parts) != 3 {
			fail(w, 404, "NOT_FOUND")
			return true
		}
		action := parts[2]
		if r.Method == "POST" {
			if action == "restart" {
				out, e = service.Restart(ctx, scope, ex.ID, r.Header.Get("Idempotency-Key"))
			} else if action == "retry-failed-targets" {
				out, e = service.Retry(ctx, scope, ex.ID, r.Header.Get("Idempotency-Key"))
			} else {
				out, e = service.Action(ctx, scope, ex.ID, action)
			}
			break
		}
		if action == "artifacts" && ex.KestraID != nil {
			uri := r.URL.Query().Get("path")
			if !flowArtifact(ex.Engine["outputs"], uri) {
				fail(w, 403, "ARTIFACT_NOT_IN_EXECUTION_OUTPUTS")
				return true
			}
			response, err := service.Engine.Request(ctx, "GET", "/executions/"+url.PathEscape(*ex.KestraID)+"/file?path="+url.QueryEscape(uri), "", nil)
			if err != nil {
				flowsFail(w, err)
				return true
			}
			defer response.Body.Close()
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment")
			io.Copy(w, response.Body)
			return true
		}
		if action == "targets" {
			out = ex.Targets
		} else if action == "outputs" {
			out = ex.Engine["outputs"]
			if out == nil {
				out = map[string]any{}
			}
		} else if action == "audit" {
			out, e = service.ExecutionAudit(ctx, scope, ex.ID)
		} else if (action == "logs" || action == "events") && ex.KestraID != nil {
			ep := "/logs/" + url.PathEscape(*ex.KestraID)
			if r.URL.Query().Get("follow") == "true" || action == "events" {
				if action == "events" {
					ep = "/executions/" + url.PathEscape(*ex.KestraID)
				}
				ep += "/follow"
				s.flowStream(w, r, ep, permission)
				return true
			}
			e = service.Engine.JSON(ctx, "GET", ep, &out)
		} else {
			fail(w, 404, "NOT_FOUND")
			return true
		}
	case path == "/ssh-trust":
		out, e = s.flowSSH(w, r, scope)
		if out == nil && e == nil {
			return true
		}
	case len(parts) >= 1:
		id := parts[0]
		if len(parts) == 2 && parts[1] == "namespace-file" {
			input := flows.NamespaceFile{Path: r.URL.Query().Get("path")}
			if r.Method == "PUT" && !readFlowInput(w, r, &input) {
				return true
			}
			out, e = service.File(ctx, scope, id, r.Method, input)
			if e != nil {
				flowsFail(w, e)
			} else {
				respond(w, 200, out)
			}
			return true
		}
		if len(parts) == 1 && r.Method == "GET" {
			out, e = service.Flow(ctx, scope, id)
		} else if len(parts) == 1 && r.Method == "PUT" {
			var input flows.SaveFlow
			if !readFlowInput(w, r, &input) {
				return true
			}
			out, e = service.Save(ctx, scope, id, input)
		} else if len(parts) == 2 && parts[1] == "validate" && r.Method == "POST" {
			var input struct {
				Source string `json:"source"`
			}
			if !readFlowInput(w, r, &input) {
				return true
			}
			out, e = service.Engine.Validate(ctx, input.Source)
		} else if len(parts) == 2 && parts[1] == "execute" && r.Method == "POST" {
			var input flows.RunRequest
			if !readFlowInput(w, r, &input) {
				return true
			}
			input.Flow = id
			input.Source = "flows"
			out, e = service.Start(ctx, scope, input, r.Header.Get("Idempotency-Key"))
			code = 201
		} else if len(parts) == 2 && r.Method == "GET" {
			f, err := service.Flow(ctx, scope, id)
			if err != nil {
				e = err
				break
			}
			if parts[1] == "dependencies" {
				out, e = service.Dependencies(ctx, scope, id)
			} else if parts[1] == "revisions" {
				e = service.Engine.JSON(ctx, "GET", "/flows/"+url.PathEscape(f.Namespace)+"/"+url.PathEscape(f.FlowID)+"/"+parts[1], &out)
			} else if parts[1] == "files" {
				e = service.Engine.JSON(ctx, "GET", "/namespaces/"+url.PathEscape(f.Namespace)+"/files/directory", &out)
			} else {
				fail(w, 404, "NOT_FOUND")
				return true
			}
		} else {
			fail(w, 404, "NOT_FOUND")
			return true
		}
	}
	if e != nil {
		flowsFail(w, e)
	} else {
		respond(w, code, out)
	}
	return true
}
func jsonBytes(v any) []byte { b, _ := json.Marshal(v); return b }
func (s *Server) flowStream(w http.ResponseWriter, r *http.Request, path, permission string) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	res, e := s.flows.Engine.Request(ctx, "GET", path, "", nil)
	if e != nil {
		flowsFail(w, e)
		return
	}
	defer res.Body.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	f, ok := w.(http.Flusher)
	if !ok {
		return
	}
	f.Flush()
	buf := make([]byte, 8192)
	for {
		n, e := res.Body.Read(buf)
		if n > 0 {
			subject := subjectOf(r.Context())
			var valid bool
			if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM app_user u JOIN api_session a ON a.user_id=u.id WHERE u.id::text=$1 AND u.status='active' AND u.permission_version=$2 AND a.token_hash=$3 AND a.revoked_at IS NULL AND a.expires_at>now())`, subject.ID, subject.Version, subject.SessionID).Scan(&valid); err != nil || !valid {
				return
			}
			if len(flowScope(r, permission).Projects) == 0 {
				return
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			f.Flush()
		}
		if e != nil {
			return
		}
	}
}

func flowArtifact(value any, path string) bool {
	if !strings.HasPrefix(path, "kestra://") {
		return false
	}
	switch v := value.(type) {
	case string:
		return v == path
	case []any:
		for _, x := range v {
			if flowArtifact(x, path) {
				return true
			}
		}
	case map[string]any:
		for _, x := range v {
			if flowArtifact(x, path) {
				return true
			}
		}
	}
	return false
}
