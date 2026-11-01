package engineering

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
)

// MergeTerminations is additive. It returns a detached model so failed input
// never mutates the existing revision. Existing IDs and other domains survive.
func MergeTerminations(current PhysicalModel, q InputPreviewRequest) (PhysicalModel, error) {
	if q.TableID != "terminations" {
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
	equipment := map[string]int{}
	ports := map[string]int{}
	connections := map[string]int{}
	used := map[string]bool{}
	for i, v := range next.Equipment {
		if v.Serial != nil && strings.TrimSpace(*v.Serial) != "" {
			key := strings.TrimSpace(*v.Serial)
			if _, ok := equipment[key]; ok {
				return current, fmt.Errorf("STEP_SERIAL_AMBIGUOUS: %s", key)
			}
			equipment[key] = i
		}
	}
	portKey := func(device, name string) string { return device + "\x00" + strings.TrimSpace(name) }
	for i, v := range next.Ports {
		key := portKey(v.EquipmentID, v.Name)
		if _, ok := ports[key]; ok {
			return current, fmt.Errorf("STEP_PORT_AMBIGUOUS: %s", v.Name)
		}
		ports[key] = i
	}
	connectionKey := func(a, b string) string { ids := []string{a, b}; sort.Strings(ids); return strings.Join(ids, "/") }
	for i, v := range next.Connections {
		key := connectionKey(v.APortID, v.ZPortID)
		if _, ok := connections[key]; ok {
			return current, fmt.Errorf("STEP_CONNECTION_AMBIGUOUS")
		}
		connections[key] = i
		used[v.APortID] = true
		used[v.ZPortID] = true
		for _, id := range v.PatchPortIDs {
			used[id] = true
		}
	}
	version := func(kind, key string) EntityVersion {
		return EntityVersion{EntityID: StableID(current.ProjectID, kind, key), Maturity: Estimated, EvidenceIDs: []string{}, Attributes: []AttributeValue{}}
	}
	for row, values := range preview.Rows {
		v := append([]string{}, values...)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		endpoints := []string{}
		for _, side := range []struct{ serial, mac, port string }{{v[0], v[1], v[2]}, {v[3], v[5], v[6]}} {
			i, ok := equipment[side.serial]
			if !ok {
				i = len(next.Equipment)
				serial := side.serial
				next.Equipment = append(next.Equipment, EquipmentPlan{EntityVersion: version("equipment_serial", serial), EquipmentType: "device", Quantity: 1, IdentityKind: "planned_instance", Label: serial, Serial: &serial})
				equipment[serial] = i
			}
			device := next.Equipment[i].EntityID
			key := portKey(device, side.port)
			p, ok := ports[key]
			mac, _ := net.ParseMAC(side.mac)
			normalized := mac.String()
			if !ok {
				p = len(next.Ports)
				next.Ports = append(next.Ports, PlannedPort{EntityVersion: version("port", key), EquipmentID: device, Name: side.port, MAC: &normalized})
				ports[key] = p
			} else {
				existing := next.Ports[p].MAC
				if existing != nil {
					parsed, err := net.ParseMAC(*existing)
					if err != nil || parsed.String() != normalized {
						return current, fmt.Errorf("STEP_MAC_CONFLICT: row %d", row+2)
					}
				} else {
					next.Ports[p].MAC = &normalized
				}
			}
			endpoints = append(endpoints, next.Ports[p].EntityID)
		}
		if endpoints[0] == endpoints[1] {
			return current, fmt.Errorf("STEP_SELF_CONNECTION: row %d", row+2)
		}
		key := connectionKey(endpoints[0], endpoints[1])
		if i, ok := connections[key]; ok {
			c := &next.Connections[i]
			if c.TerminationType != "" && c.TerminationType != v[7] {
				return current, fmt.Errorf("STEP_TERMINATION_CONFLICT: row %d", row+2)
			}
			if c.APortID != endpoints[0] {
				// The reversed row's B position describes our A endpoint. Keep the
				// original orientation; the archived input retains that new declaration.
				c.TerminationType = v[7]
				continue
			}
			if c.BRackPosition != "" && c.BRackPosition != v[4] {
				return current, fmt.Errorf("STEP_POSITION_CONFLICT: row %d", row+2)
			}
			c.TerminationType = v[7]
			c.BRackPosition = v[4]
			continue
		}
		if used[endpoints[0]] || used[endpoints[1]] {
			return current, fmt.Errorf("STEP_PORT_OCCUPIED: row %d", row+2)
		}
		used[endpoints[0]] = true
		used[endpoints[1]] = true

		connections[key] = len(next.Connections)
		next.Connections = append(next.Connections, PlannedConnection{EntityVersion: version("connection", key), APortID: endpoints[0], ZPortID: endpoints[1], PatchPortIDs: []string{}, TerminationType: v[7], BRackPosition: v[4]})
	}
	return next, nil
}
