package engineering

import (
	"fmt"
	"github.com/google/uuid"
	"net"
)

func ValidateConnectivity(m PhysicalModel) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(v EntityVersion, code, field, message, target string) {
		issues = append(issues, ValidationIssue{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(v.RevisionID+v.EntityID+code+field)).String(), ProjectID: m.ProjectID, RevisionID: v.RevisionID, EntityID: v.EntityID, Code: code, Severity: "BLOCK", ModuleID: "connectivity_cabling", Field: field, Message: message, PublishTarget: target, EvidenceIDs: append([]string{}, v.EvidenceIDs...), Resolution: "open"})
	}
	equipment := map[string]EquipmentPlan{}
	ports := map[string]PlannedPort{}
	connections := map[string]PlannedConnection{}
	for _, e := range m.Equipment {
		equipment[e.EntityID] = e
	}
	for _, p := range m.Ports {
		ports[p.EntityID] = p
	}
	for _, c := range m.Connections {
		if len(c.PatchPortIDs)%2 != 0 {
			add(c.EntityVersion, "PATCH_PASSTHROUGH_INCOMPLETE", "patch_port_ids", "每个配线节点必须明确入口和出口端口", ConnectivityTarget)
		}
		for i := 0; i+1 < len(c.PatchPortIDs); i += 2 {
			a, z := ports[c.PatchPortIDs[i]], ports[c.PatchPortIDs[i+1]]
			if a.EquipmentID != z.EquipmentID {
				add(c.EntityVersion, "PATCH_PASSTHROUGH_OWNER_MISMATCH", "patch_port_ids", "贯通两端必须属于同一配线设备", ConnectivityTarget)
			}
		}

		connections[c.EntityID] = c
	}
	names := map[string]bool{}
	indices := map[string]map[int]bool{}
	for _, p := range m.Ports {
		eq, ok := equipment[p.EquipmentID]
		if !ok || eq.IdentityKind == "group" || p.Name == "" {
			add(p.EntityVersion, "PORT_OWNER_UNRESOLVED", "equipment_id", "端口必须关联唯一设备实例和来源名称", ConnectivityTarget)
		}
		if p.MAC != nil {
			mac, err := net.ParseMAC(*p.MAC)
			if err != nil || len(mac) != 6 {
				add(p.EntityVersion, "PORT_MAC_INVALID", "mac", "端口MAC必须为48位地址", ConnectivityTarget)
			}
		}
		key := p.EquipmentID + "/" + p.Name
		if names[key] {
			add(p.EntityVersion, "DUPLICATE_PORT_NAME", "name", "同一设备端口名称重复", ConnectivityTarget)
		}
		names[key] = true
		if p.RateBPS != nil && *p.RateBPS <= 0 {
			add(p.EntityVersion, "PORT_RATE_INVALID", "rate_bps", "端口速率必须为正值", ConnectivityTarget)
		}
		if p.ParentPortID != nil {
			root, ok := ports[*p.ParentPortID]
			if !ok || root.EquipmentID != p.EquipmentID {
				add(p.EntityVersion, "BREAKOUT_PARENT_UNRESOLVED", "parent_port_id", "父端口需属于同一设备", ConnectivityTarget)
			}
			seen := map[string]bool{p.EntityID: true}
			current := p.ParentPortID
			for current != nil {
				if seen[*current] {
					add(p.EntityVersion, "BREAKOUT_CYCLE", "parent_port_id", "Breakout父端口存在循环", ConnectivityTarget)
					break
				}
				seen[*current] = true
				next, exists := ports[*current]
				if !exists {
					break
				}
				current = next.ParentPortID
			}
			if p.BreakoutIndex != nil {
				if indices[*p.ParentPortID] == nil {
					indices[*p.ParentPortID] = map[int]bool{}
				}
				if *p.BreakoutIndex < 1 || indices[*p.ParentPortID][*p.BreakoutIndex] {
					add(p.EntityVersion, "BREAKOUT_INDEX_CONFLICT", "breakout_index", "同一父端口的子端口序号必须唯一且为正", ConnectivityTarget)
				}
				indices[*p.ParentPortID][*p.BreakoutIndex] = true
			}
		} else if p.BreakoutIndex != nil {
			add(p.EntityVersion, "BREAKOUT_PARENT_REQUIRED", "parent_port_id", "声明子端口序号时需要父端口", ConnectivityTarget)
		}
	}
	used := map[string]string{}
	for _, c := range m.Connections {
		if len(c.PatchPortIDs)%2 != 0 {
			add(c.EntityVersion, "PATCH_PASSTHROUGH_INCOMPLETE", "patch_port_ids", "每个配线节点必须明确入口和出口端口", ConnectivityTarget)
		}
		for i := 0; i+1 < len(c.PatchPortIDs); i += 2 {
			a, z := ports[c.PatchPortIDs[i]], ports[c.PatchPortIDs[i+1]]
			if a.EquipmentID != z.EquipmentID {
				add(c.EntityVersion, "PATCH_PASSTHROUGH_OWNER_MISMATCH", "patch_port_ids", "贯通两端必须属于同一配线设备", ConnectivityTarget)
			}
		}

		path := append([]string{c.APortID}, c.PatchPortIDs...)
		path = append(path, c.ZPortID)
		seen := map[string]bool{}
		for _, id := range path {
			if _, ok := ports[id]; !ok {
				add(c.EntityVersion, "CONNECTION_PORT_UNRESOLVED", "a_port_id", "路径端口不存在", ConnectivityTarget)
			}
			if seen[id] {
				add(c.EntityVersion, "CONNECTION_PATH_CYCLE", "patch_port_ids", "连接路径重复经过同一端口", ConnectivityTarget)
			}
			seen[id] = true
			if previous, ok := used[id]; ok && previous != c.EntityID {
				add(c.EntityVersion, "PORT_MULTIPLE_CONNECTIONS", "a_port_id", "同一物理端口被多个连接占用", ConnectivityTarget)
			}
			used[id] = c.EntityID
		}
	}
	for port, connection := range used {
		seen := map[string]bool{}
		p := ports[port]
		for p.ParentPortID != nil && !seen[*p.ParentPortID] {
			seen[*p.ParentPortID] = true
			if parentConnection, ok := used[*p.ParentPortID]; ok && parentConnection != connection {
				add(connections[connection].EntityVersion, "BREAKOUT_PARENT_OCCUPIED", "a_port_id", "父物理端口与Breakout子端口不能独立重复接线", ConnectivityTarget)
			}
			p = ports[*p.ParentPortID]
		}
	}
	cableFor := map[string]bool{}
	for _, c := range m.Cables {
		connection, ok := connections[c.ConnectionID]
		if !ok {
			add(c.EntityVersion, "CABLE_CONNECTION_UNRESOLVED", "connection_id", "线缆规划缺少端接关系", CablingTarget)
		}
		segment := 1
		if c.SegmentIndex != nil {
			segment = *c.SegmentIndex
		}
		if segment < 1 || segment > 1+len(connection.PatchPortIDs)/2 || (len(connection.PatchPortIDs) > 0 && c.SegmentIndex == nil) {
			add(c.EntityVersion, "CABLE_SEGMENT_REQUIRED", "segment_index", "带配线节点的连接必须明确逐物理线段序号", CablingTarget)
		}
		key := fmt.Sprintf("%s/%d", c.ConnectionID, segment)
		if cableFor[key] {
			add(c.EntityVersion, "DUPLICATE_CABLE_PLAN", "connection_id", "同一物理线段有多个线缆规划", CablingTarget)
		}
		cableFor[key] = true
		if c.Spec.ID == "" || (c.Spec.Medium == "" && c.Spec.SourceModelLabel == "") {
			add(c.EntityVersion, "CABLE_SPEC_REQUIRED", "spec", "缺少明确线缆规格或介质", CablingTarget)
		}
		path := append([]string{connection.APortID}, connection.PatchPortIDs...)
		path = append(path, connection.ZPortID)
		endpoints := []string{}
		if offset := 2 * (segment - 1); offset >= 0 && offset+1 < len(path) {
			endpoints = path[offset : offset+2]
		}
		for _, portID := range endpoints {
			p := ports[portID]
			if c.Spec.Medium != "" && p.Medium != "" && c.Spec.Medium != p.Medium {
				add(c.EntityVersion, "CABLE_MEDIUM_MISMATCH", "spec", "线缆与端口介质不匹配", CablingTarget)
			}
			if c.Spec.RateBPS != nil && p.RateBPS != nil && *c.Spec.RateBPS < *p.RateBPS {
				add(c.EntityVersion, "CABLE_RATE_MISMATCH", "spec", "线缆规格速率低于端口规划速率", CablingTarget)
			}
		}
		for name, length := range map[string]*LengthEvidence{"estimated": c.Estimated, "calculated": c.Calculated, "selected_stock": c.SelectedStock, "confirmed": c.Confirmed, "as_built": c.AsBuilt} {
			if length == nil {
				continue
			}
			v, e := measure(length.Length)
			if e != nil || v.dimension != "length" || v.value.Sign() < 0 {
				add(c.EntityVersion, "CABLE_LENGTH_INVALID", name, "线缆长度必须是非负长度量", CablingTarget)
			}
			if len(length.EvidenceIDs) == 0 {
				add(c.EntityVersion, "CABLE_LENGTH_EVIDENCE_REQUIRED", name, "线缆长度必须有来源依据", CablingTarget)
			}
			if name == "calculated" && length.CalculationRunID == nil {
				add(c.EntityVersion, "CABLE_CALCULATION_REQUIRED", name, "计算长度必须关联计算运行", CablingTarget)
			}
			if name == "as_built" && length.CalculationRunID != nil {
				add(c.EntityVersion, "CALCULATED_NOT_AS_BUILT", name, "计算记录不能当成实际安装长度依据", CablingTarget)
			}
		}
		needed := c.Calculated
		if needed == nil {
			needed = c.Confirmed
		}
		if needed == nil {
			needed = c.Estimated
		}
		if needed != nil && c.SelectedStock != nil {
			a, ea := measure(needed.Length)
			b, eb := measure(c.SelectedStock.Length)
			if ea == nil && eb == nil && a.dimension == b.dimension && a.value.Cmp(b.value) > 0 {
				add(c.EntityVersion, "CABLE_STOCK_TOO_SHORT", "selected_stock", "标准线长短于工程需求", CablingTarget)
			}
		}
		for i, r := range c.Route {
			if r.FromID == "" || r.ToID == "" || r.FromID == r.ToID || (i > 0 && c.Route[i-1].ToID != r.FromID) {
				add(c.EntityVersion, "CABLE_ROUTE_DISCONTINUOUS", "route", "线缆路径段缺失或不连续", CablingTarget)
			}
			if r.Distance != nil {
				v, e := measure(*r.Distance)
				if e != nil || v.dimension != "length" || v.value.Sign() < 0 {
					add(c.EntityVersion, "CABLE_ROUTE_DISTANCE_INVALID", "route", "路径距离必须是非负长度量", CablingTarget)
				}
			}
		}
	}
	optics := map[string]bool{}
	for _, o := range m.Optics {
		p, ok := ports[o.PhysicalPortID]
		if !ok || p.ParentPortID != nil {
			add(o.EntityVersion, "OPTIC_PHYSICAL_PORT_REQUIRED", "physical_port_id", "光模块需求必须关联父物理端口", CablingTarget)
		}
		if o.SourceModelLabel == "" || o.Quantity != 1 || optics[o.PhysicalPortID] {
			add(o.EntityVersion, "OPTIC_DEMAND_CONFLICT", "quantity", "每个物理端口只计一只且规格需明确", CablingTarget)
		}
		optics[o.PhysicalPortID] = true
	}
	return issues
}
