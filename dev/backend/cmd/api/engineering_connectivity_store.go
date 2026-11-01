package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
)

func loadCableRows(ctx context.Context, q engineeringQuery, revision string, m *engineering.PhysicalModel) error {
	var raw []byte
	e := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
 'entity_id',c.entity_id,'revision_id',c.revision_id,'provenance_id',v.provenance_id,'maturity',v.maturity,'attributes','[]'::jsonb,
 'evidence_ids',COALESCE((SELECT jsonb_agg(id) FROM eng_evidence e WHERE e.revision_id=c.revision_id AND e.entity_id=c.entity_id),'[]'::jsonb),
 'connection_id',c.connection_id,'label',c.label,'spec',jsonb_build_object('id',c.spec_id,'medium',c.medium,'connector_a',c.connector_a,'connector_z',c.connector_z,'source_model_label',c.source_model_label,'rate_bps',c.rate_bps),
 'rule_version',CASE WHEN c.rule_id IS NOT NULL THEN jsonb_build_object('id',c.rule_id,'version',c.rule_version) END,
 'route',COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object('from_id',r.from_id,'to_id',r.to_id,'calculation_run_id',r.calculation_run_id,'distance',CASE WHEN r.distance_value IS NOT NULL THEN jsonb_build_object('value',r.distance_value::text,'unit',r.distance_unit) END)) ORDER BY r.position) FROM eng_cable_route r WHERE r.revision_id=c.revision_id AND r.entity_id=c.entity_id),'[]'::jsonb))||
 COALESCE((SELECT jsonb_object_agg(l.kind,jsonb_strip_nulls(jsonb_build_object('length',jsonb_build_object('value',l.value::text,'unit',l.unit),'evidence_ids',l.evidence_ids,'calculation_run_id',l.calculation_run_id))) FROM eng_cable_length l WHERE l.revision_id=c.revision_id AND l.entity_id=c.entity_id),'{}'::jsonb)) ORDER BY c.entity_id),'[]'::jsonb) FROM eng_cable_plan c JOIN eng_entity_version v USING(revision_id,entity_id) WHERE c.revision_id=$1`, revision).Scan(&raw)
	if e != nil {
		return e
	}
	return json.Unmarshal(raw, &m.Cables)
}
func saveConnectivityRows(ctx context.Context, tx pgx.Tx, revision string, m *engineering.PhysicalModel, version func(*engineering.EntityVersion, string) error) error {
	for i := range m.Ports {
		v := &m.Ports[i]
		if e := version(&v.EntityVersion, "port"); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO eng_port VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, revision, v.EntityID, v.EquipmentID, v.Name, v.ParentPortID, v.BreakoutIndex, v.Role, v.RateBPS, v.Medium); e != nil {
			return e
		}
	}
	for i := range m.Connections {
		v := &m.Connections[i]
		if e := version(&v.EntityVersion, "connection"); e != nil {
			return e
		}
		if v.PatchPortIDs == nil {
			v.PatchPortIDs = []string{}
		}
		if _, e := tx.Exec(ctx, `INSERT INTO eng_connection VALUES($1,$2,$3,$4,$5)`, revision, v.EntityID, v.APortID, v.ZPortID, v.PatchPortIDs); e != nil {
			return e
		}
	}
	for i := range m.Cables {
		v := &m.Cables[i]
		if e := version(&v.EntityVersion, "cable_plan"); e != nil {
			return e
		}
		var rule *string
		var n *int
		if v.RuleVersion != nil {
			rule = &v.RuleVersion.ID
			n = &v.RuleVersion.Version
		}
		if _, e := tx.Exec(ctx, `INSERT INTO eng_cable_plan VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, revision, v.EntityID, v.ConnectionID, v.Label, v.Spec.ID, v.Spec.Medium, v.Spec.ConnectorA, v.Spec.ConnectorZ, v.Spec.RateBPS, v.Spec.SourceModelLabel, rule, n); e != nil {
			return e
		}
		for j, r := range v.Route {
			var value, unit *string
			if r.Distance != nil {
				if r.Distance.Validate() != nil {
					return errors.New("ENGINEERING_SCHEMA_INVALID")
				}
				value = &r.Distance.Value
				unit = &r.Distance.Unit
			}
			if _, e := tx.Exec(ctx, `INSERT INTO eng_cable_route VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, revision, v.EntityID, j, r.FromID, r.ToID, value, unit, r.CalculationRunID); e != nil {
				return e
			}
		}
		for kind, l := range map[string]*engineering.LengthEvidence{"estimated": v.Estimated, "calculated": v.Calculated, "selected_stock": v.SelectedStock, "confirmed": v.Confirmed, "as_built": v.AsBuilt} {
			if l == nil {
				continue
			}
			if l.Length.Validate() != nil {
				return errors.New("ENGINEERING_SCHEMA_INVALID")
			}
			if v.Maturity == engineering.Confirmed {
				l.EvidenceIDs = append([]string{}, v.EvidenceIDs...)
			}
			if l.EvidenceIDs == nil {
				l.EvidenceIDs = []string{}
			}
			if _, e := tx.Exec(ctx, `INSERT INTO eng_cable_length VALUES($1,$2,$3,$4,$5,$6,$7)`, revision, v.EntityID, kind, l.Length.Value, l.Length.Unit, l.EvidenceIDs, l.CalculationRunID); e != nil {
				return e
			}
		}
	}
	for i := range m.Optics {
		v := &m.Optics[i]
		if e := version(&v.EntityVersion, "optic_demand"); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO eng_optic_demand VALUES($1,$2,$3,$4,$5)`, revision, v.EntityID, v.PhysicalPortID, v.SourceModelLabel, v.Quantity); e != nil {
			return e
		}
	}
	return nil
}

func (s *Server) editEngineeringConnectivity(ctx context.Context, source string, q engineering.ConnectivityEdit, actor string) error {
	count := 0
	for _, set := range []bool{q.Port != nil, q.Connection != nil, q.Cable != nil, q.Optic != nil} {
		if set {
			count++
		}
	}
	if count != 1 || q.Reason == "" {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	workspace, e := s.engineeringWorkspace(ctx, source)
	if e != nil {
		return e
	}
	if workspace.HeadRevisionID == nil {
		return errors.New("SOURCE_NOT_PARSED")
	}
	if workspace.LatestJob != nil && (workspace.LatestJob.State == "pending" || workspace.LatestJob.State == "running") {
		return errors.New("ENGINEERING_JOB_ACTIVE")
	}
	model, evidence, e := engineeringModel(ctx, s.db, *workspace.HeadRevisionID, s.project)
	if e != nil {
		return e
	}
	if q.Port != nil {
		replaced := false
		for i, v := range model.Ports {
			if v.EntityID == q.Port.EntityID {
				model.Ports[i] = *q.Port
				replaced = true
			}
		}
		if !replaced {
			model.Ports = append(model.Ports, *q.Port)
		}
	}
	if q.Connection != nil {
		replaced := false
		for i, v := range model.Connections {
			if v.EntityID == q.Connection.EntityID {
				model.Connections[i] = *q.Connection
				replaced = true
			}
		}
		if !replaced {
			model.Connections = append(model.Connections, *q.Connection)
		}
	}
	if q.Cable != nil {
		replaced := false
		for i, v := range model.Cables {
			if v.EntityID == q.Cable.EntityID {
				model.Cables[i] = *q.Cable
				replaced = true
			}
		}
		if !replaced {
			model.Cables = append(model.Cables, *q.Cable)
		}
	}
	if q.Optic != nil {
		replaced := false
		for i, v := range model.Optics {
			if v.EntityID == q.Optic.EntityID {
				model.Optics[i] = *q.Optic
				replaced = true
			}
		}
		if !replaced {
			model.Optics = append(model.Optics, *q.Optic)
		}
	}
	var raw []byte
	var proposal engineering.EngineeringProposal
	if e = s.db.QueryRow(ctx, `SELECT proposal FROM eng_revision WHERE id=$1`, *workspace.HeadRevisionID).Scan(&raw); e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &proposal); e != nil {
		return e
	}
	_, e = s.saveEngineeringDraft(ctx, source, q.ExpectedRevision, "human", actor, q.Reason, engineering.Extraction{Model: model, Evidence: evidence, TotalCells: workspace.TotalCells, MappedCells: workspace.MappedCells}, proposal)
	return e
}
