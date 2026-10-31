package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"go.yaml.in/yaml/v3"
	"keel.local/dev/internal/identity"
	"strings"
	"time"
)

type Schedule struct {
	Cron      string     `json:"cron"`
	Timezone  string     `json:"timezone"`
	Enabled   bool       `json:"enabled"`
	Run       RunRequest `json:"run"`
	Revision  int64      `json:"revision"`
	LastError *string    `json:"last_error"`
}

func (s *Service) Schedule(ctx context.Context, p Scope, id string, in *Schedule) (Schedule, error) {
	f, e := s.Flow(ctx, p, id)
	if e != nil {
		return Schedule{}, e
	}
	if in == nil {
		var out Schedule
		var b []byte
		e = s.DB.QueryRow(ctx, `SELECT cron,timezone,enabled,request,revision,last_error FROM flow_schedule WHERE flow_id=$1`, id).Scan(&out.Cron, &out.Timezone, &out.Enabled, &b, &out.Revision, &out.LastError)
		if e == pgx.ErrNoRows {
			return Schedule{Timezone: "UTC", Run: RunRequest{Project: f.Project, Flow: id, Revision: f.Revision, Targets: []string{}, Kind: "bash", Policy: "agent_preferred", Timeout: 600, Concurrency: 8, Source: "schedule"}}, nil
		}
		if e != nil {
			return out, e
		}
		json.Unmarshal(b, &out.Run)
		return out, nil
	}
	if _, e = time.LoadLocation(in.Timezone); e != nil || len(strings.Fields(in.Cron)) != 5 {
		return Schedule{}, problem(422, "SCHEDULE_CRON_OR_TIMEZONE_INVALID")
	}
	in.Run.Project = f.Project
	in.Run.Flow = f.ID
	in.Run.Revision = f.Revision
	in.Run.Source = "schedule"
	in.Run.TargetRevisions = nil
	if len(in.Run.Targets) == 0 {
		return Schedule{}, problem(422, "SCHEDULE_TARGETS_REQUIRED")
	}
	if _, e = s.Preflight(ctx, p, in.Run); e != nil {
		return Schedule{}, e
	}
	if e = s.CheckJobApproval(ctx, p, in.Run); e != nil {
		return Schedule{}, e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return Schedule{}, e
	}
	defer tx.Rollback(ctx)
	var prior int64
	e = tx.QueryRow(ctx, `SELECT revision FROM flow_schedule WHERE flow_id=$1 FOR UPDATE`, id).Scan(&prior)
	if e != nil && e != pgx.ErrNoRows {
		return Schedule{}, e
	}
	if prior != in.Revision {
		return Schedule{}, problem(409, "SCHEDULE_REVISION_CONFLICT")
	}
	engineID := "schedule-" + strings.ReplaceAll(f.ID, "-", "")
	doc := map[string]any{"id": engineID, "namespace": f.Namespace, "disabled": !in.Enabled, "tasks": []any{map[string]any{"id": "emit_job_trigger", "type": "io.kestra.plugin.core.log.Log", "message": "Keel scoped batch trigger"}}, "triggers": []any{map[string]any{"id": "cron", "type": "io.kestra.plugin.core.trigger.Schedule", "cron": in.Cron, "timezone": in.Timezone, "disabled": !in.Enabled}}}
	source, _ := yaml.Marshal(doc)
	if _, e = s.Engine.Save(ctx, string(source), prior > 0, f.Namespace, engineID); e != nil {
		return Schedule{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO flow_schedule(flow_id,owner_id,engine_flow_id,cron,timezone,enabled,request) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(flow_id) DO UPDATE SET owner_id=excluded.owner_id,cron=excluded.cron,timezone=excluded.timezone,enabled=excluded.enabled,request=excluded.request,revision=flow_schedule.revision+1,last_error=NULL,updated_at=now()`, id, p.User, engineID, in.Cron, in.Timezone, in.Enabled, raw(in.Run))
	if e != nil {
		return Schedule{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Schedule{}, e
	}
	s.audit(ctx, p, f.Project, "schedule.save", id, map[string]any{"enabled": in.Enabled, "cron": in.Cron})
	return s.Schedule(ctx, p, id, nil)
}

// Kestra owns time/cron; this is an idempotent trigger consumer, never a second scheduler.
func (s *Service) SyncSchedules(ctx context.Context, executions []map[string]any) error {
	for _, ex := range executions {
		engineFlow := fmt.Sprint(ex["flowId"])
		if !strings.HasPrefix(engineFlow, "schedule-") {
			continue
		}
		var flow, owner, project, namespace string
		var b []byte
		e := s.DB.QueryRow(ctx, `SELECT f.id::text,s.owner_id::text,f.project_id,f.namespace,s.request FROM flow_schedule s JOIN flow_binding f ON f.id=s.flow_id WHERE s.engine_flow_id=$1 AND s.enabled`, engineFlow).Scan(&flow, &owner, &project, &namespace, &b)
		if e != nil || namespace != fmt.Sprint(ex["namespace"]) {
			continue
		}
		eid := fmt.Sprint(ex["id"])
		var details map[string]any
		if e = s.Engine.JSON(ctx, "GET", "/executions/"+eid, &details); e != nil {
			return e
		}
		trigger, _ := details["trigger"].(map[string]any)
		if trigger["id"] != "cron" {
			continue
		}
		// Historical firings are not re-executed when re-enabling an old schedule.
		state, _ := details["state"].(map[string]any)
		started, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(state["startDate"]))
		var updated time.Time
		s.DB.QueryRow(ctx, `SELECT updated_at FROM flow_schedule WHERE flow_id=$1`, flow).Scan(&updated)
		if started.Before(updated) {
			continue
		}
		var known bool
		s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_schedule_fire WHERE engine_execution_id=$1 AND state IN('SUCCESS','BLOCKED'))`, eid).Scan(&known)
		if known {
			continue
		}
		subject, e := (identity.Service{DB: s.DB}).Load(ctx, owner)
		scope := Scope{User: owner, Actor: subject.Username, Projects: []string{project}}
		var run RunRequest
		json.Unmarshal(b, &run)
		if e != nil || !subject.Can("flows:execute", project, "", "") {
			e = problem(403, "SCHEDULE_OWNER_AUTHORIZATION_REVOKED")
		}
		var result Execution
		if e == nil {
			result, e = s.Start(ctx, scope, run, "schedule:"+eid)
		}
		status := "SUCCESS"
		code := ""
		if e != nil {
			status = "BLOCKED"
			code = e.Error()
		}
		_, err := s.DB.Exec(ctx, `INSERT INTO flow_schedule_fire(engine_execution_id,flow_id,state,execution_id,error_code) VALUES($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')) ON CONFLICT(engine_execution_id) DO UPDATE SET state=excluded.state,execution_id=excluded.execution_id,error_code=excluded.error_code`, eid, flow, status, result.ID, code)
		if err != nil {
			return err
		}
		s.DB.Exec(ctx, `UPDATE flow_schedule SET last_error=NULLIF($2,'') WHERE flow_id=$1`, flow, code)
	}
	return nil
}
