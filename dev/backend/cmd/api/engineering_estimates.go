package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"strings"
)

func loadEstimateRows(ctx context.Context, q engineeringQuery, revision string, m *engineering.PhysicalModel) error {
	for _, entry := range []struct {
		table, properties string
		dest              any
	}{
		{"eng_material_estimate", `'spec_ref',t.spec_ref,'source_model_label',t.source_model_label,'required',jsonb_build_object('value',t.required_value::text,'unit',t.required_unit),
 'purchased',CASE WHEN t.purchased_value IS NOT NULL THEN jsonb_build_object('value',t.purchased_value::text,'unit',t.purchased_unit) END,
 'spares',CASE WHEN t.spares_value IS NOT NULL THEN jsonb_build_object('value',t.spares_value::text,'unit',t.spares_unit) END,
 'purchase_gap',CASE WHEN t.gap_value IS NOT NULL THEN jsonb_build_object('value',t.gap_value::text,'unit',t.gap_unit) END,
 'required_calculation_run_id',t.required_calculation_run_id`, &m.Materials},
		{"eng_workload_estimate", `'work_type',t.work_type,'spec_ref',t.spec_ref,'source_model_label',t.source_model_label,'quantity',jsonb_build_object('value',t.quantity_value::text,'unit',t.quantity_unit),
 'standard_length',CASE WHEN t.length_value IS NOT NULL THEN jsonb_build_object('value',t.length_value::text,'unit',t.length_unit) END,
 'labor_hours',CASE WHEN t.labor_value IS NOT NULL THEN jsonb_build_object('value',t.labor_value::text,'unit',t.labor_unit) END,
 'quantity_calculation_run_id',t.quantity_calculation_run_id`, &m.Workloads},
	} {
		var raw []byte
		sql := `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(jsonb_build_object('entity_id',t.entity_id,'revision_id',t.revision_id,'provenance_id',v.provenance_id,'maturity',v.maturity,'attributes','[]'::jsonb,'evidence_ids',COALESCE((SELECT jsonb_agg(id) FROM eng_evidence e WHERE e.revision_id=t.revision_id AND e.entity_id=t.entity_id),'[]'::jsonb),'calculation_run_id',t.calculation_run_id,` + entry.properties + `)) ORDER BY t.entity_id),'[]'::jsonb) FROM ` + entry.table + ` t JOIN eng_entity_version v USING(revision_id,entity_id) WHERE t.revision_id=$1`
		if e := q.QueryRow(ctx, sql, revision).Scan(&raw); e != nil {
			return e
		}
		if e := json.Unmarshal(raw, entry.dest); e != nil {
			return e
		}
	}
	return nil
}
func saveEstimateRows(ctx context.Context, tx pgx.Tx, revision string, m *engineering.PhysicalModel, version func(*engineering.EntityVersion, string) error) error {
	for i := range m.Materials {
		v := &m.Materials[i]
		for _, q := range []*engineering.Quantity{&v.Required, v.Purchased, v.Spares, v.PurchaseGap} {
			if q != nil && q.Validate() != nil {
				return errors.New("ENGINEERING_SCHEMA_INVALID")
			}
		}
		if e := version(&v.EntityVersion, "material_estimate"); e != nil {
			return e
		}
		pv, pu := quantityColumns(v.Purchased)
		sv, su := quantityColumns(v.Spares)
		gv, gu := quantityColumns(v.PurchaseGap)
		if _, e := tx.Exec(ctx, `INSERT INTO eng_material_estimate VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, revision, v.EntityID, v.SpecRef, v.SourceModelLabel, v.Required.Value, v.Required.Unit, pv, pu, sv, su, gv, gu, v.CalculationRunID, v.RequiredCalculationRunID); e != nil {
			return e
		}
	}
	for i := range m.Workloads {
		v := &m.Workloads[i]
		for _, q := range []*engineering.Quantity{&v.Quantity, v.StandardLength, v.LaborHours} {
			if q != nil && q.Validate() != nil {
				return errors.New("ENGINEERING_SCHEMA_INVALID")
			}
		}
		if e := version(&v.EntityVersion, "workload_estimate"); e != nil {
			return e
		}
		lv, lu := quantityColumns(v.StandardLength)
		hv, hu := quantityColumns(v.LaborHours)
		if _, e := tx.Exec(ctx, `INSERT INTO eng_workload_estimate VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, revision, v.EntityID, v.WorkType, v.SpecRef, v.SourceModelLabel, v.Quantity.Value, v.Quantity.Unit, lv, lu, hv, hu, v.CalculationRunID, v.QuantityCalculationRunID); e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) editEngineeringEstimate(ctx context.Context, source string, q engineering.EstimateEdit, actor string) error {
	if strings.TrimSpace(q.Reason) == "" || (q.Material == nil) == (q.Workload == nil) {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	_, x, p, e := s.engineeringEditContext(ctx, source)
	if e != nil {
		return e
	}
	if q.Material != nil {
		found := false
		for i, v := range x.Model.Materials {
			if v.EntityID == q.Material.EntityID {
				x.Model.Materials[i] = *q.Material
				found = true
			}
		}
		if !found {
			x.Model.Materials = append(x.Model.Materials, *q.Material)
		}
	}
	if q.Workload != nil {
		found := false
		for i, v := range x.Model.Workloads {
			if v.EntityID == q.Workload.EntityID {
				x.Model.Workloads[i] = *q.Workload
				found = true
			}
		}
		if !found {
			x.Model.Workloads = append(x.Model.Workloads, *q.Workload)
		}
	}
	_, e = s.saveEngineeringDraft(ctx, source, q.ExpectedRevision, "human", actor, q.Reason, x, p)
	return e
}
func applyEstimateResult(m *engineering.PhysicalModel, run engineering.CalculationRun, binding engineering.ResultBinding, q engineering.Quantity) bool {
	if binding.Field == "workload_quantity" && run.ProviderID == "engineering-workload-summary" {
		items, _ := engineering.DerivedWorkloads(*m)
		for _, item := range items {
			if item.EntityID != binding.EntityID || binding.OutputName != item.EntityID {
				continue
			}
			item.RevisionID = run.RevisionID
			item.ProvenanceID = run.ID
			item.Quantity = q
			item.QuantityCalculationRunID = &run.ID
			for i, old := range m.Workloads {
				if old.EntityID == item.EntityID {
					item.LaborHours = old.LaborHours
					item.CalculationRunID = old.CalculationRunID
					m.Workloads[i] = item
					return true
				}
			}
			m.Workloads = append(m.Workloads, item)
			return true
		}
		return false
	}
	for i := range m.Materials {
		v := &m.Materials[i]
		if v.EntityID != binding.EntityID {
			continue
		}
		switch binding.Field {
		case "material_required":
			v.Required = q
			v.RequiredCalculationRunID = &run.ID
			return true
		case "material_gap":
			v.PurchaseGap = &q
			v.CalculationRunID = &run.ID
			return true
		}
	}
	for i := range m.Workloads {
		v := &m.Workloads[i]
		if v.EntityID != binding.EntityID {
			continue
		}
		switch binding.Field {
		case "workload_quantity":
			v.Quantity = q
			v.QuantityCalculationRunID = &run.ID
			return true
		case "workload_hours":
			v.LaborHours = &q
			v.CalculationRunID = &run.ID
			return true
		}
	}
	return false
}

func invalidateEstimateInputs(old engineering.PhysicalModel, current *engineering.PhysicalModel) {
	prior := map[string]engineering.MaterialEstimate{}
	for _, v := range old.Materials {
		prior[v.EntityID] = v
	}
	for i := range current.Materials {
		v := &current.Materials[i]
		p, ok := prior[v.EntityID]
		if !ok {
			continue
		}
		if engineering.Digest([]any{p.Required, p.Purchased, p.Spares}) != engineering.Digest([]any{v.Required, v.Purchased, v.Spares}) && engineering.Digest([]any{p.PurchaseGap, p.CalculationRunID}) == engineering.Digest([]any{v.PurchaseGap, v.CalculationRunID}) {
			v.PurchaseGap = nil
			v.CalculationRunID = nil
		}
	}
	work := map[string]engineering.WorkloadEstimate{}
	for _, v := range old.Workloads {
		work[v.EntityID] = v
	}
	for i := range current.Workloads {
		v := &current.Workloads[i]
		p, ok := work[v.EntityID]
		if ok && !engineering.EqualQuantities(v.Quantity, p.Quantity) && engineering.Digest([]any{p.LaborHours, p.CalculationRunID}) == engineering.Digest([]any{v.LaborHours, v.CalculationRunID}) {
			v.LaborHours = nil
			v.CalculationRunID = nil
		}
	}
}
func (s *Server) prepareWorkloadApplication(ctx context.Context, m *engineering.PhysicalModel, run engineering.CalculationRun, bindings []engineering.ResultBinding) error {
	if run.ProviderID != "engineering-workload-summary" {
		return nil
	}
	expected := map[string]bool{}
	for _, v := range run.Outputs {
		expected[v.Name] = true
	}
	if len(bindings) != len(expected) {
		return errors.New("CALC_WORKLOAD_FULL_RESULT_REQUIRED")
	}
	for _, b := range bindings {
		if b.Field != "workload_quantity" || b.OutputName != b.EntityID || !expected[b.OutputName] {
			return errors.New("CALC_OUTPUT_BINDING_MISMATCH")
		}
		delete(expected, b.OutputName)
	}
	kept := []engineering.WorkloadEstimate{}
	for _, v := range m.Workloads {
		generated := false
		if v.QuantityCalculationRunID != nil {
			if e := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_calculation_run WHERE id=$1 AND project_id=$2 AND provider_id='engineering-workload-summary')`, *v.QuantityCalculationRunID, s.project).Scan(&generated); e != nil {
				return e
			}
		}
		if !generated {
			kept = append(kept, v)
		}
	}
	m.Workloads = kept
	return nil
}
func staleWorkloadIssues(ctx context.Context, q engineeringQuery, m engineering.PhysicalModel) ([]engineering.ValidationIssue, error) {
	issues := []engineering.ValidationIssue{}
	items, _ := engineering.DerivedWorkloads(m)
	byID := map[string]engineering.WorkloadEstimate{}
	for _, v := range items {
		byID[v.EntityID] = v
	}
	for _, v := range m.Workloads {
		if v.QuantityCalculationRunID == nil {
			continue
		}
		var builtin bool
		if e := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_calculation_run WHERE id=$1 AND project_id=$2 AND provider_id='engineering-workload-summary')`, *v.QuantityCalculationRunID, m.ProjectID).Scan(&builtin); e != nil {
			return nil, e
		}
		calculated, found := byID[v.EntityID]
		if builtin && (!found || !engineering.EqualQuantities(calculated.Quantity, v.Quantity)) {
			issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(m.ProjectID, "workload-stale", v.EntityID), EntityID: v.EntityID, Code: "WORKLOAD_CALCULATION_STALE", Severity: "BLOCK", ModuleID: "material_workload", Field: "quantity", Message: "工程模型已变化，需重新统计或人工修订工程量", PublishTarget: engineering.WorkloadTarget})
		}
	}
	return issues, nil
}
