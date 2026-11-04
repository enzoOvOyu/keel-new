package kickoff

import (
	"github.com/google/uuid"
	"keel.local/dev/internal/engineering"
	"strings"
)

// NormalizeDeviceRef changes only whitespace and case, never punctuation or digits.
func NormalizeDeviceRef(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

// Resolves unique project-local references; device names use conservative normalization. Original input revisions
// remain unchanged; the API/model projection exposes stable canonical IDs.
func ResolveReferences(m engineering.PhysicalModel, inputs []ModelInput) []ModelInput {
	aliases := map[string]map[string]string{"device_id": {}, "rack_id": {}, "room_id": {}, "device_name": {}}
	add := func(kind, key, id string) {
		if key == "" {
			return
		}
		if old, ok := aliases[kind][key]; ok && old != id {
			aliases[kind][key] = ""
		} else if !ok {
			aliases[kind][key] = id
		}
	}
	id := func(kind, key string) string {
		if _, e := uuid.Parse(key); e == nil {
			return key
		}
		return engineering.StableID(m.ProjectID, kind, key)
	}
	for _, e := range m.Equipment {
		add("device_id", e.EntityID, e.EntityID)
		add("device_name", NormalizeDeviceRef(e.Label), e.EntityID)
		if e.Serial != nil {
			add("device_id", *e.Serial, e.EntityID)
		}
	}
	for _, f := range m.Facilities {
		kind := ""
		if f.Kind == "rack" {
			kind = "rack_id"
		}
		if f.Kind == "room" {
			kind = "room_id"
		}
		if kind != "" {
			add(kind, f.EntityID, f.EntityID)
			add(kind, f.Code, f.EntityID)
			add(kind, f.Label, f.EntityID)
		}
	}
	for _, input := range inputs {
		field, kind := "", ""
		if input.TableID == "rooms" {
			field = "room_id"
			kind = "room"
		}
		if input.TableID == "racks" {
			field = "rack_id"
			kind = "rack"
		}
		if field != "" {
			for _, row := range input.Rows {
				if row[field] != "" {
					canonical := id(kind, row[field])
					add(field, row[field], canonical)
				}
			}
		}
	}
	out := make([]ModelInput, 0, len(inputs))
	for _, input := range inputs {
		copy := input
		copy.Rows = make([]map[string]string, 0, len(input.Rows))
		for _, row := range input.Rows {
			value := map[string]string{}
			for key, raw := range row {
				value[key] = raw
				own := input.TableID == "racks" && key == "rack_id" || input.TableID == "rooms" && key == "room_id"
				if !own {
					if key == "device_id" {
						exact, exactExists := aliases[key][raw]
						name, nameExists := aliases["device_name"][NormalizeDeviceRef(raw)]
						// Ambiguous names or disagreement with an explicit SN/ID never choose a winner.
						if nameExists && (name == "" || exactExists && exact != name) {
							continue
						}
						if exactExists {
							if exact != "" {
								value[key] = exact
							}
						} else if nameExists {
							value[key] = name
						}
					} else if a := aliases[key]; a != nil && a[raw] != "" {
						value[key] = a[raw]
					}
				}
			}
			copy.Rows = append(copy.Rows, value)
		}
		out = append(out, copy)
	}
	return out
}

// Seed editable physical inputs from explicit historical facts. Instance height
// does not become a model-wide default; no capacity or room is guessed.
func PhysicalInputs(m engineering.PhysicalModel) map[string][]map[string]string {
	out := map[string][]map[string]string{"rooms": {}, "racks": {}, "equipment_specs": {}, "placements": {}}
	devices := map[string]bool{}
	models := map[string]bool{}
	for _, e := range m.Equipment {
		if IsDevice(e) && e.IdentityKind != "group" {
			devices[e.EntityID] = true
			if e.SourceModelLabel != "" && !models[e.SourceModelLabel] {
				models[e.SourceModelLabel] = true
				out["equipment_specs"] = append(out["equipment_specs"], map[string]string{"model": e.SourceModelLabel, "height_u": ""})
			}
		}
	}
	text := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	for _, f := range m.Facilities {
		if f.Kind == "room" {
			out["rooms"] = append(out["rooms"], map[string]string{"room_id": f.EntityID, "name": f.Label, "site": text(f.ParentID)})
		}
		if f.Kind == "rack" {
			out["racks"] = append(out["racks"], map[string]string{"rack_id": f.EntityID, "room_id": text(f.ParentID), "capacity_u": stringInt(f.CapacityU), "width_mm": text(f.WidthMM), "depth_mm": text(f.DepthMM), "orientation": text(f.OrientationDegrees), "x": text(f.XMM), "y": text(f.YMM), "frame": text(f.CoordinateFrameID), "unit": "mm"})
		}
	}
	for _, p := range m.Placements {
		if devices[p.EquipmentID] {
			out["placements"] = append(out["placements"], map[string]string{"device_id": p.EquipmentID, "rack_id": p.RackID, "start_u": stringInt(p.StartU), "height_u": stringInt(p.HeightU), "face": p.MountFace, "position": p.PositionCode})
		}
	}
	return out
}
