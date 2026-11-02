package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
)

func emptyKickoff(project string) engineering.KickoffHandoff {
	return engineering.KickoffHandoff{ProjectID: project, State: "NEEDS_INPUT", Blockers: []string{}, Publications: []engineering.KickoffPublication{}, EquipmentIDs: []string{}, PortIDs: []string{}, NetworkPlanIDs: []string{}, NextStage: "NETWORK_READINESS", NextStageState: "NOT_STARTED"}
}

func (s *Server) kickoffHandoff(ctx context.Context, q engineeringQuery, w engineering.Workspace) (engineering.KickoffHandoff, error) {
	v := emptyKickoff(s.project)
	v.SourceID = w.ID
	v.Revision = w.Revision
	model, _, err := engineeringModel(ctx, q, *w.HeadRevisionID, s.project)
	if err != nil {
		return v, err
	}
	var pending int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM eng_modeling_input WHERE source_id=$1 AND adopted_revision IS NULL`, w.ID).Scan(&pending); err != nil {
		return v, err
	}
	if pending > 0 {
		v.Blockers = append(v.Blockers, "建模输入存在尚未采用的草稿")
	}
	rows, err := q.Query(ctx, `SELECT t.target,t.state,p.id::text,p.revision_id::text FROM eng_target t LEFT JOIN eng_publication p ON p.id=t.publication_id AND p.state='published' WHERE t.source_id=$1 ORDER BY t.target`, w.ID)
	if err != nil {
		return v, err
	}
	ready := map[string]bool{}
	current := map[string]string{}
	for rows.Next() {
		var target, state string
		var pub, rev *string
		if err = rows.Scan(&target, &state, &pub, &rev); err != nil {
			break
		}
		if state == "PUBLISHED" && pub != nil && rev != nil {
			ready[target] = true
			current[target] = *pub
			v.Publications = append(v.Publications, engineering.KickoffPublication{Target: target, PublicationID: *pub, RevisionID: *rev})
		} else if targetHasContent(model, target) || state == "BLOCKED" || state == "STALE" || state == "IN_REVIEW" {
			v.Blockers = append(v.Blockers, target+"：尚未发布当前有效版本")
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return v, err
	}
	for _, target := range []string{engineering.EquipmentTarget, engineering.FacilityTarget, engineering.PlacementTarget, engineering.ConnectivityTarget} {
		if !ready[target] {
			found := false
			for _, b := range v.Blockers {
				if strings.HasPrefix(b, target+"：") {
					found = true
				}
			}
			if !found {
				v.Blockers = append(v.Blockers, target+"：缺少已发布的开局输入")
			}
		}
	}
	for _, pub := range v.Publications {
		m, _, e := engineeringModel(ctx, q, pub.RevisionID, s.project)
		if e != nil {
			return v, e
		}
		switch pub.Target {
		case engineering.EquipmentTarget:
			for _, x := range m.Equipment {
				if x.IdentityKind != "group" {
					v.EquipmentIDs = append(v.EquipmentIDs, x.EntityID)
				}
			}
		case engineering.ConnectivityTarget:
			for _, x := range m.Ports {
				v.PortIDs = append(v.PortIDs, x.EntityID)
			}
		case engineering.NetworkTarget:
			for _, x := range m.NetworkPlans {
				v.NetworkPlanIDs = append(v.NetworkPlanIDs, x.EntityID)
			}
		}
	}
	if len(v.EquipmentIDs) == 0 {
		v.Blockers = append(v.Blockers, "没有可交接的设备实例")
	}
	if len(v.Blockers) == 0 {
		v.State = "READY"
	}
	var handoffRevision int
	err = q.QueryRow(ctx, `SELECT id::text,revision FROM eng_kickoff_handoff WHERE source_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, w.ID, s.project).Scan(&v.ID, &handoffRevision)
	if err == pgx.ErrNoRows {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	rows, err = q.Query(ctx, `SELECT target,publication_id::text FROM eng_kickoff_handoff_publication WHERE handoff_id=$1`, v.ID)
	if err != nil {
		return v, err
	}
	matched := 0
	stale := len(v.Blockers) > 0
	for rows.Next() {
		var target, pub string
		if err = rows.Scan(&target, &pub); err != nil {
			break
		}
		matched++
		if current[target] != pub {
			stale = true
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return v, err
	}
	if matched != len(current) {
		stale = true
	}
	if stale {
		v.State = "STALE"
	} else {
		v.State = "HANDED_OFF"
	}
	return v, nil
}

func (s *Server) kickoffHandoffRoute(out http.ResponseWriter, r *http.Request, w engineering.Workspace) {
	ctx := r.Context()
	if r.Method == "GET" {
		v, e := s.kickoffHandoff(ctx, s.db, w)
		if e != nil {
			engineeringFailure(out, e)
		} else {
			respond(out, 200, v)
		}
		return
	}
	if r.Method != "POST" {
		fail(out, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var request engineering.KickoffHandoffRequest
	if !engineeringDecode(out, r, &request) {
		return
	}
	if strings.TrimSpace(request.Reason) == "" {
		fail(out, 422, "ENGINEERING_SCHEMA_INVALID")
		return
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		engineeringFailure(out, err)
		return
	}
	defer tx.Rollback(ctx)
	var revision int
	err = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1 AND project_id=$2 FOR UPDATE`, w.ID, s.project).Scan(&revision)
	if err == nil && revision != request.ExpectedRevision {
		err = errors.New("REVISION_CONFLICT")
	}
	if err != nil {
		engineeringFailure(out, err)
		return
	}
	v, err := s.kickoffHandoff(ctx, tx, w)
	if err != nil {
		engineeringFailure(out, err)
		return
	}
	if len(v.Blockers) > 0 {
		fail(out, 422, "ENGINEERING_TARGET_BLOCKED")
		return
	}
	if v.State == "HANDED_OFF" {
		respond(out, 200, v)
		return
	}
	v.ID = uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO eng_kickoff_handoff(id,source_id,project_id,revision,actor,reason) VALUES($1,$2,$3,$4,$5,$6)`, v.ID, w.ID, s.project, revision, subjectOf(ctx).Username, request.Reason); err != nil {
		engineeringFailure(out, err)
		return
	}
	for _, pub := range v.Publications {
		if _, err = tx.Exec(ctx, `INSERT INTO eng_kickoff_handoff_publication(handoff_id,target,publication_id) VALUES($1,$2,$3)`, v.ID, pub.Target, pub.PublicationID); err != nil {
			engineeringFailure(out, err)
			return
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,'engineering.kickoff.handoff',$4,$5)`, uuid.NewString(), s.project, subjectOf(ctx).Username, w.ID, jsonBytes(request))
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		engineeringFailure(out, err)
		return
	}
	v.State = "HANDED_OFF"
	respond(out, 200, v)
}

func (s *Server) projectKickoffRoute(out http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) != 5 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" || p[4] != "engineering-kickoff" {
		return false
	}
	if r.Method != "GET" {
		fail(out, 405, "METHOD_NOT_ALLOWED")
		return true
	}
	if !subjectOf(r.Context()).Can("bom:read", p[3], "", "") {
		fail(out, 403, "FORBIDDEN")
		return true
	}
	scoped := *s
	scoped.project = p[3]
	s = &scoped
	var source string
	err := s.db.QueryRow(r.Context(), `SELECT source_id::text FROM eng_kickoff_handoff WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`, s.project).Scan(&source)
	if err == pgx.ErrNoRows {
		v := emptyKickoff(s.project)
		v.Blockers = append(v.Blockers, "尚未接收智能开局交接")
		respond(out, 200, v)
		return true
	}
	if err != nil {
		engineeringFailure(out, err)
		return true
	}
	w, err := s.engineeringWorkspace(r.Context(), source)
	if err != nil {
		engineeringFailure(out, err)
		return true
	}
	v, err := s.kickoffHandoff(r.Context(), s.db, w)
	if err != nil {
		engineeringFailure(out, err)
	} else {
		respond(out, 200, v)
	}
	return true
}
