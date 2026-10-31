package flows

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) StartNative(ctx context.Context, p Scope, r RunRequest, key string) (Execution, error) {
	var empty Execution
	if key == "" {
		return empty, problem(400, "IDEMPOTENCY_KEY_REQUIRED")
	}
	key = p.User + ":flow:" + r.Flow + ":" + r.Parent + ":" + key
	if e := requireProject(p, r.Project); e != nil {
		return empty, e
	}
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
	doc, e := FlowDocument(f.Source)
	if e != nil {
		return empty, e
	}
	inputs, _ := doc["inputs"].([]any)
	for _, v := range inputs {
		input, _ := v.(map[string]any)
		if input["type"] == "SECRET" && r.Inputs[fmt.Sprint(input["id"])] != "" {
			return empty, problem(422, "SECRET_INPUT_USE_CONTROLLED_REFERENCE")
		}
		if input["id"] == "ticket" {
			return empty, problem(422, "TARGET_PREFLIGHT_REQUIRED")
		}
	}
	digest := hash(string(raw(r)))
	id := uuid.NewString()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.Project+":"+key); e != nil {
		return empty, e
	}
	var existing, prior string
	e = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM flow_execution_binding WHERE project_id=$1 AND idempotency_key=$2`, r.Project, key).Scan(&existing, &prior)
	if e == nil {
		if prior != digest {
			return empty, problem(409, "IDEMPOTENCY_CONFLICT")
		}
		tx.Rollback(ctx)
		return s.Execution(ctx, p, existing)
	}
	if e != pgx.ErrNoRows {
		return empty, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO flow_execution_binding(id,project_id,flow_id,namespace,engine_flow_id,flow_revision,source_module,requester_id,idempotency_key,request_hash,request) VALUES($1,$2,$3,$4,$5,$6,'flows',$7,$8,$9,$10)`, id, r.Project, f.ID, f.Namespace, f.FlowID, f.Revision, p.User, key, digest, raw(r))
	if e != nil {
		return empty, e
	}
	if e = tx.Commit(ctx); e != nil {
		return empty, e
	}
	engine, e := s.Engine.Execute(ctx, f.Namespace, f.FlowID, f.Revision, r.Inputs, id)
	if e != nil {
		s.DB.Exec(ctx, `UPDATE flow_execution_binding SET error_code='KESTRA_START_UNKNOWN' WHERE id=$1`, id)
		return empty, e
	}
	eid := fmt.Sprint(engine["id"])
	_, e = s.DB.Exec(ctx, `UPDATE flow_execution_binding SET kestra_id=$2 WHERE id=$1`, id, eid)
	if e != nil {
		return empty, e
	}
	s.audit(ctx, p, r.Project, "execution.create.native", id, map[string]any{"kestra_id": eid})
	return s.Execution(ctx, p, id)
}
