package engineering

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func MergeStandardInput(current PhysicalModel, q InputPreviewRequest) (PhysicalModel, error) {
	if q.TableID == "port_specs" || q.TableID == "patch_paths" {
		return mergeConnectivityInput(current, q)
	}
	if q.TableID == "terminations" {
		return MergeTerminations(current, q)
	}
	switch q.TableID {
	case "rooms", "racks", "equipment_specs", "placements":
	default:
		return current, fmt.Errorf("STEP_TABLE_NOT_SUPPORTED")
	}
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
	version := func(kind, key string) EntityVersion {
		return EntityVersion{EntityID: StableID(current.ProjectID, kind, key), Maturity: Estimated, EvidenceIDs: []string{}, Attributes: []AttributeValue{}}
	}
	// Codes are resolved within their parent, never by an unqualified rack name.
	find := func(kind, code, parent string) (int, error) {
		found := -1
		for i, v := range next.Facilities {
			p := ""
			if v.ParentID != nil {
				p = *v.ParentID
			}
			if v.Kind == kind && (v.Code == code || v.Code == "" && v.Label == code) && (parent == "" || p == parent) {
				if found >= 0 {
					return -1, fmt.Errorf("STEP_FACILITY_AMBIGUOUS")
				}
				found = i
			}
		}
		return found, nil
	}
	integer := func(s string) (*int, error) {
		if s == "" {
			return nil, nil
		}
		n, e := strconv.Atoi(s)
		if e != nil || n <= 0 {
			return nil, fmt.Errorf("STEP_INPUT_INVALID")
		}
		return &n, nil
	}
	text := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	for _, row := range preview.Rows {
		v := append([]string{}, row...)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		switch q.TableID {
		case "rooms":
			site, e := find("site", v[0], "")
			if e != nil {
				return current, e
			}
			if site < 0 {
				site = len(next.Facilities)
				next.Facilities = append(next.Facilities, FacilityModel{EntityVersion: version("site_code", v[0]), Kind: "site", Code: v[0], Label: v[1]})
			} else {
				next.Facilities[site].Code = v[0]
				next.Facilities[site].Label = v[1]
			}
			parent := next.Facilities[site].EntityID
			for _, existing := range next.Facilities {
				if existing.Kind == "room" && existing.Code == v[2] && (existing.ParentID == nil || *existing.ParentID != parent) {
					return current, fmt.Errorf("STEP_FACILITY_AMBIGUOUS")
				}
			}
			room, e := find("room", v[2], parent)
			if e != nil {
				return current, e
			}
			if room < 0 {
				next.Facilities = append(next.Facilities, FacilityModel{EntityVersion: version("room_code", parent+"/"+v[2]), Kind: "room", Code: v[2], Label: v[3], ParentID: &parent})
			} else {
				next.Facilities[room].Code = v[2]
				next.Facilities[room].Label = v[3]
			}
		case "racks":
			room, e := find("room", v[0], "")
			if e != nil {
				return current, e
			}
			if room < 0 {
				return current, fmt.Errorf("STEP_ROOM_REQUIRED")
			}
			parent := next.Facilities[room].EntityID
			rack, e := find("rack", v[1], parent)
			if e != nil {
				return current, e
			}
			if rack < 0 {
				rack = len(next.Facilities)
				next.Facilities = append(next.Facilities, FacilityModel{EntityVersion: version("rack_code", parent+"/"+v[1]), Kind: "rack", Code: v[1], ParentID: &parent})
			}
			r := &next.Facilities[rack]
			r.Label = v[2]
			r.Code = v[1]
			if v[3] != "" {
				r.CapacityU, e = integer(v[3])
				if e != nil {
					return current, e
				}
			}
			if v[4] != "" {
				r.WidthMM = text(v[4])
			}
			if v[5] != "" {
				r.DepthMM = text(v[5])
			}
			if v[6] != "" {
				r.HeightMM = text(v[6])
			}
		case "equipment_specs":
			found := -1
			for i, eq := range next.Equipment {
				if eq.Serial != nil && *eq.Serial == v[0] {
					if found >= 0 {
						return current, fmt.Errorf("STEP_SERIAL_AMBIGUOUS")
					}
					found = i
				}
			}
			if found < 0 {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			eq := &next.Equipment[found]
			eq.EquipmentType = v[1]
			eq.SourceModelLabel = v[2]
			if v[3] != "" {
				eq.HeightU, e = integer(v[3])
				if e != nil {
					return current, e
				}
			}
		case "placements":
			found := -1
			for i, eq := range next.Equipment {
				if eq.Serial != nil && *eq.Serial == v[0] {
					if found >= 0 {
						return current, fmt.Errorf("STEP_SERIAL_AMBIGUOUS")
					}
					found = i
				}
			}
			if found < 0 {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			equipment := next.Equipment[found].EntityID
			room, e := find("room", v[1], "")
			if e != nil {
				return current, e
			}
			if room < 0 {
				return current, fmt.Errorf("STEP_ROOM_REQUIRED")
			}
			rack, e := find("rack", v[2], next.Facilities[room].EntityID)
			if e != nil {
				return current, e
			}
			if rack < 0 {
				return current, fmt.Errorf("STEP_RACK_REQUIRED")
			}
			if v[3] != "u" && v[3] != "non_u" {
				return current, fmt.Errorf("STEP_INSTALLATION_TYPE_REQUIRED")
			}
			if v[6] != "front" && v[6] != "rear" && v[6] != "both" && v[6] != "unknown" {
				return current, fmt.Errorf("STEP_MOUNT_FACE_INVALID")
			}
			start, e := integer(v[4])
			if e != nil {
				return current, e
			}
			height, e := integer(v[5])
			if e != nil {
				return current, e
			}
			if v[3] == "u" && (start == nil || height == nil) {
				return current, fmt.Errorf("STEP_U_REQUIRED")
			}
			if v[3] == "non_u" && (v[7] == "" || start != nil || height != nil) {
				return current, fmt.Errorf("STEP_NON_U_POSITION_REQUIRED")
			}
			if height != nil && next.Equipment[found].HeightU != nil && *height != *next.Equipment[found].HeightU {
				return current, fmt.Errorf("STEP_HEIGHT_CONFLICT")
			}
			p := RackPlacement{EntityVersion: version("placement", equipment), EquipmentID: equipment, RackID: next.Facilities[rack].EntityID, StartU: start, HeightU: height, MountFace: v[6], InstallationType: v[3], PositionCode: v[7]}
			existing := -1
			for i, old := range next.Placements {
				if old.EquipmentID == equipment {
					if existing >= 0 {
						return current, fmt.Errorf("STEP_PLACEMENT_AMBIGUOUS")
					}
					existing = i
				}
			}
			if existing < 0 {
				next.Placements = append(next.Placements, p)
			} else {
				p.EntityVersion = next.Placements[existing].EntityVersion
				next.Placements[existing] = p
			}
		}
	}
	return next, nil
}
