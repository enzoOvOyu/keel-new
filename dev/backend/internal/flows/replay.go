package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ReplayInput struct {
	Mode string     `json:"mode"`
	Run  RunRequest `json:"run"`
}

func replayContext(id, nonce string) string { return "replay:" + id + ":" + nonce }
func (s *Service) ReplayRequest(ctx context.Context, p Scope, id, mode, nonce string) (RunRequest, error) {
	if !contains([]string{"all", "failed"}, mode) || nonce == "" || len(nonce) > 100 {
		return RunRequest{}, problem(400, "REPLAY_INPUT_INVALID")
	}
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return RunRequest{}, e
	}
	state, _ := ex.Engine["state"].(map[string]any)
	if !contains([]string{"SUCCESS", "FAILED", "WARNING", "KILLED", "CANCELLED"}, fmt.Sprint(state["current"])) {
		return RunRequest{}, problem(409, "EXECUTION_NOT_TERMINAL")
	}
	var b []byte
	if e = s.DB.QueryRow(ctx, `SELECT request FROM flow_execution_binding WHERE id::text=$1 AND project_id=ANY($2)`, id, p.Projects).Scan(&b); e != nil {
		return RunRequest{}, e
	}
	var r RunRequest
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	r.Project = ex.Project
	r.Revision = ex.Revision
	if ex.FlowID != nil {
		r.Flow = *ex.FlowID
	} else {
		r.Flow = ""
	}
	if len(ex.Targets) == 0 {
		safe, e := s.ExecutionVariables(ctx, p, id)
		if e != nil {
			return r, e
		}
		r.Inputs = map[string]string{}
		values, _ := safe["inputs"].(map[string]any)
		for key, value := range values {
			if text, ok := value.(string); ok {
				if !strings.HasPrefix(text, "[REDACTED") {
					r.Inputs[key] = text
				}
			} else {
				b, _ := json.Marshal(value)
				r.Inputs[key] = string(b)
			}
		}
	}
	r.Targets = []string{}
	for _, t := range ex.Targets {
		if mode == "all" || t.State == "FAILED" {
			r.Targets = append(r.Targets, t.ID)
		}
	}
	if mode == "failed" && len(r.Targets) == 0 {
		return r, problem(409, "NO_FAILED_TARGETS")
	}
	r.ApprovalID = ""
	r.ApprovalContext = replayContext(id, nonce)
	r.TargetRevisions = nil
	r.Source = "replay"
	r.Parent = id
	delete(r.Inputs, "ticket")
	if NeedsApproval(r) {
		r.TargetRevisions = map[string]int64{}
		decisions, e := s.Preflight(ctx, p, r)
		if e != nil {
			return r, e
		}
		for _, d := range decisions {
			r.TargetRevisions[d.Asset] = d.Revision
		}
	}
	return r, nil
}
func (s *Service) Replay(ctx context.Context, p Scope, id string, input ReplayInput, key string) (Execution, error) {
	if key == "" || len(key) > 100 {
		return Execution{}, problem(400, "IDEMPOTENCY_KEY_REQUIRED")
	}
	frozen, e := s.ReplayRequest(ctx, p, id, input.Mode, key)
	if e != nil {
		return Execution{}, e
	}
	run := input.Run
	if run.Project != frozen.Project || run.Flow != frozen.Flow {
		return Execution{}, problem(403, "REPLAY_SCOPE_CHANGED")
	}
	allowed := map[string]bool{}
	for _, id := range frozen.Targets {
		allowed[id] = true
	}
	if len(frozen.Targets) > 0 && len(run.Targets) == 0 {
		return Execution{}, problem(422, "REPLAY_TARGETS_REQUIRED")
	}
	for _, asset := range run.Targets {
		if !allowed[asset] {
			return Execution{}, problem(403, "REPLAY_TARGET_NOT_IN_ORIGINAL_SCOPE")
		}
	}
	run.ApprovalContext = replayContext(id, key)
	run.internalOperation = true
	run.Source = "replay"
	run.Parent = id
	return s.Start(ctx, p, run, key)
}
