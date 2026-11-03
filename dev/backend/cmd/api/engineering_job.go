package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"net"
	"regexp"
	"strings"
	"time"
)

func (s *Server) engineeringWorker(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.runEngineeringJob(ctx)
		}
	}
}
func (s *Server) runEngineeringJob(ctx context.Context) {
	claim := uuid.NewString()
	var job, source, project string
	var expected int
	e := s.db.QueryRow(ctx, `UPDATE eng_job SET state='running',claim_id=$1,updated_at=now(),error_code=NULL WHERE id=(SELECT id FROM eng_job WHERE state='pending' OR (state='running' AND updated_at<now()-interval '3 minutes') ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id::text,source_id::text,project_id,expected_revision`, claim).Scan(&job, &source, &project, &expected)
	if e != nil {
		return
	}
	run, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		tick := time.NewTicker(20 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-run.Done():
				return
			case <-tick.C:
				tag, err := s.db.Exec(run, `UPDATE eng_job SET updated_at=now() WHERE id=$1 AND claim_id=$2 AND state='running'`, job, claim)
				if err != nil || tag.RowsAffected() != 1 {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-heartbeatDone }()
	scoped := *s
	scoped.project = project
	s = &scoped
	var importID, sourceHash string
	var raw []byte
	var wb bom.Workbook
	e = s.db.QueryRow(run, `SELECT b.id::text,b.workbook,b.content_sha256 FROM eng_source s JOIN bom_import_batch b ON b.id=s.import_id WHERE s.id=$1 AND s.project_id=$2`, source, project).Scan(&importID, &raw, &sourceHash)
	if e == nil {
		e = json.Unmarshal(raw, &wb)
	}
	sequence := 0
	if e == nil {
		e = s.db.QueryRow(run, `SELECT COALESCE(max(sequence),0) FROM eng_step WHERE job_id=$1`, job).Scan(&sequence)
	}
	trace := func(tool, state, detail string) error {
		sequence++
		tag, err := s.db.Exec(run, `INSERT INTO eng_step(job_id,sequence,tool,detail,state) SELECT $1,$2,$3,$4,$5 WHERE EXISTS(SELECT 1 FROM eng_job WHERE id=$1 AND claim_id=$6 AND state='running')`, job, sequence, tool, detail, state, claim)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("ENGINEERING_JOB_CLAIM_LOST")
		}
		return nil
	}
	handled, agentErr := s.runKickoffAgentJob(run, job, claim, source, expected, trace)
	if handled {
		e = agentErr
	} else if e == nil {
		provider, _, err := s.workbenchProvider(run, importID)
		e = err
		if e == nil {
			hooks := s.kickoffToolHooks(run, source, expected, wb)
			hooks.SourceHash = sourceHash
			hooks.Resume, e = s.resumeEngineeringAnalysis(run, source, expected)
			hooks.Artifact = func(ctx context.Context, tool, state string, value any) error {
				return s.saveAnalysisArtifact(ctx, job, claim, tool, state, value)
			}
			hooks.Checkpoint = func(ctx context.Context, cp ai.EngineeringCheckpoint) error {
				return s.saveAnalysisArtifact(ctx, job, claim, "checkpoint", "succeeded", cp)
			}
			if e == nil {
				e = trace("provider_selected", "succeeded", fmt.Sprintf("分步提取 · 模型 %s · Provider版本 %d", provider.Model, provider.ConfigRevision))
			}
			if e == nil {
				var result ai.KickoffResult
				result, e = provider.ExtractKickoff(run, engineeringSafeWorkbook(wb), wb, trace, hooks)
				if e == nil {
					e = s.saveKickoffCandidates(run, source, expected, job, claim, result)
				}
			}
		}
	}
	state := "succeeded"
	var code *string
	if e != nil {
		state = "failed"
		value := "ENGINEERING_ANALYSIS_FAILED"
		for _, allowed := range []string{"KICKOFF_AGENT_NO_CANDIDATES", "KICKOFF_CANDIDATE_INVALID", "KICKOFF_AGENT_TABLE_INVALID", "KICKOFF_ANALYSIS_NOT_READY", "FORBIDDEN", "AI_PROVIDER_NOT_CONFIGURED", "AI_CREDENTIAL_UNAVAILABLE", "PROVIDER_NOT_AVAILABLE", "PROVIDER_ERROR", "PROVIDER_TIMEOUT_OR_UNAVAILABLE", "PROVIDER_RESPONSE_FAILED", "ENGINEERING_AGENT_TURN_LIMIT", "ENGINEERING_AGENT_NO_PROGRESS", "REVISION_CONFLICT", "AI_SCHEMA_VALIDATOR_UNAVAILABLE", "ANALYSIS_STORAGE_UNAVAILABLE", "ANALYSIS_ARTIFACT_HASH_MISMATCH", "PROVIDER_OUTPUT_TRUNCATED", "INVALID_JSON", "KICKOFF_SOURCE_TOO_LARGE", "KICKOFF_SOURCE_HASH_MISMATCH", "KICKOFF_TASK_TOOL_LIMIT"} {
			if strings.HasPrefix(e.Error(), allowed) {
				value = allowed
				break
			}
		}
		if errors.Is(e, context.DeadlineExceeded) {
			value = "ENGINEERING_ANALYSIS_TIMEOUT"
		}
		code = &value
	}
	finish, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	s.db.Exec(finish, `UPDATE eng_job SET state=$3,error_code=$4,updated_at=now() WHERE id=$1 AND claim_id=$2`, job, claim, state, code)
}

var engineeringLiteral = regexp.MustCompile(`"(?:[^"]|"")*"`)

func engineeringSafeWorkbook(w bom.Workbook) bom.Workbook {
	out := bom.Workbook{Sheets: []bom.Sheet{}}
	for _, sheet := range w.Sheets {
		safe := safeSheet(sheet)
		identity := map[int]bool{}
		labels := map[int]bool{}
		// Privacy inspection scans all header locations; it does not determine the
		// semantic extraction regions or impose a workbook template.
		for _, c := range sheet.Cells {
			switch bom.Field(c.Value) {
			case "serial", "management_ip", "mac":
				identity[c.Column] = true
			case "hostname", "endpoint_a_device", "endpoint_b_device", "device_alias":
				labels[c.Column] = true
			}
		}
		for i := range safe.Cells {
			c := &safe.Cells[i]
			if c.Sensitive {
				c.Value = "[REDACTED]"
				c.Formula = ""
				continue
			}
			if bom.Field(c.Value) == "" {
				_, macErr := net.ParseMAC(c.Value)
				if identity[c.Column] || net.ParseIP(c.Value) != nil || macErr == nil {
					if strings.TrimSpace(c.Value) != "" {
						c.Value = "[IDENTITY-" + bom.StableID(c.Value) + "]"
					}
				} else if labels[c.Column] && strings.TrimSpace(c.Value) != "" {
					c.Value = "[LABEL-" + bom.StableID(c.Value) + "]"
				}
			}
			if c.Formula != "" {
				c.Formula = engineeringLiteral.ReplaceAllString(c.Formula, `"[TEXT]"`)
				if _, err := json.Number(c.Value).Float64(); err != nil {
					c.Value = "[CACHED_TEXT]"
				}
			}
		}
		out.Sheets = append(out.Sheets, safe)
	}
	return out
}
