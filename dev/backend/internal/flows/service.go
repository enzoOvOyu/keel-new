package flows

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func (s *Service) Flows(ctx context.Context, p Scope) ([]Flow, error) {
	rows, e := s.DB.Query(ctx, `SELECT id::text,project_id,namespace,flow_id,name,description,target_type,revision FROM flow_binding WHERE project_id=ANY($1) ORDER BY created_at DESC LIMIT 200`, p.Projects)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Flow{}
	for rows.Next() {
		var f Flow
		if e = rows.Scan(&f.ID, &f.Project, &f.Namespace, &f.FlowID, &f.Name, &f.Description, &f.TargetType, &f.Revision); e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Service) Flow(ctx context.Context, p Scope, id string) (Flow, error) {
	var f Flow
	e := s.DB.QueryRow(ctx, `SELECT id::text,project_id,namespace,flow_id,name,description,target_type,revision FROM flow_binding WHERE id::text=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&f.ID, &f.Project, &f.Namespace, &f.FlowID, &f.Name, &f.Description, &f.TargetType, &f.Revision)
	if e != nil {
		return f, problem(404, "FLOW_NOT_FOUND")
	}
	var d map[string]any
	e = s.Engine.JSON(ctx, "GET", "/flows/"+url.PathEscape(f.Namespace)+"/"+url.PathEscape(f.FlowID)+"?source=true", &d)
	if e != nil {
		return f, e
	}
	f.Engine = d
	f.Source, _ = d["source"].(string)
	f.Disabled, _ = d["disabled"].(bool)
	if rev, ok := d["revision"].(float64); ok {
		f.Revision = int(rev)
	}
	return f, nil
}
func (s *Service) Save(ctx context.Context, p Scope, id string, input SaveFlow) (Flow, error) {
	var empty Flow
	if e := requireProject(p, input.Project); e != nil {
		return empty, e
	}
	doc, e := FlowDocument(input.Source)
	if e != nil {
		return empty, e
	}
	fid := fmt.Sprint(doc["id"])
	ns := "keel.p" + strings.ReplaceAll(input.Project, "-", "")
	if !identifier.MatchString(fid) || fmt.Sprint(doc["namespace"]) != ns {
		return empty, problem(422, "FLOW_NAMESPACE_SCOPE_INVALID")
	}
	if input.Name == "" || !contains([]string{"server", "switch", "both"}, input.TargetType) {
		return empty, problem(422, "FLOW_FIELDS_INVALID")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(ctx)
	if id != "" {
		var project, namespace, existing string
		var rev int
		e = tx.QueryRow(ctx, `SELECT project_id,namespace,flow_id,revision FROM flow_binding WHERE id::text=$1 FOR UPDATE`, id).Scan(&project, &namespace, &existing, &rev)
		if e != nil || project != input.Project || existing != fid || namespace != ns {
			return empty, problem(404, "FLOW_NOT_FOUND")
		}
		var current map[string]any
		if e = s.Engine.JSON(ctx, "GET", "/flows/"+ns+"/"+fid, &current); e != nil {
			return empty, e
		}
		if int(current["revision"].(float64)) != input.Revision {
			return empty, problem(409, "FLOW_REVISION_CONFLICT")
		}
	}
	engine, e := s.Engine.Save(ctx, input.Source, id != "", ns, fid)
	if e != nil {
		return empty, e
	}
	rev := int(engine["revision"].(float64))
	if id == "" {
		id = uuid.NewString()
		_, e = tx.Exec(ctx, `INSERT INTO flow_binding(id,project_id,namespace,flow_id,name,description,target_type,revision,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, input.Project, ns, fid, input.Name, input.Description, input.TargetType, rev, p.User)
	} else {
		_, e = tx.Exec(ctx, `UPDATE flow_binding SET name=$2,description=$3,target_type=$4,revision=$5 WHERE id=$1`, id, input.Name, input.Description, input.TargetType, rev)
	}
	if e != nil {
		return empty, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO flow_audit(id,project_id,actor,action,resource_id,details) VALUES(gen_random_uuid(),$1,$2,'flow.save',$3,$4)`, input.Project, p.Actor, id, raw(map[string]any{"revision": rev}))
	if e != nil {
		return empty, e
	}
	if e = tx.Commit(ctx); e != nil {
		return empty, e
	}
	return s.Flow(ctx, p, id)
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func (s *Service) Targets(ctx context.Context, p Scope, q string) ([]Target, error) {
	rows, e := s.DB.Query(ctx, `SELECT c.item_uid,c.name,c.asset_type,sc.project_id,c.version,c.payload || jsonb_build_object('maintenance_state',c.maintenance_state,'lifecycle_state',c.lifecycle_state,'serial',COALESCE(c.serial,c.payload->>'serial')),COALESCE(a.id::text,''),COALESCE(a.state='enrolled' AND a.last_heartbeat>now()-interval '45 seconds' AND a.certificate_expires_at>now(),false),COALESCE(a.capabilities,'[]'),COALESCE(t.id::text,'') FROM cmdb_entity c JOIN cmdb_asset_scope sc ON sc.asset_id=c.item_uid LEFT JOIN flow_agent a ON a.asset_id=c.item_uid AND a.project_id=sc.project_id LEFT JOIN flow_ssh_trust t ON t.asset_id=c.item_uid AND t.project_id=sc.project_id AND t.status='approved' WHERE sc.project_id=ANY($1) AND c.asset_type IN('server','switch','device','machine') AND (c.name ILIKE '%'||$2||'%' OR c.item_uid ILIKE '%'||$2||'%') ORDER BY c.name LIMIT 500`, p.Projects, q)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Target{}
	for rows.Next() {
		var t Target
		var fields, caps []byte
		if e = rows.Scan(&t.ID, &t.Name, &t.Type, &t.Project, &t.Revision, &fields, &t.Agent, &t.AgentOnline, &caps, &t.SSH); e != nil {
			return nil, e
		}
		json.Unmarshal(fields, &t.Fields)
		if nested, ok := t.Fields["fields"].(map[string]any); ok {
			for k, v := range nested {
				if _, exists := t.Fields[k]; !exists {
					t.Fields[k] = v
				}
			}
		}
		json.Unmarshal(caps, &t.Capabilities)
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Service) Preflight(ctx context.Context, p Scope, r RunRequest) ([]Decision, error) {
	if e := requireProject(p, r.Project); e != nil {
		return nil, e
	}
	if e := runValid(r); e != nil {
		return nil, e
	}
	targets, e := s.Targets(ctx, Scope{Projects: []string{r.Project}}, "")
	if e != nil {
		return nil, e
	}
	lookup := map[string]Target{}
	for _, t := range targets {
		lookup[t.ID] = t
	}
	out := []Decision{}
	seen := map[string]bool{}
	for _, id := range r.Targets {
		if seen[id] {
			return nil, problem(422, "DUPLICATE_TARGET")
		}
		seen[id] = true
		t, ok := lookup[id]
		if !ok {
			return nil, problem(404, "TARGET_NOT_AUTHORIZED")
		}
		cap := "exec.command"
		if r.Kind != "command" {
			cap = "script." + r.Kind
		}
		out = append(out, Route(t, r.Policy, cap))
	}
	return out, nil
}
func (s *Service) Start(ctx context.Context, p Scope, r RunRequest, key string) (Execution, error) {
	var empty Execution
	if key == "" || len(key) > 200 {
		return empty, problem(400, "IDEMPOTENCY_KEY_REQUIRED")
	}
	if r.Retries != 0 {
		return empty, problem(422, "SIDE_EFFECT_RETRY_REQUIRES_APPROVED_IDEMPOTENCY")
	}
	decisions, e := s.Preflight(ctx, p, r)
	if e != nil {
		return empty, e
	}
	for _, d := range decisions {
		if !d.Ready {
			return empty, problem(422, "PREFLIGHT_BLOCKED")
		}
	}
	// Arbitrary commands need explicit execution permission; known destructive operations remain in their approved domains.
	if highRisk.MatchString(r.Command) {
		return empty, problem(422, "HIGH_RISK_USE_APPROVED_DOMAIN_WORKFLOW")
	}
	ns := "keel.p" + strings.ReplaceAll(r.Project, "-", "")
	fid := "quick-run"
	revision := 0
	if r.Flow != "" {
		f, e := s.Flow(ctx, p, r.Flow)
		if e != nil {
			return empty, e
		}
		if f.Project != r.Project || f.Revision != r.Revision || f.Disabled {
			return empty, problem(409, "FLOW_NOT_EXECUTABLE")
		}
		ns, fid, revision = f.Namespace, f.FlowID, f.Revision
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(ctx)
	id := uuid.NewString()
	digest := hash(string(raw(r)))
	var existing, oldHash string
	e = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM flow_execution_binding WHERE project_id=$1 AND idempotency_key=$2`, r.Project, key).Scan(&existing, &oldHash)
	if e == nil {
		if oldHash != digest {
			return empty, problem(409, "IDEMPOTENCY_CONFLICT")
		}
		tx.Rollback(ctx)
		return s.Execution(ctx, p, existing)
	}
	if e != pgx.ErrNoRows {
		return empty, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO flow_execution_binding(id,project_id,flow_id,namespace,engine_flow_id,flow_revision,source_module,requester_id,parent_id,idempotency_key,request_hash,request) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid,$10,$11,$12)`, id, r.Project, r.Flow, ns, fid, revision, r.Source, p.User, r.Parent, key, digest, raw(r))
	if e != nil {
		return empty, e
	}
	for _, d := range decisions {
		_, e = tx.Exec(ctx, `INSERT INTO flow_execution_target(execution_id,asset_id,snapshot,selected_transport,command_hash,ssh_profile_id) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::uuid)`, id, d.Asset, raw(d.Target), d.Transport, hash(r.Command), d.Target.SSH)
		if e != nil {
			return empty, e
		}
	}
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	ticket := hex.EncodeToString(tokenBytes)
	_, e = tx.Exec(ctx, `INSERT INTO flow_dispatch_ticket(hash,execution_id,expires_at) VALUES($1,$2,now()+interval '24 hours')`, hash(ticket), id)
	if e != nil {
		return empty, e
	}
	if e = tx.Commit(ctx); e != nil {
		return empty, e
	}
	inputs := map[string]string{"targets": string(raw(r.Targets)), "ticket": ticket}
	if r.Flow == "" {
		base := strings.TrimRight(os.Getenv("KEEL_WORKER_CALLBACK_URL"), "/")
		if base == "" {
			s.DB.Exec(ctx, `UPDATE flow_execution_binding SET error_code='CALLBACK_NOT_CONFIGURED' WHERE id=$1`, id)
			return empty, problem(503, "CALLBACK_NOT_CONFIGURED")
		}
		fid = "quick-" + strings.ReplaceAll(id, "-", "")
		_, e = s.Engine.Save(ctx, bridgeSource(ns, fid, base, r), false, ns, fid)
		if e != nil {
			s.DB.Exec(ctx, `UPDATE flow_execution_binding SET error_code='KESTRA_FLOW_CREATE_FAILED' WHERE id=$1`, id)
			return empty, e
		}
		s.DB.Exec(ctx, `UPDATE flow_execution_binding SET engine_flow_id=$2 WHERE id=$1`, id, fid)
	}
	engine, e := s.Engine.Execute(ctx, ns, fid, revision, inputs)
	if e != nil {
		s.DB.Exec(ctx, `UPDATE flow_execution_binding SET error_code='KESTRA_START_UNKNOWN' WHERE id=$1`, id)
		return empty, e
	}
	eid, _ := engine["id"].(string)
	if eid == "" {
		return empty, problem(502, "KESTRA_EXECUTION_ID_MISSING")
	}
	_, e = s.DB.Exec(ctx, `UPDATE flow_execution_binding SET kestra_id=$2 WHERE id=$1`, id, eid)
	if e != nil {
		return empty, e
	}
	s.audit(ctx, p, r.Project, "execution.create", id, map[string]any{"target_count": len(r.Targets), "kestra_id": eid})
	return s.Execution(ctx, p, id)
}

var highRisk = regexp.MustCompile(`(?i)\b(reboot|shutdown|poweroff|mkfs|flashrom|nv\s+config\s+apply|rm\s+-[^\n]*r|dd\s+if=|systemctl\s+(stop|disable)\s+keel)\b`)

func (s *Service) Execution(ctx context.Context, p Scope, id string) (Execution, error) {
	var out Execution
	e := s.DB.QueryRow(ctx, `SELECT id::text,project_id,flow_id::text,kestra_id,engine_flow_id,flow_revision,source_module,mode,requester_id::text,created_at,error_code FROM flow_execution_binding WHERE id::text=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&out.ID, &out.Project, &out.FlowID, &out.KestraID, &out.Flow, &out.Revision, &out.Source, &out.Mode, &out.Requester, &out.Created, &out.Error)
	if e != nil {
		return out, problem(404, "EXECUTION_NOT_FOUND")
	}
	out.Engine = map[string]any{}
	if out.KestraID != nil {
		if e = s.Engine.JSON(ctx, "GET", "/executions/"+url.PathEscape(*out.KestraID), &out.Engine); e != nil {
			return out, e
		}
	}
	out.Targets = []TargetRun{}
	rows, e := s.DB.Query(ctx, `SELECT asset_id,snapshot,selected_transport,actual_transport,state,attempt,exit_code,stdout,stderr,started_at,ended_at FROM flow_execution_target WHERE execution_id=$1 ORDER BY asset_id`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var t TargetRun
		var b []byte
		if e = rows.Scan(&t.ID, &b, &t.Selected, &t.Actual, &t.State, &t.Attempt, &t.Exit, &t.Stdout, &t.Stderr, &t.Started, &t.Ended); e != nil {
			return out, e
		}
		json.Unmarshal(b, &t.Snapshot)
		out.Targets = append(out.Targets, t)
	}
	return out, rows.Err()
}
func (s *Service) Executions(ctx context.Context, p Scope) ([]Execution, error) {
	rows, e := s.DB.Query(ctx, `SELECT id::text FROM flow_execution_binding WHERE project_id=ANY($1) ORDER BY created_at DESC LIMIT 100`, p.Projects)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	out := []Execution{}
	for _, id := range ids {
		ex, e := s.Execution(ctx, p, id)
		if e != nil {
			return nil, e
		}
		out = append(out, ex)
	}
	return out, nil
}
func (s *Service) Retry(ctx context.Context, p Scope, id, key string) (Execution, error) {
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return ex, e
	}
	state, _ := ex.Engine["state"].(map[string]any)
	if !contains([]string{"SUCCESS", "FAILED", "WARNING", "KILLED", "CANCELLED"}, fmt.Sprint(state["current"])) {
		return ex, problem(409, "EXECUTION_NOT_TERMINAL")
	}
	var b []byte
	if e = s.DB.QueryRow(ctx, `SELECT request FROM flow_execution_binding WHERE id=$1`, id).Scan(&b); e != nil {
		return ex, e
	}
	var r RunRequest
	json.Unmarshal(b, &r)
	r.Targets = []string{}
	for _, t := range ex.Targets {
		if t.State == "FAILED" {
			r.Targets = append(r.Targets, t.ID)
		}
	}
	if len(r.Targets) == 0 {
		return ex, problem(409, "NO_FAILED_TARGETS")
	}
	r.Parent = id
	r.Source = "retry"
	return s.Start(ctx, p, r, key)
}
func (s *Service) Action(ctx context.Context, p Scope, id, action string) (Execution, error) {
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return ex, e
	}
	if ex.KestraID == nil {
		return ex, problem(409, "EXECUTION_NOT_STARTED")
	}
	method := "POST"
	if action == "kill" {
		method = "DELETE"
	}
	if !contains([]string{"pause", "resume", "kill"}, action) {
		return ex, problem(422, "EXECUTION_ACTION_UNSUPPORTED")
	}
	response, e := s.Engine.Request(ctx, method, "/executions/"+url.PathEscape(*ex.KestraID)+"/actions/"+action, "", nil)
	if e != nil {
		return ex, e
	}
	response.Body.Close()
	s.audit(ctx, p, ex.Project, "execution."+action, id, map[string]any{})
	return s.Execution(ctx, p, id)
}

var _ = time.Second
