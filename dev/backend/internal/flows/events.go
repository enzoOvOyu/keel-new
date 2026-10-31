package flows

import (
	"context"
	"encoding/json"
	"fmt"
)

type Event struct {
	ID   int64           `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (s *Service) RecordEvent(ctx context.Context, execution, kind string, value any) error {
	b := raw(value)
	_, e := s.DB.Exec(ctx, `INSERT INTO flow_event(execution_id,event_type,payload,fingerprint) VALUES($1,$2,$3,$4) ON CONFLICT(execution_id,fingerprint) DO NOTHING`, execution, kind, b, hash(kind+string(b)))
	return e
}
func (s *Service) CaptureEvents(ctx context.Context, p Scope, id string) error {
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return e
	}
	if e = s.RecordEvent(ctx, id, "execution", map[string]any{"id": ex.ID, "state": ex.Engine["state"], "taskRunList": ex.Engine["taskRunList"]}); e != nil {
		return e
	}
	for _, t := range ex.Targets {
		if e = s.RecordEvent(ctx, id, "target", map[string]any{"asset_id": t.ID, "state": t.State, "exit_code": t.Exit, "stdout_bytes": len(t.Stdout), "stderr_bytes": len(t.Stderr)}); e != nil {
			return e
		}
	}
	if ex.KestraID != nil {
		var logs []map[string]any
		if e = s.Engine.JSON(ctx, "GET", "/logs/"+*ex.KestraID, &logs); e != nil {
			return e
		}
		for _, log := range logs {
			log["message"] = redactOutput(fmt.Sprint(log["message"]))
			if e = s.RecordEvent(ctx, id, "log", log); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Service) Events(ctx context.Context, p Scope, id string, after int64) ([]Event, error) {
	var valid bool
	if e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_execution_binding WHERE id::text=$1 AND project_id=ANY($2))`, id, p.Projects).Scan(&valid); e != nil {
		return nil, e
	}
	if !valid {
		return nil, problem(404, "EXECUTION_NOT_FOUND")
	}
	if after < 0 {
		return nil, problem(400, "EVENT_CURSOR_INVALID")
	}
	if after > 0 {
		var belongs bool
		s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_event WHERE execution_id::text=$1 AND id=$2)`, id, after).Scan(&belongs)
		if !belongs {
			return nil, problem(400, "EVENT_CURSOR_SCOPE_INVALID")
		}
	}
	rows, e := s.DB.Query(ctx, `SELECT id,event_type,payload FROM flow_event WHERE execution_id::text=$1 AND id>$2 ORDER BY id LIMIT 200`, id, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.Type, &v.Data); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) StepResults(ctx context.Context, p Scope, id string) ([]map[string]any, error) {
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT asset_id,step_id,state,exit_code,stdout,stderr FROM flow_step_result WHERE execution_id=$1 ORDER BY asset_id,step_id`, ex.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var asset, step, state, stdout, stderr string
		var exit *int
		e = rows.Scan(&asset, &step, &state, &exit, &stdout, &stderr)
		if e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"asset_id": asset, "step_id": step, "state": state, "exit_code": exit, "stdout": stdout, "stderr": stderr})
	}
	return out, rows.Err()
}
