package engineering

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// mergeNetworkInput consumes only declared planning values. Physical links do
// not create addresses, ASNs, roles or neighbors.
func mergeNetworkInput(current PhysicalModel, q InputPreviewRequest) (PhysicalModel, error) {
	preview := PreviewStepInput(q)
	if !preview.Valid {
		return current, fmt.Errorf("STEP_INPUT_INVALID")
	}
	raw, e := json.Marshal(current)
	if e != nil {
		return current, e
	}
	var next PhysicalModel
	if e = json.Unmarshal(raw, &next); e != nil {
		return current, e
	}
	if len(next.NetworkPlans) > 1 {
		return current, fmt.Errorf("NETWORK_SINGLE_PLAN_REQUIRED")
	}
	if len(next.NetworkPlans) == 0 {
		next.NetworkPlans = []NetworkPlan{{EntityVersion: EntityVersion{EntityID: StableID(current.ProjectID, "network-plan", "standard"), Maturity: Estimated, EvidenceIDs: []string{}, Attributes: []AttributeValue{}}, SourceIDs: []string{}, Fabrics: []NetworkFabric{}, Planes: []NetworkPlane{}, DeviceRoles: []DeviceRole{}, InterfaceMappings: []InterfaceMapping{}, Allocations: []NetworkAllocation{}, BGP: []BGPPlan{}, BFD: []BFDPlan{}, QoS: []QoSPlan{}, AdaptiveRouting: []AdaptiveRoutingPlan{}, EVPNVXLAN: []EVPNVXLANPlan{}}}
	}
	p := &next.NetworkPlans[0]
	id := func(kind, key string) string { return StableID(current.ProjectID, "network-"+kind, key) }
	owners := map[string]string{}
	for _, eq := range next.Equipment {
		if eq.Serial != nil {
			serial := strings.TrimSpace(*eq.Serial)
			if owners[serial] != "" {
				return current, fmt.Errorf("STEP_SERIAL_AMBIGUOUS")
			}
			owners[serial] = eq.EntityID
		}
	}
	port := func(owner, name string) (string, error) {
		found := ""
		for _, v := range next.Ports {
			if v.EquipmentID == owner && v.Name == name {
				if found != "" {
					return "", fmt.Errorf("STEP_PORT_AMBIGUOUS")
				}
				found = v.EntityID
			}
		}
		if found == "" {
			return "", fmt.Errorf("STEP_PORT_REQUIRED")
		}
		return found, nil
	}
	plane := func(code string) (string, error) {
		found := ""
		for _, v := range p.Planes {
			qualified := ""
			for _, fabric := range p.Fabrics {
				if fabric.ID == v.FabricID {
					qualified = fabric.Name + "/" + v.Name
				}
			}
			if v.Name == code || v.ID == code || qualified == code {
				if found != "" {
					return "", fmt.Errorf("STEP_NETWORK_REFERENCE_AMBIGUOUS")
				}
				found = v.ID
			}
		}
		if found == "" {
			return "", fmt.Errorf("STEP_NETWORK_PLANE_REQUIRED")
		}
		return found, nil
	}
	mapping := func(owner, name string) (string, error) {
		found := ""
		for _, v := range p.InterfaceMappings {
			for _, physical := range next.Ports {
				if physical.EntityID == v.PhysicalPortID && physical.EquipmentID == owner && v.LogicalName == name {
					if found != "" {
						return "", fmt.Errorf("STEP_NETWORK_REFERENCE_AMBIGUOUS")
					}
					found = v.ID
				}
			}
		}
		if found == "" {
			return "", fmt.Errorf("STEP_NETWORK_INTERFACE_REQUIRED")
		}
		return found, nil
	}
	asn := func(owner string, value int64) (string, error) {
		found := ""
		for _, v := range p.Allocations {
			if v.Kind == "asn" && v.EntityID == owner && v.ASN != nil && *v.ASN == value {
				if found != "" {
					return "", fmt.Errorf("STEP_NETWORK_REFERENCE_AMBIGUOUS")
				}
				found = v.ID
			}
		}
		if found == "" {
			return "", fmt.Errorf("STEP_NETWORK_ASN_REQUIRED")
		}
		return found, nil
	}
	for _, row := range preview.Rows {
		v := append([]string{}, row...)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		switch q.TableID {
		case "network_planes":
			if v[3] != "ethernet" && v[3] != "ipv4-bgp" && v[3] != "ipv6-bgp" {
				return current, fmt.Errorf("STEP_NETWORK_PROFILE_UNSUPPORTED")
			}
			fabric := id("fabric", v[0])
			fi := -1
			for i, x := range p.Fabrics {
				if x.ID == fabric || x.Name == v[0] {
					fi = i
					fabric = x.ID
					break
				}
			}
			if fi < 0 {
				p.Fabrics = append(p.Fabrics, NetworkFabric{ID: fabric, Name: v[0], Kind: v[1], EvidenceIDs: []string{}})
			} else {
				p.Fabrics[fi].Kind = v[1]
			}
			// Configuration profile is retained as a source declaration; it does not
			// enable protocols that have no corresponding explicit tables.
			pi := -1
			for i, x := range p.Planes {
				if x.FabricID == fabric && x.Name == v[2] {
					pi = i
					break
				}
			}
			if pi < 0 {
				p.Planes = append(p.Planes, NetworkPlane{ID: id("plane", fabric+"/"+v[2]), FabricID: fabric, Name: v[2], Profile: v[3], EvidenceIDs: []string{}})
			} else {
				p.Planes[pi].Profile = v[3]
			}
		case "network_roles":
			owner := owners[v[0]]
			if owner == "" {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			pl, e := plane(v[1] + "/" + v[2])
			if e != nil {
				return current, e
			}
			fabric := ""
			for _, f := range p.Fabrics {
				if f.Name == v[1] || f.ID == v[1] {
					fabric = f.ID
				}
			}
			valid := false
			for _, x := range p.Planes {
				if x.ID == pl && x.FabricID == fabric {
					valid = true
				}
			}
			if !valid {
				return current, fmt.Errorf("STEP_NETWORK_PLANE_REQUIRED")
			}
			found := -1
			for i, x := range p.DeviceRoles {
				if x.EquipmentID == owner && x.PlaneID == pl {
					found = i
					break
				}
			}
			role := DeviceRole{EquipmentID: owner, PlaneID: pl, Role: v[3], EvidenceIDs: []string{}}
			if found < 0 {
				p.DeviceRoles = append(p.DeviceRoles, role)
			} else {
				p.DeviceRoles[found] = role
			}
		case "network_interfaces":
			owner := owners[v[0]]
			if owner == "" {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			physical, e := port(owner, v[1])
			if e != nil {
				return current, e
			}
			pl, e := plane(v[3])
			if e != nil {
				return current, e
			}
			if v[4] != "" || v[5] != "" {
				return current, fmt.Errorf("STEP_NETWORK_VLAN_VRF_REQUIRES_MODEL")
			}
			found := -1
			for i, x := range p.InterfaceMappings {
				if x.PlaneID == pl && x.PhysicalPortID == physical && x.LogicalName == v[2] {
					found = i
					break
				}
			}
			m := InterfaceMapping{ID: id("interface", owner+"/"+v[2]+"/"+pl), PhysicalPortID: physical, PlaneID: pl, LogicalName: v[2], EvidenceIDs: []string{}}
			if found < 0 {
				p.InterfaceMappings = append(p.InterfaceMappings, m)
			} else {
				m.ID = p.InterfaceMappings[found].ID
				p.InterfaceMappings[found] = m
			}
		case "network_addresses":
			owner := owners[v[0]]
			if owner == "" {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			iface, e := mapping(owner, v[1])
			if e != nil {
				return current, e
			}
			prefix, e := netip.ParsePrefix(v[2])
			if e != nil {
				return current, fmt.Errorf("STEP_NETWORK_ADDRESS_INVALID")
			}
			address := prefix.Addr().String()
			bits := prefix.Bits()
			key := id("address", iface+"/"+v[3])
			a := NetworkAllocation{ID: key, Kind: "ip", EntityID: owner, InterfaceID: &iface, Address: &address, PrefixLength: &bits, EvidenceIDs: []string{}}
			found := -1
			for i, x := range p.Allocations {
				if x.ID == key {
					found = i
					break
				}
			}
			if found < 0 {
				p.Allocations = append(p.Allocations, a)
			} else {
				p.Allocations[found] = a
			}
		case "network_asns":
			owner := owners[v[0]]
			if owner == "" {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			number, e := strconv.ParseInt(v[1], 10, 64)
			if e != nil || number < 1 || number > 4294967295 {
				return current, fmt.Errorf("STEP_NETWORK_ASN_INVALID")
			}
			found := -1
			for i, x := range p.Allocations {
				if x.Kind == "asn" && x.EntityID == owner {
					if found >= 0 {
						return current, fmt.Errorf("STEP_NETWORK_REFERENCE_AMBIGUOUS")
					}
					found = i
				}
			}
			a := NetworkAllocation{ID: id("asn", owner), Kind: "asn", EntityID: owner, ASN: &number, EvidenceIDs: []string{}}
			if found < 0 {
				p.Allocations = append(p.Allocations, a)
			} else {
				a.ID = p.Allocations[found].ID
				p.Allocations[found] = a
			}
		case "network_bgp":
			owner := owners[v[0]]
			if owner == "" {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			a, e := mapping(owner, v[1])
			if e != nil {
				return current, e
			}
			peer := ""
			peerOwner := ""
			for _, x := range p.InterfaceMappings {
				for _, physical := range next.Ports {
					if physical.EntityID != x.PhysicalPortID {
						continue
					}
					for serial, equipment := range owners {
						if equipment == physical.EquipmentID && v[2] == serial+"/"+x.LogicalName {
							if peer != "" {
								return current, fmt.Errorf("STEP_NETWORK_REFERENCE_AMBIGUOUS")
							}
							peer = x.ID
							peerOwner = equipment
						}
					}
				}
			}
			if peer == "" {
				return current, fmt.Errorf("STEP_NETWORK_INTERFACE_REQUIRED")
			}
			local, e := strconv.ParseInt(v[3], 10, 64)
			if e != nil {
				return current, fmt.Errorf("STEP_NETWORK_ASN_INVALID")
			}
			remote, e := strconv.ParseInt(v[4], 10, 64)
			if e != nil {
				return current, fmt.Errorf("STEP_NETWORK_ASN_INVALID")
			}
			localRef, e := asn(owner, local)
			if e != nil {
				return current, e
			}
			remoteRef, e := asn(peerOwner, remote)
			if e != nil {
				return current, e
			}
			family := strings.SplitN(v[5], "-", 2)
			if len(family) != 2 || family[0] == "" || family[1] == "" {
				return current, fmt.Errorf("STEP_NETWORK_ADDRESS_FAMILY_REQUIRED")
			}
			policy := v[6]
			session := BGPPlan{ID: id("bgp", a+"/"+peer+"/"+v[5]), AInterfaceID: a, ZInterfaceID: peer, LocalASNAllocationID: localRef, PeerASNAllocationID: remoteRef, AFI: family[0], SAFI: family[1], PolicyRef: &policy, EvidenceIDs: []string{}}
			found := -1
			for i, x := range p.BGP {
				if x.ID == session.ID {
					found = i
					break
				}
			}
			if found < 0 {
				p.BGP = append(p.BGP, session)
			} else {
				p.BGP[found] = session
			}
		default:
			return current, fmt.Errorf("STEP_TABLE_NOT_SUPPORTED")
		}
	}
	return next, nil
}
