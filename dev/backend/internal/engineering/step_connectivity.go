package engineering

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

func mergeConnectivityInput(current PhysicalModel, q InputPreviewRequest) (PhysicalModel, error) {
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
	equipment := map[string]string{}
	serialByID := map[string]string{}
	for _, v := range next.Equipment {
		if v.Serial != nil {
			serial := strings.TrimSpace(*v.Serial)
			if equipment[serial] != "" {
				return current, fmt.Errorf("STEP_SERIAL_AMBIGUOUS")
			}
			equipment[serial] = v.EntityID
			serialByID[v.EntityID] = serial
		}
	}
	version := func(kind, key string) EntityVersion {
		return EntityVersion{EntityID: StableID(current.ProjectID, kind, key), Maturity: Estimated, EvidenceIDs: []string{}, Attributes: []AttributeValue{}}
	}
	port := func(owner, name string, create bool) (int, error) {
		found := -1
		for i, p := range next.Ports {
			if p.EquipmentID == owner && p.Name == name {
				if found >= 0 {
					return -1, fmt.Errorf("STEP_PORT_AMBIGUOUS")
				}
				found = i
			}
		}
		if found < 0 && create {
			found = len(next.Ports)
			next.Ports = append(next.Ports, PlannedPort{EntityVersion: version("port", owner+"\x00"+name), EquipmentID: owner, Name: name})
		}
		if found < 0 {
			return -1, fmt.Errorf("STEP_PORT_REQUIRED")
		}
		return found, nil
	}
	if q.TableID == "port_specs" {
		for _, row := range preview.Rows {
			v := append([]string{}, row...)
			for i := range v {
				v[i] = strings.TrimSpace(v[i])
			}
			owner := equipment[v[0]]
			if owner == "" {
				return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
			}
			n, ok := new(big.Rat).SetString(v[4])
			if !ok {
				return current, fmt.Errorf("STEP_RATE_INVALID")
			}
			scale := int64(0)
			switch v[5] {
			case "bps":
				scale = 1
			case "kbps":
				scale = 1000
			case "Mbps":
				scale = 1000000
			case "Gbps":
				scale = 1000000000
			case "Tbps":
				scale = 1000000000000
			default:
				return current, fmt.Errorf("STEP_RATE_UNIT_INVALID")
			}
			n.Mul(n, new(big.Rat).SetInt64(scale))
			if !n.IsInt() || !n.Num().IsInt64() || n.Sign() <= 0 {
				return current, fmt.Errorf("STEP_RATE_INVALID")
			}
			rate := n.Num().Int64()
			p, e := port(owner, v[1], true)
			if e != nil {
				return current, e
			}
			if (v[6] == "") != (v[7] == "") {
				return current, fmt.Errorf("STEP_BREAKOUT_FIELDS_REQUIRED")
			}
			if v[6] != "" {
				parent, e := port(owner, v[6], true)
				if e != nil {
					return current, e
				}
				index, e := strconv.Atoi(v[7])
				if e != nil || index <= 0 || parent == p {
					return current, fmt.Errorf("STEP_BREAKOUT_FIELDS_REQUIRED")
				}
				id := next.Ports[parent].EntityID
				next.Ports[p].ParentPortID = &id
				next.Ports[p].BreakoutIndex = &index
			}
			next.Ports[p].Connector = v[3]
			next.Ports[p].Medium = v[2]
			next.Ports[p].RateBPS = &rate
		}
		for _, issue := range ValidateConnectivity(next) {
			if strings.HasPrefix(issue.Code, "BREAKOUT_") {
				return current, fmt.Errorf("STEP_BREAKOUT_CONFLICT")
			}
		}
		return next, nil
	}
	if q.TableID != "patch_paths" {
		return current, fmt.Errorf("STEP_TABLE_NOT_SUPPORTED")
	}
	// Compare readable endpoint references directly; never split untrusted names
	// at a guessed delimiter. Ambiguous display references require selection.
	portNames := map[string]string{}
	for _, p := range next.Ports {
		portNames[p.EntityID] = serialByID[p.EquipmentID] + "/" + p.Name
	}
	type segment struct {
		sequence int
		in, out  string
		reverse  bool
	}
	groups := map[int][]segment{}
	for _, row := range preview.Rows {
		v := append([]string{}, row...)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		connection := -1
		reverse := false
		for i, c := range next.Connections {
			a, b := portNames[c.APortID], portNames[c.ZPortID]
			if v[0] == c.EntityID || v[0] == a+" -> "+b || v[0] == b+" -> "+a {
				if connection >= 0 {
					return current, fmt.Errorf("STEP_CONNECTION_AMBIGUOUS")
				}
				connection = i
				reverse = v[0] == b+" -> "+a && v[0] != a+" -> "+b && v[0] != c.EntityID
			}
		}
		if connection < 0 {
			return current, fmt.Errorf("STEP_CONNECTION_REQUIRED")
		}
		owner := equipment[v[2]]
		if owner == "" {
			return current, fmt.Errorf("STEP_EQUIPMENT_REQUIRED")
		}
		a, e := port(owner, v[3], false)
		if e != nil {
			return current, e
		}
		b, e := port(owner, v[4], false)
		if e != nil {
			return current, e
		}
		if a == b {
			return current, fmt.Errorf("STEP_PATCH_PORTS_INVALID")
		}
		sequence, e := strconv.Atoi(v[1])
		if e != nil || sequence <= 0 {
			return current, fmt.Errorf("STEP_INPUT_INVALID")
		}
		groups[connection] = append(groups[connection], segment{sequence, next.Ports[a].EntityID, next.Ports[b].EntityID, reverse})
	}
	for connection, segments := range groups {
		sort.Slice(segments, func(i, j int) bool { return segments[i].sequence < segments[j].sequence })
		path := []string{}
		for i, v := range segments {
			if v.sequence != i+1 {
				return current, fmt.Errorf("STEP_PATCH_SEQUENCE_INVALID")
			}
			path = append(path, v.in, v.out)
		}
		for _, v := range segments {
			if v.reverse != segments[0].reverse {
				return current, fmt.Errorf("STEP_PATCH_PATH_INVALID")
			}
		}
		if segments[0].reverse {
			path = []string{}
			for i := len(segments) - 1; i >= 0; i-- {
				path = append(path, segments[i].out, segments[i].in)
			}
		}
		next.Connections[connection].PatchPortIDs = path
	}
	for _, issue := range ValidateConnectivity(next) {
		if issue.PublishTarget == ConnectivityTarget {
			return current, fmt.Errorf("STEP_PATCH_PATH_INVALID")
		}
	}
	return next, nil
}
