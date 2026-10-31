package flows

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/url"
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
	var windowStart, windowEnd time.Time
	var pageNumber int
	_, e = s.DB.Exec(ctx, `INSERT INTO flow_sync_cursor(name,window_start,window_end) SELECT 'executions',COALESCE(min(created_at),now()),now() FROM flow_binding ON CONFLICT DO NOTHING`)
	if e != nil {
		return e
	}
	if e = s.DB.QueryRow(ctx, `SELECT window_start,window_end,page FROM flow_sync_cursor WHERE name='executions'`).Scan(&windowStart, &windowEnd, &pageNumber); e != nil {
		return e
	}
	query := url.Values{"size": {"100"}, "page": {fmt.Sprint(pageNumber)}, "sort": {"state.startDate:asc"}, "filters[startDate][GREATER_THAN_OR_EQUAL_TO]": {windowStart.Format(time.RFC3339Nano)}, "filters[startDate][LESS_THAN_OR_EQUAL_TO]": {windowEnd.Format(time.RFC3339Nano)}}
	var page struct {
		Results []map[string]any `json:"results"`
	}
	if e = s.Engine.JSON(ctx, "GET", "/executions/search?"+query.Encode(), &page); e != nil {
		return e
	}
	if e = s.SyncSchedules(ctx, page.Results); e != nil {
		return e
	}
	for _, engine := range page.Results {
		if state, ok := engine["state"].(map[string]any); ok {
			if current, ok := state["current"].(string); ok {
				if _, err := s.DB.Exec(ctx, `UPDATE flow_execution_binding SET engine_state=$2,state_observed_at=now() WHERE kestra_id=$1`, fmt.Sprint(engine["id"]), current); err != nil {
					return err
				}
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
	if len(page.Results) == 100 {
		_, e = s.DB.Exec(ctx, `UPDATE flow_sync_cursor SET page=page+1 WHERE name='executions'`)
	} else {
		_, e = s.DB.Exec(ctx, `UPDATE flow_sync_cursor SET window_start=window_end,window_end=now(),page=1 WHERE name='executions'`)
	}
	if e != nil {
		return e
	}
	// Refresh active older executions separately from creation-time discovery.
	active, err := s.DB.Query(ctx, `SELECT kestra_id FROM flow_execution_binding WHERE kestra_id IS NOT NULL AND (engine_state IS NULL OR engine_state NOT IN('SUCCESS','FAILED','KILLED','CANCELLED','WARNING')) ORDER BY state_observed_at ASC NULLS FIRST LIMIT 100`)
	if err != nil {
		return err
	}
	ids := []string{}
	for active.Next() {
		var id string
		if err = active.Scan(&id); err != nil {
			active.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = active.Err()
	active.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		var state struct {
			State struct {
				Current string `json:"current"`
			} `json:"state"`
		}
		if err = s.Engine.JSON(ctx, "GET", "/executions/"+url.PathEscape(id), &state); err != nil {
			return err
		}
		if _, err = s.DB.Exec(ctx, `UPDATE flow_execution_binding SET engine_state=$2,state_observed_at=now() WHERE kestra_id=$1`, id, state.State.Current); err != nil {
			return err
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
			if err := s.Sync(task); err != nil {
				log.Printf("flows sync failed: %v", err)
			}
			cancel()
		}
	}
}
