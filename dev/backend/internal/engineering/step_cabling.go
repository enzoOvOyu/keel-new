package engineering

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// CableRuleRunner resolves reviewed project rules and durably records each run.
// Workbook values are inputs, never executable expressions.
type CableRuleRunner func(reference string, inputs []NamedValue) (Quantity, ID, VersionReference, error)

func MergeCablingWorkbook(current PhysicalModel, inputs []InputPreviewRequest, evidence []ID, run CableRuleRunner) (PhysicalModel, error) {
	raw, err := json.Marshal(current)
	if err != nil {
		return current, err
	}
	var next PhysicalModel
	if err = json.Unmarshal(raw, &next); err != nil {
		return current, err
	}
	tables := map[string][][]string{}
	for _, input := range inputs {
		p := PreviewStepInput(input)
		if !p.Valid {
			return current, fmt.Errorf("STEP_INPUT_INVALID")
		}
		if _, exists := tables[input.TableID]; exists {
			return current, fmt.Errorf("STEP_TABLE_DUPLICATE")
		}
		tables[input.TableID] = p.Rows
	}
	for _, name := range []string{"port_specs", "patch_paths"} {
		for _, input := range inputs {
			if input.TableID == name {
				var err error
				next, err = mergeConnectivityInput(next, input)
				if err != nil {
					return current, err
				}
			}
		}
	}
	for _, name := range []string{"route_nodes", "route_edges", "rack_access", "cable_specs", "cabling_rules"} {
		if len(tables[name]) == 0 {
			return current, fmt.Errorf("CABLE_INPUT_REQUIRED: %s", name)
		}
	}
	if len(tables["cabling_rules"]) != 1 {
		return current, fmt.Errorf("CABLE_RULE_SCOPE_AMBIGUOUS")
	}
	policy := tables["cabling_rules"][0]
	if policy[2] != "shortest_path" {
		return current, fmt.Errorf("CABLE_STRATEGY_UNSUPPORTED")
	}
	facilities := map[string]FacilityModel{}
	byID := map[ID]FacilityModel{}
	for _, f := range next.Facilities {
		if old, ok := facilities[f.Code]; ok && old.EntityID != f.EntityID {
			return current, fmt.Errorf("CABLE_LOCATION_AMBIGUOUS")
		}
		facilities[f.Code] = f
		byID[f.EntityID] = f
	}
	nodes := []ID{}
	nodeIDs := map[string]ID{}
	nodeRacks := map[ID]ID{}
	for _, r := range tables["route_nodes"] {
		if nodeIDs[r[0]] != "" {
			return current, fmt.Errorf("CABLE_NODE_DUPLICATE")
		}
		room, ok := facilities[r[1]]
		if !ok || room.Kind != "room" {
			return current, fmt.Errorf("CABLE_ROOM_REQUIRED")
		}
		id := StableID(next.ProjectID, "cabling-node", r[0])
		nodeIDs[r[0]] = id
		nodes = append(nodes, id)
		if r[2] != "" {
			rack, ok := facilities[r[2]]
			if !ok || rack.Kind != "rack" || rack.ParentID == nil || *rack.ParentID != room.EntityID {
				return current, fmt.Errorf("CABLE_RACK_ROOM_MISMATCH")
			}
			nodeRacks[id] = rack.EntityID
		}
	}
	edges := []CableRoutingEdge{}
	edgeIDs := map[string]bool{}
	for _, r := range tables["route_edges"] {
		if edgeIDs[r[0]] {
			return current, fmt.Errorf("CABLE_EDGE_DUPLICATE")
		}
		edgeIDs[r[0]] = true
		if nodeIDs[r[1]] == "" || nodeIDs[r[2]] == "" || nodeIDs[r[1]] == nodeIDs[r[2]] {
			return current, fmt.Errorf("CABLE_EDGE_INVALID")
		}
		measure, err := measure(Quantity{Value: r[3], Unit: r[4]})
		if err != nil || measure.dimension != "length" || measure.value.Sign() <= 0 {
			return current, fmt.Errorf("CABLE_EDGE_DISTANCE_INVALID")
		}
		edges = append(edges, CableRoutingEdge{FromID: nodeIDs[r[1]], ToID: nodeIDs[r[2]], Distance: Quantity{Value: r[3], Unit: r[4]}, Medium: r[5]})
	}
	type access struct {
		node ID
		rule string
	}
	accesses := map[ID]access{}
	for _, r := range tables["rack_access"] {
		rack, ok := facilities[r[0]]
		node := nodeIDs[r[1]]
		if !ok || rack.Kind != "rack" || node == "" || (nodeRacks[node] != "" && nodeRacks[node] != rack.EntityID) {
			return current, fmt.Errorf("CABLE_ACCESS_UNRESOLVED")
		}
		if _, ok := accesses[rack.EntityID]; ok {
			return current, fmt.Errorf("CABLE_ACCESS_AMBIGUOUS")
		}
		accesses[rack.EntityID] = access{node, r[2]}
	}
	placements := map[ID]RackPlacement{}
	for _, p := range next.Placements {
		if _, ok := placements[p.EquipmentID]; ok {
			return current, fmt.Errorf("CABLE_PLACEMENT_AMBIGUOUS")
		}
		placements[p.EquipmentID] = p
	}
	ports := map[ID]PlannedPort{}
	for _, p := range next.Ports {
		ports[p.EntityID] = p
	}
	version := func(kind, key string) EntityVersion {
		return EntityVersion{EntityID: StableID(next.ProjectID, kind, key), Maturity: Calculated, EvidenceIDs: append([]ID{}, evidence...), Attributes: []AttributeValue{}}
	}
	quantityInput := func(q Quantity) []NamedValue {
		v, u := q.Value, q.Unit
		return []NamedValue{{Name: "length", Value: TypedValue{Kind: "decimal", Decimal: &v, Unit: &u}}}
	}
	generated := []CablePlan{}
	optics := map[ID]OpticDemand{}
	for _, c := range next.Connections {
		path := append([]ID{c.APortID}, c.PatchPortIDs...)
		path = append(path, c.ZPortID)
		if len(path)%2 != 0 {
			return current, fmt.Errorf("CABLE_PATCH_INCOMPLETE")
		}
		for i := 0; i < len(path); i += 2 {
			a, aok := ports[path[i]]
			z, zok := ports[path[i+1]]
			if !aok || !zok || a.Medium == "" || a.Medium != z.Medium || a.Connector == "" || z.Connector == "" || a.RateBPS == nil || z.RateBPS == nil || *a.RateBPS != *z.RateBPS {
				return current, fmt.Errorf("CABLE_PORT_SPEC_REQUIRED")
			}
			aa, ok := accesses[placements[a.EquipmentID].RackID]
			if !ok {
				return current, fmt.Errorf("CABLE_ACCESS_REQUIRED")
			}
			za, ok := accesses[placements[z.EquipmentID].RackID]
			if !ok {
				return current, fmt.Errorf("CABLE_ACCESS_REQUIRED")
			}
			route := []RouteSegment{}
			distance := Quantity{Value: "0", Unit: "m"}
			var err error
			if aa.node != za.node {
				route, distance, err = ShortestCablePath(nodes, edges, aa.node, za.node, a.Medium)
				if err != nil {
					return current, err
				}
			}
			leads := []Quantity{}
			for _, ac := range []access{aa, za} {
				ref := ac.rule
				if ref == "" {
					ref = policy[3]
				}
				q, _, _, err := run(ref, []NamedValue{})
				if err != nil {
					return current, err
				}
				leads = append(leads, q)
			}
			base, err := ApplyCableAllowance(distance, leads, "1")
			if err != nil {
				return current, err
			}
			length, calc, rule, err := run(policy[4], quantityInput(base))
			if err != nil {
				return current, err
			}
			if _, err = ApplyCableAllowance(length, nil, "1"); err != nil {
				return current, err
			}
			stock, stockRun, _, err := run(policy[5], quantityInput(length))
			if err != nil {
				return current, err
			}
			type candidate struct {
				spec  CableSpec
				stock Quantity
				mode  string
			}
			matches := map[string]candidate{}
			for _, r := range tables["cable_specs"] {
				rate, err := cablingRate(r[4], r[5])
				if err != nil {
					return current, err
				}
				q := Quantity{Value: r[6], Unit: r[7]}
				if !EqualQuantities(stock, q) || r[1] != a.Medium || rate != *a.RateBPS || !((r[2] == a.Connector && r[3] == z.Connector) || (r[3] == a.Connector && r[2] == z.Connector)) {
					continue
				}
				key := r[0]
				v := candidate{CableSpec{ID: StableID(next.ProjectID, "cable-spec", key), Medium: r[1], ConnectorA: a.Connector, ConnectorZ: z.Connector, RateBPS: &rate, SourceModelLabel: key}, q, r[8]}
				if old, ok := matches[key]; ok && Digest([]any{old.spec, old.stock, old.mode}) != Digest([]any{v.spec, v.stock, v.mode}) {
					return current, fmt.Errorf("CABLE_SPEC_CONFLICT")
				}
				matches[key] = v
			}
			if len(matches) != 1 {
				return current, fmt.Errorf("CABLE_STOCK_SPEC_UNRESOLVED")
			}
			var chosen candidate
			for _, v := range matches {
				chosen = v
			}
			if _, err = SelectCableStock(length, []Quantity{chosen.stock}); err != nil {
				return current, err
			}
			segment := i/2 + 1
			key := c.EntityID + "/" + strconv.Itoa(segment)
			label := key
			cable := CablePlan{EntityVersion: version("standard-cable", key), SegmentIndex: &segment, Label: &label, ConnectionID: c.EntityID, Spec: chosen.spec, Route: route, RuleVersion: &rule, Calculated: &LengthEvidence{Length: length, EvidenceIDs: append([]ID{}, evidence...), CalculationRunID: &calc}, SelectedStock: &LengthEvidence{Length: chosen.stock, EvidenceIDs: append([]ID{}, evidence...), CalculationRunID: &stockRun}}
			for _, old := range current.Cables {
				if old.ConnectionID == c.EntityID && (old.SegmentIndex == nil && segment == 1 || old.SegmentIndex != nil && *old.SegmentIndex == segment) {
					cable.EntityVersion = old.EntityVersion
					cable.Maturity = Calculated
					cable.Confirmed = old.Confirmed
					cable.AsBuilt = old.AsBuilt
				}
			}
			generated = append(generated, cable)
			switch chosen.mode {
			case "separate-optics":
				for _, endpoint := range []PlannedPort{a, z} {
					seen := map[ID]bool{}
					for endpoint.ParentPortID != nil {
						id := *endpoint.ParentPortID
						if seen[id] {
							return current, fmt.Errorf("CABLE_BREAKOUT_CYCLE")
						}
						seen[id] = true
						var ok bool
						endpoint, ok = ports[id]
						if !ok {
							return current, fmt.Errorf("CABLE_PARENT_PORT_REQUIRED")
						}
					}
					if endpoint.RateBPS == nil || endpoint.Connector == "" || endpoint.Medium == "" {
						return current, fmt.Errorf("CABLE_PARENT_SPEC_REQUIRED")
					}
					models := map[string]bool{}
					for _, r := range tables["optic_specs"] {
						rate, err := cablingRate(r[3], r[4])
						if err != nil {
							return current, err
						}
						if r[1] == endpoint.Connector && r[2] == endpoint.Medium && rate == *endpoint.RateBPS {
							if _, err := SelectCableStock(chosen.stock, []Quantity{{Value: r[5], Unit: r[6]}}); err == nil {
								models[r[0]] = true
							}
						}
					}
					if len(models) != 1 {
						return current, fmt.Errorf("CABLE_OPTIC_SPEC_UNRESOLVED")
					}
					model := ""
					for name := range models {
						model = name
					}
					if old, ok := optics[endpoint.EntityID]; ok && old.SourceModelLabel != model {
						return current, fmt.Errorf("CABLE_OPTIC_CONFLICT")
					}
					optics[endpoint.EntityID] = OpticDemand{EntityVersion: version("standard-optic", endpoint.EntityID), PhysicalPortID: endpoint.EntityID, SourceModelLabel: model, Quantity: 1}
				}
			case "no-optics", "DAC", "AOC":
			default:
				return current, fmt.Errorf("CABLE_OPTIC_MODE_REQUIRED")
			}
		}
	}
	next.Cables = generated
	next.Optics = []OpticDemand{}
	for _, p := range next.Ports {
		if o, ok := optics[p.EntityID]; ok {
			next.Optics = append(next.Optics, o)
		}
	}
	// Derived planning quantities never create purchased or observed assets.
	counts := map[string]int{}
	lengths := map[string]Quantity{}
	specs := map[string]ID{}
	for _, c := range generated {
		key := c.Spec.SourceModelLabel + "/" + c.SelectedStock.Length.Value + c.SelectedStock.Length.Unit
		counts[key]++
		lengths[key] = c.SelectedStock.Length
		specs[key] = c.Spec.ID
	}
	for _, o := range next.Optics {
		key := o.SourceModelLabel
		counts[key]++
		specs[key] = StableID(next.ProjectID, "optic-spec", key)
	}
	// Retire only quantities generated by this importer when the plan changes.
	// Purchased stock remains recorded even when the current required count is zero.
	materials := []MaterialEstimate{}
	for _, v := range next.Materials {
		if v.EntityID == StableID(next.ProjectID, "standard-material", v.SourceModelLabel) {
			if _, ok := counts[v.SourceModelLabel]; !ok {
				if v.Purchased == nil && v.Spares == nil {
					continue
				}
				v.Required = Quantity{Value: "0", Unit: "count"}
				v.RequiredCalculationRunID = nil
				v.PurchaseGap = nil
				v.CalculationRunID = nil
			}
		}
		materials = append(materials, v)
	}
	next.Materials = materials
	workloads := []WorkloadEstimate{}
	for _, v := range next.Workloads {
		if v.EntityID == StableID(next.ProjectID, "standard-workload", v.SourceModelLabel) {
			if _, ok := lengths[v.SourceModelLabel]; !ok {
				continue
			}
		}
		workloads = append(workloads, v)
	}
	next.Workloads = workloads
	keys := []string{}
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		q := Quantity{Value: strconv.Itoa(counts[key]), Unit: "count"}
		id := version("standard-material", key)
		mat := MaterialEstimate{EntityVersion: id, SpecRef: specs[key], SourceModelLabel: key, Required: q}
		found := false
		for i, v := range next.Materials {
			if v.EntityID == id.EntityID {
				mat.Purchased = v.Purchased
				mat.Spares = v.Spares
				next.Materials[i] = mat
				found = true
			}
		}
		if !found {
			next.Materials = append(next.Materials, mat)
		}
		if l, ok := lengths[key]; ok {
			v := WorkloadEstimate{EntityVersion: version("standard-workload", key), SourceModelLabel: key, WorkType: "cable_installation", SpecRef: &mat.SpecRef, Quantity: q, StandardLength: &l}
			found := false
			for i, old := range next.Workloads {
				if old.EntityID == v.EntityID {
					next.Workloads[i] = v
					found = true
				}
			}
			if !found {
				next.Workloads = append(next.Workloads, v)
			}
		}
	}
	return next, nil
}

func cablingRate(value, unit string) (int64, error) {
	scales := map[string]int64{"bps": 1, "kbps": 1000, "Mbps": 1000000, "Gbps": 1000000000, "Tbps": 1000000000000}
	scale, ok := scales[strings.TrimSpace(unit)]
	if !ok {
		return 0, fmt.Errorf("CABLE_RATE_UNIT_INVALID")
	}
	n, ok := new(big.Rat).SetString(value)
	if !ok || n.Sign() <= 0 {
		return 0, fmt.Errorf("CABLE_RATE_INVALID")
	}
	n.Mul(n, new(big.Rat).SetInt64(scale))
	if !n.IsInt() || !n.Num().IsInt64() {
		return 0, fmt.Errorf("CABLE_RATE_INVALID")
	}
	return n.Num().Int64(), nil
}
