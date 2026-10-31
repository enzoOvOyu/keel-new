package flows

import (
	"context"
	"encoding/json"
)

func (s *Service) ExecutionAudit(ctx context.Context, p Scope, id string) ([]map[string]any, error) {
	if _, e := s.Execution(ctx, p, id); e != nil {
		return nil, e
	}
	return s.history(ctx, `SELECT jsonb_build_object('id',id,'actor',actor,'action',action,'details',details,'created_at',created_at) FROM flow_audit WHERE resource_id=$1 AND project_id=ANY($2) ORDER BY created_at`, id, p.Projects)
}
func (s *Service) AgentHistory(ctx context.Context, p Scope, id, kind string) ([]map[string]any, error) {
	if kind == "tasks" {
		return s.history(ctx, `SELECT jsonb_build_object('id',t.id,'state',t.state,'execution_id',t.execution_id,'updated_at',t.updated_at) FROM flow_agent_task t JOIN flow_agent a ON a.id=t.agent_id WHERE a.id::text=$1 AND a.project_id=ANY($2) ORDER BY t.created_at DESC LIMIT 100`, id, p.Projects)
	}
	return s.history(ctx, `SELECT jsonb_build_object('id',id,'actor',actor,'action',action,'details',details,'created_at',created_at) FROM flow_audit WHERE resource_id=$1 AND project_id=ANY($2) ORDER BY created_at DESC LIMIT 100`, id, p.Projects)
}
func (s *Service) history(ctx context.Context, sql string, id string, projects []string) ([]map[string]any, error) {
	rows, e := s.DB.Query(ctx, sql, id, projects)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		var v map[string]any
		json.Unmarshal(b, &v)
		out = append(out, v)
	}
	return out, rows.Err()
}
