package engineering

import (
	"github.com/google/uuid"
	"math/big"
	"time"
)

func extractPower(project string, rows []rowData, out *Extraction, version func(string, rowData, ...string) EntityVersion, issue func(string, string, string, string, string)) {
	plan := PowerPlan{EquipmentProfiles: []EquipmentPowerProfile{}, Racks: []RackPowerPlan{}}
	id := StableID(project, "power-plan", "planning")
	eq := map[string]bool{}
	for _, e := range out.Model.Equipment {
		eq[e.EntityID] = true
	}
	profiles := map[string]int{}
	racks := map[string]int{}
	quantity := func(r rowData, field, unitField string) *Quantity {
		if r.values[field] == "" {
			return nil
		}
		unit := r.units[field]
		if unit == "" {
			unit = r.values[unitField]
		}
		q := Quantity{r.values[field], unit}
		v, e := measure(q)
		if e != nil || v.dimension != "power" || v.value.Sign() < 0 {
			issue(id, "POWER_QUANTITY_INVALID", field, "功率需明确非负数值和W/kW单位", PowerTarget)
			return nil
		}
		return &q
	}
	for _, r := range rows {
		if r.kind != "power" && r.values["power"] == "" && r.values["power_capacity"] == "" && r.values["cooling_capacity"] == "" {
			continue
		}
		v := version(id, r, "device", "label", "rack", "site", "room", "power", "power_unit", "power_basis", "cooling_type", "power_capacity", "power_capacity_unit", "cooling_capacity", "cooling_capacity_unit")
		if plan.EntityID == "" {
			plan.EntityVersion = v
		} else {
			plan.EvidenceIDs = append(plan.EvidenceIDs, v.EvidenceIDs...)
		}
		if r.values["power"] != "" {
			label := r.values["device"]
			if label == "" {
				label = r.values["label"]
			}
			owner := StableID(project, "equipment", label)
			if label != "" && !eq[owner] {
				out.Model.Equipment = append(out.Model.Equipment, EquipmentPlan{EntityVersion: version(owner, r, "device", "label"), EquipmentType: "device", Label: label, Quantity: 1, IdentityKind: "planned_instance"})
				eq[owner] = true
			}
			q := quantity(r, "power", "power_unit")
			if q != nil {
				profile := EquipmentPowerProfile{EquipmentID: owner, Basis: r.values["power_basis"], Power: *q, CoolingType: r.values["cooling_type"], EvidenceIDs: v.EvidenceIDs}
				key := owner + "/" + profile.Basis
				if i, ok := profiles[key]; ok {
					old := plan.EquipmentProfiles[i]
					if Digest([]any{old.Power, old.CoolingType}) != Digest([]any{profile.Power, profile.CoolingType}) {
						issue(id, "POWER_SOURCE_CONFLICT", "power", "同一设备和功率依据存在冲突", PowerTarget)
					}
				} else {
					profiles[key] = len(plan.EquipmentProfiles)
					plan.EquipmentProfiles = append(plan.EquipmentProfiles, profile)
				}
			}
		}
		if r.values["rack"] != "" && (r.values["power_capacity"] != "" || r.values["cooling_capacity"] != "" || r.values["cooling_type"] != "") {
			rack := StableID(project, "facility-rack", r.values["rack"])
			// Match the already extracted facility hierarchy, without creating a parallel rack identity.
			for _, f := range out.Model.Facilities {
				if f.Kind == "rack" && f.Label == r.values["rack"] {
					rack = f.EntityID
				}
			}
			rp := RackPowerPlan{RackID: rack, Capacity: quantity(r, "power_capacity", "power_capacity_unit"), CoolingCapacity: quantity(r, "cooling_capacity", "cooling_capacity_unit"), CoolingType: r.values["cooling_type"]}
			if i, ok := racks[rack]; ok {
				old := &plan.Racks[i]
				if old.Capacity != nil && rp.Capacity != nil && Digest(old.Capacity) != Digest(rp.Capacity) {
					issue(id, "RACK_CAPACITY_CONFLICT", "power_capacity", "机柜供电容量来源冲突", PowerTarget)
				} else if rp.Capacity != nil {
					old.Capacity = rp.Capacity
				}
				if old.CoolingCapacity != nil && rp.CoolingCapacity != nil && Digest(old.CoolingCapacity) != Digest(rp.CoolingCapacity) {
					issue(id, "RACK_COOLING_CONFLICT", "cooling_capacity", "机柜冷却容量来源冲突", PowerTarget)
				} else if rp.CoolingCapacity != nil {
					old.CoolingCapacity = rp.CoolingCapacity
				}
				if old.CoolingType != "" && rp.CoolingType != "" && old.CoolingType != rp.CoolingType {
					issue(id, "COOLING_TYPE_CONFLICT", "cooling_type", "冷却方式来源冲突", PowerTarget)
				} else if rp.CoolingType != "" {
					old.CoolingType = rp.CoolingType
				}
			} else {
				racks[rack] = len(plan.Racks)
				plan.Racks = append(plan.Racks, rp)
			}
		}
	}
	if plan.EntityID != "" {
		out.Model.PowerPlans = append(out.Model.PowerPlans, plan)
	}
}

func ValidatePower(m PhysicalModel) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(p PowerPlan, code, field, message string) {
		issues = append(issues, ValidationIssue{ID: StableID(m.ProjectID, "power-issue", p.EntityID+"/"+code+"/"+field), EntityID: p.EntityID, ProjectID: m.ProjectID, RevisionID: p.RevisionID, Code: code, Severity: "BLOCK", ModuleID: "power_cooling", Field: field, Message: message, PublishTarget: PowerTarget, EvidenceIDs: p.EvidenceIDs})
	}
	eq := map[string]bool{}
	fac := map[string]bool{}
	for _, e := range m.Equipment {
		eq[e.EntityID] = true
	}
	for _, f := range m.Facilities {
		fac[f.EntityID] = f.Kind == "rack"
	}
	for _, p := range m.PowerPlans {
		issues = append(issues, ValidatePowerSupply(m, p, true)...)
		seen := map[string]bool{}
		for _, v := range p.EquipmentProfiles {
			key := v.EquipmentID + "/" + v.Basis
			if !eq[v.EquipmentID] || seen[key] {
				add(p, "POWER_EQUIPMENT_INVALID", "equipment_profiles", "功率档案设备未明确或同依据重复")
			}
			seen[key] = true
			if v.Basis != "planned" && v.Basis != "nameplate" && v.Basis != "measured" {
				add(p, "POWER_BASIS_REQUIRED", "power_basis", "功率必须区分规划、铭牌和实测依据")
			}
			n, e := measure(v.Power)
			if e != nil || n.dimension != "power" || n.value.Sign() < 0 {
				add(p, "POWER_VALUE_INVALID", "power", "功率必须是明确的非负功率量")
			}
		}
		seen = map[string]bool{}
		for _, v := range p.Racks {
			if !fac[v.RackID] || seen[v.RackID] {
				add(p, "POWER_RACK_INVALID", "racks", "容量必须关联唯一机柜")
			}
			seen[v.RackID] = true
			for field, q := range map[string]*Quantity{"capacity": v.Capacity, "cooling_capacity": v.CoolingCapacity, "planned_load": v.PlannedLoad, "remaining": v.Remaining} {
				if q == nil {
					continue
				}
				n, e := measure(*q)
				if e != nil || n.dimension != "power" || (field != "remaining" && n.value.Sign() < 0) {
					add(p, "POWER_QUANTITY_INVALID", field, "负荷与容量必须是功率量")
				} else if field == "remaining" && n.value.Sign() < 0 {
					add(p, "RACK_POWER_EXCEEDED", field, "规划负荷超过机柜供电容量")
				}
			}
			if v.PlannedLoad != nil || v.Remaining != nil {
				if v.CalculationRunID == nil {
					add(p, "POWER_CALCULATION_REQUIRED", "calculation_run_id", "负荷和余量必须关联确定性计算")
				}
			}
			if v.PlannedLoad != nil && v.CoolingCapacity != nil {
				a, ea := measure(*v.PlannedLoad)
				b, eb := measure(*v.CoolingCapacity)
				if ea == nil && eb == nil && a.dimension == b.dimension && a.value.Cmp(b.value) > 0 {
					add(p, "RACK_COOLING_EXCEEDED", "cooling_capacity", "规划热负荷超过冷却容量")
				}
			}
		}
	}
	return issues
}

// Power balance is intrinsic arithmetic, not a project diversity/redundancy rule.
// It never substitutes nameplate/measured power or estimates missing rack inputs.
func PowerBalance(m PhysicalModel, revision string) CalculationRun {
	now := time.Now().UTC()
	run := CalculationRun{ID: uuid.NewString(), ProjectID: m.ProjectID, RevisionID: revision, ModuleInstanceID: "power_cooling", ProviderID: "engineering-power-balance", ProviderVersion: "1", Status: "succeeded", Compatibility: "compatible", Inputs: []NamedValue{}, Outputs: []NamedValue{}, Formulas: []FormulaEvidence{}, Errors: []CalculationError{}, StartedAt: now, FinishedAt: &now}
	value := func(name string, q Quantity) NamedValue {
		return NamedValue{Name: name, Value: TypedValue{Kind: "decimal", Decimal: &q.Value, Unit: &q.Unit}}
	}
	eq := map[string]EquipmentPlan{}
	for _, e := range m.Equipment {
		eq[e.EntityID] = e
	}
	placementCount := map[string]int{}
	for _, p := range m.Placements {
		placementCount[p.EquipmentID]++
	}
	for _, plan := range m.PowerPlans {
		profiles := map[string]Quantity{}
		for _, p := range plan.EquipmentProfiles {
			if p.Basis == "planned" {
				profiles[p.EquipmentID] = p.Power
				run.Inputs = append(run.Inputs, value(p.EquipmentID+"/planned_power", p.Power))
			}
		}
		for _, rack := range plan.Racks {
			sum := new(big.Rat)
			seen := map[string]bool{}
			valid := true
			for _, placement := range m.Placements {
				if placement.RackID != rack.RackID {
					continue
				}
				if seen[placement.EquipmentID] || placementCount[placement.EquipmentID] != 1 {
					valid = false
				}
				seen[placement.EquipmentID] = true
				q, ok := profiles[placement.EquipmentID]
				v, e := measure(q)
				if !ok || e != nil || v.dimension != "power" || v.value.Sign() < 0 || eq[placement.EquipmentID].Quantity != 1 {
					valid = false
					continue
				}
				sum.Add(sum, v.value)
			}
			if !valid || len(seen) == 0 {
				run.Errors = append(run.Errors, CalculationError{Code: "POWER_INPUT_INCOMPLETE", Message: "机柜 " + rack.RackID + " 缺少唯一落位或明确规划功率"})
				continue
			}
			load, _ := resultValue(measured{sum, "power"}, "W")
			run.Outputs = append(run.Outputs, NamedValue{Name: rack.RackID + "/load", Value: load})
			if rack.Capacity == nil {
				run.Errors = append(run.Errors, CalculationError{Code: "POWER_CAPACITY_MISSING", Message: "机柜 " + rack.RackID + " 未提供供电容量"})
				continue
			}
			cap, e := measure(*rack.Capacity)
			if e != nil || cap.dimension != "power" || cap.value.Sign() < 0 {
				run.Errors = append(run.Errors, CalculationError{Code: "POWER_CAPACITY_INVALID", Message: "机柜容量数值或单位无效"})
				continue
			}
			run.Inputs = append(run.Inputs, value(rack.RackID+"/capacity", *rack.Capacity))
			remaining, _ := resultValue(measured{new(big.Rat).Sub(cap.value, sum), "power"}, "W")
			run.Outputs = append(run.Outputs, NamedValue{Name: rack.RackID + "/remaining", Value: remaining})
		}
	}
	if len(run.Outputs) == 0 {
		run.Status = "failed"
	} else if len(run.Errors) > 0 {
		run.Status = "partial"
	}
	run.InputDigest = Digest(run.Inputs)
	return run
}
