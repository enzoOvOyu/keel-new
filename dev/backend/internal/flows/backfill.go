package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"time"
)

type BackfillRequest struct {
	Start            time.Time `json:"start"`
	End              time.Time `json:"end"`
	ScheduleRevision int64     `json:"expected_schedule_revision"`
	FlowRevision     int       `json:"expected_flow_revision"`
	PreviewHash      string    `json:"preview_hash"`
}
type BackfillSnapshot struct {
	Request BackfillRequest `json:"request"`
	Run     RunRequest      `json:"run"`
}
type BackfillPreview struct {
	Project          string      `json:"project_id"`
	Flow             string      `json:"flow_id"`
	Cron             string      `json:"cron"`
	Timezone         string      `json:"timezone"`
	Occurrences      []time.Time `json:"occurrences"`
	Count            int         `json:"count"`
	Truncated        bool        `json:"truncated"`
	RequiresApproval bool        `json:"requires_approval"`
	Hash             string      `json:"preview_hash"`
	Run              RunRequest  `json:"run"`
}

func historicalOccurrences(expression, zone string, start, end, now time.Time) ([]time.Time, bool, error) {
	if start.IsZero() || end.IsZero() || start.After(end) || end.After(now) {
		return nil, false, problem(422, "BACKFILL_HISTORICAL_WINDOW_REQUIRED")
	}
	location, e := time.LoadLocation(zone)
	if e != nil {
		return nil, false, problem(422, "SCHEDULE_CRON_OR_TIMEZONE_INVALID")
	}
	schedule, e := cron.ParseStandard("CRON_TZ=" + location.String() + " " + expression)
	if e != nil {
		return nil, false, problem(422, "BACKFILL_UNSUPPORTED_CRON_VARIANT")
	}
	dates := []time.Time{}
	for next := schedule.Next(start.Add(-time.Nanosecond)); !next.IsZero() && !next.After(end); next = schedule.Next(next) {
		if len(dates) == 200 {
			return dates, true, nil
		}
		dates = append(dates, next.UTC())
	}
	return dates, false, nil
}
func (s *Service) BackfillPreview(ctx context.Context, p Scope, id string, input BackfillRequest) (BackfillPreview, error) {
	out := BackfillPreview{Occurrences: []time.Time{}}
	flow, e := s.Flow(ctx, p, id)
	if e != nil {
		return out, e
	}
	if flow.Draft || flow.Disabled || flow.Revision != input.FlowRevision {
		return out, problem(409, "FLOW_NOT_EXECUTABLE")
	}
	schedule, e := s.Schedule(ctx, p, id, nil)
	if e != nil {
		return out, e
	}
	if schedule.Revision == 0 || schedule.Revision != input.ScheduleRevision {
		return out, problem(409, "BACKFILL_SCHEDULE_REVISION_REQUIRED")
	}
	out.Occurrences, out.Truncated, e = historicalOccurrences(schedule.Cron, schedule.Timezone, input.Start, input.End, time.Now())
	if e != nil {
		return out, e
	}
	run := schedule.Run
	run.Project = flow.Project
	run.Flow = flow.ID
	run.Revision = flow.Revision
	run.ApprovalID = ""
	run.ApprovalContext = ""
	run.Source = "backfill"
	decisions, e := s.Preflight(ctx, p, run)
	if e != nil {
		return out, e
	}
	run.TargetRevisions = map[string]int64{}
	for _, d := range decisions {
		if !d.Ready {
			return out, problem(422, "BACKFILL_PREFLIGHT_BLOCKED")
		}
		run.TargetRevisions[d.Asset] = d.Revision
	}
	out.Project = flow.Project
	out.Flow = id
	out.Cron = schedule.Cron
	out.Timezone = schedule.Timezone
	out.Count = len(out.Occurrences)
	out.RequiresApproval = NeedsApproval(run)
	out.Run = run
	out.Hash = hash(string(raw(struct {
		Request BackfillRequest
		Run     RunRequest
		Dates   []time.Time
	}{BackfillRequest{Start: input.Start, End: input.End, ScheduleRevision: input.ScheduleRevision, FlowRevision: input.FlowRevision}, run, out.Occurrences})))
	return out, nil
}
func (s *Service) CreateBackfill(ctx context.Context, p Scope, id string, input BackfillRequest, key string) (OperationJob, error) {
	if key == "" {
		return OperationJob{}, problem(400, "IDEMPOTENCY_KEY_REQUIRED")
	}
	preview, e := s.BackfillPreview(ctx, p, id, input)
	if e != nil {
		return OperationJob{}, e
	}
	if input.PreviewHash != preview.Hash {
		return OperationJob{}, problem(409, "BACKFILL_PREVIEW_CHANGED")
	}
	if preview.Count == 0 || preview.Truncated {
		return OperationJob{}, problem(422, "BACKFILL_NARROW_HISTORY_WINDOW")
	}
	return s.createOperation(ctx, p, preview.Project, "backfill", BackfillSnapshot{Request: input, Run: preview.Run}, key, nil, preview.Occurrences)
}
func operationApprovalContext(id string, ordinal int) string {
	return fmt.Sprintf("backfill:%s:%d", id, ordinal)
}
func (s *Service) OperationItemRequest(ctx context.Context, p Scope, id string, ordinal int) (RunRequest, error) {
	job, e := s.OperationJob(ctx, p, id)
	if e != nil {
		return RunRequest{}, e
	}
	if job.Kind != "backfill" || ordinal < 0 || ordinal >= len(job.Items) {
		return RunRequest{}, problem(404, "OPERATION_ITEM_NOT_FOUND")
	}
	var rawRequest []byte
	if e = s.DB.QueryRow(ctx, `SELECT request FROM flow_operation_job WHERE id=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&rawRequest); e != nil {
		return RunRequest{}, e
	}
	var snapshot BackfillSnapshot
	if e = json.Unmarshal(rawRequest, &snapshot); e != nil {
		return RunRequest{}, e
	}
	snapshot.Run.ApprovalID = ""
	snapshot.Run.ApprovalContext = operationApprovalContext(id, ordinal)
	return snapshot.Run, nil
}
