package kickoff

import (
	"fmt"
	"keel.local/dev/internal/engineering"
	"sort"
	"strings"
	"time"
)

type Publication struct {
	ID             string         `json:"id"`
	Revision       int            `json:"revision"`
	CreatedAt      time.Time      `json:"createdAt"`
	Actor          string         `json:"actor"`
	Reason         string         `json:"reason"`
	State          string         `json:"state"`
	ErrorCode      *string        `json:"errorCode,omitempty"`
	DeviceIDs      []string       `json:"deviceIds"`
	PortIDs        []string       `json:"portIds"`
	ConnectionIDs  []string       `json:"connectionIds"`
	ComponentIDs   []string       `json:"componentIds"`
	PlacementIDs   []string       `json:"placementIds"`
	FacilityIDs    []string       `json:"facilityIds"`
	LocationIDs    []string       `json:"-"`
	ModelVersions  map[string]int `json:"modelVersions"`
	ModelSnapshot  []ModelInput   `json:"modelSnapshot"`
	RackSnapshot   []Rack         `json:"rackSnapshot"`
	DeviceSnapshot []Device       `json:"deviceSnapshot"`
	Counts         map[string]int `json:"counts"`
}

func Select(m engineering.PhysicalModel, v View, ids []string) (engineering.PhysicalModel, Publication, error) {
	return SelectScope(m, v, ids, nil, nil)
}

// Existing ports are references verified in this project's CMDB, not objects to
// republish. Their devices and observations remain outside the selected scope.
func SelectScope(m engineering.PhysicalModel, v View, ids, facilityIDs []string, existingPorts map[string]engineering.PlannedPort) (engineering.PhysicalModel, Publication, error) {
	p := Publication{DeviceIDs: []string{}, PortIDs: []string{}, ConnectionIDs: []string{}, ComponentIDs: []string{}, PlacementIDs: []string{}, FacilityIDs: []string{}, LocationIDs: []string{}, ModelVersions: map[string]int{}, ModelSnapshot: []ModelInput{}, RackSnapshot: []Rack{}, DeviceSnapshot: []Device{}, Counts: map[string]int{}}
	subset := engineering.EmptyPhysical(m.ProjectID)
	selected := map[string]bool{}
	for _, id := range ids {
		if selected[id] {
			return subset, p, fmt.Errorf("所选范围存在重复设备")
		}
		selected[id] = true
	}
	for _, d := range v.Devices {
		if selected[d.ID] {
			p.DeviceIDs = append(p.DeviceIDs, d.ID)
			p.DeviceSnapshot = append(p.DeviceSnapshot, d)
		}
	}
	if len(ids) != len(p.DeviceIDs) {
		return subset, p, fmt.Errorf("请选择当前项目明确存在的设备")
	}
	if len(ids) == 0 && len(facilityIDs) == 0 {
		return subset, p, fmt.Errorf("请选择当前项目明确存在的设备或设施")
	}
	facilities := map[string]bool{}
	knownFacilities := map[string]bool{}
	for _, f := range m.Facilities {
		knownFacilities[f.EntityID] = true
	}
	for _, id := range facilityIDs {
		if facilities[id] {
			return subset, p, fmt.Errorf("所选范围存在重复设施")
		}
		if !knownFacilities[id] {
			return subset, p, fmt.Errorf("请选择当前项目明确存在的设施")
		}
		facilities[id] = true
	}
	included := map[string]bool{}
	for k := range selected {
		included[k] = true
	}
	for progress := true; progress; {
		progress = false
		for _, a := range m.Assemblies {
			if included[a.ParentID] && !included[a.ChildID] {
				included[a.ChildID] = true
				progress = true
			}
		}
	}
	for _, e := range m.Equipment {
		if included[e.EntityID] {
			if e.IdentityKind == "group" || e.Quantity != 1 {
				return subset, p, fmt.Errorf("采购数量不能作为实际设备入库")
			}
			subset.Equipment = append(subset.Equipment, e)
			if !selected[e.EntityID] {
				p.ComponentIDs = append(p.ComponentIDs, e.EntityID)
			}
		}
	}
	for _, a := range m.Assemblies {
		if included[a.ParentID] && included[a.ChildID] {
			subset.Assemblies = append(subset.Assemblies, a)
		}
	}
	ports := map[string]bool{}
	for _, port := range m.Ports {
		if included[port.EquipmentID] {
			subset.Ports = append(subset.Ports, port)
			ports[port.EntityID] = true
			p.PortIDs = append(p.PortIDs, port.EntityID)
		}
	}
	references := map[string]bool{}
	for _, link := range m.Connections {
		if !ports[link.APortID] && !ports[link.ZPortID] {
			continue
		}
		_, aExists := existingPorts[link.APortID]
		_, zExists := existingPorts[link.ZPortID]
		if (ports[link.APortID] || aExists) && (ports[link.ZPortID] || zExists) {
			for _, patch := range link.PatchPortIDs {
				if _, exists := existingPorts[patch]; !ports[patch] && !exists {
					return subset, p, fmt.Errorf("配线路径依赖未纳入本次范围")
				}
			}
			subset.Connections = append(subset.Connections, link)
			p.ConnectionIDs = append(p.ConnectionIDs, link.EntityID)
			for _, id := range append([]string{link.APortID, link.ZPortID}, link.PatchPortIDs...) {
				if !ports[id] {
					references[id] = true
				}
			}
		}
	}
	placementCount := map[string]int{}
	for _, placement := range m.Placements {
		placementCount[placement.EquipmentID]++
	}
	for _, placement := range m.Placements {
		if !selected[placement.EquipmentID] {
			continue
		}
		knownRack := false
		for _, rack := range m.Facilities {
			if rack.EntityID == placement.RackID && rack.Kind == "rack" {
				knownRack = true
			}
		}
		if !knownRack || placementCount[placement.EquipmentID] != 1 {
			continue
		}
		facilities[placement.RackID] = true
		valid := placement.UsesNonUPosition() && placement.PositionCode != ""
		if placement.StartU != nil && placement.HeightU != nil && *placement.StartU > 0 && *placement.HeightU > 0 {
			for _, rack := range m.Facilities {
				if rack.EntityID == placement.RackID && rack.CapacityU != nil && *placement.StartU+*placement.HeightU-1 <= *rack.CapacityU {
					valid = true
				}
			}
		}
		for _, i := range v.Issues {
			if !i.Resolved && i.Scope == "placement" && (i.EntityID == placement.EntityID || i.EntityID == placement.EquipmentID) {
				valid = false
			}
		}
		if valid {
			subset.Placements = append(subset.Placements, placement)
			p.PlacementIDs = append(p.PlacementIDs, placement.EquipmentID)
		} else {
			// A known rack association is useful before geometric occupancy is complete.
			// Keep it distinct from the count of validated placements.
			p.LocationIDs = append(p.LocationIDs, placement.EquipmentID)
		}
	}
	for progress := true; progress; {
		progress = false
		for _, f := range m.Facilities {
			if facilities[f.EntityID] && f.ParentID != nil && !facilities[*f.ParentID] {
				facilities[*f.ParentID] = true
				progress = true
			}
		}
	}
	for _, f := range m.Facilities {
		if facilities[f.EntityID] {
			subset.Facilities = append(subset.Facilities, f)
			p.FacilityIDs = append(p.FacilityIDs, f.EntityID)
		}
	}
	affectedIDs := map[string]bool{}
	modelPorts := map[string]engineering.PlannedPort{}
	modelEquipment := map[string]bool{}
	for _, port := range m.Ports {
		modelPorts[port.EntityID] = port
	}
	for _, equipment := range m.Equipment {
		modelEquipment[equipment.EntityID] = true
	}
	referenceModel := m
	referenceModel.Ports = append([]engineering.PlannedPort{}, m.Ports...)
	for id := range references {
		affectedIDs[id] = true
		affectedIDs[existingPorts[id].EquipmentID] = true
		if port, ok := modelPorts[id]; ok {
			if port.EquipmentID != existingPorts[id].EquipmentID {
				return subset, p, fmt.Errorf("对端端口所属设备与CMDB身份不一致")
			}
		} else {
			referenceModel.Ports = append(referenceModel.Ports, existingPorts[id])
		}
	}
	resolvedReferences := map[string]bool{}
	if len(references) > 0 {
		remaining := map[string]bool{}
		for _, issue := range engineering.ValidateConnectivity(referenceModel) {
			remaining[issue.ID] = true
		}
		for _, issue := range engineering.ValidateConnectivity(m) {
			switch issue.Code {
			case "CONNECTION_PORT_UNRESOLVED", "PATCH_PASSTHROUGH_OWNER_MISMATCH":
				resolvedReferences[issue.ID] = !remaining[issue.ID]
			case "PORT_OWNER_UNRESOLVED":
				port := modelPorts[issue.EntityID]
				resolvedReferences[issue.ID] = references[issue.EntityID] && !modelEquipment[port.EquipmentID] && port.Name != ""
			}
		}
	}
	for _, issue := range v.Issues {
		if issue.Resolved || issue.Scope != "identity" || resolvedReferences[issue.ID] {
			continue
		}
		affected := included[issue.EntityID] || ports[issue.EntityID] || facilities[issue.EntityID] || affectedIDs[issue.EntityID]
		for _, link := range subset.Connections {
			if link.EntityID == issue.EntityID {
				affected = true
			}
		}
		if affected {
			return subset, p, fmt.Errorf("%s：%s", issue.Title, issue.Description)
		}
	}
	// Validate connection paths with their CMDB references present, but do not
	// revalidate or publish the external ports' source records or owner devices.
	validation := subset
	validation.Ports = append([]engineering.PlannedPort{}, subset.Ports...)
	for id := range references {
		validation.Ports = append(validation.Ports, existingPorts[id])
	}
	for _, i := range ValidationIssues(validation) {
		if references[i.EntityID] {
			continue
		}
		return subset, p, fmt.Errorf("%s：%s", i.Code, i.Message)
	}
	for _, r := range v.Racks {
		if facilities[r.ID] {
			p.RackSnapshot = append(p.RackSnapshot, r)
		}
	}
	p.Counts = map[string]int{"devices": len(p.DeviceIDs), "components": len(p.ComponentIDs), "ports": len(p.PortIDs), "connections": len(p.ConnectionIDs), "placements": len(p.PlacementIDs), "facilities": len(p.FacilityIDs), "locations": len(p.LocationIDs)}
	return subset, p, nil
}

// FrozenModel reconstructs the approved object closure rather than rerunning
// today's selection logic over an older publication.
func FrozenModel(m engineering.PhysicalModel, p Publication) engineering.PhysicalModel {
	keys := func(ids []string) map[string]bool {
		out := map[string]bool{}
		for _, id := range ids {
			out[id] = true
		}
		return out
	}
	equipment := keys(append(append([]string{}, p.DeviceIDs...), p.ComponentIDs...))
	ports, connections, placements, facilities := keys(p.PortIDs), keys(p.ConnectionIDs), keys(p.PlacementIDs), keys(p.FacilityIDs)
	locations := keys(p.LocationIDs)
	out := engineering.EmptyPhysical(m.ProjectID)
	for _, v := range m.Equipment {
		if equipment[v.EntityID] {
			out.Equipment = append(out.Equipment, v)
		}
	}
	for _, v := range m.Assemblies {
		if equipment[v.ParentID] && equipment[v.ChildID] {
			out.Assemblies = append(out.Assemblies, v)
		}
	}
	for _, v := range m.Ports {
		if ports[v.EntityID] {
			out.Ports = append(out.Ports, v)
		}
	}
	for _, v := range m.Connections {
		if connections[v.EntityID] {
			out.Connections = append(out.Connections, v)
		}
	}
	for _, v := range m.Placements {
		if placements[v.EquipmentID] {
			out.Placements = append(out.Placements, v)
		}
		if placements[v.EquipmentID] || locations[v.EquipmentID] {
			facilities[v.RackID] = true
		}
	}
	// Publications before the facility-closure migration derived these ancestors
	// exclusively from their approved complete placements.
	for progress := true; progress; {
		progress = false
		for _, f := range m.Facilities {
			if facilities[f.EntityID] && f.ParentID != nil && !facilities[*f.ParentID] {
				facilities[*f.ParentID] = true
				progress = true
			}
		}
	}
	for _, f := range m.Facilities {
		if facilities[f.EntityID] {
			out.Facilities = append(out.Facilities, f)
		}
	}
	return out
}

// FacilityPublicationFacts compares the facility and its required ancestors,
// including scoped physical inputs. Evidence/revision churn is not a change.
func FacilityPublicationFacts(m engineering.PhysicalModel, inputs []ModelInput) map[string]map[string]string {
	out := map[string]map[string]string{}
	facilities := map[string]engineering.FacilityModel{}
	for _, f := range m.Facilities {
		facilities[f.EntityID] = f
	}
	inputs = ResolveReferences(m, inputs)
	for _, root := range m.Facilities {
		facts := map[string]string{}
		related := map[string]map[string]bool{"room": {}, "rack": {}}
		seen := map[string]bool{}
		for id := root.EntityID; id != "" && !seen[id]; {
			seen[id] = true
			f, ok := facilities[id]
			if !ok {
				break
			}
			if aliases := related[f.Kind]; aliases != nil {
				for _, key := range []string{f.EntityID, f.Code, f.Label} {
					if key != "" {
						aliases[key] = true
					}
				}
			}
			f.RevisionID, f.ProvenanceID, f.EvidenceIDs = "", "", nil
			facts["facility:"+id] = engineering.Digest(f)
			if f.ParentID == nil {
				break
			}
			id = *f.ParentID
		}
		for _, input := range inputs {
			kind, field := "", ""
			switch input.TableID {
			case "rooms":
				kind, field = "room", "room_id"
			case "racks":
				kind, field = "rack", "rack_id"
			default:
				continue
			}
			rows := []string{}
			for _, row := range input.Rows {
				id := row[field]
				if kind == "room" && id == "" {
					id = row["name"]
				}
				if id == "" || !related[kind][id] && !seen[engineering.StableID(m.ProjectID, kind, id)] {
					continue
				}
				value := map[string]string{}
				for key, valueText := range row {
					if valueText != "" {
						value[key] = valueText
					}
				}
				rows = append(rows, engineering.Digest(value))
			}
			sort.Strings(rows)
			facts["input:"+input.TableID] = strings.Join(rows, ",")
		}
		out[root.EntityID] = facts
	}
	return out
}

func KnownLocations(m engineering.PhysicalModel, p Publication) engineering.PhysicalModel {
	out := FrozenModel(m, p)
	out.Placements = nil
	for _, v := range m.Placements {
		for _, id := range p.LocationIDs {
			if v.EquipmentID == id {
				out.Placements = append(out.Placements, v)
				break
			}
		}
	}
	return out
}
