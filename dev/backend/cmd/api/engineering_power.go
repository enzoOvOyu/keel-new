package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"sort"
	"strings"
)

func loadPowerRows(ctx context.Context, q engineeringQuery, revision string, m *engineering.PhysicalModel) error {
	var raw []byte
	e := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('scope',CASE WHEN p.scenario IS NOT NULL THEN jsonb_build_object('scenario',p.scenario,'basis',p.basis,'demand_rule',jsonb_build_object('id',p.demand_rule_id,'version',p.demand_rule_version),'redundancy_mode',p.redundancy_mode) END,'entity_id',p.entity_id,'revision_id',p.revision_id,'provenance_id',v.provenance_id,'maturity',v.maturity,'attributes','[]'::jsonb,
 'evidence_ids',COALESCE((SELECT jsonb_agg(id) FROM eng_evidence e WHERE e.revision_id=p.revision_id AND e.entity_id=p.entity_id),'[]'::jsonb),
 'equipment_profiles',COALESCE((SELECT jsonb_agg(jsonb_build_object('equipment_id',e.equipment_id,'basis',e.basis,'power',jsonb_build_object('value',e.power_value::text,'unit',e.power_unit),'cooling_type',e.cooling_type,'evidence_ids',e.evidence_ids) ORDER BY e.equipment_id,e.basis) FROM eng_equipment_power e WHERE e.revision_id=p.revision_id AND e.entity_id=p.entity_id),'[]'::jsonb),
 'racks',COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object('rack_id',r.rack_id,'cooling_type',r.cooling_type,'capacity_basis',r.capacity_basis,
 'capacity',CASE WHEN r.capacity_value IS NOT NULL THEN jsonb_build_object('value',r.capacity_value::text,'unit',r.capacity_unit) END,
 'cooling_capacity',CASE WHEN r.cooling_value IS NOT NULL THEN jsonb_build_object('value',r.cooling_value::text,'unit',r.cooling_unit) END,
 'planned_load',CASE WHEN r.load_value IS NOT NULL THEN jsonb_build_object('value',r.load_value::text,'unit',r.load_unit) END,
 'remaining',CASE WHEN r.remaining_value IS NOT NULL THEN jsonb_build_object('value',r.remaining_value::text,'unit',r.remaining_unit) END,'calculation_run_id',r.calculation_run_id)) ORDER BY r.rack_id) FROM eng_rack_power r WHERE r.revision_id=p.revision_id AND r.entity_id=p.entity_id),'[]'::jsonb)) ORDER BY p.entity_id),'[]'::jsonb) FROM eng_power_plan p JOIN eng_entity_version v USING(revision_id,entity_id) WHERE p.revision_id=$1`, revision).Scan(&raw)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &m.PowerPlans); e != nil {
		return e
	}
	for i := range m.PowerPlans {
		p := &m.PowerPlans[i]
		if e = loadPowerSupplyRows(ctx, q, revision, p); e != nil {
			return e
		}
	}
	return nil
}
func quantityColumns(q *engineering.Quantity) (*string, *string) {
	if q == nil {
		return nil, nil
	}
	return &q.Value, &q.Unit
}
func savePowerRows(ctx context.Context, tx pgx.Tx, revision string, m *engineering.PhysicalModel, version func(*engineering.EntityVersion, string) error) error {
	for i := range m.PowerPlans {
		p := &m.PowerPlans[i]
		if e := version(&p.EntityVersion, "power_plan"); e != nil {
			return e
		}
		var scenario, basis, rule, mode *string
		var versionNumber *int
		if p.Scope != nil {
			scenario = &p.Scope.Scenario
			basis = &p.Scope.Basis
			rule = &p.Scope.DemandRule.ID
			versionNumber = &p.Scope.DemandRule.Version
			mode = &p.Scope.RedundancyMode
		}
		if _, e := tx.Exec(ctx, `INSERT INTO eng_power_plan(revision_id,entity_id,scenario,basis,demand_rule_id,demand_rule_version,redundancy_mode) VALUES($1,$2,$3,$4,$5,$6,$7)`, revision, p.EntityID, scenario, basis, rule, versionNumber, mode); e != nil {
			return e
		}
		for _, v := range p.EquipmentProfiles {
			if v.Power.Validate() != nil {
				return errors.New("ENGINEERING_SCHEMA_INVALID")
			}
			evidence := v.EvidenceIDs
			if p.Maturity == engineering.Confirmed {
				evidence = p.EvidenceIDs
			}
			if evidence == nil {
				evidence = []string{}
			}
			if _, e := tx.Exec(ctx, `INSERT INTO eng_equipment_power VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, revision, p.EntityID, v.EquipmentID, v.Basis, v.Power.Value, v.Power.Unit, v.CoolingType, evidence); e != nil {
				return e
			}
		}
		for _, v := range p.Racks {
			for _, q := range []*engineering.Quantity{v.Capacity, v.CoolingCapacity, v.PlannedLoad, v.Remaining} {
				if q != nil && q.Validate() != nil {
					return errors.New("ENGINEERING_SCHEMA_INVALID")
				}
			}
			cv, cu := quantityColumns(v.Capacity)
			tv, tu := quantityColumns(v.CoolingCapacity)
			lv, lu := quantityColumns(v.PlannedLoad)
			rv, ru := quantityColumns(v.Remaining)
			if _, e := tx.Exec(ctx, `INSERT INTO eng_rack_power(revision_id,entity_id,rack_id,capacity_value,capacity_unit,cooling_value,cooling_unit,cooling_type,load_value,load_unit,remaining_value,remaining_unit,calculation_run_id,capacity_basis) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, revision, p.EntityID, v.RackID, cv, cu, tv, tu, v.CoolingType, lv, lu, rv, ru, v.CalculationRunID, v.CapacityBasis); e != nil {
				return e
			}
		}
		if e := savePowerSupplyRows(ctx, tx, revision, *p); e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) engineeringEditContext(ctx context.Context, source string) (engineering.Workspace, engineering.Extraction, engineering.EngineeringProposal, error) {
	w, e := s.engineeringWorkspace(ctx, source)
	x := engineering.Extraction{}
	p := engineering.EngineeringProposal{}
	if e != nil {
		return w, x, p, e
	}
	if w.HeadRevisionID == nil {
		return w, x, p, errors.New("SOURCE_NOT_PARSED")
	}
	if w.LatestJob != nil && (w.LatestJob.State == "pending" || w.LatestJob.State == "running") {
		return w, x, p, errors.New("ENGINEERING_JOB_ACTIVE")
	}
	x.Model, x.Evidence, e = engineeringModel(ctx, s.db, *w.HeadRevisionID, s.project)
	if e != nil {
		return w, x, p, e
	}
	x.TotalCells = w.TotalCells
	x.MappedCells = w.MappedCells
	var raw []byte
	e = s.db.QueryRow(ctx, `SELECT proposal FROM eng_revision WHERE id=$1`, *w.HeadRevisionID).Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &p)
	}
	return w, x, p, e
}
func (s *Server) editEngineeringPower(ctx context.Context, source string, q engineering.PowerEdit, actor string) error {
	if strings.TrimSpace(q.Reason) == "" {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	_, x, p, e := s.engineeringEditContext(ctx, source)
	if e != nil {
		return e
	}
	seenProfiles := map[string]bool{}
	for _, profile := range q.Plan.EquipmentProfiles {
		key := profile.EquipmentID + "/" + profile.Basis
		if profile.EquipmentID == "" || seenProfiles[key] {
			return errors.New("ENGINEERING_SCHEMA_INVALID")
		}
		seenProfiles[key] = true
	}
	seenRacks := map[string]bool{}
	for _, r := range q.Plan.Racks {
		if r.RackID == "" || seenRacks[r.RackID] {
			return errors.New("ENGINEERING_SCHEMA_INVALID")
		}
		seenRacks[r.RackID] = true
	}
	x.Model.PowerPlans = []engineering.PowerPlan{q.Plan}
	_, e = s.saveEngineeringDraft(ctx, source, q.ExpectedRevision, "human", actor, q.Reason, x, p)
	return e
}
func (s *Server) applyEngineeringCalculation(ctx context.Context, source string, q engineering.CalculationApplication, actor string) error {
	if strings.TrimSpace(q.Reason) == "" || len(q.Bindings) == 0 {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	w, x, p, e := s.engineeringEditContext(ctx, source)
	if e != nil {
		return e
	}
	var belongs bool
	if e = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_calculation_run WHERE id::text=$1 AND project_id=$2 AND source_id=$3 AND revision_id=$4 AND status IN('succeeded','partial'))`, q.RunID, s.project, source, *w.HeadRevisionID).Scan(&belongs); e != nil {
		return e
	}
	if !belongs {
		return errors.New("CALC_RESULT_STALE_OR_UNAVAILABLE")
	}
	page, e := s.engineeringCalculation(ctx, q.RunID, 0, 0)
	if e != nil {
		return e
	}
	outputs := map[string]engineering.TypedValue{}
	for _, v := range page.Run.Outputs {
		outputs[v.Name] = v.Value
	}
	if e = s.prepareWorkloadApplication(ctx, &x.Model, page.Run, q.Bindings); e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, binding := range q.Bindings {
		key := binding.EntityID + "/" + binding.Field
		if seen[key] {
			return errors.New("CALC_DUPLICATE_BINDING")
		}
		seen[key] = true
		value, ok := outputs[binding.OutputName]
		if !ok || value.Kind != "decimal" || value.Decimal == nil || value.Unit == nil {
			return errors.New("CALC_OUTPUT_NOT_NUMERIC")
		}
		quantity := engineering.Quantity{Value: *value.Decimal, Unit: *value.Unit}
		found := false
		entity := binding.EntityID
		switch binding.Field {
		case "power_load", "power_remaining":
			for i := range x.Model.PowerPlans {
				for j := range x.Model.PowerPlans[i].Racks {
					r := &x.Model.PowerPlans[i].Racks[j]
					if r.RackID != binding.EntityID {
						continue
					}
					if page.Run.ProviderID == "engineering-power-balance" && binding.OutputName != r.RackID+map[string]string{"power_load": "/load", "power_remaining": "/remaining"}[binding.Field] {
						return errors.New("CALC_OUTPUT_BINDING_MISMATCH")
					}
					found = true
					entity = x.Model.PowerPlans[i].EntityID
					if binding.Field == "power_load" {
						r.PlannedLoad = &quantity
					} else {
						r.Remaining = &quantity
					}
					r.CalculationRunID = &q.RunID
				}
			}
		case "circuit_load", "circuit_remaining", "supply_load":
			for i := range x.Model.PowerPlans {
				p := &x.Model.PowerPlans[i]
				if binding.Field == "supply_load" {
					for j := range p.Supplies {
						s := &p.Supplies[j]
						if s.ID != binding.EntityID {
							continue
						}
						if page.Run.ProviderID != "engineering-power-balance" || binding.OutputName != s.ID+"/load" {
							return errors.New("CALC_OUTPUT_BINDING_MISMATCH")
						}
						s.Load = &quantity
						s.CalculationRunID = &q.RunID
						found = true
						entity = p.EntityID
					}
				}
				for j := range p.Circuits {
					c := &p.Circuits[j]
					if c.ID != binding.EntityID || binding.Field == "supply_load" {
						continue
					}
					suffix := map[string]string{"circuit_load": "/load", "circuit_remaining": "/remaining"}[binding.Field]
					if page.Run.ProviderID != "engineering-power-balance" || binding.OutputName != c.ID+suffix {
						return errors.New("CALC_OUTPUT_BINDING_MISMATCH")
					}
					if binding.Field == "circuit_load" {
						c.Load = &quantity
					} else {
						c.Remaining = &quantity
					}
					c.CalculationRunID = &q.RunID
					found = true
					entity = p.EntityID
				}
			}
		case "cable_calculated", "cable_selected_stock":
			for i := range x.Model.Cables {
				c := &x.Model.Cables[i]
				if c.EntityID != binding.EntityID {
					continue
				}
				found = true
				ev := engineering.StableID(source, "calculation-evidence", q.RunID+key)
				l := &engineering.LengthEvidence{Length: quantity, CalculationRunID: &q.RunID, EvidenceIDs: []string{ev}}
				if binding.Field == "cable_calculated" {
					c.Calculated = l
				} else {
					c.SelectedStock = l
				}
				c.RuleVersion = page.Run.Rule
			}
		case "material_required", "material_gap", "workload_quantity", "workload_hours":
			found = applyEstimateResult(&x.Model, page.Run, binding, quantity)
		default:
			return errors.New("CALC_BINDING_FIELD_UNSUPPORTED")
		}
		if !found {
			return errors.New("CALC_BINDING_ENTITY_UNKNOWN")
		}
		ev := engineering.StableID(source, "calculation-evidence", q.RunID+key)
		x.Evidence = append(x.Evidence, engineering.SourceEvidence{ID: ev, EntityID: entity, Sheet: "[calculation]", Cell: binding.OutputName, Field: binding.Field, CalculationRunID: &q.RunID})
	}
	_, e = s.saveEngineeringDraft(ctx, source, q.ExpectedRevision, "human", actor, q.Reason, x, p)
	return e
}

func powerInputDigest(m engineering.PhysicalModel) string {
	values := []any{}
	for _, p := range m.PowerPlans {
		for _, e := range p.EquipmentProfiles {
			values = append(values, []any{e.EquipmentID, e.Basis, e.Power})
		}
		if p.Scope != nil {
			values = append(values, p.Scope)
		}
		for _, c := range p.Circuits {
			values = append(values, []any{c.ID, c.Code, c.ObjectCode, c.LocationID, c.UpstreamCode, c.Capacity})
		}
		for _, s := range p.Supplies {
			values = append(values, []any{s.ID, s.EquipmentID, s.PSU, s.CircuitCode, s.FeedGroup, s.AllocationRule})
		}
		for _, r := range p.Racks {
			values = append(values, []any{r.RackID, r.Capacity, r.CoolingCapacity})
			if r.CapacityBasis != "" {
				values = append(values, []string{r.RackID, "capacity_basis", r.CapacityBasis})
			}
		}
	}
	for _, e := range m.Equipment {
		values = append(values, []any{e.EntityID, e.Quantity})
	}
	for _, p := range m.Placements {
		values = append(values, []string{p.EquipmentID, p.RackID})
	}
	// DB collection ordering and source ordering must not change the calculation input identity.
	parts := []string{}
	for _, v := range values {
		parts = append(parts, string(jsonBytes(v)))
	}
	sort.Strings(parts)
	return engineering.Digest(parts)
}
