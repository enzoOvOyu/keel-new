package flows

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/cmdb"
)

type ProjectionEvent struct {
	Execution string   `json:"execution_id"`
	Targets   []string `json:"target_ids"`
	Actor     string   `json:"actor"`
	State     string   `json:"state"`
}

func (s *Service) ProjectEvent(ctx context.Context, tx pgx.Tx, eventID, project string, payload []byte) error {
	var event ProjectionEvent
	if json.Unmarshal(payload, &event) != nil || event.Execution == "" {
		return problem(422, "FLOW_PROJECTION_INVALID")
	}
	var seen bool
	if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_ingest_event WHERE event_id=$1)`, eventID).Scan(&seen); e != nil {
		return e
	}
	if seen {
		return nil
	}
	domain := cmdb.Service{DB: s.DB}
	for _, asset := range event.Targets {
		var authorized bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_execution_target t JOIN flow_execution_binding b ON b.id=t.execution_id WHERE b.project_id=$1 AND t.asset_id=$2 AND b.id::text=$3)`, project, asset, event.Execution).Scan(&authorized); e != nil {
			return e
		}
		if !authorized {
			return problem(409, "FLOW_ASSET_SCOPE_CHANGED")
		}
		if e := domain.AddEvent(ctx, tx, project, eventID+":"+asset, asset, "execution", "Flow / Execution · "+event.State, event.State, "flows", "execution:"+event.Execution, event.Actor, map[string]any{"execution_id": event.Execution, "fact": "job_event_not_delivery_verdict"}, []map[string]string{{"label": "Execution", "url": "/flows/executions/" + event.Execution + "?target=" + asset}}); e != nil {
			return e
		}
	}
	_, e := tx.Exec(ctx, `INSERT INTO cmdb_ingest_event(event_id,source_type,source_ref) VALUES($1,'flows',$2)`, eventID, event.Execution)
	return e
}
