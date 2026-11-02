package engineering

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type PowerScope struct {
	Scenario       string           `json:"scenario"`
	Basis          string           `json:"basis"`
	DemandRule     VersionReference `json:"demand_rule"`
	RedundancyMode string           `json:"redundancy_mode"`
}
type PowerCircuit struct {
	ID               ID        `json:"id"`
	Code             string    `json:"code"`
	ObjectCode       string    `json:"object_code"`
	LocationID       ID        `json:"location_id"`
	UpstreamCode     string    `json:"upstream_code"`
	Capacity         Quantity  `json:"capacity"`
	Load             *Quantity `json:"load,omitempty"`
	Remaining        *Quantity `json:"remaining,omitempty"`
	CalculationRunID *ID       `json:"calculation_run_id,omitempty"`
}
type EquipmentSupply struct {
	ID               ID               `json:"id"`
	Load             *Quantity        `json:"load,omitempty"`
	CalculationRunID *ID              `json:"calculation_run_id,omitempty"`
	EquipmentID      ID               `json:"equipment_id"`
	PSU              string           `json:"psu"`
	CircuitCode      string           `json:"circuit_code"`
	FeedGroup        string           `json:"feed_group"`
	AllocationRule   VersionReference `json:"allocation_rule"`
}

func PowerRuleReference(project, reference string) (VersionReference, error) {
	code, version, ok := strings.Cut(strings.TrimSpace(reference), "@")
	if !ok || code == "" {
		return VersionReference{}, fmt.Errorf("STEP_POWER_RULE_REFERENCE_INVALID")
	}
	n, e := strconv.Atoi(version)
	if e != nil || n < 1 {
		return VersionReference{}, fmt.Errorf("STEP_POWER_RULE_REFERENCE_INVALID")
	}
	id := code
	if _, e := uuid.Parse(code); e != nil {
		id = StableID(project, "power-rule-code", code)
	}
	return VersionReference{ID: id, Version: n}, nil
}
func mergePowerInput(current PhysicalModel, q InputPreviewRequest) (PhysicalModel, error) {
	preview := PreviewStepInput(q)
	if !preview.Valid {
		return current, fmt.Errorf("STEP_INPUT_INVALID")
	}
	if q.TableID == "power_rules" {
		return current, nil
	}
	if q.TableID == "power_scope" && len(preview.Rows) != 1 {
		return current, fmt.Errorf("STEP_POWER_SCOPE_AMBIGUOUS")
	}
	keys := map[string][]int{"power_profiles": {0, 3}, "rack_capacity": {0}, "power_circuits": {3}, "power_connections": {0, 1}}
	seen := map[string]string{}
	for _, row := range preview.Rows {
		key := ""
		for _, column := range keys[q.TableID] {
			key += strings.TrimSpace(row[column]) + "\x00"
		}
		if key == "" {
			continue
		}
		values := append([]string{}, row...)
		if q.TableID == "power_profiles" {
			values = values[:4]
		}
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}
		digest := Digest(values)
		if old, ok := seen[key]; ok && old != digest {
			return current, fmt.Errorf("STEP_POWER_SOURCE_CONFLICT")
		}
		seen[key] = digest
	}
	raw, e := json.Marshal(current)
	if e != nil {
		return current, e
	}
	var next PhysicalModel
	if e = json.Unmarshal(raw, &next); e != nil {
		return current, e
	}
	if len(next.PowerPlans) > 1 {
		return current, fmt.Errorf("STEP_POWER_PLAN_AMBIGUOUS")
	}
	if len(next.PowerPlans) == 0 {
		next.PowerPlans = []PowerPlan{{EntityVersion: EntityVersion{EntityID: StableID(current.ProjectID, "power-plan", "planning"), Maturity: Estimated, EvidenceIDs: []string{}, Attributes: []AttributeValue{}}, EquipmentProfiles: []EquipmentPowerProfile{}, Racks: []RackPowerPlan{}}}
	}
	plan := &next.PowerPlans[0]
	owner := func(serial string) (string, error) {
		found := ""
		for _, eq := range next.Equipment {
			if eq.Serial != nil && strings.TrimSpace(*eq.Serial) == serial {
				if found != "" {
					return "", fmt.Errorf("STEP_SERIAL_AMBIGUOUS")
				}
				found = eq.EntityID
			}
		}
		if found == "" {
			return "", fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
		}
		return found, nil
	}
	facility := func(code, kind string) (string, error) {
		found := ""
		for _, f := range next.Facilities {
			if (kind == "" || f.Kind == kind) && (f.Code == code || f.Code == "" && f.Label == code) {
				if found != "" {
					return "", fmt.Errorf("STEP_FACILITY_AMBIGUOUS")
				}
				found = f.EntityID
			}
		}
		if found == "" {
			return "", fmt.Errorf("STEP_RACK_REQUIRED")
		}
		return found, nil
	}
	power := func(value, unit string) (Quantity, error) {
		q := Quantity{value, unit}
		n, e := measure(q)
		if e != nil || n.dimension != "power" || n.value.Sign() < 0 {
			return q, fmt.Errorf("STEP_POWER_QUANTITY_INVALID")
		}
		return q, nil
	}
	for _, row := range preview.Rows {
		v := append([]string{}, row...)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		switch q.TableID {
		case "power_scope":
			rule, e := PowerRuleReference(current.ProjectID, v[2])
			if e != nil {
				return current, e
			}
			if v[1] != "planned" && v[1] != "nameplate" && v[1] != "measured" {
				return current, fmt.Errorf("STEP_POWER_BASIS_INVALID")
			}
			if v[3] != "none" && v[3] != "dual_independent" {
				return current, fmt.Errorf("STEP_POWER_REDUNDANCY_INVALID")
			}
			plan.Scope = &PowerScope{Scenario: v[0], Basis: v[1], DemandRule: rule, RedundancyMode: v[3]}
		case "power_profiles":
			eq, e := owner(v[0])
			if e != nil {
				return current, e
			}
			q, e := power(v[1], v[2])
			if e != nil {
				return current, e
			}
			if v[3] != "planned" && v[3] != "nameplate" && v[3] != "measured" {
				return current, fmt.Errorf("STEP_POWER_BASIS_INVALID")
			}
			found := -1
			for i, x := range plan.EquipmentProfiles {
				if x.EquipmentID == eq && x.Basis == v[3] {
					found = i
					break
				}
			}
			profile := EquipmentPowerProfile{EquipmentID: eq, Basis: v[3], Power: q, EvidenceIDs: []string{}}
			if found < 0 {
				plan.EquipmentProfiles = append(plan.EquipmentProfiles, profile)
			} else {
				profile.CoolingType = plan.EquipmentProfiles[found].CoolingType
				plan.EquipmentProfiles[found] = profile
			}
		case "rack_capacity":
			rack, e := facility(v[0], "rack")
			if e != nil {
				return current, e
			}
			q, e := power(v[1], v[2])
			if e != nil {
				return current, e
			}
			found := -1
			for i, x := range plan.Racks {
				if x.RackID == rack {
					found = i
					break
				}
			}
			var cooling *Quantity
			if v[5] != "" {
				c, e := power(v[5], v[2])
				if e != nil {
					return current, e
				}
				cooling = &c
			}
			r := RackPowerPlan{RackID: rack, Capacity: &q, CoolingType: v[4], CoolingCapacity: cooling, CapacityBasis: v[3]}
			if found < 0 {
				plan.Racks = append(plan.Racks, r)
			} else {
				plan.Racks[found] = r
			}
		case "power_circuits":
			location, e := facility(v[1], "")
			if e != nil {
				return current, e
			}
			q, e := power(v[4], v[5])
			if e != nil {
				return current, e
			}
			found := -1
			for i, x := range plan.Circuits {
				if x.Code == v[3] {
					found = i
					break
				}
			}
			c := PowerCircuit{ID: StableID(current.ProjectID, "power-circuit", v[3]), ObjectCode: v[0], LocationID: location, UpstreamCode: v[2], Code: v[3], Capacity: q}
			if found < 0 {
				plan.Circuits = append(plan.Circuits, c)
			} else {
				c.ID = plan.Circuits[found].ID
				plan.Circuits[found] = c
			}
		case "power_connections":
			eq, e := owner(v[0])
			if e != nil {
				return current, e
			}
			rule, e := PowerRuleReference(current.ProjectID, v[4])
			if e != nil {
				return current, e
			}
			if v[3] != "A" && v[3] != "B" {
				return current, fmt.Errorf("STEP_POWER_FEED_INVALID")
			}
			s := EquipmentSupply{ID: StableID(current.ProjectID, "equipment-supply", eq+"/"+v[1]), EquipmentID: eq, PSU: v[1], CircuitCode: v[2], FeedGroup: v[3], AllocationRule: rule}
			found := -1
			for i, x := range plan.Supplies {
				if x.EquipmentID == eq && x.PSU == v[1] {
					found = i
					break
				}
			}
			if found < 0 {
				plan.Supplies = append(plan.Supplies, s)
			} else {
				plan.Supplies[found] = s
			}
		default:
			return current, fmt.Errorf("STEP_TABLE_NOT_SUPPORTED")
		}
	}
	return next, nil
}
