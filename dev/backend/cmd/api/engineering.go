package main

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"net/http"
	"strconv"
	"strings"
)

func engineeringDecode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "ENGINEERING_SCHEMA_INVALID")
		return false
	}
	return true
}
func engineeringFailure(w http.ResponseWriter, e error) {
	if e.Error() == "NETWORK_PHYSICAL_REVISION_STALE" || e.Error() == "NETWORK_PUBLICATION_REQUIRED" {
		fail(w, 409, e.Error())
		return
	}
	if strings.HasPrefix(e.Error(), "CALC_") || strings.HasPrefix(e.Error(), "NETWORK_") {
		fail(w, 422, e.Error())
		return
	}
	if errors.Is(e, pgx.ErrNoRows) || e.Error() == "NOT_FOUND" {
		fail(w, 404, "NOT_FOUND")
		return
	}
	code := e.Error()
	switch code {
	case "REVISION_CONFLICT", "ENGINEERING_JOB_ACTIVE", "IDEMPOTENCY_KEY_REUSED", "ENGINEERING_PUBLICATION_PENDING":
		fail(w, 409, code)
	case "PROJECT_SCOPE_MISMATCH":
		fail(w, 403, code)
	case "SOURCE_NOT_PARSED", "ENGINEERING_SCHEMA_INVALID", "ENGINEERING_TARGET_REQUIRED", "ENGINEERING_REVIEW_REQUIRED", "ENGINEERING_TARGET_BLOCKED", "ENGINEERING_DEPENDENCY_UNPUBLISHED", "INVALID_OR_DUPLICATE_ENTITY_ID", "EXTENSIONS_REQUIRE_APPROVED_DEFINITION":
		fail(w, 422, code)
	default:
		fail(w, 500, "ENGINEERING_STORAGE_FAILED")
	}
}
func (s *Server) engineeringRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) < 5 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" || (p[4] != "engineering-imports" && p[4] != "engineering-physical-revisions") {
		return false
	}
	project := p[3]
	permission := "bom:read"
	action := ""
	if len(p) == 7 {
		action = p[6]
	}
	if r.Method != "GET" {
		permission = "bom:edit"
		if len(p) == 5 {
			permission = "bom:import"
		}
		if action == "review" {
			permission = "bom:review"
		}
		if action == "publish" {
			permission = "bom:execute"
		}
	}
	if !subjectOf(r.Context()).Can(permission, project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	scoped := *s
	scoped.project = project
	s = &scoped
	ctx := r.Context()
	if p[4] == "engineering-physical-revisions" {
		if len(p) != 5 || r.Method != "GET" {
			fail(w, 405, "METHOD_NOT_ALLOWED")
			return true
		}
		values, e := s.engineeringPhysicalRevisions(ctx)
		if e != nil {
			engineeringFailure(w, e)
		} else {
			respond(w, 200, values)
		}
		return true
	}

	reply := func(id string, status int) {
		v, e := s.engineeringWorkspace(ctx, id)
		if e != nil {
			engineeringFailure(w, e)
		} else {
			respond(w, status, v)
		}
	}
	if len(p) == 5 {
		switch r.Method {
		case "GET":
			rows, e := s.db.Query(ctx, `SELECT id::text FROM eng_source WHERE project_id=$1 ORDER BY created_at DESC LIMIT 50`, project)
			if e != nil {
				engineeringFailure(w, e)
				return true
			}
			ids := []string{}
			for rows.Next() {
				var id string
				if e = rows.Scan(&id); e != nil {
					break
				}
				ids = append(ids, id)
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
			if e != nil {
				engineeringFailure(w, e)
				return true
			}
			out := []engineering.Workspace{}
			for _, id := range ids {
				v, err := s.engineeringWorkspace(ctx, id)
				if err != nil {
					engineeringFailure(w, err)
					return true
				}
				out = append(out, v)
			}
			respond(w, 200, out)
		case "POST":
			var q struct {
				ImportID string `json:"import_id"`
			}
			if !engineeringDecode(w, r, &q) {
				return true
			}
			var state string
			e := s.db.QueryRow(ctx, `SELECT status FROM bom_import_batch WHERE id::text=$1 AND project_id=$2`, q.ImportID, project).Scan(&state)
			if e != nil {
				engineeringFailure(w, e)
				return true
			}
			if state == "UPLOADED" || state == "PARSING" || state == "FAILED" {
				engineeringFailure(w, errors.New("SOURCE_NOT_PARSED"))
				return true
			}
			id, e := s.attachEngineering(ctx, q.ImportID, subjectOf(ctx).Username)
			if e != nil {
				engineeringFailure(w, e)
				return true
			}
			reply(id, 201)
		default:
			fail(w, 405, "METHOD_NOT_ALLOWED")
		}
		return true
	}
	if len(p) > 7 {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	id := p[5]
	workspace, e := s.engineeringWorkspace(ctx, id)
	if e != nil {
		engineeringFailure(w, e)
		return true
	}
	if len(p) == 6 && r.Method == "GET" {
		respond(w, 200, workspace)
		return true
	}
	if action == "analyze" && r.Method == "POST" {
		var q struct {
			Expected int `json:"expected_revision"`
		}
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if q.Expected != workspace.Revision {
			engineeringFailure(w, errors.New("REVISION_CONFLICT"))
			return true
		}
		tx, e := s.db.Begin(ctx)
		if e != nil {
			engineeringFailure(w, e)
			return true
		}
		defer tx.Rollback(ctx)
		var version int
		if e = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1 FOR UPDATE`, id).Scan(&version); e == nil && version != q.Expected {
			e = errors.New("REVISION_CONFLICT")
		}
		if e == nil {
			var active bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_job WHERE source_id=$1 AND state IN('pending','running'))`, id).Scan(&active)
			if active {
				e = errors.New("ENGINEERING_JOB_ACTIVE")
			}
		}
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO eng_job(id,source_id,project_id,expected_revision,state) VALUES($1,$2,$3,$4,'pending')`, uuid.NewString(), id, project, q.Expected)
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
		if e != nil {
			engineeringFailure(w, e)
		} else {
			reply(id, 202)
		}
		return true
	}
	if action == "network" && r.Method == "PUT" {
		var q engineering.NetworkEdit
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if err := s.editEngineeringNetwork(ctx, id, q, subjectOf(ctx).Username); err != nil {
			engineeringFailure(w, err)
		} else {
			reply(id, 200)
		}
		return true
	}
	if action == "golden-input" && r.Method == "GET" {
		value, err := s.engineeringGoldenInput(ctx, id)
		if err != nil {
			engineeringFailure(w, err)
		} else {
			respond(w, 200, value)
		}
		return true
	}
	if action == "estimates" && r.Method == "PUT" {
		var q engineering.EstimateEdit
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if err := s.editEngineeringEstimate(ctx, id, q, subjectOf(ctx).Username); err != nil {
			engineeringFailure(w, err)
		} else {
			reply(id, 200)
		}
		return true
	}
	if action == "power" && r.Method == "PUT" {
		var q engineering.PowerEdit
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if err := s.editEngineeringPower(ctx, id, q, subjectOf(ctx).Username); err != nil {
			engineeringFailure(w, err)
		} else {
			reply(id, 200)
		}
		return true
	}
	if action == "calculation-application" && r.Method == "POST" {
		var q engineering.CalculationApplication
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if err := s.applyEngineeringCalculation(ctx, id, q, subjectOf(ctx).Username); err != nil {
			calculationFailure(w, err)
		} else {
			reply(id, 200)
		}
		return true
	}
	if action == "connectivity" && r.Method == "PUT" {
		var q engineering.ConnectivityEdit
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if err := s.editEngineeringConnectivity(ctx, id, q, subjectOf(ctx).Username); err != nil {
			engineeringFailure(w, err)
		} else {
			reply(id, 200)
		}
		return true
	}
	if action == "model" && r.Method == "PUT" {
		var q engineering.ModelEdit
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if strings.TrimSpace(q.Reason) == "" || q.Model.ProjectID != project || workspace.HeadRevisionID == nil {
			engineeringFailure(w, errors.New("ENGINEERING_SCHEMA_INVALID"))
			return true
		}
		if workspace.LatestJob != nil && (workspace.LatestJob.State == "pending" || workspace.LatestJob.State == "running") {
			engineeringFailure(w, errors.New("ENGINEERING_JOB_ACTIVE"))
			return true
		}
		_, evidence, err := engineeringModel(ctx, s.db, *workspace.HeadRevisionID, project)
		if err != nil {
			engineeringFailure(w, err)
			return true
		}
		var raw []byte
		var proposal engineering.EngineeringProposal
		if err = s.db.QueryRow(ctx, `SELECT proposal FROM eng_revision WHERE id=$1`, *workspace.HeadRevisionID).Scan(&raw); err == nil {
			err = json.Unmarshal(raw, &proposal)
		}
		if err != nil {
			engineeringFailure(w, err)
			return true
		}
		_, err = s.saveEngineeringDraft(ctx, id, q.ExpectedRevision, "human", subjectOf(ctx).Username, q.Reason, engineering.Extraction{Model: q.Model, Evidence: evidence, TotalCells: workspace.TotalCells, MappedCells: workspace.MappedCells}, proposal)
		if err != nil {
			engineeringFailure(w, err)
		} else {
			reply(id, 200)
		}
		return true
	}
	if (action == "review" || action == "publish") && r.Method == "POST" {
		var q engineering.TargetAction
		if !engineeringDecode(w, r, &q) {
			return true
		}
		if e = s.engineeringAction(r, id, q, action); e != nil {
			engineeringFailure(w, e)
		} else {
			status := 200
			if action == "publish" {
				status = 202
			}
			reply(id, status)
		}
		return true
	}
	if r.Method == "GET" {
		switch action {
		case "model", "diff":
			revision := r.URL.Query().Get("revision_id")
			if revision == "" && workspace.HeadRevisionID != nil {
				revision = *workspace.HeadRevisionID
			}
			var belongs bool
			if e = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_revision WHERE id::text=$1 AND source_id=$2)`, revision, id).Scan(&belongs); e != nil || !belongs {
				fail(w, 404, "NOT_FOUND")
				return true
			}
			m, ev, err := engineeringModel(ctx, s.db, revision, project)
			if err != nil {
				engineeringFailure(w, err)
				return true
			}
			if action == "model" {
				respond(w, 200, map[string]any{"model": m, "evidence": ev})
			} else {
				if workspace.HeadRevisionID == nil {
					fail(w, 404, "NOT_FOUND")
					return true
				}
				current, _, err := engineeringModel(ctx, s.db, *workspace.HeadRevisionID, project)
				if err != nil {
					engineeringFailure(w, err)
				} else {
					respond(w, 200, engineeringDiff(m, current))
				}
			}
			return true
		case "revisions":
			rows, err := s.db.Query(ctx, `SELECT id::text,sequence,actor,origin,reason,created_at FROM eng_revision WHERE source_id=$1 ORDER BY sequence DESC`, id)
			if err != nil {
				engineeringFailure(w, err)
				return true
			}
			defer rows.Close()
			out := []engineering.RevisionEntry{}
			for rows.Next() {
				var v engineering.RevisionEntry
				if e = rows.Scan(&v.ID, &v.Sequence, &v.Actor, &v.Origin, &v.Reason, &v.CreatedAt); e != nil {
					engineeringFailure(w, e)
					return true
				}
				out = append(out, v)
			}
			if e = rows.Err(); e != nil {
				engineeringFailure(w, e)
			} else {
				respond(w, 200, out)
			}
			return true
		case "source":
			var raw []byte
			var wb bom.Workbook
			e = s.db.QueryRow(ctx, `SELECT workbook FROM bom_import_batch WHERE id=$1 AND project_id=$2`, workspace.ImportID, project).Scan(&raw)
			if e == nil {
				e = json.Unmarshal(raw, &wb)
			}
			if e != nil {
				engineeringFailure(w, e)
				return true
			}
			number := func(key string) int { v, _ := strconv.Atoi(r.URL.Query().Get(key)); return v }
			cells, err := engineering.SourceRange(wb, r.URL.Query().Get("sheet"), engineering.Bounds{StartRow: number("start_row"), EndRow: number("end_row"), StartColumn: number("start_column"), EndColumn: number("end_column")})
			if err != nil {
				fail(w, 422, "ENGINEERING_SOURCE_RANGE_INVALID")
			} else {
				respond(w, 200, cells)
			}
			return true
		}
	}
	fail(w, 405, "METHOD_NOT_ALLOWED")
	return true
}
func (s *Server) engineeringAction(r *http.Request, source string, q engineering.TargetAction, action string) error {
	if len(q.Targets) == 0 || strings.TrimSpace(q.Reason) == "" {
		return errors.New("ENGINEERING_TARGET_REQUIRED")
	}
	seen := map[string]bool{}
	for _, t := range q.Targets {
		valid := false
		for _, known := range engineering.PublishTargets {
			if t == known {
				valid = true
			}
		}
		if !valid || seen[t] {
			return errors.New("ENGINEERING_TARGET_REQUIRED")
		}
		seen[t] = true
	}
	if action == "review" && q.Decision != "approve" && q.Decision != "reject" || action == "publish" && q.Decision != "publish" {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	ctx := r.Context()
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var version int
	if e = tx.QueryRow(ctx, `SELECT revision FROM eng_source WHERE id=$1 AND project_id=$2 FOR UPDATE`, source, s.project).Scan(&version); e != nil {
		return e
	}
	if version != q.ExpectedRevision {
		return errors.New("REVISION_CONFLICT")
	}
	if action == "publish" {
		key := r.Header.Get("Idempotency-Key")
		if key == "" || len(key) > 200 {
			return errors.New("ENGINEERING_SCHEMA_INVALID")
		}
		rows, err := tx.Query(ctx, `SELECT source_id::text,target,digest FROM eng_publication WHERE project_id=$1 AND idempotency_key=$2`, s.project, key)
		if err != nil {
			return err
		}
		count := 0
		matched := true
		for rows.Next() {
			var oldSource, target, digest string
			if err = rows.Scan(&oldSource, &target, &digest); err != nil {
				rows.Close()
				return err
			}
			count++
			if oldSource != source || !seen[target] {
				matched = false
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if count > 0 {
			if count != len(q.Targets) || !matched {
				return errors.New("IDEMPOTENCY_KEY_REUSED")
			}
			var changed bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_publication p LEFT JOIN eng_target t ON t.source_id=p.source_id AND t.target=p.target WHERE p.project_id=$1 AND p.idempotency_key=$2 AND (t.digest IS DISTINCT FROM p.digest))`, s.project, key).Scan(&changed)
			if err != nil {
				return err
			}
			if changed {
				return errors.New("IDEMPOTENCY_KEY_REUSED")
			}
			return nil
		}
	}
	for _, target := range q.Targets {
		var state, revision, digest string
		var review, publication *string
		if e = tx.QueryRow(ctx, `SELECT state,revision_id::text,digest,review_id::text,publication_id::text FROM eng_target WHERE source_id=$1 AND target=$2 FOR UPDATE`, source, target).Scan(&state, &revision, &digest, &review, &publication); e != nil {
			return e
		}
		if action == "publish" && publication != nil {
			return errors.New("ENGINEERING_PUBLICATION_PENDING")
		}
		if action == "review" {
			if q.Decision == "approve" {
				var blocked bool
				if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_issue WHERE revision_id=$1 AND severity='BLOCK' AND (target=$2 OR target=''))`, revision, target).Scan(&blocked); e != nil {
					return e
				}
				if blocked || state == "NEEDS_INPUT" {
					return errors.New("ENGINEERING_TARGET_BLOCKED")
				}
			}
			id := uuid.NewString()
			if _, e = tx.Exec(ctx, `INSERT INTO eng_review(id,project_id,source_id,target,revision_id,digest,decision,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, s.project, source, target, revision, digest, q.Decision, subjectOf(ctx).Username, q.Reason); e != nil {
				return e
			}
			next := "BLOCKED"
			if q.Decision == "approve" {
				next = "APPROVED"
			}
			if _, e = tx.Exec(ctx, `UPDATE eng_target SET state=$3,review_id=$4,publication_id=NULL WHERE source_id=$1 AND target=$2`, source, target, next, id); e != nil {
				return e
			}
		} else {
			if state != "APPROVED" || review == nil {
				return errors.New("ENGINEERING_REVIEW_REQUIRED")
			}
			var valid bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_review WHERE id=$1 AND decision='approve' AND revision_id=$2 AND digest=$3 AND project_id=$4)`, *review, revision, digest, s.project).Scan(&valid); e != nil {
				return e
			}
			if !valid {
				return errors.New("ENGINEERING_REVIEW_REQUIRED")
			}
			if dependencies := engineering.TargetDependencies(target); len(dependencies) > 0 {
				for _, dependency := range dependencies {
					if seen[dependency] {
						continue
					}
					var published bool
					if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_target WHERE source_id=$1 AND target=$2 AND state='PUBLISHED')`, source, dependency).Scan(&published); e != nil {
						return e
					}
					if !published {
						return errors.New("ENGINEERING_DEPENDENCY_UNPUBLISHED")
					}
				}
			}
			pub, event := uuid.NewString(), uuid.NewString()
			if _, e = tx.Exec(ctx, `INSERT INTO eng_publication(id,project_id,source_id,target,revision_id,review_id,digest,idempotency_key,outbox_id,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending')`, pub, s.project, source, target, revision, *review, digest, r.Header.Get("Idempotency-Key"), event); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES($1,$2,'engineering.published',$3)`, event, s.project, jsonBytes(map[string]string{"publication_id": pub, "source_id": source, "target": target})); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE eng_target SET publication_id=$3 WHERE source_id=$1 AND target=$2`, source, target, pub); e != nil {
				return e
			}
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), s.project, subjectOf(ctx).Username, "engineering."+action, source, jsonBytes(q)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func engineeringDiff(before, after engineering.PhysicalModel) []engineering.RevisionDifference {
	flatten := func(m engineering.PhysicalModel) map[string]engineering.RevisionDifference {
		out := map[string]engineering.RevisionDifference{}
		add := func(id, kind string, v any) {
			var fields map[string]any
			json.Unmarshal(jsonBytes(v), &fields)
			for _, key := range []string{"revision_id", "provenance_id", "evidence_ids"} {
				delete(fields, key)
			}
			out[id] = engineering.RevisionDifference{EntityID: id, Kind: kind, After: string(jsonBytes(fields))}
		}
		for _, v := range m.Equipment {
			add(v.EntityID, "equipment", v)
		}
		for _, v := range m.Assemblies {
			add(v.EntityID, "assembly", v)
		}
		for _, v := range m.Facilities {
			add(v.EntityID, "facility", v)
		}
		for _, v := range m.Placements {
			add(v.EntityID, "placement", v)
		}
		for _, v := range m.Ports {
			add(v.EntityID, "port", v)
		}
		for _, v := range m.Connections {
			add(v.EntityID, "connection", v)
		}
		for _, v := range m.NetworkPlans {
			add(v.EntityID, "network_plan", v)
		}
		for _, v := range m.Materials {
			add(v.EntityID, "material_estimate", v)
		}
		for _, v := range m.Workloads {
			add(v.EntityID, "workload_estimate", v)
		}
		for _, v := range m.PowerPlans {
			add(v.EntityID, "power_plan", v)
		}
		for _, v := range m.Cables {
			add(v.EntityID, "cable", v)
		}
		for _, v := range m.Optics {
			add(v.EntityID, "optic", v)
		}
		return out
	}
	a, b := flatten(before), flatten(after)
	out := []engineering.RevisionDifference{}
	for id, v := range b {
		if old, ok := a[id]; !ok {
			v.Change = "added"
			out = append(out, v)
		} else if old.After != v.After {
			v.Change = "changed"
			v.Before = old.After
			out = append(out, v)
		}
	}
	for id, v := range a {
		if _, ok := b[id]; !ok {
			v.Change = "removed"
			v.Before = v.After
			v.After = ""
			out = append(out, v)
		}
	}
	return out
}
