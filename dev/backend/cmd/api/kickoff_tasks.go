package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type kickoffTask struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	State         string    `json:"state"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	ErrorCode     *string   `json:"errorCode,omitempty"`
	AnalysisJobID *string   `json:"analysisJobId,omitempty"`
}
type kickoffTaskStep struct {
	Sequence  int       `json:"sequence"`
	Tool      string    `json:"tool"`
	State     string    `json:"state"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}
type kickoffTaskPage struct {
	Items  []kickoffTask `json:"items"`
	Offset int           `json:"offset"`
	Limit  int           `json:"limit"`
	Total  int           `json:"total"`
}
type kickoffTaskDetail struct {
	Task          kickoffTask          `json:"task"`
	Steps         []kickoffTaskStep    `json:"steps"`
	Understanding *understandingReport `json:"understanding,omitempty"`
}

const kickoffTaskScope = ` FROM eng_job j
 JOIN eng_source e ON e.id=j.source_id AND e.project_id=j.project_id
 JOIN kickoff_source k ON k.id::text=j.proposal->>'sourceId' AND k.project_id=j.project_id
 WHERE j.project_id=$1 AND k.id::text=$2 AND k.detached_at IS NULL
 AND (j.proposal->>'kind'='kickoff-domain-agent/v1' AND j.proposal->>'flowVersion'='termination-v1'
 OR j.proposal->>'kind'='kickoff-adoption/v1' AND EXISTS(SELECT 1 FROM eng_job a WHERE a.id::text=j.proposal->'body'->>'jobId' AND a.project_id=j.project_id AND a.proposal->>'flowVersion'='termination-v1'))`
const kickoffTaskColumns = `j.id::text,CASE WHEN j.proposal->>'kind'='kickoff-domain-agent/v1' THEN 'analysis' ELSE 'adoption' END,
 j.state,j.created_at,j.updated_at,j.error_code,
 CASE WHEN j.proposal->>'kind'='kickoff-adoption/v1' THEN NULLIF(j.proposal->'body'->>'jobId','') END`

func scanKickoffTask(row pgx.Row) (kickoffTask, error) {
	var task kickoffTask
	err := row.Scan(&task.ID, &task.Kind, &task.State, &task.CreatedAt, &task.UpdatedAt, &task.ErrorCode, &task.AnalysisJobID)
	return task, err
}
func kickoffTaskPagination(r *http.Request) (int, int, error) {
	offset, limit := 0, 20
	var err error
	if v, ok := r.URL.Query()["offset"]; ok {
		offset, err = strconv.Atoi(v[0])
		if err != nil || offset < 0 {
			return 0, 0, errors.New("KICKOFF_TASK_PAGINATION_INVALID")
		}
	}
	if v, ok := r.URL.Query()["limit"]; ok {
		limit, err = strconv.Atoi(v[0])
		if err != nil || limit < 1 || limit > 50 {
			return 0, 0, errors.New("KICKOFF_TASK_PAGINATION_INVALID")
		}
	}
	return offset, limit, nil
}

func (s *Server) kickoffTasks(w http.ResponseWriter, r *http.Request, source, job string) {
	if !kickoffAgentPermission(w, r, s.project, "bom:read") {
		return
	}
	offset, limit, err := kickoffTaskPagination(r)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kickoff_source WHERE project_id=$1 AND id::text=$2 AND detached_at IS NULL)`, s.project, source).Scan(&exists)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if !exists {
		fail(w, 404, "KICKOFF_SOURCE_NOT_FOUND")
		return
	}
	if job == "" {
		page := kickoffTaskPage{Items: []kickoffTask{}, Offset: offset, Limit: limit}
		err = tx.QueryRow(ctx, `SELECT count(*)`+kickoffTaskScope, s.project, source).Scan(&page.Total)
		if err != nil {
			kickoffAgentFailure(w, err)
			return
		}
		rows, e := tx.Query(ctx, `SELECT `+kickoffTaskColumns+kickoffTaskScope+` ORDER BY j.created_at DESC,j.id DESC OFFSET $3 LIMIT $4`, s.project, source, offset, limit)
		if e != nil {
			kickoffAgentFailure(w, e)
			return
		}
		for rows.Next() {
			task, e := scanKickoffTask(rows)
			if e != nil {
				rows.Close()
				kickoffAgentFailure(w, e)
				return
			}
			page.Items = append(page.Items, task)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			kickoffAgentFailure(w, err)
			return
		}
		respond(w, 200, page)
		return
	}
	task, err := scanKickoffTask(tx.QueryRow(ctx, `SELECT `+kickoffTaskColumns+kickoffTaskScope+` AND j.id::text=$3`, s.project, source, job))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "KICKOFF_TASK_NOT_FOUND")
		return
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	out := kickoffTaskDetail{Task: task, Steps: []kickoffTaskStep{}}
	rows, err := tx.Query(ctx, `SELECT sequence,tool,state,detail,created_at FROM eng_step WHERE job_id::text=$1 ORDER BY sequence`, job)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	for rows.Next() {
		var step kickoffTaskStep
		if err = rows.Scan(&step.Sequence, &step.Tool, &step.State, &step.Detail, &step.CreatedAt); err != nil {
			rows.Close()
			kickoffAgentFailure(w, err)
			return
		}
		out.Steps = append(out.Steps, step)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	analysis := task.ID
	if task.Kind == "adoption" {
		analysis = ""
		if task.AnalysisJobID != nil {
			analysis = *task.AnalysisJobID
		}
	}
	// Only the selected analysis (or the adoption's explicit analysis) may supply
	// a report. No latest-job fallback, historical report or synthetic steps.
	var evidence bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1`+kickoffTaskScope+` AND j.id::text=$3 AND j.proposal->>'kind'=$4
 AND EXISTS(SELECT 1 FROM eng_analysis_artifact a WHERE a.job_id=j.id AND a.kind='kickoff_domain_understanding' AND a.state='succeeded'))`, s.project, source, analysis, kickoffAgentKind).Scan(&evidence)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if evidence {
		var report understandingReport
		err = s.kickoffAgentArtifact(ctx, analysis, "kickoff_domain_understanding", &report)
		if err != nil {
			kickoffAgentFailure(w, err)
			return
		}
		out.Understanding = &report
	}
	respond(w, 200, out)
}

// Cancel only the selected same-project analysis. Clearing the claim fences late
// writes and the existing worker heartbeat cancels the provider request.
func (s *Server) kickoffCancelAnalysis(w http.ResponseWriter, r *http.Request, source, job string) {
	if !kickoffAgentPermission(w, r, s.project, "bom:import") {
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	defer tx.Rollback(ctx)
	task, err := scanKickoffTask(tx.QueryRow(ctx, `SELECT `+kickoffTaskColumns+kickoffTaskScope+` AND j.id::text=$3 AND j.proposal->>'kind'='kickoff-domain-agent/v1' FOR UPDATE OF j`, s.project, source, job))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "KICKOFF_ANALYSIS_NOT_FOUND")
		return
	}
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if task.State == "pending" || task.State == "running" {
		_, err = tx.Exec(ctx, `UPDATE eng_job SET state='failed',error_code='CANCELLED_BY_USER',claim_id=NULL,updated_at=now() WHERE id::text=$1`, job)
		if err == nil {
			err = s.kickoffAudit(ctx, tx, "kickoff.analysis.cancelled", source, map[string]any{"jobId": job})
		}
		if err != nil {
			kickoffAgentFailure(w, err)
			return
		}
		task.State = "failed"
		code := "CANCELLED_BY_USER"
		task.ErrorCode = &code
		task.UpdatedAt = time.Now()
	}
	if err = tx.Commit(ctx); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	respond(w, 200, task)
}
