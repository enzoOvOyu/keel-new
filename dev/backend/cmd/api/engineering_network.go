package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"strings"
)

func (s *Server) engineeringPhysicalRevisions(ctx context.Context) ([]engineering.PhysicalRevisionOption, error) {
	values := []engineering.PhysicalRevisionOption{}
	rows, e := s.db.Query(ctx, `SELECT p.revision_id::text,p.source_id::text,p.id::text,p.target,b.filename,p.published_at FROM eng_publication p JOIN eng_target t ON t.publication_id=p.id JOIN eng_source s ON s.id=p.source_id JOIN bom_import_batch b ON b.id=s.import_id WHERE p.project_id=$1 AND p.target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_CONNECTIVITY') AND p.state='published' AND t.state='PUBLISHED' ORDER BY p.published_at DESC LIMIT 100`, s.project)
	if e != nil {
		return values, e
	}
	defer rows.Close()
	for rows.Next() {
		var v engineering.PhysicalRevisionOption
		if e = rows.Scan(&v.RevisionID, &v.SourceID, &v.PublicationID, &v.Target, &v.Filename, &v.PublishedAt); e != nil {
			return values, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func networkPhysicalContext(ctx context.Context, q engineeringQuery, project string, p engineering.NetworkPlan) (engineering.PhysicalModel, error) {
	if p.PhysicalRevisionID == "" {
		return engineering.EmptyPhysical(project), nil
	}
	m, _, e := engineeringModel(ctx, q, p.PhysicalRevisionID, project)
	if e != nil {
		return m, errors.New("NETWORK_PHYSICAL_REFERENCE_UNAVAILABLE")
	}
	return m, nil
}
func networkPublicationReady(ctx context.Context, q engineeringQuery, project string, p engineering.NetworkPlan) bool {
	target := engineering.EquipmentTarget
	if len(p.InterfaceMappings) > 0 {
		target = engineering.ConnectivityTarget
	}
	var valid bool
	e := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_publication p JOIN eng_target t ON t.publication_id=p.id WHERE p.project_id=$1 AND p.revision_id::text=$2 AND p.target=$3 AND p.state='published' AND t.state='PUBLISHED')`, project, p.PhysicalRevisionID, target).Scan(&valid)
	return e == nil && valid
}
func validateNetworkDomain(ctx context.Context, q engineeringQuery, project string, m engineering.PhysicalModel) ([]engineering.ValidationIssue, error) {
	issues := []engineering.ValidationIssue{}
	if len(m.NetworkPlans) > 1 {
		return nil, errors.New("NETWORK_SINGLE_PLAN_REQUIRED")
	}
	for _, p := range m.NetworkPlans {
		physical, e := networkPhysicalContext(ctx, q, project, p)
		if e != nil {
			return nil, e
		}
		issues = append(issues, engineering.ValidateNetwork(p, physical)...)
		if !engineering.NetworkHasContent(p) {
			continue
		}
		if !networkPublicationReady(ctx, q, project, p) {
			issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(project, "network-physical-state", p.EntityID), EntityID: p.EntityID, Code: "NETWORK_PHYSICAL_NOT_PUBLISHED", Severity: "BLOCK", ModuleID: "network_design", Field: "physical_revision_id", Message: "需要当前有效的已发布设备/端口版本；不以物理草案代替", PublishTarget: engineering.NetworkTarget})
		}
		for _, source := range p.SourceIDs {
			var exists bool
			if e = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_source WHERE id::text=$1 AND project_id=$2)`, source, project).Scan(&exists); e != nil {
				return nil, e
			}
			if !exists {
				issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(project, "network-source", p.EntityID+source), EntityID: p.EntityID, Code: "NETWORK_SOURCE_UNRESOLVED", Severity: "BLOCK", ModuleID: "network_design", Field: "source_ids", Message: "网络资料引用必须属于当前项目", PublishTarget: engineering.NetworkTarget})
			}
		}
	}
	return issues, nil
}
func (s *Server) editEngineeringNetwork(ctx context.Context, source string, q engineering.NetworkEdit, actor string) error {
	if strings.TrimSpace(q.Reason) == "" {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	_, x, p, e := s.engineeringEditContext(ctx, source)
	if e != nil {
		return e
	}
	x.Model.NetworkPlans = []engineering.NetworkPlan{q.Plan}
	_, e = s.saveEngineeringDraft(ctx, source, q.ExpectedRevision, "human", actor, q.Reason, x, p)
	return e
}
func (s *Server) engineeringGoldenInput(ctx context.Context, source string) (engineering.GoldenInput, error) {
	value := engineering.GoldenInput{}
	e := s.db.QueryRow(ctx, `SELECT p.id::text,p.revision_id::text FROM eng_publication p JOIN eng_target t ON t.publication_id=p.id WHERE p.source_id=$1 AND p.project_id=$2 AND p.target='NETWORK_REVISION' AND p.state='published' AND t.state='PUBLISHED' ORDER BY p.published_at DESC LIMIT 1`, source, s.project).Scan(&value.PublicationID, &value.RevisionID)
	if errors.Is(e, pgx.ErrNoRows) {
		return value, errors.New("NETWORK_PUBLICATION_REQUIRED")
	}
	if e != nil {
		return value, e
	}
	m, _, e := engineeringModel(ctx, s.db, value.RevisionID, s.project)
	if e != nil {
		return value, e
	}
	if len(m.NetworkPlans) != 1 {
		return value, errors.New("NETWORK_PLAN_REQUIRED")
	}
	value.Plan = m.NetworkPlans[0]
	value.PhysicalRevisionID = value.Plan.PhysicalRevisionID
	if !networkPublicationReady(ctx, s.db, s.project, value.Plan) {
		return value, errors.New("NETWORK_PHYSICAL_REVISION_STALE")
	}
	return value, nil
}
