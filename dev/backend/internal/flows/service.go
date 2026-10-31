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
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

func (s *Service) Flows(ctx context.Context, p Scope, paging ...url.Values) ([]Flow, error) {
	limit, offset, e := collectionWindow(paging)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT id::text,project_id,namespace,flow_id,name,description,target_type,revision FROM flow_binding WHERE project_id=ANY($1) ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, p.Projects, limit, offset)
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
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	namespaces := map[string]bool{}
	metadata := map[string]map[string]any{}
	for _, f := range out {
		if namespaces[f.Namespace] {
			continue
		}
		namespaces[f.Namespace] = true
		var values []map[string]any
		if e = s.Engine.JSON(ctx, "GET", "/flows/"+url.PathEscape(f.Namespace), &values); e != nil {
			return nil, e
		}
		for _, value := range values {
			metadata[f.Namespace+":"+fmt.Sprint(value["id"])] = value
		}
	}
	for i := range out {
		f := &out[i]
		f.Engine = metadata[f.Namespace+":"+f.FlowID]
		if f.Engine == nil {
			f.Engine = map[string]any{}
		}
		f.Disabled, _ = f.Engine["disabled"].(bool)
		f.Draft, _ = f.Engine["draft"].(bool)
		if revision, ok := f.Engine["revision"].(float64); ok {
			f.Revision = int(revision)
		}
	}
	return out, nil
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
	f.Draft, _ = d["draft"].(bool)
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
	ns := Namespace(input.Project)
	if id != "" {
		if e := s.DB.QueryRow(ctx, `SELECT namespace FROM flow_binding WHERE id::text=$1 AND project_id=$2`, id, input.Project).Scan(&ns); e != nil {
			return empty, problem(404, "FLOW_NOT_FOUND")
		}
	}
	if !identifier.MatchString(fid) || fmt.Sprint(doc["namespace"]) != ns {
		return empty, problem(422, "FLOW_NAMESPACE_SCOPE_INVALID")
	}
	if e := ValidatePolicy(input.Source, ns); e != nil {
		return empty, e
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
	engine, e := s.Engine.Save(ctx, input.Source, id != "", ns, fid, input.Draft)
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
func (s *Service) Targets(ctx context.Context, p Scope, q string, paging ...url.Values) ([]Target, error) {
	limit, offset, e := collectionWindow(paging)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT c.item_uid,c.name,c.asset_type,sc.project_id,c.version,c.payload || jsonb_build_object('maintenance_state',CASE WHEN c.maintenance_state='maintained' OR c.payload->>'maintenance_state'='maintained' OR c.payload->'fields'->>'maintenance_state'='maintained' THEN 'maintained' ELSE c.maintenance_state END,'lifecycle_state',CASE WHEN c.lifecycle_state='maintained' OR c.payload->>'lifecycle_state'='maintained' OR c.payload->'fields'->>'lifecycle_state'='maintained' THEN 'maintained' ELSE c.lifecycle_state END,'serial',COALESCE(c.serial,c.payload->>'serial')),COALESCE(a.id::text,''),COALESCE(a.state='enrolled' AND a.last_heartbeat>now()-interval '45 seconds' AND a.certificate_expires_at>now(),false),COALESCE(a.capabilities,'[]'),COALESCE(t.id::text,'') FROM cmdb_entity c JOIN cmdb_asset_scope sc ON sc.asset_id=c.item_uid LEFT JOIN flow_agent a ON a.asset_id=c.item_uid AND a.project_id=sc.project_id LEFT JOIN flow_ssh_trust t ON t.asset_id=c.item_uid AND t.project_id=sc.project_id AND t.status='approved' WHERE sc.project_id=ANY($1) AND c.asset_type IN('server','switch','device','machine') AND (c.name ILIKE '%'||$2||'%' OR c.item_uid ILIKE '%'||$2||'%') ORDER BY c.name,c.item_uid,sc.project_id LIMIT $3 OFFSET $4`, p.Projects, q, limit, offset)
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
		allowedFields := map[string]bool{}
		for _, key := range []string{"os", "os_name", "management_ip", "mgmt_ip", "ip", "lifecycle_state", "lifecycle_status", "maintenance_state", "serial", "machine_id", "dmi_uuid", "hostname", "model", "rack_id", "role", "is_demo"} {
			allowedFields[key] = true
		}
		for key := range t.Fields {
			if !allowedFields[key] {
				delete(t.Fields, key)
			}
		}
		json.Unmarshal(caps, &t.Capabilities)
		if t.Capabilities == nil {
			t.Capabilities = []string{}
		}
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
	if e := ValidateTaskParameters(r.Kind, r.Command); e != nil {
		return nil, e
	}
	if len(r.Steps) > 0 {
		plan, _ := PlanSteps(r.Steps)
		var merged []Decision
		sshOnlyPossible := true
		for _, step := range plan {
			if !contains([]string{"command", "bash", "python"}, step.Kind) {
				sshOnlyPossible = false
			}
		}
		for _, step := range plan {
			part := r
			part.Steps = nil
			part.Kind = step.Kind
			part.Command = step.Command
			decisions, e := s.Preflight(ctx, p, part)
			if e != nil {
				return nil, e
			}
			if merged == nil {
				merged = decisions
				continue
			}
			for i, d := range decisions {
				if !d.Ready {
					merged[i] = d
				} else if merged[i].Ready && merged[i].Transport != d.Transport {
					if r.Policy == "agent_preferred" && sshOnlyPossible && d.Target.SSH != "" {
						merged[i].Transport = "SSH"
					} else {
						merged[i].Ready = false
						merged[i].Transport = "BLOCKED"
						merged[i].Reason = "PIPELINE_TRANSPORT_CAPABILITY_MISMATCH"
					}
				}
			}
		}
		for i, d := range merged {
			if d.Ready && d.Transport == "AGENT" && !contains(d.Target.Capabilities, "job.pipeline") {
				if r.Policy == "agent_preferred" && sshOnlyPossible && d.Target.SSH != "" {
					merged[i].Transport = "SSH"
				} else {
					merged[i].Ready = false
					merged[i].Transport = "BLOCKED"
					merged[i].Reason = "AGENT_PIPELINE_CAPABILITY_REQUIRED"
				}
			}
		}
		return merged, nil
	}
	targets, e := s.Targets(ctx, Scope{Projects: []string{r.Project}}, "")
	if e != nil {
		return nil, e
	}
	unresolved := map[string]bool{}
	existing, e := s.DB.Query(ctx, `SELECT DISTINCT asset_id FROM flow_execution_target WHERE asset_id=ANY($1) AND state='RECONCILING'`, r.Targets)
	if e != nil {
		return nil, e
	}
	for existing.Next() {
		var id string
		if e = existing.Scan(&id); e != nil {
			existing.Close()
			return nil, e
		}
		unresolved[id] = true
	}
	existing.Close()
	trusted := map[string]bool{}
	profiles, e := s.DB.Query(ctx, `SELECT id::text,secret_ref,known_hosts_ref FROM flow_ssh_trust WHERE project_id=$1 AND asset_id=ANY($2) AND status='approved'`, r.Project, r.Targets)
	if e != nil {
		return nil, e
	}
	for profiles.Next() {
		var id, key, known string
		if e = profiles.Scan(&id, &key, &known); e != nil {
			profiles.Close()
			return nil, e
		}
		_, keyErr := ResolveSecret(key)
		_, knownErr := ResolveSecret(known)
		trusted[id] = keyErr == nil && knownErr == nil
	}
	profiles.Close()
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
		if r.Kind == "bash" || r.Kind == "python" {
			cap = "script." + r.Kind
		} else if r.Kind != "command" {
			cap = r.Kind
		}
		if t.SSH != "" && !trusted[t.SSH] {
			t.SSH = ""
		}
		decision := Route(t, r.Policy, cap)
		if contains([]string{"file.upload", "file.download", "inventory.collect"}, r.Kind) && decision.Transport != "AGENT" {
			decision.Ready = false
			decision.Transport = "BLOCKED"
			decision.Reason = "CAPABILITY_REQUIRES_AGENT"
		}
		if t.Type == "switch" && (r.Kind != "command" || !SwitchReadOnly(r.Command)) {
			decision.Ready = false
			decision.Transport = "BLOCKED"
			decision.Reason = "SWITCH_CHANGE_REQUIRES_APPROVED_FABRIC_ADAPTER"
		}
		if unresolved[t.ID] {
			decision.Ready = false
			decision.Transport = "BLOCKED"
			decision.Reason = "TARGET_HAS_UNRESOLVED_TASK"
		}
		out = append(out, decision)
	}
	return out, nil
}
func (s *Service) Start(ctx context.Context, p Scope, r RunRequest, key string) (Execution, error) {
	if r.Flow != "" && len(r.Targets) == 0 {
		return s.StartNative(ctx, p, r, key)
	}
	var empty Execution
	if key == "" || len(key) > 200 {
		return empty, problem(400, "IDEMPOTENCY_KEY_REQUIRED")
	}
	key = p.User + ":" + r.Source + ":" + r.Flow + ":" + r.Parent + ":" + key
	if r.Retries != 0 {
		return empty, problem(422, "SIDE_EFFECT_RETRY_REQUIRES_APPROVED_IDEMPOTENCY")
	}
	decisions, e := s.Preflight(ctx, p, r)
	if e != nil {
		return empty, e
	}
	for _, d := range decisions {
		if len(r.TargetRevisions) > 0 && r.TargetRevisions[d.Asset] != d.Revision {
			return empty, problem(409, "PREFLIGHT_TARGET_REVISION_CHANGED")
		}
		if !d.Ready {
			return empty, problem(422, "PREFLIGHT_BLOCKED")
		}
	}
	// Arbitrary commands need explicit execution permission; known destructive operations remain in their approved domains.
	if e := s.CheckJobApproval(ctx, p, r); e != nil {
		return empty, e
	}
	ns := Namespace(r.Project)
	fid := "quick-run"
	revision := 0
	if r.Flow != "" {
		f, e := s.Flow(ctx, p, r.Flow)
		if e != nil {
			return empty, e
		}
		if f.Project != r.Project || f.Revision != r.Revision || f.Disabled || f.Draft {
			return empty, problem(409, "FLOW_NOT_EXECUTABLE")
		}
		if e := ValidatePolicy(f.Source, f.Namespace); e != nil {
			return empty, e
		}
		for _, decision := range decisions {
			if f.TargetType != "both" && decision.Target.Type != f.TargetType {
				return empty, problem(422, "FLOW_TARGET_TYPE_MISMATCH")
			}
		}
		ns, fid, revision = f.Namespace, f.FlowID, f.Revision
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.Project+":"+key); e != nil {
		return empty, e
	}
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
	if r.ApprovalID != "" {
		if _, e = tx.Exec(ctx, `UPDATE flow_execution_binding SET approval_id=$2 WHERE id=$1`, id, r.ApprovalID); e != nil {
			return empty, problem(409, "APPROVAL_ALREADY_CLAIMED")
		}
	}
	for _, d := range decisions {
		_, e = tx.Exec(ctx, `INSERT INTO flow_execution_target(execution_id,asset_id,snapshot,selected_transport,command_hash,ssh_profile_id) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::uuid)`, id, d.Asset, raw(d.Target), d.Transport, hash(r.Command), d.Target.SSH)
		if e != nil {
			return empty, e
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES(gen_random_uuid(),$1,'flows.execution',$2)`, r.Project, raw(ProjectionEvent{Execution: id, Targets: r.Targets, Actor: p.Actor, State: "REQUESTED"}))
	if e != nil {
		return empty, e
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
	targetsInput := any(r.Targets)
	v2 := r.Flow == ""
	if r.Flow != "" {
		f, err := s.Flow(ctx, p, r.Flow)
		if err != nil {
			return empty, err
		}
		doc, _ := FlowDocument(f.Source)
		labels, _ := doc["labels"].([]any)
		for _, value := range labels {
			label, _ := value.(map[string]any)
			if label["key"] == "keel_job_contract" && label["value"] == "v2" {
				v2 = true
			}
		}
	}
	if v2 {
		values := []map[string]string{}
		for _, decision := range decisions {
			values = append(values, map[string]string{"asset_id": decision.Asset, "transport": decision.Transport})
		}
		targetsInput = values
	}
	inputs := map[string]string{"targets": string(raw(targetsInput)), "ticket": ticket}
	if r.Flow == "" {
		base := strings.TrimRight(os.Getenv("KEEL_WORKER_CALLBACK_URL"), "/")
		if base == "" {
			s.DB.Exec(ctx, `UPDATE flow_execution_binding SET error_code='CALLBACK_NOT_CONFIGURED' WHERE id=$1`, id)
			return empty, problem(503, "CALLBACK_NOT_CONFIGURED")
		}
		fid = "quick-" + strings.ReplaceAll(id, "-", "")
		var saved map[string]any
		saved, e = s.Engine.Save(ctx, bridgeSource(ns, fid, base, r), false, ns, fid)
		if e != nil {
			s.DB.Exec(ctx, `UPDATE flow_execution_binding SET error_code='KESTRA_FLOW_CREATE_FAILED' WHERE id=$1`, id)
			return empty, e
		}
		revision, e = engineRevision(saved)
		if e != nil {
			return empty, e
		}
		if _, e = s.DB.Exec(ctx, `UPDATE flow_execution_binding SET engine_flow_id=$2,flow_revision=$3 WHERE id=$1`, id, fid, revision); e != nil {
			return empty, e
		}
	}
	engine, e := s.Engine.Execute(ctx, ns, fid, revision, inputs, id)
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
	e := s.DB.QueryRow(ctx, `SELECT id::text,project_id,flow_id::text,kestra_id,engine_flow_id,flow_revision,source_module,mode,requester_id::text,created_at,error_code,parent_id::text FROM flow_execution_binding WHERE id::text=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&out.ID, &out.Project, &out.FlowID, &out.KestraID, &out.Flow, &out.Revision, &out.Source, &out.Mode, &out.Requester, &out.Created, &out.Error, &out.ParentID)
	if e != nil {
		return out, problem(404, "EXECUTION_NOT_FOUND")
	}
	out.Engine = map[string]any{}
	if out.KestraID != nil {
		if e = s.Engine.JSON(ctx, "GET", "/executions/"+url.PathEscape(*out.KestraID), &out.Engine); e != nil {
			return out, e
		}
	}
	if state, ok := out.Engine["state"].(map[string]any); ok {
		if current, ok := state["current"].(string); ok {
			s.DB.Exec(ctx, `UPDATE flow_execution_binding SET engine_state=$2,state_observed_at=now() WHERE id=$1`, id, current)
		}
	}
	if inputs, ok := out.Engine["inputs"].(map[string]any); ok {
		delete(inputs, "ticket")
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
	if action == "kill" {
		CancelExecution(ex.ID)
		for _, target := range ex.Targets {
			if target.Snapshot.Agent != "" {
				s.Gateway.Cancel(target.Snapshot.Agent, ex.ID+":"+target.ID)
			}
		}
		s.DB.Exec(ctx, `DELETE FROM flow_dispatch_ticket WHERE execution_id=$1`, ex.ID)
	}
	s.audit(ctx, p, ex.Project, "execution."+action, id, map[string]any{})
	return s.Execution(ctx, p, id)
}

// Restart is an explicit new job for all original targets, with fresh preflight and a new ticket.
// It never reuses a consumed dispatch ticket or rewrites the original execution history.
func (s *Service) Restart(ctx context.Context, p Scope, id, key string) (Execution, error) {
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
	if e = json.Unmarshal(b, &r); e != nil {
		return ex, e
	}
	r.Parent = id
	r.Source = "restart"
	return s.Start(ctx, p, r, key)
}
