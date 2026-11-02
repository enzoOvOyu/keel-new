package engineering

import (
	"net/netip"
	"strings"
)

func NetworkHasContent(p NetworkPlan) bool {
	return len(p.Fabrics)+len(p.Planes)+len(p.DeviceRoles)+len(p.InterfaceMappings)+len(p.Allocations)+len(p.BGP)+len(p.BFD)+len(p.QoS)+len(p.AdaptiveRouting)+len(p.EVPNVXLAN) > 0
}
func ValidateNetwork(p NetworkPlan, physical PhysicalModel) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(code, field, message string) {
		issues = append(issues, ValidationIssue{ID: StableID(physical.ProjectID, "network-issue", p.EntityID+"/"+code+"/"+field), EntityID: p.EntityID, ProjectID: physical.ProjectID, RevisionID: p.RevisionID, ModuleID: "network_design", Code: code, Severity: "BLOCK", Field: field, Message: message, PublishTarget: NetworkTarget, EvidenceIDs: p.EvidenceIDs})
	}
	if !NetworkHasContent(p) {
		return issues
	}
	if p.PhysicalRevisionID == "" {
		add("NETWORK_PHYSICAL_REVISION_REQUIRED", "physical_revision_id", "网络规划需明确引用已发布物理版本")
	}
	if len(p.SourceIDs) == 0 {
		add("NETWORK_SOURCE_REQUIRED", "source_ids", "逻辑配置必须有规划来源")
	}
	equipment := map[string]bool{}
	ports := map[string]PlannedPort{}
	for _, v := range physical.Equipment {
		equipment[v.EntityID] = true
	}
	for _, v := range physical.Ports {
		ports[v.EntityID] = v
	}
	fabrics := map[string]bool{}
	planes := map[string]NetworkPlane{}
	interfaces := map[string]InterfaceMapping{}
	allocations := map[string]NetworkAllocation{}
	bgp := map[string]bool{}
	identities := map[string]bool{}
	check := func(id string, ev []string) {
		if identities[id] {
			add("NETWORK_IDENTITY_DUPLICATE", id, "网络对象身份重复")
		}
		identities[id] = true
		if id == "" || len(ev) == 0 {
			add("NETWORK_FACT_EVIDENCE_REQUIRED", id, "网络对象必须有明确身份及来源证据")
		}
	}
	for _, v := range p.Fabrics {
		check(v.ID, v.EvidenceIDs)
		if fabrics[v.ID] || v.Name == "" {
			add("NETWORK_FABRIC_INVALID", v.ID, "Fabric名称缺失或身份重复")
		}
		fabrics[v.ID] = true
	}
	for _, v := range p.Planes {
		check(v.ID, v.EvidenceIDs)
		if _, ok := planes[v.ID]; ok || !fabrics[v.FabricID] || v.Name == "" {
			add("NETWORK_PLANE_INVALID", v.ID, "网络平面名称或Fabric引用无效")
		}
		planes[v.ID] = v
	}
	roleKeys := map[string]bool{}
	for _, v := range p.DeviceRoles {
		key := v.EquipmentID + "/" + v.PlaneID
		if roleKeys[key] {
			add("NETWORK_ROLE_DUPLICATE", key, "同一设备在同一平面存在多条角色定义")
		}
		roleKeys[key] = true
		_, ok := planes[v.PlaneID]
		if !equipment[v.EquipmentID] || !ok || v.Role == "" || len(v.EvidenceIDs) == 0 {
			add("NETWORK_ROLE_UNRESOLVED", v.EquipmentID, "设备角色、物理设备或网络平面未明确")
		}
	}
	for _, v := range p.InterfaceMappings {
		check(v.ID, v.EvidenceIDs)
		_, plane := planes[v.PlaneID]
		_, port := ports[v.PhysicalPortID]
		_, duplicate := interfaces[v.ID]
		if !plane || !port || duplicate || v.LogicalName == "" {
			add("NETWORK_INTERFACE_UNRESOLVED", v.ID, "逻辑接口缺少唯一物理端口、平面或接口名称")
		}
		interfaces[v.ID] = v
	}
	addresses := map[string]bool{}
	for _, v := range p.Allocations {
		check(v.ID, v.EvidenceIDs)
		_, duplicate := allocations[v.ID]
		allocations[v.ID] = v
		if duplicate || !equipment[v.EntityID] {
			add("NETWORK_ALLOCATION_OWNER_INVALID", v.ID, "地址或ASN分配缺少唯一设备")
		}
		switch v.Kind {
		case "ip":
			if v.Address == nil || v.InterfaceID == nil || v.PrefixLength == nil {
				add("NETWORK_IP_INCOMPLETE", v.ID, "IP分配需明确地址、前缀长度和接口")
				continue
			}
			address, e := netip.ParseAddr(*v.Address)
			mapping, ok := interfaces[*v.InterfaceID]
			if e != nil || !ok || *v.PrefixLength < 0 || *v.PrefixLength > address.BitLen() || ports[mapping.PhysicalPortID].EquipmentID != v.EntityID {
				add("NETWORK_IP_INVALID", v.ID, "IP、前缀或接口归属无效")
				continue
			}
			vrf := ""
			if v.VRFRef != nil {
				vrf = *v.VRFRef
			}
			key := mapping.PlaneID + "/" + vrf + "/" + address.String()
			if addresses[key] {
				add("NETWORK_IP_DUPLICATE", v.ID, "同一平面和VRF内IP重复")
			}
			addresses[key] = true
		case "asn":
			if v.ASN == nil || *v.ASN < 1 || *v.ASN > 4294967295 {
				add("NETWORK_ASN_INVALID", v.ID, "ASN需明确且在32位合法范围")
			}
		default:
			add("NETWORK_ALLOCATION_KIND_REQUIRED", v.ID, "分配类型必须明确为ip或asn")
		}
	}
	for _, v := range p.BGP {
		check(v.ID, v.EvidenceIDs)
		a, ao := interfaces[v.AInterfaceID]
		z, zo := interfaces[v.ZInterfaceID]
		local, lo := allocations[v.LocalASNAllocationID]
		peer, po := allocations[v.PeerASNAllocationID]
		if bgp[v.ID] || !ao || !zo || v.AInterfaceID == v.ZInterfaceID || !lo || !po || local.Kind != "asn" || peer.Kind != "asn" || local.EntityID != ports[a.PhysicalPortID].EquipmentID || peer.EntityID != ports[z.PhysicalPortID].EquipmentID || v.AFI == "" || v.SAFI == "" {
			add("BGP_PLAN_INCOMPLETE", v.ID, "BGP必须有明确两端接口、对应ASN及地址族")
		}
		bgp[v.ID] = true
	}
	bfdKeys := map[string]bool{}
	for _, v := range p.BFD {
		if bfdKeys[v.SessionID] {
			add("BFD_SESSION_DUPLICATE", v.SessionID, "同一BGP会话的BFD参数重复")
		}
		bfdKeys[v.SessionID] = true
		if !bgp[v.SessionID] || v.MinTXMilliseconds <= 0 || v.MinRXMilliseconds <= 0 || v.Multiplier <= 0 || len(v.EvidenceIDs) == 0 {
			add("BFD_PLAN_INCOMPLETE", v.SessionID, "BFD会话和计时参数未明确")
		}
	}
	for _, v := range p.QoS {
		check(v.ID, v.EvidenceIDs)
		if len(v.InterfaceIDs) == 0 {
			add("QOS_INTERFACE_REQUIRED", v.ID, "QoS需要明确接口范围")
		}
		for _, id := range v.InterfaceIDs {
			if _, ok := interfaces[id]; !ok {
				add("QOS_INTERFACE_UNRESOLVED", v.ID, "QoS引用不存在的逻辑接口")
			}
		}
		if v.RoCEVersion != nil && *v.RoCEVersion != 1 && *v.RoCEVersion != 2 {
			add("ROCE_VERSION_INVALID", v.ID, "RoCE版本未明确")
		}
		seen := map[int]bool{}
		for _, n := range v.LosslessPriorities {
			if n < 0 || n > 7 || seen[n] {
				add("QOS_PRIORITY_INVALID", v.ID, "优先级需在0至7范围且不可重复")
			}
			seen[n] = true
		}
		if v.PFCEnabled != nil && *v.PFCEnabled && len(v.LosslessPriorities) == 0 {
			add("PFC_PRIORITIES_REQUIRED", v.ID, "启用PFC需要明确优先级")
		}
	}
	arKeys := map[string]bool{}
	for _, v := range p.AdaptiveRouting {
		if arKeys[v.FabricID] {
			add("AR_PLAN_DUPLICATE", v.FabricID, "同一Fabric的自适应路由定义重复")
		}
		arKeys[v.FabricID] = true
		if !fabrics[v.FabricID] || len(v.EvidenceIDs) == 0 || (v.Enabled && v.PolicyRef == "") {
			add("AR_PLAN_INCOMPLETE", v.FabricID, "自适应路由需要Fabric、明确状态与启用策略依据")
		}
	}
	vnis := map[string]map[int]bool{}
	for _, v := range p.EVPNVXLAN {
		check(v.ID, v.EvidenceIDs)
		if vnis[v.FabricID] == nil {
			vnis[v.FabricID] = map[int]bool{}
		}
		if !fabrics[v.FabricID] || v.VNI < 1 || v.VNI > 16777215 || vnis[v.FabricID][v.VNI] || (v.VLAN != nil && (*v.VLAN < 1 || *v.VLAN > 4094)) || !strings.Contains(v.RouteDistinguisher, ":") || len(v.ImportTargets) == 0 || len(v.ExportTargets) == 0 || len(v.VTEPAllocationIDs) == 0 {
			add("EVPN_PLAN_INCOMPLETE", v.ID, "EVPN/VXLAN需要有效Fabric、VNI、VTEP、RD和RT")
		}
		vnis[v.FabricID][v.VNI] = true
		for _, id := range v.VTEPAllocationIDs {
			a, ok := allocations[id]
			if !ok || a.Kind != "ip" {
				add("VTEP_ALLOCATION_UNRESOLVED", v.ID, "VTEP必须引用明确IP分配")
			}
		}
	}
	for _, plane := range p.Planes {
		if plane.Profile == "" || plane.Profile == "ethernet" {
			continue
		}
		if plane.Profile != "ipv4-bgp" && plane.Profile != "ipv6-bgp" {
			add("NETWORK_PROFILE_UNSUPPORTED", plane.ID, "配置集尚无明确校验规则")
			continue
		}
		count := 0
		for _, iface := range p.InterfaceMappings {
			if iface.PlaneID != plane.ID {
				continue
			}
			count++
			hasIP, hasNeighbor := false, false
			for _, allocation := range p.Allocations {
				if allocation.Kind == "ip" && allocation.InterfaceID != nil && *allocation.InterfaceID == iface.ID && allocation.Address != nil {
					address, e := netip.ParseAddr(*allocation.Address)
					if e == nil && ((plane.Profile == "ipv4-bgp" && address.Is4()) || (plane.Profile == "ipv6-bgp" && address.Is6())) {
						hasIP = true
					}
				}
			}
			for _, session := range p.BGP {
				if (session.AInterfaceID == iface.ID || session.ZInterfaceID == iface.ID) && session.SAFI == "unicast" && ((plane.Profile == "ipv4-bgp" && session.AFI == "ipv4") || (plane.Profile == "ipv6-bgp" && session.AFI == "ipv6")) {
					hasNeighbor = true
				}
			}
			if !hasIP {
				add("NETWORK_PROFILE_ADDRESS_REQUIRED", iface.ID, "所选BGP配置集需要对应地址族的接口地址")
			}
			if !hasNeighbor {
				add("NETWORK_PROFILE_NEIGHBOR_REQUIRED", iface.ID, "所选BGP配置集需要明确邻居")
			}
		}
		if count == 0 {
			add("NETWORK_PROFILE_INTERFACES_REQUIRED", plane.ID, "所选BGP配置集需要接口映射")
		}
	}
	return issues
}

// Source labels resolve only to unambiguous identities in the selected physical
// revision. No new physical identities are created by logical network planning.
func ResolveNetworkPhysical(p *NetworkPlan, physical PhysicalModel) {
	eqAliases, portAliases := map[string]string{}, map[string]string{}
	labels := map[string]string{}
	add := func(m map[string]string, alias, id string) {
		if old, exists := m[alias]; exists && old != id {
			m[alias] = ""
		} else {
			m[alias] = id
		}
	}
	for _, v := range physical.Equipment {
		add(eqAliases, StableID(physical.ProjectID, "equipment", v.Label), v.EntityID)
		labels[v.EntityID] = v.Label
	}
	for _, v := range physical.Ports {
		alias := StableID(physical.ProjectID, "port", StableID(physical.ProjectID, "equipment", labels[v.EquipmentID])+"/"+v.Name)
		add(portAliases, alias, v.EntityID)
	}
	for i := range p.DeviceRoles {
		v := &p.DeviceRoles[i]
		if resolved, ok := eqAliases[v.EquipmentID]; ok {
			v.EquipmentID = resolved
		}
	}
	for i := range p.InterfaceMappings {
		v := &p.InterfaceMappings[i]
		if resolved, ok := portAliases[v.PhysicalPortID]; ok {
			v.PhysicalPortID = resolved
		}
	}
	for i := range p.Allocations {
		v := &p.Allocations[i]
		if resolved, ok := eqAliases[v.EntityID]; ok {
			v.EntityID = resolved
		}
	}
}
