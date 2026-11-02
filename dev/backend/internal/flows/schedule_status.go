package flows

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"time"
)

type TriggerRuntime struct {
	TriggerID          string     `json:"triggerId"`
	UpdatedAt          *time.Time `json:"updatedAt"`
	EvaluatedAt        *time.Time `json:"evaluatedAt"`
	NextEvaluationDate *time.Time `json:"nextEvaluationDate"`
	LastTriggeredDate  *time.Time `json:"lastTriggeredDate"`
	Disabled           bool       `json:"disabled"`
	SourceDisabled     bool       `json:"sourceDisabled"`
	Locked             bool       `json:"locked"`
}
type ScheduleFire struct {
	EngineExecutionID string    `json:"engine_execution_id"`
	ExecutionID       *string   `json:"execution_id"`
	State             string    `json:"state"`
	ErrorCode         *string   `json:"error_code"`
	CreatedAt         time.Time `json:"created_at"`
}
type ScheduleStatus struct {
	Configured   bool             `json:"configured"`
	Runtime      []TriggerRuntime `json:"runtime"`
	RuntimeError *string          `json:"runtime_error"`
	History      []ScheduleFire   `json:"history"`
	Source       string           `json:"source"`
}

// ScheduleStatus reads the engine's actual next evaluation; it never calculates
// future trigger state from a browser clock or treats a trigger as a device PASS.
func (s *Service) ScheduleStatus(ctx context.Context, p Scope, id string) (ScheduleStatus, error) {
	out := ScheduleStatus{Runtime: []TriggerRuntime{}, History: []ScheduleFire{}, Source: "Kestra trigger runtime; Keel scoped trigger consumption history (latest 50)"}
	f, e := s.Flow(ctx, p, id)
	if e != nil {
		return out, e
	}
	var engineFlow string
	e = s.DB.QueryRow(ctx, `SELECT engine_flow_id FROM flow_schedule WHERE flow_id=$1`, id).Scan(&engineFlow)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	out.Configured = true
	var page struct {
		Results []TriggerRuntime `json:"results"`
	}
	if s.Engine == nil {
		code := "KESTRA_NOT_CONFIGURED"
		out.RuntimeError = &code
	} else if e = s.Engine.JSON(ctx, "GET", "/triggers/"+url.PathEscape(f.Namespace)+"/"+url.PathEscape(engineFlow)+"?page=1&size=100", &page); e != nil {
		code := "SCHEDULE_RUNTIME_UNAVAILABLE"
		out.RuntimeError = &code
	} else {
		for _, v := range page.Results {
			if v.TriggerID == "cron" {
				out.Runtime = append(out.Runtime, v)
			}
		}
	}
	rows, e := s.DB.Query(ctx, `SELECT engine_execution_id,execution_id::text,state,error_code,created_at FROM flow_schedule_fire WHERE flow_id=$1 ORDER BY created_at DESC,engine_execution_id DESC LIMIT 50`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v ScheduleFire
		if e = rows.Scan(&v.EngineExecutionID, &v.ExecutionID, &v.State, &v.ErrorCode, &v.CreatedAt); e != nil {
			return out, e
		}
		out.History = append(out.History, v)
	}
	return out, rows.Err()
}
