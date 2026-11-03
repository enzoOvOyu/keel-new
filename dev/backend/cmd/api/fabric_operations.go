package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	fab "keel.local/dev/internal/fabric"
	"net/http"
	"time"
)

// Reuse identity approvals; Fabric never substitutes local role toggles for signatures.
func (s *Server) fabricApprovalCommand(ctx context.Context, r *http.Request, tx pgx.Tx, state *fab.State, c fab.Command) (string, error) {
	var change *fab.Change
	for i := range state.Changes {
		if state.Changes[i].ID == c.ID {
			change = &state.Changes[i]
		}
	}
	if change == nil {
		return "", studioError("FABRIC_CHANGE_NOT_FOUND")
	}
	if state.Input == nil || change.Input != *state.Input {
		return "", studioError("FABRIC_INPUT_CHANGED")
	}
	if c.Action == "submit" {
		if change.Status != "DRAFT" || change.CreatedBy != subjectOf(ctx).ID {
			return "", studioError("FABRIC_APPLICANT_DRAFT_REQUIRED")
		}
		if !subjectOf(ctx).Can("fabric:execute", state.Project, "fabric_change", change.ID) {
			return "", studioError("FORBIDDEN")
		}
		groups := []string{"NE", "DE"}
		status := "pending"
		if change.Risk == "critical" {
			groups = append(groups, "NH")
			status = "blocked_policy"
		}
		change.ApprovalID = uuid.NewString()
		_, e := tx.Exec(ctx, `INSERT INTO approval_record(id,project_id,requester_id,action,resource_type,resource_id,snapshot_version,risk,state,required_groups,targets,reason,expires_at) VALUES($1,$2,$3,'network_config','fabric_change',$4,$5,$6,$7,$8,$9,$10,$11)`, change.ApprovalID, state.Project, change.CreatedBy, change.ID, fabricDigest(*change), change.Risk, status, jsonBytes(groups), jsonBytes(change.Targets), change.Purpose, time.Now().Add(24*time.Hour))
		if e != nil {
			return "", e
		}
		change.Status = "IN_REVIEW"
		return change.ID, nil
	}
	if change.ApprovalID == "" {
		return "", studioError("FABRIC_APPROVAL_REQUIRED")
	}
	var status string
	var expiry time.Time
	if e := tx.QueryRow(ctx, `SELECT state,expires_at FROM approval_record WHERE id::text=$1 AND project_id=$2 AND resource_id=$3`, change.ApprovalID, state.Project, change.ID).Scan(&status, &expiry); e != nil {
		return "", e
	}
	if time.Now().After(expiry) {
		change.Status = "STALE"
		return change.ID, nil
	}
	switch status {
	case "approved":
		change.Status = "APPROVED"
	case "rejected":
		change.Status = "REJECTED"
	default:
		change.Status = "IN_REVIEW"
	}
	return change.ID, nil
}

func (s *Server) fabricOnboarding(ctx context.Context, r *http.Request, state *fab.State, input *fabricPublishedInput, c fab.Command) (string, error) {
	// Planned identities are not discovery receipts. A bootstrap profile and adapter must exist.
	if c.Action == "finishZtp" {
		return "", studioError("FABRIC_ZTP_EXECUTION_EVIDENCE_REQUIRED")
	}
	if c.Action == "match" {
		return "", studioError("FABRIC_DISCOVERED_IDENTITY_EVIDENCE_REQUIRED")
	}
	return "", studioError("FABRIC_ZTP_ADAPTER_NOT_CONFIGURED")
}
func (s *Server) fabricExecutionCommand(ctx context.Context, r *http.Request, tx pgx.Tx, state *fab.State, input *fabricPublishedInput, c fab.Command) (string, error) {
	if c.Action == "deploy" {
		return "", studioError("FABRIC_NATIVE_RENDERER_AND_APPROVED_EXECUTION_ADAPTER_REQUIRED")
	}
	var ref *fab.Execution
	for i := range state.Executions {
		if state.Executions[i].ID == c.ID {
			ref = &state.Executions[i]
		}
	}
	if ref == nil {
		return "", studioError("FABRIC_EXECUTION_NOT_FOUND")
	}
	if s.flows == nil {
		return "", studioError("FABRIC_DEPENDENCY_NOT_READY")
	}
	scope := flowScope(r, "flows:read")
	if c.Action == "control" {
		if !subjectOf(ctx).Can("flows:execute", state.Project, "execution", ref.ID) {
			return "", studioError("FORBIDDEN")
		}
		action := c.Control
		if action == "cancel" {
			action = "kill"
		}
		ex, e := s.flows.Action(ctx, scope, ref.ID, action)
		if e != nil {
			return "", e
		}
		fabricExecution(ref, ex)
	} else {
		ex, e := s.flows.Execution(ctx, scope, ref.ID)
		if e != nil {
			return "", e
		}
		fabricExecution(ref, ex)
	}
	return ref.ID, nil
}
func (s *Server) fabricCollect(ctx context.Context, r *http.Request, state *fab.State, input *fabricPublishedInput) (string, error) {
	b := fabricActiveBuild(state)
	if b == nil {
		return "", studioError("FABRIC_BUILD_REQUIRED")
	}
	// No collector adapter is registered yet. Preserve explicit gaps, never invent observations.
	a := fab.Assurance{ID: uuid.NewString(), Build: b.ID, Input: input.ID, CreatedAt: time.Now().UTC(), Executions: []string{}, Records: []fab.Comparison{}, Status: "UNKNOWN"}
	for _, id := range b.Targets {
		artifact := ""
		rendered := "UNSUPPORTED"
		for _, v := range state.Artifacts {
			if v.Device == id && v.Input == input.ID {
				artifact = v.ID
				if v.Commands != "" {
					rendered = v.Hash
				}
			}
		}
		a.Records = append(a.Records, fab.Comparison{Device: id, Artifact: artifact, Desired: b.ID, Rendered: rendered, Applied: "UNKNOWN", Operational: "UNKNOWN", State: "UNKNOWN", Source: "collector-not-configured", Findings: []fab.Finding{{ID: uuid.NewString(), Device: id, Check: "Operational", Domain: "Operational", Status: "NOT_RUN", Message: "未配置独立采集适配器；没有设备运行证据", Required: true}}})
	}
	state.Assurances = append(state.Assurances, a)
	return a.ID, nil
}
