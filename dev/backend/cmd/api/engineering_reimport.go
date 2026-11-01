package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"strings"
)

func copyEngineeringTarget(dst *engineering.PhysicalModel, src engineering.PhysicalModel, target string) {
	switch target {
	case engineering.EquipmentTarget:
		dst.Equipment = src.Equipment
		dst.Assemblies = src.Assemblies
	case engineering.FacilityTarget:
		dst.Facilities = src.Facilities
	case engineering.PlacementTarget:
		dst.Placements = src.Placements
	case engineering.ConnectivityTarget:
		dst.Ports = src.Ports
		dst.Connections = src.Connections
	case engineering.CablingTarget:
		dst.Cables = src.Cables
		dst.Optics = src.Optics
	case engineering.PowerTarget:
		dst.PowerPlans = src.PowerPlans
	case engineering.MaterialTarget:
		dst.Materials = src.Materials
	case engineering.WorkloadTarget:
		dst.Workloads = src.Workloads
	case engineering.NetworkTarget:
		dst.NetworkPlans = src.NetworkPlans
	}
}
func ownTargetDigest(model engineering.PhysicalModel, target string) string {
	own := engineering.EmptyPhysical(model.ProjectID)
	copyEngineeringTarget(&own, model, target)
	return suggestionDigest(own, target)
}
func (s *Server) reimportEngineering(ctx context.Context, w engineering.Workspace, q engineering.ReimportRequest, actor string) (string, error) {
	if w.Revision != q.ExpectedRevision || w.HeadRevisionID == nil {
		return "", errors.New("REVISION_CONFLICT")
	}
	if q.ImportID == w.ImportID {
		return "", errors.New("REIMPORT_NEW_SOURCE_REQUIRED")
	}
	var status string
	if e := s.db.QueryRow(ctx, `SELECT status FROM bom_import_batch WHERE id::text=$1 AND project_id=$2`, q.ImportID, s.project).Scan(&status); e != nil {
		return "", e
	}
	if status == "UPLOADED" || status == "PARSING" || status == "FAILED" {
		return "", errors.New("SOURCE_NOT_PARSED")
	}
	var bound *string
	if e := s.db.QueryRow(ctx, `SELECT base_revision_id::text FROM eng_source WHERE import_id=$1`, q.ImportID).Scan(&bound); e == nil && (bound == nil || *bound != *w.HeadRevisionID) {
		return "", errors.New("REIMPORT_SOURCE_ALREADY_BOUND")
	} else if e != nil && e != pgx.ErrNoRows {
		return "", e
	}
	return s.attachEngineering(ctx, q.ImportID, actor, *w.HeadRevisionID)
}
func (s *Server) reviewEngineeringDifferences(ctx context.Context, source string, q engineering.DifferenceReview, actor string) error {
	if strings.TrimSpace(q.Reason) == "" || len(q.Resolutions) == 0 {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	_, x, p, e := s.engineeringEditContext(ctx, source)
	if e != nil {
		return e
	}
	x.Resolutions = q.Resolutions
	_, e = s.saveEngineeringDraft(ctx, source, q.ExpectedRevision, "human", actor, q.Reason, x, p)
	return e
}
func reimportBase(ctx context.Context, q engineeringQuery, project, source string) (*string, engineering.PhysicalModel, []engineering.SourceEvidence, error) {
	var revision *string
	if e := q.QueryRow(ctx, `SELECT base_revision_id::text FROM eng_source WHERE id=$1 AND project_id=$2`, source, project).Scan(&revision); e != nil {
		return nil, engineering.EmptyPhysical(project), nil, e
	}
	if revision == nil {
		return nil, engineering.EmptyPhysical(project), nil, nil
	}
	m, ev, e := engineeringModel(ctx, q, *revision, project)
	return revision, m, ev, e
}
func applyDifferenceResolutions(ctx context.Context, tx pgx.Tx, project, source, revision, actor, reason, origin string, x *engineering.Extraction) error {
	if len(x.Resolutions) == 0 {
		return nil
	}
	if origin != "human" {
		return errors.New("ENGINEERING_REVIEW_REQUIRED")
	}
	baseID, base, evidence, e := reimportBase(ctx, tx, project, source)
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, resolution := range x.Resolutions {
		valid := false
		for _, target := range engineering.PublishTargets {
			if target == resolution.Target {
				valid = true
			}
		}
		if !valid || seen[resolution.Target] {
			return errors.New("ENGINEERING_TARGET_REQUIRED")
		}
		seen[resolution.Target] = true
		if resolution.Decision != "retain_base" && resolution.Decision != "accept_incoming" && resolution.Decision != "retain_current" {
			return errors.New("ENGINEERING_SCHEMA_INVALID")
		}
		if baseID == nil && resolution.Decision != "retain_current" {
			return errors.New("REIMPORT_BASE_REQUIRED")
		}
		incoming, previous := ownTargetDigest(x.Model, resolution.Target), ownTargetDigest(base, resolution.Target)
		if resolution.Decision == "retain_base" {
			copyEngineeringTarget(&x.Model, base, resolution.Target)
			selected := engineering.EmptyPhysical(project)
			copyEngineeringTarget(&selected, base, resolution.Target)
			ids := map[string]bool{}
			for _, d := range engineeringDiff(engineering.EmptyPhysical(project), selected) {
				ids[d.EntityID] = true
			}
			retained := []engineering.SourceEvidence{}
			for _, ev := range x.Evidence {
				if !ids[ev.EntityID] {
					retained = append(retained, ev)
				}
			}
			for _, ev := range evidence {
				if ids[ev.EntityID] {
					retained = append(retained, ev)
				}
			}
			x.Evidence = retained
		}
		if _, e = tx.Exec(ctx, `INSERT INTO eng_difference_review(id,project_id,source_id,revision_id,base_revision_id,target,incoming_digest,base_digest,decision,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, uuid.NewString(), project, source, revision, baseID, resolution.Target, incoming, previous, resolution.Decision, actor, reason); e != nil {
			return e
		}
	}
	kept := []engineering.ValidationIssue{}
	for _, issue := range x.Issues {
		if issue.Code == "REANALYSIS_REVIEWED_CONFLICT" && seen[issue.PublishTarget] {
			continue
		}
		kept = append(kept, issue)
	}
	x.Issues = kept
	return nil
}
func reimportValidationIssues(ctx context.Context, q engineeringQuery, project, source string, model engineering.PhysicalModel) ([]engineering.ValidationIssue, error) {
	issues := []engineering.ValidationIssue{}
	baseID, base, _, e := reimportBase(ctx, q, project, source)
	if e != nil || baseID == nil {
		return issues, e
	}
	var head string
	if e = q.QueryRow(ctx, `SELECT b.head_revision_id::text FROM eng_source s JOIN eng_source b ON b.id=s.base_source_id WHERE s.id=$1`, source).Scan(&head); e != nil {
		return issues, e
	}
	latest := base
	if head != *baseID {
		latest, _, e = engineeringModel(ctx, q, head, project)
		if e != nil {
			return issues, e
		}
	}
	for _, target := range engineering.PublishTargets {
		if ownTargetDigest(latest, target) != ownTargetDigest(base, target) {
			issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(source, "reimport-base-stale", target), EntityID: source, ProjectID: project, ModuleID: "validation_release", Code: "REIMPORT_BASE_STALE", Severity: "BLOCK", Field: "revision", Message: "基准导入已有人修改此工程域，请基于其最新版本重新导入，不能覆盖后来确认的数据", PublishTarget: target, EvidenceIDs: []string{}})
			continue
		}
		incoming, previous := ownTargetDigest(model, target), ownTargetDigest(base, target)
		if incoming == previous {
			continue
		}
		var decided bool
		if e = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_difference_review WHERE source_id=$1 AND target=$2 AND base_revision_id=$3 AND incoming_digest=$4 AND base_digest=$5 AND decision IN('accept_incoming','retain_current'))`, source, target, *baseID, incoming, previous).Scan(&decided); e != nil {
			return issues, e
		}
		if !decided {
			issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(source, "reimport-difference", target), EntityID: source, ProjectID: project, ModuleID: "validation_release", Code: "REIMPORT_DIFFERENCE_REVIEW_REQUIRED", Severity: "BLOCK", Field: "revision", Message: "新版资料与基准版本不同，请在版本差异中决定采用新版或保留基准；批准发布前必须处理", PublishTarget: target, EvidenceIDs: []string{}})
		}
	}
	return issues, nil
}
