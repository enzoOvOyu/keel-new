package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func kickoffLock(ctx context.Context, tx pgx.Tx, project string, expected int) error {
	var version int
	err := tx.QueryRow(ctx, `SELECT revision FROM kickoff_workspace WHERE project_id=$1 FOR UPDATE`, project).Scan(&version)
	if err != nil {
		return err
	}
	if version != expected {
		return errors.New("REVISION_CONFLICT")
	}
	return nil
}
func kickoffFailure(w http.ResponseWriter, err error) {
	if err.Error() == "REVISION_CONFLICT" {
		fail(w, 409, err.Error())
	} else {
		fail(w, 422, err.Error())
	}
}
func loadKickoffCalculations(ctx context.Context, q engineeringQuery, project string) ([]kickoff.Calculation, error) {
	out := []kickoff.Calculation{}
	rows, e := q.Query(ctx, `SELECT id::text,domain,input_digest,created_at,status,rows,power,issues FROM kickoff_calculation WHERE project_id=$1 ORDER BY created_at`, project)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var v kickoff.Calculation
		var a, b, c []byte
		if e = rows.Scan(&v.ID, &v.Domain, &v.InputDigest, &v.CreatedAt, &v.Status, &a, &b, &c); e != nil {
			return nil, e
		}
		for _, pair := range []struct {
			raw    []byte
			target any
		}{{a, &v.Rows}, {b, &v.Power}, {c, &v.Issues}} {
			if e = json.Unmarshal(pair.raw, pair.target); e != nil {
				return nil, e
			}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Server) kickoffCalculate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain           string `json:"domain"`
		ExpectedRevision int    `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	if body.Domain != "cabling" && body.Domain != "power" {
		fail(w, 400, "KICKOFF_CALCULATION_DOMAIN_INVALID")
		return
	}
	ctx := r.Context()
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "KICKOFF_CALCULATION_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	if e = kickoffLock(ctx, tx, s.project, body.ExpectedRevision); e != nil {
		kickoffFailure(w, e)
		return
	}
	model, ev, source, e := s.kickoffModel(ctx, tx)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs, e := loadKickoffInputs(ctx, tx, s.project)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs = kickoff.ResolveReferences(model, inputs)
	result := kickoff.Calculate(kickoff.Project(model, ev, source, inputs), inputs, body.Domain)
	_, e = tx.Exec(ctx, `INSERT INTO kickoff_calculation(id,project_id,domain,workspace_revision,input_digest,status,rows,power,issues,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, result.ID, s.project, body.Domain, body.ExpectedRevision, result.InputDigest, result.Status, jsonBytes(result.Rows), jsonBytes(result.Power), jsonBytes(result.Issues), result.CreatedAt)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1`, s.project)
	}
	if e == nil {
		e = s.kickoffAudit(ctx, tx, "kickoff.calculated", result.ID, map[string]any{"domain": body.Domain, "status": result.Status})
	}
	if e != nil || tx.Commit(ctx) != nil {
		fail(w, 500, "KICKOFF_CALCULATION_UNAVAILABLE")
		return
	}
	respond(w, 201, result)
}
func (s *Server) kickoffResolve(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Value            string `json:"value"`
		ExpectedRevision int    `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	ctx := r.Context()
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "KICKOFF_REVIEW_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	if e = kickoffLock(ctx, tx, s.project, body.ExpectedRevision); e != nil {
		kickoffFailure(w, e)
		return
	}
	model, ev, source, e := s.kickoffModel(ctx, tx)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs, e := loadKickoffInputs(ctx, tx, s.project)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs = kickoff.ResolveReferences(model, inputs)
	view := kickoff.Project(model, ev, source, inputs)
	var issue *kickoff.Issue
	for i := range view.Issues {
		if view.Issues[i].ID == id {
			issue = &view.Issues[i]
			break
		}
	}
	if issue == nil {
		fail(w, 409, "KICKOFF_ISSUE_STALE")
		return
	}
	if issue.Scope == "identity" {
		var published bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE project_id=$1 AND asset_id=$2)`, s.project, issue.EntityID).Scan(&published); e != nil {
			kickoffFailure(w, e)
			return
		}
		if published {
			fail(w, 409, "已入库身份需通过CMDB变更流程核对")
			return
		}
		if e = kickoff.ResolveIdentity(&model, issue.EntityID, body.Value); e != nil {
			kickoffFailure(w, e)
			return
		}
		for _, input := range inputs {
			changed := false
			for _, row := range input.Rows {
				for _, key := range []string{"device_id", "peer", "scope", "reference"} {
					if row[key] == issue.EntityID {
						row[key] = body.Value
						changed = true
					}
				}
			}
			if changed {
				if _, e = tx.Exec(ctx, `INSERT INTO kickoff_input_revision(project_id,table_id,version,workspace_revision,rows,source,actor) VALUES($1,$2,$3,$4,$5,'人工确认身份关联',$6)`, s.project, input.TableID, input.Version+1, body.ExpectedRevision+1, jsonBytes(input.Rows), subjectOf(ctx).ID); e != nil {
					kickoffFailure(w, e)
					return
				}
			}
		}
		for i := range ev {
			if ev[i].EntityID == issue.EntityID {
				ev[i].EntityID = body.Value
			}
		}
	} else {
		found := false
		for i := range model.Placements {
			p := &model.Placements[i]
			if p.EntityID == issue.EntityID || p.EquipmentID == issue.EntityID {
				n, parseErr := strconv.Atoi(body.Value)
				if parseErr != nil || n <= 0 {
					fail(w, 422, "占用高度必须为正整数")
					return
				}
				p.HeightU = &n
				found = true
			}
		}
		if !found {
			fail(w, 422, "请在设备落位表补充缺失的位置与高度")
			return
		}
	}
	var version int
	if e = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1`, source).Scan(&version); e != nil {
		kickoffFailure(w, e)
		return
	}
	_, e = s.saveEngineeringDraftTx(ctx, tx, source, version, "human", subjectOf(ctx).ID, "人工核对落位", engineering.Extraction{Model: model, Evidence: ev}, engineering.EngineeringProposal{Summary: "人工核对落位"})
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO kickoff_issue_decision(project_id,issue_id,workspace_revision,actor,value) VALUES($1,$2,$3,$4,$5)`, s.project, id, body.ExpectedRevision, subjectOf(ctx).ID, body.Value)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1`, s.project)
	}
	if e == nil {
		e = s.kickoffAudit(ctx, tx, "kickoff.issue.resolved", id, map[string]any{"entity_id": issue.EntityID, "scope": issue.Scope})
	}
	if e != nil || tx.Commit(ctx) != nil {
		if e == nil {
			e = errors.New("KICKOFF_REVIEW_UNAVAILABLE")
		}
		kickoffFailure(w, e)
		return
	}
	respond(w, 200, map[string]bool{"resolved": true})
}
func (s *Server) kickoffPublish(w http.ResponseWriter, r *http.Request) {
	if !subjectOf(r.Context()).Can("bom:review", s.project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return
	}
	var body struct {
		DeviceIDs        []string `json:"deviceIds"`
		Reason           string   `json:"reason"`
		ExpectedRevision int      `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		fail(w, 422, "入库需要确认说明")
		return
	}
	ctx := r.Context()
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	var priorID string
	priorErr := tx.QueryRow(ctx, `SELECT id::text FROM kickoff_publication WHERE project_id=$1 AND expected_revision=$2`, s.project, body.ExpectedRevision).Scan(&priorID)
	if priorErr == nil {
		prior, err := s.loadKickoffPublication(ctx, tx, priorID)
		if err != nil {
			kickoffFailure(w, err)
			return
		}
		if engineering.Digest(sortedKickoffIDs(prior.DeviceIDs)) != engineering.Digest(sortedKickoffIDs(body.DeviceIDs)) || prior.Reason != body.Reason {
			fail(w, 409, "IDEMPOTENCY_KEY_REUSED")
			return
		}
		respond(w, 202, prior)
		return
	}
	if priorErr != pgx.ErrNoRows {
		kickoffFailure(w, priorErr)
		return
	}
	if e = kickoffLock(ctx, tx, s.project, body.ExpectedRevision); e != nil {
		kickoffFailure(w, e)
		return
	}
	model, ev, source, e := s.kickoffModel(ctx, tx)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs, e := loadKickoffInputs(ctx, tx, s.project)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs = kickoff.ResolveReferences(model, inputs)
	view := kickoff.Project(model, ev, source, inputs)
	_, p, e := kickoff.Select(model, view, body.DeviceIDs)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	var revision, importID string
	if e = tx.QueryRow(ctx, `SELECT head_revision_id::text,import_id::text FROM eng_source WHERE id=$1`, source).Scan(&revision, &importID); e != nil {
		kickoffFailure(w, e)
		return
	}
	p.ID = uuid.NewString()
	p.Actor = subjectOf(ctx).ID
	p.Reason = body.Reason
	p.CreatedAt = time.Now().UTC()
	p.State = "pending"
	p.ModelSnapshot = inputs
	for _, input := range inputs {
		p.ModelVersions[input.TableID] = input.Version
	}
	if e = tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM kickoff_publication WHERE project_id=$1`, s.project).Scan(&p.Revision); e != nil {
		kickoffFailure(w, e)
		return
	}
	event := uuid.NewString()
	_, e = tx.Exec(ctx, `INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES($1,$2,'kickoff.published',$3)`, event, s.project, jsonBytes(map[string]string{"publication_id": p.ID}))
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO kickoff_publication(id,project_id,revision,expected_revision,model_revision,actor,reason,import_id,outbox_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, p.ID, s.project, p.Revision, body.ExpectedRevision, revision, p.Actor, p.Reason, importID, event, p.CreatedAt)
	}
	for kind, ids := range map[string][]string{"device": p.DeviceIDs, "port": p.PortIDs, "component": p.ComponentIDs, "connection": p.ConnectionIDs, "placement": p.PlacementIDs} {
		for _, id := range ids {
			if e == nil {
				_, e = tx.Exec(ctx, `INSERT INTO kickoff_publication_object VALUES($1,$2,$3)`, p.ID, id, kind)
			}
		}
	}
	for _, input := range inputs {
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO kickoff_publication_input VALUES($1,$2,$3,$4)`, p.ID, s.project, input.TableID, input.Version)
		}
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO eng_review(id,project_id,source_id,target,revision_id,digest,decision,actor,reason) VALUES($1,$2,$3,'KICKOFF_SELECTED_SCOPE',$4,$5,'approve',$6,$7)`, p.ID, s.project, source, revision, engineering.Digest(p.DeviceIDs), p.Actor, p.Reason)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'kickoff.confirmed',$4,$5)`, uuid.NewString(), s.project, p.Actor, p.ID, jsonBytes(p.Counts))
	}
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1`, s.project)
	}
	if e != nil || tx.Commit(ctx) != nil {
		fail(w, 500, "KICKOFF_PUBLICATION_UNAVAILABLE")
		return
	}
	respond(w, 202, p)
}

func sortedKickoffIDs(ids []string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)
	return out
}
