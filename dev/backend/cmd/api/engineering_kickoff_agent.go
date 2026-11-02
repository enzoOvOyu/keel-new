package main

import (
	"context"
	"fmt"
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"strings"
)

// Agent proposals only enter the existing editable staging store. Existing
// human/adopted inputs are preserved; adoption still uses normal domain gates.
func (s *Server) saveKickoffCandidates(ctx context.Context, source string, expected int, job, claim string, result ai.KickoffResult) error {
	w, err := s.engineeringWorkspace(ctx, source)
	if err != nil {
		return err
	}
	if w.Revision != expected {
		return fmt.Errorf("REVISION_CONFLICT")
	}
	current, err := s.modelingInputs(ctx, w, false)
	if err != nil {
		return err
	}
	edits := []engineering.ModelingInputEdit{}
	for _, draft := range result.Drafts {
		exists := false
		inputVersion := 0
		for _, input := range current {
			if input.TableID == draft.TableID {
				if input.Actor == "kickoff-agent" && input.State == "draft" {
					inputVersion = input.Version
				} else {
					exists = true
				}
				break
			}
		}
		if exists {
			result.Missing = append(result.Missing, "已有输入保持不变，新候选供差异核对："+draft.TableID)
			continue
		}
		edits = append(edits, engineering.ModelingInputEdit{ExpectedRevision: expected, InputVersion: inputVersion, InputPreviewRequest: engineering.InputPreviewRequest{TableID: draft.TableID, CSV: draft.CSV}, Reason: "Agent固定格式候选 · " + job})
	}
	if len(edits) > 0 {
		if err = s.saveModelingInputs(ctx, w, edits, "kickoff-agent"); err != nil {
			return err
		}
	}
	// The report is immutable and project-scoped; do not embed full candidates in
	// checkpoints or overwrite the current published engineering revision.
	if err = s.saveAnalysisArtifact(ctx, job, claim, "kickoff_result", "succeeded", result); err != nil {
		return err
	}
	for _, message := range result.Missing {
		if _, err = s.db.Exec(ctx, `INSERT INTO eng_step(job_id,sequence,tool,detail,state) SELECT $1,COALESCE(max(sequence),0)+1,'emit_missing_input',$2,'partial' FROM eng_step WHERE job_id=$1`, job, message); err != nil {
			return err
		}
	}
	summary := fmt.Sprintf("分步Agent已生成%d份固定格式候选，%d项资料待补充；请在资料与建模核对采用。", len(edits), len(result.Missing))
	tag, err := s.db.Exec(ctx, `UPDATE eng_source SET summary=$3 WHERE id=$1 AND project_id=$2 AND revision=$4`, source, s.project, strings.TrimSpace(summary), expected)
	if err == nil && tag.RowsAffected() != 1 {
		return fmt.Errorf("REVISION_CONFLICT")
	}
	return err
}
