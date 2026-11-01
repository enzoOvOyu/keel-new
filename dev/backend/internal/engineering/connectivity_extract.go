package engineering

import (
	"math/big"
	"sort"
	"strconv"
	"strings"
)

func extractConnectivity(project, source, revision string, rows []rowData, out *Extraction, version func(string, rowData, ...string) EntityVersion, issue func(string, string, string, string, string)) {
	equipment := map[string]int{}
	ports := map[string]int{}
	connections := map[string]int{}
	cables := map[string]int{}
	for i, e := range out.Model.Equipment {
		equipment[e.EntityID] = i
	}
	ensureEquipment := func(label string, r rowData, field string) string {
		id := StableID(project, "equipment", label)
		if label != "" {
			if _, ok := equipment[id]; !ok {
				equipment[id] = len(out.Model.Equipment)
				out.Model.Equipment = append(out.Model.Equipment, EquipmentPlan{EntityVersion: version(id, r, field), EquipmentType: "device", Label: label, Quantity: 1, IdentityKind: "planned_instance"})
			}
		}
		return id
	}
	var port func(string, string, string, string, rowData, string, string) string
	port = func(device, name, parent, index string, r rowData, deviceField, portField string) string {
		owner := ensureEquipment(device, r, deviceField)
		id := StableID(project, "port", owner+"/"+name)
		p := PlannedPort{EntityVersion: version(id, r, deviceField, portField, "parent_port", "a_parent_port", "z_parent_port", "breakout_index", "rate_bps", "medium", "role"), EquipmentID: owner, Name: name, Role: r.values["role"], Medium: r.values["medium"]}
		if device == "" || name == "" {
			issue(id, "PORT_IDENTITY_INCOMPLETE", portField, "设备与物理端口必须有明确来源", ConnectivityTarget)
		}
		if rate := r.values["rate_bps"]; rate != "" {
			p.RateBPS = parseRate(rate, r.units["rate_bps"])
			if p.RateBPS == nil {
				issue(id, "PORT_RATE_UNRESOLVED", "rate_bps", "速率或单位无法识别", ConnectivityTarget)
			}
		}
		if parent == name && parent != "" {
			issue(id, "BREAKOUT_CYCLE", "parent_port", "端口不能将自身作为父端口", ConnectivityTarget)
		}
		if parent != "" && parent != name {
			empty := rowData{kind: "port", values: map[string]string{}, cells: r.cells, units: map[string]string{}}
			root := port(device, parent, "", "", empty, deviceField, "parent_port")
			p.ParentPortID = &root
			if n, e := strconv.Atoi(index); e == nil {
				p.BreakoutIndex = &n
			}
		}
		if i, ok := ports[id]; ok {
			old := &out.Model.Ports[i]
			old.EvidenceIDs = append(old.EvidenceIDs, p.EvidenceIDs...)
			if old.ParentPortID != nil && p.ParentPortID != nil && *old.ParentPortID != *p.ParentPortID {
				issue(id, "PORT_PARENT_CONFLICT", "parent_port", "同一端口父级冲突", ConnectivityTarget)
			}
			if p.ParentPortID != nil {
				old.ParentPortID = p.ParentPortID
			}
			if p.BreakoutIndex != nil {
				old.BreakoutIndex = p.BreakoutIndex
			}
			if old.RateBPS != nil && p.RateBPS != nil && *old.RateBPS != *p.RateBPS {
				issue(id, "PORT_RATE_CONFLICT", "rate_bps", "同一端口速率冲突", ConnectivityTarget)
			} else if p.RateBPS != nil {
				old.RateBPS = p.RateBPS
			}
			if old.Medium != "" && p.Medium != "" && old.Medium != p.Medium {
				issue(id, "PORT_MEDIUM_CONFLICT", "medium", "同一端口介质冲突", ConnectivityTarget)
			} else if p.Medium != "" {
				old.Medium = p.Medium
			}
			if p.Role != "" {
				old.Role = p.Role
			}
		} else {
			ports[id] = len(out.Model.Ports)
			out.Model.Ports = append(out.Model.Ports, p)
		}
		return id
	}
	type demand struct {
		port, label string
		row         rowData
		field       string
	}
	demands := []demand{}
	quantity := func(r rowData, field, unitField string) *Quantity {
		value := r.values[field]
		if value == "" {
			return nil
		}
		unit := r.values[unitField]
		if unit == "" {
			unit = r.units[field]
		}
		if !decimal(value) || unit == "" {
			return nil
		}
		return &Quantity{Value: value, Unit: unit}
	}
	for _, r := range rows {
		v := r.values
		if r.kind == "port" {
			port(v["device"], v["port"], v["parent_port"], v["breakout_index"], r, "device", "port")
			continue
		}
		if r.kind != "connection" && r.kind != "cable" {
			continue
		}
		a := port(v["a_device"], v["a_port"], v["a_parent_port"], "", r, "a_device", "a_port")
		z := port(v["z_device"], v["z_port"], v["z_parent_port"], "", r, "z_device", "z_port")
		pair := []string{a, z}
		sort.Strings(pair)
		id := StableID(project, "connection", strings.Join(pair, "/"))
		c := PlannedConnection{EntityVersion: version(id, r, "a_device", "a_port", "z_device", "z_port", "patch_device", "patch_port", "patch_exit_port"), APortID: a, ZPortID: z, PatchPortIDs: []string{}}
		if v["patch_device"] != "" || v["patch_port"] != "" || v["patch_exit_port"] != "" {
			c.PatchPortIDs = append(c.PatchPortIDs, port(v["patch_device"], v["patch_port"], "", "", r, "patch_device", "patch_port"))
			if v["patch_exit_port"] != "" {
				c.PatchPortIDs = append(c.PatchPortIDs, port(v["patch_device"], v["patch_exit_port"], "", "", r, "patch_device", "patch_exit_port"))
			}
		}
		if i, ok := connections[id]; ok {
			out.Model.Connections[i].EvidenceIDs = append(out.Model.Connections[i].EvidenceIDs, c.EvidenceIDs...)
			for _, p := range c.PatchPortIDs {
				known := false
				for _, old := range out.Model.Connections[i].PatchPortIDs {
					known = known || old == p
				}
				if !known {
					out.Model.Connections[i].PatchPortIDs = append(out.Model.Connections[i].PatchPortIDs, p)
				}
			}
		} else {
			connections[id] = len(out.Model.Connections)
			out.Model.Connections = append(out.Model.Connections, c)
		}
		if v["a_optic"] != "" {
			demands = append(demands, demand{a, v["a_optic"], r, "a_optic"})
		}
		if v["z_optic"] != "" {
			demands = append(demands, demand{z, v["z_optic"], r, "z_optic"})
		}
		if r.kind != "cable" {
			continue
		}
		var segment *int
		cableKey := id
		if v["segment_index"] != "" {
			n, err := strconv.Atoi(strings.TrimSpace(v["segment_index"]))
			if err != nil || n < 1 {
				issue(id, "CABLE_SEGMENT_INVALID", "segment_index", "物理线段序号必须是正整数", CablingTarget)
			} else {
				segment = &n
				if n != 1 {
					cableKey += "/segment/" + strconv.Itoa(n)
				}
			}
		}
		cableID := StableID(project, "cable-plan", cableKey)
		spec := CableSpec{ID: StableID(project, "cable-spec", strings.Join([]string{v["cable_model"], v["medium"], v["connector_a"], v["connector_z"], v["rate_bps"]}, "/")), SourceModelLabel: v["cable_model"], Medium: v["medium"], ConnectorA: v["connector_a"], ConnectorZ: v["connector_z"], RateBPS: parseRate(v["rate_bps"], r.units["rate_bps"])}
		cable := CablePlan{EntityVersion: version(cableID, r, "a_device", "a_port", "z_device", "z_port", "cable_label", "segment_index", "cable_model", "medium", "connector_a", "connector_z", "rate_bps", "length", "length_unit", "length_state", "standard_length", "standard_length_unit", "route_from", "route_to", "distance", "distance_unit"), ConnectionID: id, SegmentIndex: segment, Spec: spec, Route: []RouteSegment{}}
		if v["cable_label"] != "" {
			label := v["cable_label"]
			cable.Label = &label
		}
		if q := quantity(r, "length", "length_unit"); q != nil {
			length := &LengthEvidence{Length: *q, EvidenceIDs: append([]string{}, cable.EvidenceIDs...)}
			formula := false
			for _, cell := range r.cells["length"] {
				formula = formula || cell.Formula != ""
			}
			switch v["length_state"] {
			case "as_built":
				if formula {
					issue(cableID, "CALCULATED_NOT_AS_BUILT", "length_state", "公式计算长度不能标为实际安装长度", CablingTarget)
				} else {
					cable.AsBuilt = length
				}
			case "confirmed":
				cable.Confirmed = length
			default:
				if formula {
					cable.Calculated = length
				} else {
					cable.Estimated = length
				}
			}
		} else if v["length"] != "" {
			issue(cableID, "CABLE_LENGTH_UNIT_REQUIRED", "length", "长度数值与单位未明确", CablingTarget)
		}
		if q := quantity(r, "standard_length", "standard_length_unit"); q != nil {
			cable.SelectedStock = &LengthEvidence{Length: *q, EvidenceIDs: append([]string{}, cable.EvidenceIDs...)}
		} else if v["standard_length"] != "" {
			issue(cableID, "CABLE_STOCK_UNIT_REQUIRED", "standard_length", "标准规格的数值与单位未明确", CablingTarget)
		}
		if v["route_from"] != "" || v["route_to"] != "" {
			from, to := StableID(project, "route-point", v["route_from"]), StableID(project, "route-point", v["route_to"])
			cable.Route = append(cable.Route, RouteSegment{FromID: from, ToID: to, Distance: quantity(r, "distance", "distance_unit")})
			if v["route_from"] == "" || v["route_to"] == "" {
				issue(cableID, "ROUTE_ENDPOINT_REQUIRED", "route_from", "路径段的两端必须明确", CablingTarget)
			}
		}
		if i, ok := cables[cableID]; ok {
			old := &out.Model.Cables[i]
			old.EvidenceIDs = append(old.EvidenceIDs, cable.EvidenceIDs...)
			old.Route = append(old.Route, cable.Route...)
			if Digest(old.Spec) != Digest(spec) {
				issue(cableID, "CABLE_SPEC_CONFLICT", "cable_model", "同一连接的线缆规格存在冲突", CablingTarget)
			}
			for _, pair := range [][2]**LengthEvidence{{&old.Estimated, &cable.Estimated}, {&old.Calculated, &cable.Calculated}, {&old.SelectedStock, &cable.SelectedStock}, {&old.Confirmed, &cable.Confirmed}, {&old.AsBuilt, &cable.AsBuilt}} {
				if *pair[1] != nil {
					if *pair[0] != nil && Digest((*pair[0]).Length) != Digest((*pair[1]).Length) {
						issue(cableID, "CABLE_LENGTH_CONFLICT", "length", "同一连接的长度来源冲突", CablingTarget)
					} else {
						*pair[0] = *pair[1]
					}
				}
			}
		} else {
			cables[cableID] = len(out.Model.Cables)
			out.Model.Cables = append(out.Model.Cables, cable)
		}
	}
	seen := map[string]int{}
	for _, d := range demands {
		root := d.port
		visited := map[string]bool{}
		for {
			i, ok := ports[root]
			if !ok || visited[root] || out.Model.Ports[i].ParentPortID == nil {
				break
			}
			visited[root] = true
			root = *out.Model.Ports[i].ParentPortID
		}
		id := StableID(project, "optic-demand", root)
		v := version(id, d.row, d.field)
		if i, ok := seen[root]; ok {
			old := &out.Model.Optics[i]
			old.EvidenceIDs = append(old.EvidenceIDs, v.EvidenceIDs...)
			if old.SourceModelLabel != d.label {
				issue(id, "OPTIC_PHYSICAL_PORT_CONFLICT", d.field, "同一物理端口的模块规格冲突", CablingTarget)
			}
		} else {
			seen[root] = len(out.Model.Optics)
			out.Model.Optics = append(out.Model.Optics, OpticDemand{EntityVersion: v, PhysicalPortID: root, SourceModelLabel: d.label, Quantity: 1})
		}
	}
}

func parseRate(value, unit string) *int64 {
	if value == "" {
		return nil
	}
	v := strings.TrimSpace(value)
	u := strings.ToLower(strings.TrimSpace(unit))
	scales := map[string]int64{"gbps": 1000000000, "gbit/s": 1000000000, "g": 1000000000, "mbps": 1000000, "mbit/s": 1000000, "m": 1000000, "kbps": 1000, "kbit/s": 1000, "k": 1000, "bps": 1, "bit/s": 1}
	if u == "" {
		for _, suffix := range []string{"gbit/s", "mbit/s", "kbit/s", "gbps", "mbps", "kbps", "bit/s", "bps", "g", "m", "k"} {
			if strings.HasSuffix(strings.ToLower(v), suffix) {
				u = suffix
				break
			}
		}
	}
	scale, ok := scales[u]
	if !ok {
		return nil
	}
	if strings.HasSuffix(strings.ToLower(v), u) {
		v = strings.TrimSpace(v[:len(v)-len(u)])
	}
	if !decimal(v) {
		return nil
	}
	n, ok := new(big.Rat).SetString(v)
	if !ok {
		return nil
	}
	n.Mul(n, new(big.Rat).SetInt64(scale))
	if !n.IsInt() || !n.Num().IsInt64() || n.Sign() <= 0 {
		return nil
	}
	result := n.Num().Int64()
	return &result
}
