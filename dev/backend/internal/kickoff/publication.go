package kickoff

import (
	"fmt"
	"keel.local/dev/internal/engineering"
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
	FacilityIDs    []string       `json:"-"`
	LocationIDs    []string       `json:"-"`
	ModelVersions  map[string]int `json:"modelVersions"`
	ModelSnapshot  []ModelInput   `json:"modelSnapshot"`
	RackSnapshot   []Rack         `json:"rackSnapshot"`
	DeviceSnapshot []Device       `json:"deviceSnapshot"`
	Counts         map[string]int `json:"counts"`
}

func Select(m engineering.PhysicalModel, v View, ids []string) (engineering.PhysicalModel, Publication, error) {
	p := Publication{DeviceIDs: []string{}, PortIDs: []string{}, ConnectionIDs: []string{}, ComponentIDs: []string{}, PlacementIDs: []string{}, ModelVersions: map[string]int{}, ModelSnapshot: []ModelInput{}, RackSnapshot: []Rack{}, DeviceSnapshot: []Device{}, Counts: map[string]int{}}
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
	if len(ids) == 0 || len(ids) != len(p.DeviceIDs) {
		return subset, p, fmt.Errorf("请选择当前项目明确存在的设备")
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
	for _, link := range m.Connections {
		if ports[link.APortID] && ports[link.ZPortID] {
			for _, patch := range link.PatchPortIDs {
				if !ports[patch] {
					return subset, p, fmt.Errorf("配线路径依赖未纳入本次范围")
				}
			}
			subset.Connections = append(subset.Connections, link)
			p.ConnectionIDs = append(p.ConnectionIDs, link.EntityID)
		}
	}
	facilities := map[string]bool{}
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
	for _, issue := range v.Issues {
		if issue.Resolved || issue.Scope != "identity" {
			continue
		}
		affected := included[issue.EntityID] || ports[issue.EntityID]
		for _, link := range subset.Connections {
			if link.EntityID == issue.EntityID {
				affected = true
			}
		}
		if affected {
			return subset, p, fmt.Errorf("%s：%s", issue.Title, issue.Description)
		}
	}
	for _, i := range ValidationIssues(subset) {
		return subset, p, fmt.Errorf("%s：%s", i.Code, i.Message)
	}
	for _, r := range v.Racks {
		if facilities[r.ID] {
			p.RackSnapshot = append(p.RackSnapshot, r)
		}
	}
	p.Counts = map[string]int{"devices": len(p.DeviceIDs), "components": len(p.ComponentIDs), "ports": len(p.PortIDs), "connections": len(p.ConnectionIDs), "placements": len(p.PlacementIDs)}
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
