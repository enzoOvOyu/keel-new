package engineering

import (
	"fmt"
	"math/big"
)

func ValidatePowerSupply(m PhysicalModel, p PowerPlan, results bool) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(code, field, message string) {
		issues = append(issues, ValidationIssue{ID: StableID(m.ProjectID, "power-supply-issue", p.EntityID+"/"+code+"/"+field), EntityID: p.EntityID, ProjectID: m.ProjectID, RevisionID: p.RevisionID, Code: code, Severity: "BLOCK", ModuleID: "power_cooling", Field: field, Message: message, PublishTarget: PowerTarget, EvidenceIDs: p.EvidenceIDs})
	}
	if p.Scope == nil {
		if len(p.Circuits)+len(p.Supplies) > 0 {
			add("POWER_SCOPE_REQUIRED", "scope", "供电建模需要明确场景、功率依据和规则")
		}
		return issues
	}
	if p.Scope.Scenario == "" || p.Scope.DemandRule.ID == "" || p.Scope.DemandRule.Version < 1 || (p.Scope.Basis != "planned" && p.Scope.Basis != "nameplate" && p.Scope.Basis != "measured") {
		add("POWER_SCOPE_INVALID", "scope", "电力场景、功率依据或需求规则不完整")
	}
	if p.Scope.RedundancyMode != "none" && p.Scope.RedundancyMode != "dual_independent" {
		add("POWER_REDUNDANCY_MODE_INVALID", "scope", "必须明确是否要求资料根节点之间双路独立")
	}
	if len(p.Racks) == 0 {
		add("POWER_RACK_CAPACITY_REQUIRED", "racks", "需要明确的机柜容量记录")
	}
	if len(p.Circuits) == 0 || len(p.Supplies) == 0 {
		add("POWER_SUPPLY_GRAPH_REQUIRED", "circuits", "需要供电回路与设备电源口关系")
	}
	facilities := map[string]bool{}
	equipment := map[string]bool{}
	for _, f := range m.Facilities {
		facilities[f.EntityID] = true
	}
	for _, e := range m.Equipment {
		equipment[e.EntityID] = true
	}
	aliases := map[string]int{}
	ids := map[string]bool{}
	for i, c := range p.Circuits {
		if c.ID == "" || ids[c.ID] || c.Code == "" || c.ObjectCode == "" || !facilities[c.LocationID] {
			add("POWER_CIRCUIT_INVALID", c.ID, "回路身份、位置或编码不完整/重复")
		}
		ids[c.ID] = true
		for _, code := range []string{c.Code, c.ObjectCode} {
			if previous, ok := aliases[code]; ok && previous != i {
				add("POWER_CIRCUIT_ALIAS_AMBIGUOUS", code, "供电对象/回路编码重复")
			}
			aliases[code] = i
		}
		capacity, e := measure(c.Capacity)
		if e != nil || capacity.dimension != "power" || capacity.value.Sign() < 0 {
			add("POWER_CIRCUIT_CAPACITY_INVALID", c.ID, "回路容量须为明确非负功率量")
		}
		if results && (c.Load == nil || c.Remaining == nil || c.CalculationRunID == nil) {
			add("POWER_CIRCUIT_CALCULATION_REQUIRED", c.ID, "回路负载/余量需要确定性计算记录")
		}
		if c.Load != nil {
			v, e := measure(*c.Load)
			if e != nil || v.dimension != "power" || v.value.Sign() < 0 {
				add("POWER_CIRCUIT_RESULT_INVALID", c.ID, "回路负载必须为非负功率量")
			}
		}
		if c.Remaining != nil {
			remaining, e := measure(*c.Remaining)
			if e != nil || remaining.dimension != "power" {
				add("POWER_CIRCUIT_RESULT_INVALID", c.ID, "回路结果单位无效")
			} else if remaining.value.Sign() < 0 {
				add("POWER_CIRCUIT_EXCEEDED", c.ID, "回路负载超过容量")
			}
		}
	}
	root := func(code string) (string, error) {
		seen := map[int]bool{}
		for {
			i, ok := aliases[code]
			if !ok {
				return "", fmt.Errorf("POWER_CIRCUIT_UNRESOLVED")
			}
			if seen[i] {
				return "", fmt.Errorf("POWER_CIRCUIT_CYCLE")
			}
			seen[i] = true
			c := p.Circuits[i]
			if c.UpstreamCode == "" {
				return c.ID, nil
			}
			code = c.UpstreamCode
		}
	}
	for _, c := range p.Circuits {
		if _, e := root(c.Code); e != nil {
			add(e.Error(), c.ID, "供电上游引用缺失或存在循环")
		}
	}
	seenPSU := map[string]bool{}
	groups := map[string]map[string]string{}
	for _, s := range p.Supplies {
		key := s.EquipmentID + "/" + s.PSU
		if s.ID == "" || !equipment[s.EquipmentID] || s.PSU == "" || seenPSU[key] || s.AllocationRule.ID == "" || s.AllocationRule.Version < 1 || (s.FeedGroup != "A" && s.FeedGroup != "B") {
			add("POWER_SUPPLY_INVALID", s.ID, "设备电源口、分组、规则或唯一关系无效")
		}
		seenPSU[key] = true
		r, e := root(s.CircuitCode)
		if e != nil {
			add(e.Error(), s.ID, "电源口引用的回路无法解析")
		} else {
			if groups[s.EquipmentID] == nil {
				groups[s.EquipmentID] = map[string]string{}
			}
			if old, ok := groups[s.EquipmentID][s.FeedGroup]; ok && old != r {
				add("POWER_FEED_ROOT_AMBIGUOUS", s.ID, "同一供电组存在不同来源根节点")
			}
			groups[s.EquipmentID][s.FeedGroup] = r
		}
		if s.Load != nil {
			v, e := measure(*s.Load)
			if e != nil || v.dimension != "power" || v.value.Sign() < 0 {
				add("POWER_SUPPLY_RESULT_INVALID", s.ID, "电源口负载必须为非负功率量")
			}
		}
		if results && (s.Load == nil || s.CalculationRunID == nil) {
			add("POWER_SUPPLY_CALCULATION_REQUIRED", s.ID, "电源口分配负载需要确定性计算记录")
		}
	}
	for _, profile := range p.EquipmentProfiles {
		if profile.Basis != p.Scope.Basis {
			continue
		}
		g := groups[profile.EquipmentID]
		if len(g) == 0 {
			add("POWER_EQUIPMENT_SUPPLY_REQUIRED", profile.EquipmentID, "设备缺少供电关系")
		}
		if p.Scope.RedundancyMode == "dual_independent" && (g["A"] == "" || g["B"] == "" || g["A"] == g["B"]) {
			add("POWER_FEEDS_NOT_INDEPENDENT", profile.EquipmentID, "要求A/B两组且在已声明资料根节点上独立")
		}
	}
	if results {
		for _, r := range p.Racks {
			if r.PlannedLoad == nil || r.Remaining == nil || r.CalculationRunID == nil {
				add("POWER_RACK_CALCULATION_REQUIRED", r.RackID, "机柜负载/余量需要确定性计算记录")
			}
		}
	}
	return issues
}

// ScopedPowerBalance consumes project-approved rules. The old intrinsic rack
// sum remains available for legacy basic plans without a supply scope.
func ScopedPowerBalance(m PhysicalModel, revision string, rules map[string]CalculationRule) (CalculationRun, error) {
	if len(m.PowerPlans) != 1 {
		for _, p := range m.PowerPlans {
			if p.Scope != nil {
				r := PowerBalance(EmptyPhysical(m.ProjectID), revision)
				r.Status = "failed"
				return r, fmt.Errorf("POWER_SCOPE_SINGLE_PLAN_REQUIRED")
			}
		}
		return PowerBalance(m, revision), nil
	}
	if m.PowerPlans[0].Scope == nil {
		return PowerBalance(m, revision), nil
	}
	p := m.PowerPlans[0]
	fail := func(code string) (CalculationRun, error) {
		run := PowerBalance(EmptyPhysical(m.ProjectID), revision)
		run.Status = "failed"
		run.Errors = []CalculationError{{Code: code, Message: code}}
		return run, fmt.Errorf("%s", code)
	}
	if issues := ValidatePowerSupply(m, p, false); len(issues) > 0 {
		return fail(issues[0].Code)
	}
	q := func(n *big.Rat) Quantity {
		v, _ := resultValue(measured{n, "power"}, "W")
		return Quantity{Value: *v.Decimal, Unit: "W"}
	}
	call := func(ref VersionReference, input Quantity) (Quantity, error) {
		rule, ok := rules[ref.ID+"@"+fmt.Sprint(ref.Version)]
		if !ok {
			return Quantity{}, fmt.Errorf("POWER_RULE_UNAVAILABLE")
		}
		run, e := RunRule(rule, m.ProjectID, revision, []NamedValue{{Name: "power", Value: TypedValue{Kind: "decimal", Decimal: &input.Value, Unit: &input.Unit}}})
		if e != nil {
			return Quantity{}, e
		}
		for _, output := range run.Outputs {
			if output.Name == "power" && output.Value.Decimal != nil && output.Value.Unit != nil {
				value := Quantity{Value: *output.Value.Decimal, Unit: *output.Value.Unit}
				n, e := measure(value)
				if e == nil && n.dimension == "power" && n.value.Sign() >= 0 {
					return value, nil
				}
			}
		}
		return Quantity{}, fmt.Errorf("POWER_RULE_OUTPUT_INVALID")
	}
	temporary := m
	temporary.PowerPlans = append([]PowerPlan{}, m.PowerPlans...)
	temporary.PowerPlans[0].EquipmentProfiles = []EquipmentPowerProfile{}
	effective := map[string]*big.Rat{}
	original := []NamedValue{}
	for _, profile := range p.EquipmentProfiles {
		if profile.Basis != p.Scope.Basis {
			continue
		}
		power, e := call(p.Scope.DemandRule, profile.Power)
		if e != nil {
			return fail(e.Error())
		}
		n, _ := measure(power)
		effective[profile.EquipmentID] = n.value
		copy := profile
		copy.Basis = "planned"
		copy.Power = power
		temporary.PowerPlans[0].EquipmentProfiles = append(temporary.PowerPlans[0].EquipmentProfiles, copy)
		original = append(original, NamedValue{Name: profile.EquipmentID + "/source_power", Value: TypedValue{Kind: "decimal", Decimal: &profile.Power.Value, Unit: &profile.Power.Unit}})
	}
	run := PowerBalance(temporary, revision)
	run.ProviderVersion = "2"
	run.Inputs = append(run.Inputs, original...)
	addValue := func(name string, value Quantity) {
		run.Outputs = append(run.Outputs, NamedValue{Name: name, Value: TypedValue{Kind: "decimal", Decimal: &value.Value, Unit: &value.Unit}})
	}
	recordRule := func(name string, ref VersionReference) {
		value := ref.ID + "@" + fmt.Sprint(ref.Version)
		run.Inputs = append(run.Inputs, NamedValue{Name: name, Value: TypedValue{Kind: "text", Text: &value}})
	}
	recordRule("demand_rule", p.Scope.DemandRule)
	aliases := map[string]int{}
	loads := make([]*big.Rat, len(p.Circuits))
	for i, c := range p.Circuits {
		aliases[c.Code] = i
		aliases[c.ObjectCode] = i
		loads[i] = new(big.Rat)
	}
	allocated := map[string]*big.Rat{}
	for _, s := range p.Supplies {
		power, ok := effective[s.EquipmentID]
		if !ok {
			return fail("POWER_SUPPLY_PROFILE_REQUIRED")
		}
		value, e := call(s.AllocationRule, q(power))
		if e != nil {
			return fail(e.Error())
		}
		n, _ := measure(value)
		if allocated[s.EquipmentID] == nil {
			allocated[s.EquipmentID] = new(big.Rat)
		}
		allocated[s.EquipmentID].Add(allocated[s.EquipmentID], n.value)
		addValue(s.ID+"/load", value)
		recordRule(s.ID+"/allocation_rule", s.AllocationRule)
		code := s.CircuitCode
		for code != "" {
			i := aliases[code]
			loads[i].Add(loads[i], n.value)
			code = p.Circuits[i].UpstreamCode
		}
	}
	for eq, power := range effective {
		if allocated[eq] == nil || allocated[eq].Cmp(power) != 0 {
			return fail("POWER_ALLOCATION_NOT_CONSERVED")
		}
		addValue(eq+"/effective_power", q(power))
	}
	for i, c := range p.Circuits {
		capacity, _ := measure(c.Capacity)
		addValue(c.ID+"/load", q(loads[i]))
		addValue(c.ID+"/remaining", q(new(big.Rat).Sub(capacity.value, loads[i])))
		run.Inputs = append(run.Inputs, NamedValue{Name: c.ID + "/capacity", Value: TypedValue{Kind: "decimal", Decimal: &c.Capacity.Value, Unit: &c.Capacity.Unit}})
	}
	run.InputDigest = Digest(run.Inputs)
	return run, nil
}
