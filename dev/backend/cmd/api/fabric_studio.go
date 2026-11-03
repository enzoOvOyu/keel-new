package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	fab "keel.local/dev/internal/fabric"
	"keel.local/dev/internal/flows"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type fabricWorkspaceEnvelope struct {
	Revision    int64           `json:"revision"`
	State       fab.State       `json:"state"`
	Inputs      fabricInputPage `json:"inputs"`
	Tasks       []fabricTask    `json:"tasks"`
	Result      string          `json:"resultId,omitempty"`
	InputDigest string          `json:"inputDigest"`
}
type fabricTask struct {
	ID            string            `json:"id"`
	Provider      string            `json:"provider"`
	Namespace     string            `json:"namespace"`
	FlowID        string            `json:"flowId"`
	Revision      int               `json:"flowRevision"`
	Name          string            `json:"name"`
	SchemaVersion string            `json:"inputSchemaVersion"`
	Inputs        []fabricTaskInput `json:"inputSchema"`
	Outputs       []string          `json:"outputSchema"`
	Targets       []string          `json:"supportedTargetKinds"`
	Tags          []string          `json:"capabilityTags"`
	DryRun        bool              `json:"supportsDryRun"`
	Rollback      bool              `json:"supportsRollback"`
	Pause         bool              `json:"supportsPause"`
	Cancel        bool              `json:"supportsCancel"`
	Retry         string            `json:"retryPolicyRef"`
	Concurrency   string            `json:"concurrencyPolicy"`
	Available     bool              `json:"available"`
	Source        string            `json:"source"`
	Timeout       int               `json:"timeoutSeconds"`
}
type fabricTaskInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

func (s *Server) fabricStudioRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) != 6 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" || p[4] != "fabric" || (p[5] != "workspace" && p[5] != "commands") {
		return false
	}
	actor := subjectOf(r.Context())
	project := p[3]
	if !fabricAuthorizedProject(r, project) || !actor.Can("fabric:read", project, "", "") || !actor.Can("cmdb:read", project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	if p[5] == "workspace" && r.Method == "GET" {
		value, e := s.fabricWorkspace(r.Context(), r, project)
		if e != nil {
			fabricFailure(w, e)
		} else {
			respond(w, 200, value)
		}
		return true
	}
	if p[5] != "commands" || r.Method != "POST" {
		fail(w, 405, "METHOD_NOT_ALLOWED")
		return true
	}
	var cmd fab.Command
	if !engineeringDecode(w, r, &cmd) {
		return true
	}
	permission := "fabric:edit"
	switch cmd.Action {
	case "match", "startZtp", "finishZtp", "deploy", "advance", "control", "collect":
		permission = "fabric:execute"
	case "ready":
		permission = "delivery:approve"
	case "approve":
		permission = "approval:read"
	}
	if !actor.Can(permission, project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 200 {
		fail(w, 400, "IDEMPOTENCY_KEY_REQUIRED")
		return true
	}
	value, e := s.fabricCommand(r.Context(), r, project, cmd, key)
	if e != nil {
		fabricFailure(w, e)
	} else {
		respond(w, 200, value)
	}
	return true
}
func fabricFailure(w http.ResponseWriter, e error) {
	var fe *flows.Error
	if errors.As(e, &fe) {
		fail(w, fe.Status, fe.Code)
		return
	}
	code := e.Error()
	switch code {
	case "FABRIC_REVISION_CONFLICT", "FABRIC_INPUT_CHANGED", "FABRIC_IN_FLIGHT", "FABRIC_ALREADY_EXECUTED":
		fail(w, 409, code)
	case "FORBIDDEN":
		fail(w, 403, code)
	case "FABRIC_DEPENDENCY_NOT_READY":
		fail(w, 503, code)
	default:
		if !strings.HasPrefix(code, "FABRIC_") && code != "IDEMPOTENCY_KEY_REUSED" {
			fail(w, 500, "FABRIC_STORAGE_ERROR")
			return
		}
		fail(w, 422, code)
	}
}
func studioError(code string) error { return errors.New(code) }
func (s *Server) fabricLoadState(ctx context.Context, project string) (fab.State, int64, string, error) {
	state := fab.NewState(project)
	var raw []byte
	var revision int64
	var digest string
	err := s.db.QueryRow(ctx, `SELECT revision,state,input_digest FROM fabric_workspace WHERE project_id=$1`, project).Scan(&revision, &raw, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, 0, "", nil
	}
	if err != nil {
		return state, 0, "", err
	}
	err = json.Unmarshal(raw, &state)
	return state, revision, digest, err
}
func (s *Server) fabricWorkspace(ctx context.Context, r *http.Request, project string) (fabricWorkspaceEnvelope, error) {
	var out fabricWorkspaceEnvelope
	var err error
	out.State, out.Revision, out.InputDigest, err = s.fabricLoadState(ctx, project)
	if err != nil {
		return out, err
	}
	out.Inputs, err = s.fabricPublishedInputs(ctx, project, "")
	if err != nil {
		return out, err
	}
	out.State.Actor = subjectOf(r.Context()).ID
	out.Tasks, err = s.fabricTaskCatalog(ctx, r, project)
	if err != nil {
		return out, err
	}
	out.State.Connected = s.flows != nil && s.flows.Engine != nil && s.flows.Engine.Base != ""
	// Reads project existing executions only; no receipt or health is synthesized.
	for i := range out.State.Executions {
		e := &out.State.Executions[i]
		ex, readErr := s.flows.Execution(ctx, flowScope(r, "flows:read"), e.ID)
		if readErr != nil {
			e.State = "UNKNOWN"
			e.RawState = "UNKNOWN"
			continue
		}
		fabricExecution(e, ex)
	}
	return out, nil
}
func (s *Server) fabricTaskCatalog(ctx context.Context, r *http.Request, project string) ([]fabricTask, error) {
	tasks := []fabricTask{}
	if s.flows == nil || s.flows.Engine == nil || s.flows.Engine.Base == "" {
		return tasks, nil
	}
	values, err := s.flows.Flows(ctx, flows.Scope{User: subjectOf(r.Context()).ID, Actor: subjectOf(r.Context()).Username, Projects: []string{project}})
	if err != nil {
		return tasks, nil
	}
	for _, f := range values {
		t := fabricTask{ID: f.ID, Provider: "KESTRA", Namespace: f.Namespace, FlowID: f.FlowID, Revision: f.Revision, Name: f.Name, Inputs: []fabricTaskInput{}, Outputs: []string{}, Targets: []string{}, Tags: []string{}, Source: "FLOW_CATALOG", Available: !f.Draft && !f.Disabled && len(f.Engine) > 0}
		doc, e := flows.FlowDocument(f.Source)
		if e != nil {
			t.Available = false
			tasks = append(tasks, t)
			continue
		}
		inputs, _ := doc["inputs"].([]any)
		for _, v := range inputs {
			m, _ := v.(map[string]any)
			name, _ := m["id"].(string)
			kind, _ := m["type"].(string)
			required, _ := m["required"].(bool)
			t.Inputs = append(t.Inputs, fabricTaskInput{name, kind, required})
		}
		variables, _ := doc["variables"].(map[string]any)
		meta, _ := variables["keel_fabric"].(map[string]any)
		if meta == nil {
			t.Available = false
		} else {
			t.SchemaVersion, _ = meta["schema_version"].(string)
			t.Timeout, _ = meta["timeout_seconds"].(int)
			t.Tags = fabricStrings(meta["capability_tags"])
			t.Targets = fabricStrings(meta["supported_target_kinds"])
			t.Outputs = fabricStrings(meta["output_schema"])
			t.DryRun, _ = meta["supports_dry_run"].(bool)
			t.Rollback, _ = meta["supports_rollback"].(bool)
			t.Pause, _ = meta["supports_pause"].(bool)
			t.Cancel, _ = meta["supports_cancel"].(bool)
			t.Retry, _ = meta["retry_policy_ref"].(string)
			t.Concurrency, _ = meta["concurrency_policy"].(string)
			t.Available = t.Available && t.SchemaVersion != "" && len(t.Tags) > 0 && len(t.Targets) > 0
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
func fabricStrings(v any) []string {
	out := []string{}
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
func (s *Server) fabricCommand(ctx context.Context, r *http.Request, project string, cmd fab.Command, key string) (fabricWorkspaceEnvelope, error) {
	var empty fabricWorkspaceEnvelope
	inputs, e := s.fabricPublishedInputs(ctx, project, "")
	if e != nil {
		return empty, e
	}
	if inputs.Status == "WAITING_FOR_KICKOFF" {
		return empty, studioError("FABRIC_DEPENDENCY_NOT_READY")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(ctx)
	initial := fab.NewState(project)
	if _, e = tx.Exec(ctx, `INSERT INTO fabric_workspace(project_id,state) VALUES($1,$2) ON CONFLICT DO NOTHING`, project, jsonBytes(initial)); e != nil {
		return empty, e
	}
	var state fab.State
	var raw []byte
	var revision int64
	var digest string
	if e = tx.QueryRow(ctx, `SELECT revision,state,input_digest FROM fabric_workspace WHERE project_id=$1 FOR UPDATE`, project).Scan(&revision, &raw, &digest); e != nil {
		return empty, e
	}
	if e = json.Unmarshal(raw, &state); e != nil {
		return empty, e
	}
	requestHash := fabricDigest(cmd)
	var previousHash, result string
	e = tx.QueryRow(ctx, `SELECT request_hash,result->>'id' FROM fabric_command_receipt WHERE project_id=$1 AND idempotency_key=$2`, project, key).Scan(&previousHash, &result)
	if e == nil {
		if previousHash != requestHash {
			return empty, studioError("IDEMPOTENCY_KEY_REUSED")
		}
		tx.Rollback(ctx)
		out, e := s.fabricWorkspace(ctx, r, project)
		out.Result = result
		return out, e
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return empty, e
	}
	if revision != cmd.Revision {
		return empty, studioError("FABRIC_REVISION_CONFLICT")
	}
	var input *fabricPublishedInput
	inputID := ""
	if state.Input != nil {
		inputID = *state.Input
	}
	if cmd.Action == "adopt" {
		inputID = cmd.ID
	}
	for i := range inputs.Inputs {
		if inputs.Inputs[i].ID == inputID {
			input = &inputs.Inputs[i]
			break
		}
	}
	if input == nil && cmd.Action != "publishProfile" {
		return empty, studioError("FABRIC_PUBLISHED_INPUT_REQUIRED")
	}
	if cmd.Action != "adopt" && input != nil && digest != input.Digest {
		return empty, studioError("FABRIC_INPUT_CHANGED")
	}
	state.Actor = subjectOf(r.Context()).ID
	if len(state.Profiles) == 0 {
		state.Profiles = fabricIntentProfiles()
	}
	result, e = s.fabricApplyCommand(ctx, r, tx, &state, input, &digest, cmd)
	if e != nil {
		return empty, e
	}
	state.Audit = append([]fab.Audit{{ID: uuid.NewString(), At: time.Now().UTC(), Actor: subjectOf(r.Context()).Username, Action: cmd.Action, Ref: result}}, state.Audit...)
	if _, e = tx.Exec(ctx, `UPDATE fabric_workspace SET state=$2,revision=revision+1,input_digest=$3,updated_at=now() WHERE project_id=$1`, project, jsonBytes(state), digest); e != nil {
		return empty, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO fabric_command_receipt(project_id,idempotency_key,request_hash,result) VALUES($1,$2,$3,$4)`, project, key, requestHash, jsonBytes(map[string]string{"id": result})); e != nil {
		return empty, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), project, subjectOf(r.Context()).Username, "fabric."+cmd.Action, result, jsonBytes(map[string]any{"revision": revision + 1, "input": inputID, "reason": cmd.Reason})); e != nil {
		return empty, e
	}
	if e = tx.Commit(ctx); e != nil {
		return empty, e
	}
	out, e := s.fabricWorkspace(ctx, r, project)
	out.Result = result
	return out, e
}
func fabricIntentProfiles() []fab.Profile {
	values := []fab.Profile{}
	for _, domain := range []string{"SWITCH", "HOST", "MULTIPLANE"} {
		values = append(values, fab.Profile{ID: "keel-intent-" + strings.ToLower(domain), Name: domain + " 已发布规划意图", Version: "1.0.0", Domain: domain, Roles: []string{"*"}, Policies: []string{}, Constraints: []string{}, Schema: "keel.fabric.intent/v1", Status: "PUBLISHED", Features: []string{"typed-intent"}, Description: "确定性编译已发布NetworkPlan；不冒充已验证的厂商命令适配器"})
	}
	return values
}
func fabricInFlight(state fab.State) bool {
	for _, e := range state.Executions {
		if e.State == "RUNNING" || e.State == "PAUSED" || e.State == "CREATED" {
			return true
		}
	}
	return false
}
func fabricInvalidate(state *fab.State) {
	for i := range state.Builds {
		state.Builds[i].Status = "STALE"
	}
	for i := range state.Validations {
		state.Validations[i].Status = "STALE"
	}
	for i := range state.Changes {
		state.Changes[i].Status = "STALE"
	}
}
func fabricActiveBuild(state *fab.State) *fab.Build {
	if len(state.Builds) == 0 {
		return nil
	}
	return &state.Builds[len(state.Builds)-1]
}
func fabricActiveBlueprint(state *fab.State) *fab.Blueprint {
	if len(state.Blueprints) == 0 {
		return nil
	}
	return &state.Blueprints[len(state.Blueprints)-1]
}
func (s *Server) fabricApplyCommand(ctx context.Context, r *http.Request, tx pgx.Tx, state *fab.State, input *fabricPublishedInput, digest *string, cmd fab.Command) (string, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	bp := fabricActiveBlueprint(state)
	build := fabricActiveBuild(state)
	switch cmd.Action {
	case "adopt":
		if fabricInFlight(*state) {
			return "", studioError("FABRIC_IN_FLIGHT")
		}
		if len(input.MissingIDs) > 0 {
			return "", studioError("FABRIC_CMDB_RECEIPT_MISSING")
		}
		if state.Input != nil && *state.Input == input.ID && *digest == input.Digest {
			return "", studioError("FABRIC_INPUT_ALREADY_ADOPTED")
		}
		state.Input = &input.ID
		*digest = input.Digest
		fabricInvalidate(state)
		planes := 0
		groups := map[string]bool{}
		for _, p := range input.NetworkPlans {
			planes += len(p.Planes)
			for _, d := range p.DeviceRoles {
				groups[d.Role] = true
			}
		}
		roles := []string{}
		for role := range groups {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		state.Blueprints = append(state.Blueprints, fab.Blueprint{ID: id, FabricID: "fabric-workspace:" + state.Project, Revision: len(state.Blueprints) + 1, Input: input.ID, Topology: "published-network-plan", PlaneCount: planes, Groups: roles, Profiles: []string{}, Status: "DRAFT", CreatedAt: now})
		return id, nil
	case "commitBlueprint":
		if bp == nil || bp.Status != "DRAFT" {
			return "", studioError("FABRIC_BLUEPRINT_DRAFT_REQUIRED")
		}
		bp.Status = "COMMITTED"
		return bp.ID, nil
	case "reviseBlueprint", "revertBlueprint":
		if fabricInFlight(*state) {
			return "", studioError("FABRIC_IN_FLIGHT")
		}
		if bp == nil {
			return "", studioError("FABRIC_BLUEPRINT_REQUIRED")
		}
		old := *bp
		if cmd.Action == "revertBlueprint" {
			found := false
			for i := len(state.Blueprints) - 2; i >= 0; i-- {
				if state.Blueprints[i].Status == "COMMITTED" && state.Blueprints[i].Input == *state.Input {
					old = state.Blueprints[i]
					found = true
					break
				}
			}
			if !found {
				return "", studioError("FABRIC_BLUEPRINT_COMMITTED_REVISION_REQUIRED")
			}
		}
		old.ID = id
		old.Revision = len(state.Blueprints) + 1
		old.CreatedAt = now
		old.Status = "DRAFT"
		state.Blueprints = append(state.Blueprints, old)
		fabricInvalidate(state)
		return id, nil
	case "publishProfile":
		if strings.TrimSpace(cmd.Reason) == "" {
			return "", studioError("FABRIC_REASON_REQUIRED")
		}
		for i := len(state.Profiles) - 1; i >= 0; i-- {
			if state.Profiles[i].ID == cmd.ID {
				p := state.Profiles[i]
				p.Version = "1.0." + strconv.Itoa(len(state.Profiles))
				p.Description = cmd.Reason
				state.Profiles = append(state.Profiles, p)
				return p.ID + "@" + p.Version, nil
			}
		}
		return "", studioError("FABRIC_PROFILE_NOT_FOUND")
	case "compile":
		return s.fabricCompile(state, input, cmd)
	case "validate":
		return s.fabricValidate(state, input)
	case "createChange":
		return s.fabricCreateChange(ctx, r, state, input, cmd)
	case "submit", "approve":
		return s.fabricApprovalCommand(ctx, r, tx, state, cmd)
	case "match", "startZtp", "finishZtp":
		return s.fabricOnboarding(ctx, r, state, input, cmd)
	case "deploy", "advance", "control":
		return s.fabricExecutionCommand(ctx, r, tx, state, input, cmd)
	case "collect":
		return s.fabricCollect(ctx, r, state, input)
	case "ready":
		if build == nil || build.Status != "BUILT" || len(state.Assurances) == 0 {
			return "", studioError("FABRIC_CURRENT_ASSURANCE_REQUIRED")
		}
		a := state.Assurances[len(state.Assurances)-1]
		if a.Status != "COMPLIANT" || a.Build != build.ID || a.Input != input.ID || len(a.Records) != len(build.Targets) {
			return "", studioError("FABRIC_CURRENT_ASSURANCE_REQUIRED")
		}
		for _, record := range a.Records {
			if record.State != "COMPLIANT" || record.ObservedAt == nil {
				return "", studioError("FABRIC_EVIDENCE_REQUIRED")
			}
		}
		state.Ready = append(state.Ready, fab.Ready{ID: id, Input: input.ID, Build: build.ID, Assurance: a.ID, Targets: build.Targets, CreatedAt: now, Status: "READY", Limitations: []string{"仅限记录中目标及证据窗口；不代表NetworkTest/NCCL通过"}})
		return id, nil
	default:
		return "", studioError("FABRIC_ACTION_UNSUPPORTED")
	}
}
func fabricExecution(dst *fab.Execution, ex flows.Execution) {
	state, _ := ex.Engine["state"].(map[string]any)
	dst.RawState = fmt.Sprint(state["current"])
	dst.State = dst.RawState
	if dst.State == "<nil>" {
		dst.State = "UNKNOWN"
	}
	dst.Targets = []fab.TargetRun{}
	dst.Outputs = []string{}
	dst.LogRef = "/flows/executions/" + ex.ID + "?tab=Logs"
	for _, t := range ex.Targets {
		dst.Targets = append(dst.Targets, fab.TargetRun{Device: t.ID, State: t.State, Attempt: t.Attempt, Stage: t.State, Logs: []string{}, Outputs: []string{}, Started: t.Started, Ended: t.Ended})
	}
	dst.Started = ex.Created
	dst.Ended = nil
	for _, t := range ex.Targets {
		if t.Ended != nil && (dst.Ended == nil || t.Ended.After(*dst.Ended)) {
			dst.Ended = t.Ended
		}
	}
}

func fabricAuthorizedProject(r *http.Request, project string) bool {
	for _, p := range subjectOf(r.Context()).Projects {
		if p.ID == project {
			return true
		}
	}
	return false
}
