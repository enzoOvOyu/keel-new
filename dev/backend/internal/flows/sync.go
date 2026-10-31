package flows

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
)

// Project Kestra-triggered native executions; never invent a state or start a device task.
func (s *Service) Sync(ctx context.Context) error {
	rows, e := s.DB.Query(ctx, `SELECT id::text,project_id,namespace,flow_id,created_by::text FROM flow_binding`)
	if e != nil {
		return e
	}
	type binding struct{ id, project, ns, flow, owner string }
	bindings := map[string]binding{}
	for rows.Next() {
		var b binding
		if e = rows.Scan(&b.id, &b.project, &b.ns, &b.flow, &b.owner); e != nil {
			rows.Close()
			return e
		}
		bindings[b.ns+":"+b.flow] = b
	}
	rows.Close()
	if len(bindings) == 0 {
		var executionsExist bool
		if e = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_execution_binding)`).Scan(&executionsExist); e != nil {
			return e
		}
		if !executionsExist {
			return nil
		}
	}
	var page struct {
		Results []map[string]any `json:"results"`
	}
	if e = s.Engine.JSON(ctx, "GET", "/executions/search?size=100&sort=state.startDate:desc", &page); e != nil {
		return e
	}
	if e = s.SyncSchedules(ctx, page.Results); e != nil {
		return e
	}
	for _, engine := range page.Results {
		if state, ok := engine["state"].(map[string]any); ok {
			if current, ok := state["current"].(string); ok {
				s.DB.Exec(ctx, `UPDATE flow_execution_binding SET engine_state=$2,state_observed_at=now() WHERE kestra_id=$1`, fmt.Sprint(engine["id"]), current)
			}
		}
		b, ok := bindings[fmt.Sprint(engine["namespace"])+":"+fmt.Sprint(engine["flowId"])]
		if !ok {
			continue
		}
		eid, _ := engine["id"].(string)
		if eid == "" {
			continue
		}
		var known bool
		if e = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_execution_binding WHERE kestra_id=$1)`, eid).Scan(&known); e != nil {
			return e
		}
		if known {
			continue
		}
		var details map[string]any
		if e = s.Engine.JSON(ctx, "GET", "/executions/"+eid, &details); e != nil {
			return e
		}
		revision := 0
		if value, ok := details["flowRevision"].(float64); ok {
			revision = int(value)
		}
		// Do not race a manual creation before it has committed its engine ID.
		labels, _ := details["labels"].([]any)
		manual := false
		for _, label := range labels {
			m, _ := label.(map[string]any)
			if m["key"] == "keel_binding" {
				manual = true
				_, e = s.DB.Exec(ctx, `UPDATE flow_execution_binding SET kestra_id=$2,error_code=NULL WHERE id::text=$1 AND project_id=$3 AND namespace=$4 AND engine_flow_id=$5 AND kestra_id IS NULL`, fmt.Sprint(m["value"]), eid, b.project, b.ns, b.flow)
				if e != nil {
					return e
				}
			}
		}
		if manual {
			continue
		}
		r := RunRequest{Project: b.project, Flow: b.id, Revision: revision, Source: "kestra-trigger"}
		id := uuid.NewString()
		tx, e := s.DB.Begin(ctx)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO flow_execution_binding(id,project_id,flow_id,kestra_id,namespace,engine_flow_id,flow_revision,source_module,requester_id,idempotency_key,request_hash,request) VALUES($1,$2,$3,$4,$5,$6,$7,'kestra-trigger',$8,$9,$10,$11) ON CONFLICT DO NOTHING`, id, b.project, b.id, eid, b.ns, b.flow, revision, b.owner, "engine:"+eid, hash(eid), raw(r))
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO flow_audit(id,project_id,actor,action,resource_id,details) SELECT gen_random_uuid(),$1,'kestra-projector','execution.discovered',$2,$3 WHERE EXISTS(SELECT 1 FROM flow_execution_binding WHERE id=$2)`, b.project, id, raw(map[string]any{"kestra_id": eid}))
		}
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) SyncLoop(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			task, cancel := context.WithTimeout(ctx, 15*time.Second)
			s.Sync(task)
			cancel()
		}
	}
}
