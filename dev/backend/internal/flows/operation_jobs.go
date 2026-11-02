package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/identity"
	"strings"
	"time"
)

type OperationItem struct {
	Ordinal     int            `json:"ordinal"`
	Asset       *string        `json:"asset_id"`
	ScheduledAt *time.Time     `json:"scheduled_at"`
	State       string         `json:"state"`
	Execution   *string        `json:"execution_id"`
	Result      map[string]any `json:"result"`
	Error       *string        `json:"error_code"`
	Started     *time.Time     `json:"started_at"`
	Ended       *time.Time     `json:"ended_at"`
}
type OperationJob struct {
	ID       string          `json:"id"`
	Project  string          `json:"project_id"`
	Owner    string          `json:"owner_id"`
	Kind     string          `json:"kind"`
	State    string          `json:"state"`
	Revision int64           `json:"revision"`
	Created  time.Time       `json:"created_at"`
	Updated  time.Time       `json:"updated_at"`
	Items    []OperationItem `json:"items"`
}
type BootstrapBatch struct {
	Project string   `json:"project_id"`
	Assets  []string `json:"asset_ids"`
	Version string   `json:"version"`
}
type OperationAction struct {
	Ordinal    *int   `json:"ordinal,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"`
	Action     string `json:"action"`
	Revision   int64  `json:"expected_revision"`
}

func (s *Service) OperationJob(ctx context.Context, p Scope, id string) (OperationJob, error) {
	out := OperationJob{Items: []OperationItem{}}
	e := s.DB.QueryRow(ctx, `SELECT id::text,project_id,owner_id::text,kind,state,revision,created_at,updated_at FROM flow_operation_job WHERE id::text=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&out.ID, &out.Project, &out.Owner, &out.Kind, &out.State, &out.Revision, &out.Created, &out.Updated)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return out, problem(404, "OPERATION_JOB_NOT_FOUND")
		}
		return out, e
	}
	rows, e := s.DB.Query(ctx, `SELECT ordinal,asset_id,scheduled_at,state,execution_id::text,result,error_code,started_at,ended_at FROM flow_operation_item WHERE job_id=$1 ORDER BY ordinal`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v OperationItem
		var b []byte
		if e = rows.Scan(&v.Ordinal, &v.Asset, &v.ScheduledAt, &v.State, &v.Execution, &b, &v.Error, &v.Started, &v.Ended); e != nil {
			return out, e
		}
		if e = json.Unmarshal(b, &v.Result); e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, rows.Err()
}
func (s *Service) OperationJobs(ctx context.Context, p Scope, kind string) ([]OperationJob, error) {
	rows, e := s.DB.Query(ctx, `SELECT id::text FROM flow_operation_job WHERE project_id=ANY($1) AND kind=$2 ORDER BY created_at DESC,id DESC LIMIT 50`, p.Projects, kind)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []OperationJob{}
	for _, id := range ids {
		v, e := s.OperationJob(ctx, p, id)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Service) CreateBootstrapBatch(ctx context.Context, p Scope, in BootstrapBatch, key string) (OperationJob, error) {
	if e := requireProject(p, in.Project); e != nil {
		return OperationJob{}, e
	}
	if key == "" {
		return OperationJob{}, problem(400, "IDEMPOTENCY_KEY_REQUIRED")
	}
	if in.Version != "0.2.0" || len(in.Assets) == 0 || len(in.Assets) > 200 {
		return OperationJob{}, problem(422, "BOOTSTRAP_BATCH_INVALID")
	}
	targets, e := s.Targets(ctx, Scope{Projects: []string{in.Project}}, "")
	if e != nil {
		return OperationJob{}, e
	}
	byID := map[string]Target{}
	for _, t := range targets {
		byID[t.ID] = t
	}
	seen := map[string]bool{}
	for _, id := range in.Assets {
		t, ok := byID[id]
		if !ok || seen[id] || t.Type == "switch" || Maintained(t) || t.SSH == "" {
			return OperationJob{}, problem(422, "BOOTSTRAP_TARGET_NOT_READY")
		}
		seen[id] = true
	}
	return s.createOperation(ctx, p, in.Project, "bootstrap", in, key, in.Assets, nil)
}
func (s *Service) createOperation(ctx context.Context, p Scope, project, kind string, request any, key string, assets []string, dates []time.Time) (OperationJob, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return OperationJob{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, project+":"+p.User+":"+key); e != nil {
		return OperationJob{}, e
	}
	digest := hash(string(raw(request)))
	var id, prior string
	e = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM flow_operation_job WHERE project_id=$1 AND owner_id=$2 AND idempotency_key=$3`, project, p.User, key).Scan(&id, &prior)
	if e == nil {
		if prior != digest {
			return OperationJob{}, problem(409, "IDEMPOTENCY_CONFLICT")
		}
		tx.Rollback(ctx)
		return s.OperationJob(ctx, p, id)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return OperationJob{}, e
	}
	id = uuid.NewString()
	if _, e = tx.Exec(ctx, `INSERT INTO flow_operation_job(id,project_id,owner_id,kind,state,request,request_hash,idempotency_key) VALUES($1,$2,$3,$4,'PENDING',$5,$6,$7)`, id, project, p.User, kind, raw(request), digest, key); e != nil {
		return OperationJob{}, e
	}
	count := len(assets)
	if kind == "backfill" {
		count = len(dates)
	}
	for i := 0; i < count; i++ {
		var asset, at any
		if kind == "bootstrap" {
			asset = assets[i]
		} else {
			at = dates[i]
		}
		if _, e = tx.Exec(ctx, `INSERT INTO flow_operation_item(job_id,ordinal,asset_id,scheduled_at,state) VALUES($1,$2,$3,$4,'PENDING')`, id, i, asset, at); e != nil {
			return OperationJob{}, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return OperationJob{}, e
	}
	s.audit(ctx, p, project, "operation.create", id, map[string]any{"kind": kind, "items": count})
	return s.OperationJob(ctx, p, id)
}
func (s *Service) OperationAction(ctx context.Context, p Scope, id string, in OperationAction) (OperationJob, error) {
	job, e := s.OperationJob(ctx, p, id)
	if e != nil {
		return job, e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return job, e
	}
	defer tx.Rollback(ctx)
	var state string
	var revision int64
	if e = tx.QueryRow(ctx, `SELECT state,revision FROM flow_operation_job WHERE id=$1 FOR UPDATE`, id).Scan(&state, &revision); e != nil {
		return job, e
	}
	if revision != in.Revision {
		return job, problem(409, "OPERATION_JOB_REVISION_CONFLICT")
	}
	switch in.Action {
	case "pause":
		if !contains([]string{"PENDING", "RUNNING"}, state) {
			return job, problem(409, "OPERATION_JOB_NOT_ACTIVE")
		}
		state = "PAUSED"
	case "resume":
		if state != "PAUSED" {
			return job, problem(409, "OPERATION_JOB_NOT_PAUSED")
		}
		state = "PENDING"
	case "cancel":
		if contains([]string{"COMPLETED", "CANCELLED"}, state) {
			return job, problem(409, "OPERATION_JOB_ALREADY_FINISHED")
		}
		state = "CANCELLED"
		_, e = tx.Exec(ctx, `UPDATE flow_operation_item SET state='CANCELLED',ended_at=now() WHERE job_id=$1 AND state='PENDING'`, id)
	case "retry-blocked":
		if !contains([]string{"PARTIAL_FAILURE", "PAUSED"}, state) {
			return job, problem(409, "OPERATION_JOB_NOT_RETRYABLE")
		}
		_, e = tx.Exec(ctx, `UPDATE flow_operation_item SET state='PENDING',error_code=NULL WHERE job_id=$1 AND state='BLOCKED'`, id)
		state = "PENDING"
	case "authorize-item":
		if p.User != job.Owner {
			return job, problem(403, "OPERATION_APPROVAL_OWNER_REQUIRED")
		}
		if job.Kind != "backfill" || in.Ordinal == nil || in.ApprovalID == "" {
			return job, problem(422, "OPERATION_APPROVAL_INPUT_INVALID")
		}
		run, err := s.OperationItemRequest(ctx, p, id, *in.Ordinal)
		if err != nil {
			return job, err
		}
		run.ApprovalID = in.ApprovalID
		if err = s.CheckJobApproval(ctx, p, run); err != nil {
			return job, err
		}
		command, err := tx.Exec(ctx, `UPDATE flow_operation_item SET state='PENDING',error_code=NULL,result=jsonb_build_object('approval_id',$3::text) WHERE job_id=$1 AND ordinal=$2 AND state='BLOCKED'`, id, *in.Ordinal, in.ApprovalID)
		if err != nil {
			return job, err
		}
		if command.RowsAffected() != 1 {
			return job, problem(409, "OPERATION_ITEM_NOT_BLOCKED")
		}
		state = "PENDING"
	case "reconcile":
		if job.Kind == "backfill" {
			_, e = tx.Exec(ctx, `UPDATE flow_operation_item item SET state='SUCCEEDED',execution_id=b.id,error_code=NULL,result=jsonb_build_object('state','execution_binding_reconciled','execution_id',b.id),ended_at=now() FROM flow_execution_binding b WHERE item.job_id=$1 AND item.state='UNKNOWN' AND b.project_id=$2 AND b.kestra_id IS NOT NULL AND b.request->>'approval_context'=($3::text||item.ordinal::text)`, id, job.Project, "backfill:"+id+":")
			break
		}
		_, e = tx.Exec(ctx, `UPDATE flow_operation_item item SET state='SUCCEEDED',error_code=NULL,result='{"state":"verified_by_agent_registry"}',ended_at=now() FROM flow_agent a WHERE item.job_id=$1 AND item.state='UNKNOWN' AND a.project_id=$2 AND a.asset_id=item.asset_id AND a.state='enrolled' AND a.version=(SELECT request->>'version' FROM flow_operation_job WHERE id=$1) AND a.last_heartbeat>=item.started_at`, id, job.Project)
	default:
		return job, problem(400, "OPERATION_ACTION_INVALID")
	}
	if e != nil {
		return job, e
	}
	if _, e = tx.Exec(ctx, `UPDATE flow_operation_job SET state=$2,revision=revision+1,updated_at=now() WHERE id=$1`, id, state); e != nil {
		return job, e
	}
	if e = tx.Commit(ctx); e != nil {
		return job, e
	}
	s.audit(ctx, p, job.Project, "operation."+in.Action, id, map[string]any{"revision": revision})
	if e = s.finishOperation(ctx, id); e != nil {
		return job, e
	}
	return s.OperationJob(ctx, p, id)
}
func (s *Service) finishOperation(ctx context.Context, id string) error {
	_, e := s.DB.Exec(ctx, `UPDATE flow_operation_job j SET state=CASE WHEN EXISTS(SELECT 1 FROM flow_operation_item i WHERE i.job_id=j.id AND i.state IN('BLOCKED','UNKNOWN')) THEN 'PARTIAL_FAILURE' ELSE 'COMPLETED' END,revision=revision+1,updated_at=now() WHERE id=$1 AND state NOT IN('PAUSED','CANCELLED','COMPLETED') AND NOT EXISTS(SELECT 1 FROM flow_operation_item i WHERE i.job_id=j.id AND i.state IN('PENDING','RUNNING'))`, id)
	return e
}
func (s *Service) OperationWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e := s.runOperationItem(ctx); e != nil {
				continue
			}
		}
	}
}
func (s *Service) runOperationItem(ctx context.Context) error {
	if _, e := s.DB.Exec(ctx, `UPDATE flow_operation_item SET state='UNKNOWN',error_code='WORKER_LEASE_EXPIRED_RECONCILE_REQUIRED',ended_at=now() WHERE state='RUNNING' AND lease_until<now()`); e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id, project, owner, kind string
	var b []byte
	e = tx.QueryRow(ctx, `SELECT j.id::text,j.project_id,j.owner_id::text,j.kind,j.request FROM flow_operation_job j WHERE j.state IN('PENDING','RUNNING') AND EXISTS(SELECT 1 FROM flow_operation_item i WHERE i.job_id=j.id AND i.state='PENDING') ORDER BY j.created_at,j.id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &project, &owner, &kind, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		_, e = tx.Exec(ctx, `UPDATE flow_operation_job j SET state='PARTIAL_FAILURE',updated_at=now(),revision=revision+1 WHERE state IN('RUNNING','PENDING') AND NOT EXISTS(SELECT 1 FROM flow_operation_item i WHERE i.job_id=j.id AND i.state IN('PENDING','RUNNING')) AND EXISTS(SELECT 1 FROM flow_operation_item i WHERE i.job_id=j.id AND i.state IN('UNKNOWN','BLOCKED'))`)
		if e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	if e != nil {
		return e
	}
	var ordinal int
	var asset *string
	var at *time.Time
	var priorResult []byte
	if e = tx.QueryRow(ctx, `SELECT ordinal,asset_id,scheduled_at,result FROM flow_operation_item WHERE job_id=$1 AND state='PENDING' ORDER BY ordinal LIMIT 1 FOR UPDATE`, id).Scan(&ordinal, &asset, &at, &priorResult); e != nil {
		return e
	}
	worker := uuid.NewString()
	if _, e = tx.Exec(ctx, `UPDATE flow_operation_item SET state='RUNNING',worker_id=$3,lease_until=now()+interval '4 minutes',started_at=now() WHERE job_id=$1 AND ordinal=$2`, id, ordinal, worker); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE flow_operation_job SET state='RUNNING',revision=revision+1,updated_at=now() WHERE id=$1`, id); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	subject, loadErr := (identity.Service{DB: s.DB}).Load(ctx, owner)
	scope := Scope{User: owner, Actor: subject.Username, Projects: []string{project}}
	itemState := "SUCCEEDED"
	var code *string
	result := map[string]any{}
	var execution *string
	runCtx, cancel := context.WithTimeout(ctx, 190*time.Second)
	defer cancel()
	if loadErr != nil || !subject.Can(map[string]string{"bootstrap": "flows:edit", "backfill": "flows:execute"}[kind], project, "", "") {
		e = problem(403, "OPERATION_OWNER_AUTHORIZATION_REVOKED")
	} else if kind == "bootstrap" {
		var in BootstrapBatch
		if err := json.Unmarshal(b, &in); err != nil {
			e = err
		} else if asset == nil {
			e = problem(422, "BOOTSTRAP_TARGET_NOT_READY")
		} else {
			result, e = s.Bootstrap(runCtx, scope, BootstrapRequest{Project: project, Asset: *asset, Version: in.Version})
		}
	} else {
		var snapshot BackfillSnapshot
		if err := json.Unmarshal(b, &snapshot); err != nil {
			e = err
		} else {
			var previous map[string]any
			json.Unmarshal(priorResult, &previous)
			run := snapshot.Run
			run.ApprovalContext = operationApprovalContext(id, ordinal)
			run.internalOperation = true
			run.ApprovalID, _ = previous["approval_id"].(string)
			run.Source = "backfill"
			var created Execution
			created, e = s.Start(runCtx, scope, run, fmt.Sprintf("backfill:%s:%d", id, ordinal))
			if e == nil {
				execution = &created.ID
				result = map[string]any{"state": "execution_created", "scheduled_at": at, "execution_id": created.ID}
			}
		}
	}
	if e != nil {
		itemState = "UNKNOWN"
		var domain *Error
		if errors.As(e, &domain) && contains([]string{"BOOTSTRAP_PACKAGE_OR_TLS_NOT_CONFIGURED", "BOOTSTRAP_TARGET_NOT_READY", "BOOTSTRAP_CONFIGURATION_INVALID", "AGENT_VERSION_UNAVAILABLE", "AGENT_ACTIVE_TASKS_BLOCK_BOOTSTRAP", "OPERATION_OWNER_AUTHORIZATION_REVOKED", "PROJECT_REQUIRED", "MAINTAINED_IPMI_ONLY", "HIGH_RISK_APPROVAL_REQUIRED", "APPROVAL_REQUIRES_FROZEN_TARGET_VERSIONS", "APPROVAL_MISSING_EXPIRED_OR_SCOPE_CHANGED", "APPROVAL_SIGNER_REVOKED_OR_SELF", "APPROVAL_SIGNATURE_INCOMPLETE", "PREFLIGHT_TARGET_REVISION_CHANGED", "PREFLIGHT_BLOCKED", "FLOW_NOT_EXECUTABLE", "APPROVED_DOMAIN_PERMISSION_REQUIRED"}, domain.Code) {
			itemState = "BLOCKED"
		}
		v := e.Error()
		if !strings.HasPrefix(v, "BOOTSTRAP_") && !strings.HasPrefix(v, "OPERATION_") && !strings.HasPrefix(v, "MAINTAINED_") && !strings.HasPrefix(v, "APPROVAL_") && !strings.HasPrefix(v, "HIGH_RISK_") && !strings.HasPrefix(v, "PREFLIGHT_") && !strings.HasPrefix(v, "FLOW_") {
			v = "OPERATION_FAILED_RECONCILE_REQUIRED"
		}
		code = &v
	}
	finishCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_, e = s.DB.Exec(finishCtx, `UPDATE flow_operation_item SET state=$4,result=$5,error_code=$6,execution_id=$7,ended_at=now(),lease_until=NULL WHERE job_id=$1 AND ordinal=$2 AND worker_id=$3 AND state='RUNNING'`, id, ordinal, worker, itemState, raw(result), code, execution)
	if e != nil {
		return e
	}
	s.audit(finishCtx, scope, project, "operation.item", id, map[string]any{"ordinal": ordinal, "state": itemState, "kind": kind})
	return s.finishOperation(finishCtx, id)
}
