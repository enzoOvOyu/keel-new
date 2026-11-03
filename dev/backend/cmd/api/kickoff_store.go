package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"time"
)

func (s *Server) kickoffModel(ctx context.Context, q engineeringQuery) (engineering.PhysicalModel, []engineering.SourceEvidence, string, error) {
	var source, revision *string
	err := q.QueryRow(ctx, `SELECT w.source_id::text,s.head_revision_id::text FROM kickoff_workspace w LEFT JOIN eng_source s ON s.id=w.source_id WHERE w.project_id=$1`, s.project).Scan(&source, &revision)
	if err != nil {
		return engineering.EmptyPhysical(s.project), nil, "", err
	}
	if source == nil || revision == nil {
		return engineering.EmptyPhysical(s.project), nil, "", nil
	}
	model, evidence, err := engineeringModel(ctx, q, *revision, s.project)
	return model, evidence, *source, err
}
func loadKickoffInputs(ctx context.Context, q engineeringQuery, project string) ([]kickoff.ModelInput, error) {
	values := []kickoff.ModelInput{}
	rows, err := q.Query(ctx, `SELECT DISTINCT ON(table_id) table_id,rows,version,created_at,source FROM kickoff_input_revision WHERE project_id=$1 ORDER BY table_id,version DESC`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var value kickoff.ModelInput
		var raw []byte
		if err = rows.Scan(&value.TableID, &raw, &value.Version, &value.UpdatedAt, &value.Source); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &value.Rows); err != nil {
			return nil, err
		}
		value.Confirmed = true
		values = append(values, value)
	}
	return values, rows.Err()
}

// Establishes an immutable source anchor in the shared engineering repository.
// The workspace owns its orchestration; no old step adoption or handoff is used.
func (s *Server) ensureKickoffDomain(ctx context.Context, tx pgx.Tx, actor string) (string, error) {
	var existing *string
	if err := tx.QueryRow(ctx, `SELECT source_id::text FROM kickoff_workspace WHERE project_id=$1 FOR UPDATE`, s.project).Scan(&existing); err != nil {
		return "", err
	}
	if existing != nil {
		return *existing, nil
	}
	var id, name, key, hash string
	err := tx.QueryRow(ctx, `SELECT id::text,filename,object_key,sha256 FROM kickoff_source WHERE project_id=$1 AND detached_at IS NULL ORDER BY created_at LIMIT 1`, s.project).Scan(&id, &name, &key, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,import_mode) VALUES($1,$2,$1,$3,$4,$5,'uploaded','engineering')`, id, s.project, key, name, hash); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO eng_source(id,project_id,import_id) VALUES($1,$2,$1)`, id, s.project); err != nil {
		return "", err
	}
	revision := uuid.NewString()
	proposal := engineering.EngineeringProposal{Summary: "智能开局资料锚点", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	if _, err = tx.Exec(ctx, `INSERT INTO eng_revision(id,source_id,sequence,actor,origin,reason,total_cells,mapped_cells,proposal) VALUES($1,$2,1,$3,'human','source attachment',0,0,$4)`, revision, id, actor, jsonBytes(proposal)); err != nil {
		return "", err
	}
	if err = saveEngineeringIntelligence(ctx, tx, s.project, id, revision, 1, proposal, engineering.Extraction{Model: engineering.EmptyPhysical(s.project)}, nil); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE eng_source SET head_revision_id=$2 WHERE id=$1`, id, revision); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE kickoff_workspace SET source_id=$2 WHERE project_id=$1`, s.project, id)
	return id, err
}
func (s *Server) applyKickoffInputs(ctx context.Context, tx pgx.Tx, actor, reason string) error {
	source, err := s.ensureKickoffDomain(ctx, tx, actor)
	if err != nil || source == "" {
		return err
	}
	model, evidence, _, err := s.kickoffModel(ctx, tx)
	if err != nil {
		return err
	}
	inputs, err := loadKickoffInputs(ctx, tx, s.project)
	if err != nil {
		return err
	}
	before := engineering.Digest(model)
	inputs = kickoff.ResolveReferences(model, inputs)
	if err = kickoff.ApplyInputs(&model, inputs); err != nil {
		return err
	}
	if before == engineering.Digest(model) {
		return nil
	}
	var version int
	if err = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1`, source).Scan(&version); err != nil {
		return err
	}
	proposal := engineering.EngineeringProposal{}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT r.proposal FROM eng_revision r JOIN eng_source s ON s.head_revision_id=r.id WHERE s.id=$1`, source).Scan(&raw); err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &proposal); err != nil {
		return err
	}
	proposal.Summary = reason
	_, err = s.saveEngineeringDraftTx(ctx, tx, source, version, "human", actor, reason, engineering.Extraction{Model: model, Evidence: evidence}, proposal)
	return err
}

type kickoffActivity struct {
	ID     string    `json:"id"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
	At     time.Time `json:"at"`
}

func loadKickoffActivity(ctx context.Context, q engineeringQuery, project string) ([]kickoffActivity, error) {
	out := []kickoffActivity{}
	rows, e := q.Query(ctx, `SELECT id,action,detail,at FROM (
 SELECT 'input:'||table_id||':'||version::text id,'更新工程输入' action,table_id||' · v'||version::text detail,created_at at FROM kickoff_input_revision WHERE project_id=$1
 UNION ALL SELECT 'source:'||id::text,'接入资料',filename,created_at FROM kickoff_source WHERE project_id=$1 AND detached_at IS NULL
 UNION ALL SELECT 'calculation:'||id::text,'完成确定性计算',domain||' · '||status,created_at FROM kickoff_calculation WHERE project_id=$1
 UNION ALL SELECT 'publication:'||id::text,'确认入库范围','r'||revision::text||' · '||state,created_at FROM kickoff_publication WHERE project_id=$1
 ) events ORDER BY at DESC,id LIMIT 30`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var v kickoffActivity
		if e = rows.Scan(&v.ID, &v.Action, &v.Detail, &v.At); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Server) kickoffAudit(ctx context.Context, tx pgx.Tx, action, resource string, details any) error {
	_, e := tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), s.project, subjectOf(ctx).ID, action, resource, jsonBytes(details))
	return e
}
