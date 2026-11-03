package kickoff

import (
	"keel.local/dev/internal/engineering"
	"sort"
	"strings"
)

// PublicationFacts is a disposable, device-scoped comparison of engineering
// content. Revision/evidence identifiers and input row order are not changes.
func PublicationFacts(m engineering.PhysicalModel, inputs []ModelInput) map[string]map[string]string {
	out := map[string]map[string]string{}
	owner := map[string]string{}
	devices := map[string]engineering.EquipmentPlan{}
	for _, e := range m.Equipment {
		if IsDevice(e) {
			owner[e.EntityID] = e.EntityID
			devices[e.EntityID] = e
			out[e.EntityID] = map[string]string{}
		}
	}
	for progress := true; progress; {
		progress = false
		for _, a := range m.Assemblies {
			if root := owner[a.ParentID]; root != "" && owner[a.ChildID] == "" {
				owner[a.ChildID] = root
				progress = true
			}
		}
	}
	clean := func(v engineering.EntityVersion) engineering.EntityVersion {
		v.RevisionID = ""
		v.EvidenceIDs = nil
		v.ProvenanceID = ""
		return v
	}
	add := func(root, key string, value any) {
		if out[root] != nil {
			out[root][key] = engineering.Digest(value)
		}
	}
	for _, e := range m.Equipment {
		e.EntityVersion = clean(e.EntityVersion)
		add(owner[e.EntityID], "equipment:"+e.EntityID, e)
	}
	for _, a := range m.Assemblies {
		a.EntityVersion = clean(a.EntityVersion)
		add(owner[a.ParentID], "assembly:"+a.EntityID, a)
	}
	portOwner := map[string]string{}
	for _, p := range m.Ports {
		portOwner[p.EntityID] = owner[p.EquipmentID]
		p.EntityVersion = clean(p.EntityVersion)
		add(owner[p.EquipmentID], "port:"+p.EntityID, p)
	}
	for _, c := range m.Connections {
		c.EntityVersion = clean(c.EntityVersion)
		roots := map[string]bool{}
		for _, p := range append([]string{c.APortID, c.ZPortID}, c.PatchPortIDs...) {
			roots[portOwner[p]] = true
		}
		for root := range roots {
			add(root, "connection:"+c.EntityID, c)
		}
	}
	facilities := map[string]engineering.FacilityModel{}
	for _, f := range m.Facilities {
		facilities[f.EntityID] = f
	}
	related := map[string]map[string]bool{}
	for root := range out {
		related[root] = map[string]bool{root: true}
	}
	for _, p := range m.Placements {
		root := owner[p.EquipmentID]
		if out[root] == nil {
			continue
		}
		p.EntityVersion = clean(p.EntityVersion)
		add(root, "placement:"+p.EquipmentID, p)
		for id := p.RackID; id != "" && !related[root][id]; {
			related[root][id] = true
			f, ok := facilities[id]
			if !ok {
				break
			}
			related[root][f.Code] = f.Code != ""
			related[root][f.Label] = f.Label != ""
			f.EntityVersion = clean(f.EntityVersion)
			add(root, "facility:"+id, f)
			if f.ParentID == nil {
				break
			}
			id = *f.ParentID
		}
	}
	inputs = ResolveReferences(m, inputs)
	for _, input := range inputs {
		rows := map[string][]string{}
		for _, row := range input.Rows {
			value := map[string]string{}
			for k, v := range row {
				if v != "" {
					value[k] = v
				}
			}
			if len(value) == 0 {
				continue
			}
			digest := engineering.Digest(value)
			for root, device := range devices {
				matches := true
				if id := row["device_id"]; id != "" {
					matches = owner[id] == root
				}
				if id := row["rack_id"]; id != "" && row["device_id"] == "" {
					matches = matches && related[root][id]
				}
				if id := row["room_id"]; id != "" && row["device_id"] == "" && row["rack_id"] == "" {
					matches = matches && related[root][id]
				}
				if model := row["model"]; input.TableID == "equipment_specs" && model != "" {
					matches = matches && (device.SourceModelLabel == model || device.ModelRef != nil && *device.ModelRef == model)
				}
				if scope := row["scope"]; scope != "" {
					matches = matches && (scope == root || scope == device.Label || device.Serial != nil && scope == *device.Serial || scope == "all" || scope == "devices" || scope == device.EquipmentType || scope == device.EquipmentType+"s" || scope == "servers" && strings.Contains(device.EquipmentType, "server") || scope == "switches" && strings.Contains(device.EquipmentType, "switch"))
				}
				if matches {
					rows[root] = append(rows[root], digest)
				}
			}
		}
		for root := range devices {
			sort.Strings(rows[root])
			out[root]["input:"+input.TableID] = strings.Join(rows[root], ",")
		}
	}
	return out
}

// Physical removals require their own CMDB change process; publication only
// writes present objects. Empty scoped input snapshots still detect removed rows.
func PublicationChanged(current, published map[string]string) bool {
	for key, value := range current {
		if published[key] != value {
			return true
		}
	}
	return false
}
