package engineering

import (
	"sort"
	"strconv"
	"strings"
)

// ResolveEndpointPlacements joins only exact SNs from the same task. Original
// input rows retain the evidence; no combined location string is manufactured.
func ResolveEndpointPlacements(model PhysicalModel, inputs []ModelingInput) []EndpointPlacementReference {
	refs := map[string]EndpointPlacementReference{}
	for _, input := range inputs {
		if input.TableID != "terminations" {
			continue
		}
		for _, row := range PreviewStepInput(InputPreviewRequest{TableID: input.TableID, CSV: input.CSV}).Rows {
			for _, column := range []int{0, 3} {
				sn := strings.TrimSpace(row[column])
				if sn != "" {
					refs[sn] = EndpointPlacementReference{Serial: sn, State: "missing", InputRows: []int{}}
				}
			}
		}
	}
	for _, input := range inputs {
		if input.TableID != "placements" {
			continue
		}
		for i, row := range PreviewStepInput(InputPreviewRequest{TableID: input.TableID, CSV: input.CSV}).Rows {
			sn := strings.TrimSpace(row[0])
			old, used := refs[sn]
			if !used {
				continue
			}
			v := EndpointPlacementReference{Serial: sn, State: "missing", RoomCode: strings.TrimSpace(row[1]), RackCode: strings.TrimSpace(row[2]), PositionCode: strings.TrimSpace(row[7]), InputVersion: input.Version, InputRows: []int{i + 2}}
			if u, err := strconv.Atoi(strings.TrimSpace(row[4])); err == nil && u > 0 {
				v.StartU = &u
			}
			if v.RackCode != "" && (v.StartU != nil || v.PositionCode != "") {
				v.State = "candidate"
			}
			if len(old.InputRows) > 0 {
				old.InputRows = append(old.InputRows, v.InputRows...)
				if old.RoomCode != v.RoomCode || old.RackCode != v.RackCode || old.PositionCode != v.PositionCode || !sameEndpointU(old.StartU, v.StartU) {
					old.State = "conflict"
				}
				refs[sn] = old
			} else {
				refs[sn] = v
			}
		}
	}
	equipment := map[ID]string{}
	counts := map[string]int{}
	for _, e := range model.Equipment {
		if e.Serial != nil {
			sn := strings.TrimSpace(*e.Serial)
			equipment[e.EntityID] = sn
			counts[sn]++
			if v, ok := refs[sn]; ok {
				v.EquipmentID = e.EntityID
				refs[sn] = v
			}
		}
	}
	facilities := map[ID]FacilityModel{}
	for _, f := range model.Facilities {
		facilities[f.EntityID] = f
	}
	for _, p := range model.Placements {
		sn := equipment[p.EquipmentID]
		old, used := refs[sn]
		if !used {
			continue
		}
		rack, found := facilities[p.RackID]
		v := EndpointPlacementReference{Serial: sn, EquipmentID: p.EquipmentID, PlacementID: p.EntityID, State: "model", RackCode: rack.Code, StartU: p.StartU, PositionCode: p.PositionCode, InputVersion: old.InputVersion, InputRows: old.InputRows}
		if v.RackCode == "" {
			v.RackCode = rack.Label
		}
		if rack.ParentID != nil {
			v.RoomCode = facilities[*rack.ParentID].Code
		}
		if !found || rack.Kind != "rack" || (v.StartU == nil && v.PositionCode == "") {
			v.State = "missing"
		}
		if old.State == "conflict" || old.PlacementID != "" || old.RackCode != "" && old.RackCode != v.RackCode || old.StartU != nil && !sameEndpointU(old.StartU, v.StartU) || old.RoomCode != "" && v.RoomCode != "" && old.RoomCode != v.RoomCode || old.PositionCode != "" && old.PositionCode != v.PositionCode {
			v.State = "conflict"
		}
		refs[sn] = v
	}
	serials := []string{}
	for sn := range refs {
		serials = append(serials, sn)
	}
	sort.Strings(serials)
	out := []EndpointPlacementReference{}
	for _, sn := range serials {
		v := refs[sn]
		if counts[sn] > 1 {
			v.State = "conflict"
			v.EquipmentID = ""
		}
		out = append(out, v)
	}
	return out
}

func sameEndpointU(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
