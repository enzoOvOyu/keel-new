package ai

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"sort"
	"strings"
)

// Coordinates are selected by the Agent; every effective row is inspected locally.
// The graph reports evidence, never speed-based or degree-based classifications.
type KickoffGraphRange struct {
	Region   string `json:"region"`
	Sheet    string `json:"sheet"`
	StartRow int    `json:"start_row"`
	EndRow   int    `json:"end_row"`
	ADevice  int    `json:"a_device"`
	APort    int    `json:"a_port"`
	BDevice  int    `json:"b_device"`
	BPort    int    `json:"b_port"`
	Rate     int    `json:"rate,omitempty"`
	AModel   int    `json:"a_model,omitempty"`
	BModel   int    `json:"b_model,omitempty"`
}
type kickoffGraphNode struct {
	ID        string           `json:"id"`
	Ports     int              `json:"ports"`
	Neighbors int              `json:"neighbors"`
	Models    []string         `json:"models"`
	Rates     []string         `json:"rates"`
	Peers     []string         `json:"peers"`
	Evidence  []KickoffCellRef `json:"evidence"`
}

func inspectKickoffConnectionGraph(original, safe bom.Workbook, ranges []KickoffGraphRange, offset int) (any, error) {
	if len(ranges) == 0 || len(ranges) > 128 || offset < 0 {
		return nil, fmt.Errorf("GRAPH_RANGE_INVALID")
	}
	type node struct {
		ports, peers, models, rates map[string]bool
		evidence                    []KickoffCellRef
	}
	nodes := map[string]*node{}
	originalCells := map[string]map[[2]int]bom.Cell{}
	safeCells := kickoffCellIndex(safe)
	for _, s := range original.Sheets {
		cells := map[[2]int]bom.Cell{}
		for _, c := range s.Cells {
			c.Sheet = s.Name
			cells[[2]int{c.Row, c.Column}] = c
		}
		originalCells[s.Name] = cells
	}
	edges := map[string]bool{}
	claims := map[string]map[string]bool{}
	rows, incomplete := 0, 0
	exceptions := []map[string]any{}
	for _, r := range ranges {
		columns := []int{r.ADevice, r.APort, r.BDevice, r.BPort}
		for _, col := range columns {
			if col < 1 || !kickoffBounds(original, r.Sheet, engineering.Bounds{StartRow: r.StartRow, EndRow: r.EndRow, StartColumn: col, EndColumn: col}) {
				return nil, fmt.Errorf("GRAPH_RANGE_INVALID: %s", r.Region)
			}
		}
		cells := originalCells[r.Sheet]
		for row := r.StartRow; row <= r.EndRow; row++ {
			values := []string{}
			refs := []KickoffCellRef{}
			nonempty := false
			for _, col := range columns {
				c := cells[[2]int{row, col}]
				v := strings.TrimSpace(c.Value)
				if c.Sensitive || bom.Sensitive(v) || c.Formula != "" {
					v = ""
				}
				nonempty = nonempty || v != ""
				values = append(values, v)
				refs = append(refs, KickoffCellRef{r.Sheet, c.Ref})
			}
			if !nonempty {
				continue
			}
			rows++
			if values[0] == "" || values[1] == "" || values[2] == "" || values[3] == "" {
				incomplete++
				if len(exceptions) < 20 {
					exceptions = append(exceptions, map[string]any{"region": r.Region, "row": row, "reason": "missing endpoint or port"})
				}
				continue
			}
			a, b := kickoffRelationKey(values[0]), kickoffRelationKey(values[2])
			endpoints := []string{a + "\x00" + values[1], b + "\x00" + values[3]}
			key := endpoints[0] + "\x01" + endpoints[1]
			if endpoints[0] > endpoints[1] {
				key = endpoints[1] + "\x01" + endpoints[0]
			}
			edges[key] = true
			for side, name := range []string{a, b} {
				n := nodes[name]
				if n == nil {
					n = &node{ports: map[string]bool{}, peers: map[string]bool{}, models: map[string]bool{}, rates: map[string]bool{}, evidence: []KickoffCellRef{}}
					nodes[name] = n
				}
				n.ports[values[side*2+1]] = true
				peer := []string{b, a}[side]
				n.peers[peer] = true
				if len(n.evidence) < 4 {
					n.evidence = append(n.evidence, refs[side*2])
				}
				for field, col := range map[string]int{"model": []int{r.AModel, r.BModel}[side], "rate": r.Rate} {
					if col <= 0 {
						continue
					}
					c := cells[[2]int{row, col}]
					sc := safeCells[KickoffCellRef{r.Sheet, c.Ref}]
					if sc.Value != "" && !sc.Sensitive && sc.Formula == "" {
						if field == "model" {
							n.models[sc.Value] = true
						} else {
							n.rates[sc.Value] = true
						}
					}
				}
				if claims[endpoints[side]] == nil {
					claims[endpoints[side]] = map[string]bool{}
				}
				claims[endpoints[side]][endpoints[1-side]] = true
			}
		}
	}
	conflictPorts := 0
	for _, peers := range claims {
		if len(peers) > 1 {
			conflictPorts++
		}
	}
	keys := []string{}
	for k := range nodes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(nodes[keys[i]].peers) != len(nodes[keys[j]].peers) {
			return len(nodes[keys[i]].peers) > len(nodes[keys[j]].peers)
		}
		return keys[i] < keys[j]
	})
	list := []kickoffGraphNode{}
	sorted := func(m map[string]bool, hash bool) []string {
		v := []string{}
		for k := range m {
			if hash {
				k = "node-" + bom.StableID(k)
			}
			v = append(v, k)
		}
		sort.Strings(v)
		return v[:min(12, len(v))]
	}
	for _, key := range keys[min(offset, len(keys)):min(offset+32, len(keys))] {
		n := nodes[key]
		list = append(list, kickoffGraphNode{ID: "node-" + bom.StableID(key), Ports: len(n.ports), Neighbors: len(n.peers), Models: sorted(n.models, false), Rates: sorted(n.rates, false), Peers: sorted(n.peers, true), Evidence: n.evidence})
	}
	return map[string]any{"source_rows": rows, "unique_connections": len(edges), "device_count": len(nodes), "incomplete_rows": incomplete, "conflicting_ports": conflictPorts, "exceptions": exceptions, "nodes": list, "offset": offset, "has_more": offset+32 < len(keys), "classification": "Use source models, port context and neighbors together; speed and degree alone do not prove roles. Same node across networks remains one device."}, nil
}
