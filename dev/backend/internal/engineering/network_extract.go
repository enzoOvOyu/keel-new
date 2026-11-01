package engineering

import (
	"net/netip"
	"strconv"
	"strings"
)

func EmptyNetwork(source string) NetworkPlan {
	return NetworkPlan{SourceIDs: []string{source}, Fabrics: []NetworkFabric{}, Planes: []NetworkPlane{}, DeviceRoles: []DeviceRole{}, InterfaceMappings: []InterfaceMapping{}, Allocations: []NetworkAllocation{}, BGP: []BGPPlan{}, BFD: []BFDPlan{}, QoS: []QoSPlan{}, AdaptiveRouting: []AdaptiveRoutingPlan{}, EVPNVXLAN: []EVPNVXLANPlan{}}
}
func networkList(s string) []string {
	values := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '，' || r == '；' || r == '\n' })
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	return values
}
func networkBool(s string) *bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "enabled", "enable", "是", "启用":
		v := true
		return &v
	case "false", "no", "0", "disabled", "disable", "否", "禁用":
		v := false
		return &v
	}
	return nil
}
func networkInteger(s string) *int64 {
	v, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if e != nil {
		return nil
	}
	return &v
}
func extractNetwork(project, source, revision string, rows []rowData, out *Extraction, version func(string, rowData, ...string) EntityVersion, issue func(string, string, string, string, string)) {
	p := EmptyNetwork(source)
	id := StableID(project, "network-plan", source)
	fabrics, planes := map[string]int{}, map[string]bool{}
	seen := map[string]string{}
	device := func(label string) string {
		if label == "" {
			return ""
		}
		return StableID(project, "equipment", label)
	}
	iface := func(owner, name string) string {
		if owner == "" || name == "" {
			return ""
		}
		return StableID(project, "network-interface", device(owner)+"/"+name)
	}
	asnID := func(owner string) string {
		if owner == "" {
			return ""
		}
		return StableID(project, "network-asn", device(owner))
	}
	bgpID := func(v map[string]string) string {
		return StableID(project, "network-bgp", iface(v["local_device"], v["local_interface"])+"/"+iface(v["peer_device"], v["peer_interface"]))
	}
	asnDeclarations := map[string]int{}
	addASN := func(owner, text string, ev EntityVersion) {
		if text == "" {
			return
		}
		key := asnID(owner)
		v := NetworkAllocation{ID: key, Kind: "asn", EntityID: device(owner), ASN: networkInteger(text), EvidenceIDs: ev.EvidenceIDs}
		if i, ok := asnDeclarations[key]; ok {
			if Digest(p.Allocations[i].ASN) != Digest(v.ASN) {
				issue(id, "NETWORK_ASN_SOURCE_CONFLICT", "asn", "同一设备ASN来源冲突", NetworkTarget)
			}
			return
		}
		asnDeclarations[key] = len(p.Allocations)
		p.Allocations = append(p.Allocations, v)
	}
	for _, r := range rows {
		if !strings.HasPrefix(r.kind, "network_") {
			continue
		}
		v := r.values
		fields := []string{}
		for field := range r.cells {
			fields = append(fields, field)
		}
		ev := version(id, r, fields...)
		if p.EntityID == "" {
			p.EntityVersion = ev
		} else {
			p.EvidenceIDs = append(p.EvidenceIDs, ev.EvidenceIDs...)
		}
		fid, pid := "", ""
		if v["fabric"] != "" {
			fid = StableID(project, "network-fabric", v["fabric"])
			if i, ok := fabrics[fid]; !ok {
				fabrics[fid] = len(p.Fabrics)
				p.Fabrics = append(p.Fabrics, NetworkFabric{ID: fid, Name: v["fabric"], Kind: v["fabric_kind"], EvidenceIDs: ev.EvidenceIDs})
			} else if p.Fabrics[i].Kind != "" && v["fabric_kind"] != "" && p.Fabrics[i].Kind != v["fabric_kind"] {
				issue(id, "NETWORK_FABRIC_CONFLICT", "fabric_kind", "同一Fabric的类型来源冲突", NetworkTarget)
			} else if p.Fabrics[i].Kind == "" {
				p.Fabrics[i].Kind = v["fabric_kind"]
			}
		}
		if v["plane"] != "" {
			pid = StableID(project, "network-plane", fid+"/"+v["plane"])
			if !planes[pid] {
				planes[pid] = true
				p.Planes = append(p.Planes, NetworkPlane{ID: pid, FabricID: fid, Name: v["plane"], EvidenceIDs: ev.EvidenceIDs})
			}
		}
		mark := func(key string) bool {
			key = r.kind + "/" + key
			digest := Digest(v)
			if old, ok := seen[key]; ok {
				if old != digest {
					issue(id, "NETWORK_SOURCE_CONFLICT", r.kind, "同一网络对象来源冲突", NetworkTarget)
				}
				return false
			}
			seen[key] = digest
			return true
		}
		switch r.kind {
		case "network_role":
			key := device(v["device"]) + "/" + pid
			if mark(key) {
				p.DeviceRoles = append(p.DeviceRoles, DeviceRole{EquipmentID: device(v["device"]), PlaneID: pid, Role: v["role"], EvidenceIDs: ev.EvidenceIDs})
			}
		case "network_interface":
			key := iface(v["device"], v["interface"])
			if mark(key) {
				port := ""
				if v["device"] != "" && v["port"] != "" {
					port = StableID(project, "port", device(v["device"])+"/"+v["port"])
				}
				p.InterfaceMappings = append(p.InterfaceMappings, InterfaceMapping{ID: key, PhysicalPortID: port, PlaneID: pid, LogicalName: v["interface"], EvidenceIDs: ev.EvidenceIDs})
			}
		case "network_allocation":
			kind := v["allocation_kind"]
			if kind == "asn" {
				addASN(v["device"], v["asn"], ev)
				break
			}
			key := asnID(v["device"])
			if kind != "asn" {
				key = StableID(project, "network-ip", iface(v["device"], v["interface"])+"/"+v["address"]+"/"+v["vrf_ref"])
			}
			if mark(key) {
				address := networkText(v["address"])
				prefix := networkInt(v["prefix_length"])
				if address != nil {
					if parsed, e := netip.ParsePrefix(*address); e == nil {
						a := parsed.Addr().String()
						address = &a
						if prefix != nil && *prefix != parsed.Bits() {
							issue(id, "NETWORK_PREFIX_CONFLICT", "prefix_length", "CIDR与独立前缀长度不一致", NetworkTarget)
						}
						if prefix == nil {
							bits := parsed.Bits()
							prefix = &bits
						}
					}
				}
				p.Allocations = append(p.Allocations, NetworkAllocation{ID: key, Kind: kind, EntityID: device(v["device"]), InterfaceID: networkText(iface(v["device"], v["interface"])), Address: address, PrefixLength: prefix, ASN: networkInteger(v["asn"]), VRFRef: networkText(v["vrf_ref"]), EvidenceIDs: ev.EvidenceIDs})
			}
		case "network_bgp":
			addASN(v["local_device"], v["local_asn"], ev)
			addASN(v["peer_device"], v["peer_asn"], ev)
			key := bgpID(v)
			if mark(key) {
				p.BGP = append(p.BGP, BGPPlan{ID: key, AInterfaceID: iface(v["local_device"], v["local_interface"]), ZInterfaceID: iface(v["peer_device"], v["peer_interface"]), LocalASNAllocationID: asnID(v["local_device"]), PeerASNAllocationID: asnID(v["peer_device"]), AFI: v["afi"], SAFI: v["safi"], PolicyRef: networkText(v["policy_ref"]), EvidenceIDs: ev.EvidenceIDs})
			}
		case "network_bfd":
			key := bgpID(v)
			number := func(field string) int { n, _ := strconv.Atoi(v[field]); return n }
			if mark(key) {
				p.BFD = append(p.BFD, BFDPlan{SessionID: key, MinTXMilliseconds: number("bfd_tx_ms"), MinRXMilliseconds: number("bfd_rx_ms"), Multiplier: number("bfd_multiplier"), EvidenceIDs: ev.EvidenceIDs})
			}
		case "network_qos":
			key := StableID(project, "network-qos", iface(v["device"], v["interface"]))
			if mark(key) {
				priorities := []int{}
				for _, text := range networkList(v["lossless_priorities"]) {
					n, e := strconv.Atoi(strings.TrimSpace(text))
					if e != nil {
						issue(id, "QOS_PRIORITY_INVALID", "lossless_priorities", "优先级无法解析", NetworkTarget)
						continue
					}
					priorities = append(priorities, n)
				}
				p.QoS = append(p.QoS, QoSPlan{ID: key, InterfaceIDs: []string{iface(v["device"], v["interface"])}, RoCEVersion: networkInt(v["roce_version"]), LosslessPriorities: priorities, PFCEnabled: networkBool(v["pfc_enabled"]), ECNEnabled: networkBool(v["ecn_enabled"]), PolicyRef: networkText(v["policy_ref"]), EvidenceIDs: ev.EvidenceIDs})
				for _, field := range []string{"pfc_enabled", "ecn_enabled"} {
					if v[field] != "" && networkBool(v[field]) == nil {
						issue(id, "NETWORK_BOOLEAN_UNKNOWN", field, "开关状态不明确", NetworkTarget)
					}
				}
			}
		case "network_ar":
			if mark(fid) {
				enabled := networkBool(v["adaptive_enabled"])
				if enabled == nil {
					issue(id, "AR_STATE_REQUIRED", "adaptive_enabled", "自适应路由必须有明确启用状态", NetworkTarget)
				} else {
					p.AdaptiveRouting = append(p.AdaptiveRouting, AdaptiveRoutingPlan{FabricID: fid, Enabled: *enabled, PolicyRef: v["policy_ref"], EvidenceIDs: ev.EvidenceIDs})
				}
			}
		case "network_evpn":
			key := StableID(project, "network-evpn", fid+"/"+v["vni"])
			if mark(key) {
				vni, _ := strconv.Atoi(v["vni"])
				vteps := []string{}

				p.EVPNVXLAN = append(p.EVPNVXLAN, EVPNVXLANPlan{ID: key, FabricID: fid, VNI: vni, VLAN: networkInt(v["vlan"]), VRFRef: networkText(v["vrf_ref"]), VTEPAllocationIDs: vteps, RouteDistinguisher: v["route_distinguisher"], ImportTargets: networkList(v["import_targets"]), ExportTargets: networkList(v["export_targets"]), EvidenceIDs: ev.EvidenceIDs})
			}
		}
	}
	// Resolve VTEP references after all allocation regions have been visited.
	for _, r := range rows {
		if r.kind != "network_evpn" {
			continue
		}
		v := r.values
		fid := StableID(project, "network-fabric", v["fabric"])
		key := StableID(project, "network-evpn", fid+"/"+v["vni"])
		for i := range p.EVPNVXLAN {
			e := &p.EVPNVXLAN[i]
			if e.ID != key {
				continue
			}
			e.VTEPAllocationIDs = []string{}
			for _, a := range p.Allocations {
				if a.Kind == "ip" && a.InterfaceID != nil && *a.InterfaceID == iface(v["vtep_device"], v["vtep_interface"]) && (v["vtep_address"] == "" || (a.Address != nil && *a.Address == v["vtep_address"])) {
					e.VTEPAllocationIDs = append(e.VTEPAllocationIDs, a.ID)
				}
			}
			if len(e.VTEPAllocationIDs) > 1 && v["vtep_address"] == "" {
				issue(id, "VTEP_ADDRESS_AMBIGUOUS", "vtep_interface", "接口有多个IP分配，需要明确VTEP地址", NetworkTarget)
			}
		}
	}
	if p.EntityID != "" {
		out.Model.NetworkPlans = append(out.Model.NetworkPlans, p)
	}
}

func networkText(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
func networkInt(s string) *int {
	n, e := strconv.Atoi(strings.TrimSpace(s))
	if e != nil {
		return nil
	}
	return &n
}
