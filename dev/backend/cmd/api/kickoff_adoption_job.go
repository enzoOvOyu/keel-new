package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/identity"
	"net/http"
)

const kickoffAdoptionKind = "kickoff-adoption/v1"

type kickoffAdoptionRequest struct {
	Kind     string           `json:"kind"`
	SourceID string           `json:"sourceId"`
	Actor    string           `json:"actor"`
	Body     kickoffAdoptBody `json:"body"`
}
type kickoffAdoptionStatus struct {
	JobID     string          `json:"jobId"`
	State     string          `json:"state"`
	ErrorCode *string         `json:"errorCode,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

func (s *Server) adoptionStatus(ctx context.Context, source string) (*kickoffAdoptionStatus, error) {
	var v kickoffAdoptionStatus
	err := s.db.QueryRow(ctx, `SELECT id::text,state,error_code,proposal->'result' FROM eng_job WHERE project_id=$1 AND proposal->>'sourceId'=$2 AND proposal->>'kind'=$3 ORDER BY created_at DESC,id DESC LIMIT 1`, s.project, source, kickoffAdoptionKind).Scan(&v.JobID, &v.State, &v.ErrorCode, &v.Result)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}
func (s *Server) kickoffAgentAdopt(w http.ResponseWriter, r *http.Request, source string) {
	if !kickoffAgentPermission(w, r, s.project, "bom:edit") {
		return
	}
	var body kickoffAdoptBody
	if !engineeringDecode(w, r, &body) {
		return
	}
	if body.JobID == "" {
		fail(w, 400, "KICKOFF_AGENT_JOB_REQUIRED")
		return
	}
	job, state, _, _, err := s.kickoffAgentJob(r.Context(), source, body.JobID)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if state != "succeeded" {
		fail(w, 409, "KICKOFF_ANALYSIS_NOT_READY")
		return
	}
	existing, err := s.adoptionStatus(r.Context(), source)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if existing != nil && (existing.State == "pending" || existing.State == "running") {
		respond(w, 202, existing)
		return
	}
	var workspaceRevision int
	if err = s.db.QueryRow(r.Context(), `SELECT revision FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&workspaceRevision); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if workspaceRevision != body.ExpectedRevision {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	var original string
	var expected int
	if err = s.db.QueryRow(r.Context(), `SELECT source_id::text,expected_revision FROM eng_job WHERE id=$1 AND project_id=$2`, job, s.project).Scan(&original, &expected); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	request := kickoffAdoptionRequest{Kind: kickoffAdoptionKind, SourceID: source, Actor: subjectOf(r.Context()).ID, Body: body}
	id := uuid.NewString()
	tag, err := s.db.Exec(r.Context(), `INSERT INTO eng_job(id,source_id,project_id,expected_revision,state,proposal) VALUES($1,$2,$3,$4,'pending',$5) ON CONFLICT DO NOTHING`, id, original, s.project, expected, jsonBytes(request))
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		existing, err = s.adoptionStatus(r.Context(), source)
		if err == nil && existing != nil && (existing.State == "pending" || existing.State == "running") {
			respond(w, 202, existing)
			return
		}
		fail(w, 409, "ENGINEERING_JOB_ACTIVE")
		return
	}
	respond(w, 202, kickoffAdoptionStatus{JobID: id, State: "pending"})
}
func (s *Server) runKickoffAdoption(ctx context.Context, job, claim string, raw []byte) error {
	var request kickoffAdoptionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return err
	}
	actor, err := (identity.Service{DB: s.db}).Load(ctx, request.Actor)
	if err != nil {
		return err
	}
	if !actor.Can("bom:edit", s.project, "", "") {
		return errors.New("FORBIDDEN")
	}
	ctx = context.WithValue(ctx, principalKey{}, principal{Actor: actor.ID, Project: s.project, Subject: actor})
	_, err = s.applyKickoffAdoption(ctx, request.SourceID, request.Body, request.Actor, job, claim)
	return err
}

func (s *Server) kickoffAdoptionStatus(w http.ResponseWriter, r *http.Request, source string) {
	if !kickoffAgentPermission(w, r, s.project, "bom:read") {
		return
	}
	var exists bool
	if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM kickoff_source WHERE id::text=$1 AND project_id=$2 AND detached_at IS NULL)`, source, s.project).Scan(&exists); err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if !exists {
		fail(w, 404, "KICKOFF_SOURCE_NOT_FOUND")
		return
	}
	status, err := s.adoptionStatus(r.Context(), source)
	if err != nil {
		kickoffAgentFailure(w, err)
		return
	}
	if status == nil {
		fail(w, 404, "KICKOFF_ADOPTION_NOT_FOUND")
		return
	}
	respond(w, 200, status)
}
