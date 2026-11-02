package engineering

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func MergeStandardInput(current PhysicalModel, q InputPreviewRequest) (PhysicalModel, error) {
	if preview := PreviewStepInput(q); preview.Valid && len(preview.Rows) == 0 {
		return current, nil
	}
	if strings.HasPrefix(q.TableID, "power_") || q.TableID == "rack_capacity" {
		return mergePowerInput(current, q)
	}
	if strings.HasPrefix(q.TableID, "network_") {
		return mergeNetworkInput(current, q)
	}
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
		if code == "" {
			return -1, nil
		}
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
	only := func(kind, parent string) int {
		found := -1
		for i, f := range next.Facilities {
			if f.Kind != kind || parent != "" && (f.ParentID == nil || *f.ParentID != parent) {
				continue
			}
			if found >= 0 {
				return -1
			}
			found = i
		}
		return found
	}
	roomFor := func(code string) (int, error) {
		if code == "" {
			return only("room", ""), nil
		}
		room, err := find("room", code, "")
		if err == nil && room < 0 {
			err = fmt.Errorf("STEP_ROOM_REQUIRED")
		}
		return room, err
	}
	rackFor := func(room int, code, label string) (int, error) {
		parent := ""
		if room >= 0 {
			parent = next.Facilities[room].EntityID
		}
		key := code
		if key == "" {
			key = label
		}
		if key == "" {
			return only("rack", parent), nil
		}
		rack, err := find("rack", key, parent)
		if err == nil && rack < 0 && label != "" && label != key {
			rack, err = find("rack", label, parent)
		}
		if err != nil {
			return -1, err
		}
		if rack < 0 && parent != "" {
			for i, f := range next.Facilities {
				if f.Kind == "rack" && f.ParentID == nil && (f.Code == key || f.Code == "" && (f.Label == key || label != "" && f.Label == label)) {
					if rack >= 0 {
						return -1, fmt.Errorf("STEP_FACILITY_AMBIGUOUS")
					}
					rack = i
				}
			}
		}
		if rack < 0 {
			rack = len(next.Facilities)
			next.Facilities = append(next.Facilities, FacilityModel{EntityVersion: version("rack_code", parent+"/"+key), Kind: "rack", Code: code, Label: label})
		}
		if parent != "" {
			next.Facilities[rack].ParentID = &parent
		}
		return rack, nil
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
			siteKey := v[0]
			if siteKey == "" {
				siteKey = v[1]
			}
			site, e := find("site", siteKey, "")
			if e == nil && site < 0 && v[1] != "" && v[1] != siteKey {
				site, e = find("site", v[1], "")
			}
			if e != nil {
				return current, e
			}
			if siteKey == "" {
				site = only("site", "")
			}
			if site < 0 && siteKey != "" {
				site = len(next.Facilities)
				next.Facilities = append(next.Facilities, FacilityModel{EntityVersion: version("site_code", siteKey), Kind: "site", Code: v[0], Label: v[1]})
			}
			parent := ""
			if site >= 0 {
				f := &next.Facilities[site]
				if v[0] != "" {
					f.Code = v[0]
				}
				if v[1] != "" {
					f.Label = v[1]
				}
				parent = f.EntityID
			}
			roomKey := v[2]
			if roomKey == "" {
				roomKey = v[3]
			}
			room, e := find("room", roomKey, parent)
			if e == nil && room < 0 && v[3] != "" && v[3] != roomKey {
				room, e = find("room", v[3], parent)
			}
			if e != nil {
				return current, e
			}
			if roomKey == "" {
				room = only("room", parent)
			}
			if room < 0 && roomKey == "" {
				continue
			}
			if room < 0 && parent != "" {
				for i, f := range next.Facilities {
					if f.Kind == "room" && f.ParentID == nil && (f.Code == roomKey || f.Code == "" && (f.Label == roomKey || v[3] != "" && f.Label == v[3])) {
						if room >= 0 {
							return current, fmt.Errorf("STEP_FACILITY_AMBIGUOUS")
						}
						room = i
					}
				}
			}
			if room < 0 {
				room = len(next.Facilities)
				next.Facilities = append(next.Facilities, FacilityModel{EntityVersion: version("room_code", parent+"/"+roomKey), Kind: "room", Code: v[2], Label: v[3]})
			}
			f := &next.Facilities[room]
			if v[2] != "" {
				f.Code = v[2]
			}
			if v[3] != "" {
				f.Label = v[3]
			}
			if parent != "" {
				f.ParentID = &parent
			}
			if v[4] != "" {
				frame := StableID(current.ProjectID, "coordinate-frame", v[4])
				f.CoordinateFrameID = &frame
			}
			if v[5] != "" {
				dimension, _ := unitScale(v[5])
				if dimension != "length" {
					return current, fmt.Errorf("STEP_COORDINATE_UNIT_INVALID")
				}
			}

		case "racks":
			room, e := roomFor(v[0])
			if e != nil {
				return current, e
			}
			rack, e := rackFor(room, v[1], v[2])
			if e != nil {
				return current, e
			}
			// A row without a unique physical reference remains in its immutable
			// standard input. Do not manufacture a rack code or an empty identity.
			if rack < 0 {
				continue
			}
			r := &next.Facilities[rack]
			if v[2] != "" {
				r.Label = v[2]
			}
			if v[1] != "" {
				r.Code = v[1]
			}

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

			if v[7] != "" {
				frame := StableID(current.ProjectID, "coordinate-frame", v[7])
				r.CoordinateFrameID = &frame
			} else if r.CoordinateFrameID == nil && room >= 0 {
				r.CoordinateFrameID = next.Facilities[room].CoordinateFrameID
			}
			for index := 0; index < 3; index++ {
				if v[8+index] == "" {
					continue
				}
				if r.CoordinateFrameID == nil {
					return current, fmt.Errorf("STEP_COORDINATE_FRAME_REQUIRED")
				}
				dimension, scale := unitScale(v[11])
				if dimension != "length" {
					return current, fmt.Errorf("STEP_COORDINATE_UNIT_INVALID")
				}
				value, ok := new(big.Rat).SetString(v[8+index])
				if !ok {
					return current, fmt.Errorf("STEP_INPUT_INVALID")
				}
				value.Mul(value, scale).Mul(value, big.NewRat(1000, 1))
				mm := strings.TrimRight(strings.TrimRight(value.FloatString(12), "0"), ".")
				if mm == "" || mm == "-0" {
					mm = "0"
				}
				switch index {
				case 0:
					r.XMM = &mm
				case 1:
					r.YMM = &mm
				case 2:
					r.ZMM = &mm
				}
			}
			if v[12] != "" {
				r.OrientationDegrees = text(v[12])
			}
		case "equipment_specs":
			// Inventory rows with an explicit SN can introduce a planned instance
			// even when the device has no termination row. Model-only rows remain
			// in the source input and must not manufacture physical identities.
			if v[0] == "" {
				continue
			}
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
				found = len(next.Equipment)
				serial := v[0]
				next.Equipment = append(next.Equipment, EquipmentPlan{EntityVersion: version("equipment_serial", serial), EquipmentType: v[1], SourceModelLabel: v[2], Serial: &serial, Label: serial, Quantity: 1, IdentityKind: "planned_instance"})
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
			room, e := roomFor(v[1])
			if e != nil {
				return current, e
			}
			rack := -1
			if v[2] == "" {
				for _, old := range next.Placements {
					if old.EquipmentID == equipment {
						for i, f := range next.Facilities {
							if f.EntityID == old.RackID {
								rack = i
							}
						}
					}
				}
			}
			if rack < 0 {
				rack, e = rackFor(room, v[2], "")
				if e != nil {
					return current, e
				}
			}
			if rack < 0 {
				continue
			}
			if v[3] != "" && v[3] != "u" && v[3] != "non_u" {
				return current, fmt.Errorf("STEP_INSTALLATION_TYPE_REQUIRED")
			}
			if v[6] != "" && v[6] != "front" && v[6] != "rear" && v[6] != "both" && v[6] != "unknown" {
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
				if p.InstallationType == "" {
					p.InstallationType = next.Placements[existing].InstallationType
				}
				if p.MountFace == "" {
					p.MountFace = next.Placements[existing].MountFace
				}
				next.Placements[existing] = p
			}
		}
	}
	return next, nil
}
