package ai

import (
	"fmt"
	"sort"
	"strings"

	"keel.local/dev/internal/bom"
)

// Recover explicit table headers independently of the model's chosen header row.
// Only repair an existing extractor: this never invents a table or source value.
func sourceHeaders(s bom.Sheet, p SheetPlan) map[int]bom.Cell {
	scores := map[int]int{}
	for _, c := range s.Cells {
		if c.Row <= 50 && !c.Sensitive && bom.Field(c.Value) != "" {
			scores[c.Row]++
		}
	}
	best := 0
	for row, score := range scores {
		if score > scores[best] || score == scores[best] && row < best {
			best = row
		}
	}
	rows := map[int]bool{}
	for _, row := range p.HeaderRows {
		rows[row] = true
	}
	if scores[best] >= 3 {
		rows[best] = true
	}
	out := map[int]bom.Cell{}
	for _, c := range s.Cells {
		if !c.Sensitive && rows[c.Row] {
			old, ok := out[c.Column]
			if !ok || c.Row == best || old.Row != best && c.Row > old.Row {
				out[c.Column] = c
			}
		}
	}
	return out
}

func compatibleBinding(field, meaning string, t TablePlan) bool {
	if field == meaning {
		return true
	}
	if (field == "brand" || field == "manufacturer") && (meaning == "brand" || meaning == "manufacturer") {
		return true
	}
	if (field == "role" || field == "network_role") && (meaning == "role" || meaning == "network_role") {
		return true
	}
	if field == "name" || field == "description" || field == "specification" {
		return true
	}
	if t.Kind == "physical_connection" {
		if (field == "endpoint_a_device" || field == "endpoint_b_device") && meaning == "hostname" {
			return true
		}
		if (field == "endpoint_a_port" || field == "endpoint_b_port") && meaning == "port_index" {
			return true
		}
		if (field == "endpoint_a_optic" || field == "endpoint_b_optic") && meaning == "model" {
			return true
		}
	}
	if t.Kind == "component" {
		if field == "model" && (meaning == "endpoint_a_optic" || meaning == "endpoint_b_optic") {
			return true
		}
		if field == "parent_uid" && (meaning == "hostname" || meaning == "endpoint_a_device" || meaning == "endpoint_b_device") {
			return true
		}
		if field == "port_index" && (meaning == "endpoint_a_port" || meaning == "endpoint_b_port") {
			return true
		}
	}
	return false
}

func guardedField(field string) bool {
	switch field {
	case "vlan", "vrf", "vni", "asn", "mtu", "u_height", "rack_height", "wavelength", "uplink_ports", "downlink_ports", "endpoint_a_uid", "endpoint_b_uid":
		return true
	}
	return false
}

func normalizeExplicitBindings(s bom.Sheet, p *SheetPlan) {
	heads := sourceHeaders(s, *p)
	columns := map[string][]int{}
	headerRows := map[int]bool{}
	for col, c := range heads {
		if f := bom.Field(c.Value); f != "" {
			columns[f] = append(columns[f], col)
			headerRows[c.Row] = true
		}
	}
	for f := range columns {
		sort.Ints(columns[f])
	}
	for row := range headerRows {
		found := false
		for _, h := range p.HeaderRows {
			if row == h {
				found = true
			}
		}
		if !found {
			p.HeaderRows = append(p.HeaderRows, row)
		}
	}
	sort.Ints(p.HeaderRows)
	for j := range p.Tables {
		t := &p.Tables[j]
		if t.Kind == "reference" {
			continue
		}
		for f, col := range t.Fields {
			meaning := bom.Field(heads[col].Value)
			if f == "wavelength" && strings.EqualFold(strings.ReplaceAll(heads[col].Value, " ", ""), "fibercorecount") {
				continue
			}
			// Known conflicting semantics or unsupported sensitive engineering meanings
			// stay in immutable raw evidence, not in invented normalized attributes.
			if meaning != "" && !compatibleBinding(f, meaning, *t) || meaning == "" && guardedField(f) {
				delete(t.Fields, f)
			}
		}
		fields := []string{}
		if t.Kind == "planned_device" && t.Type != "rack" {
			fields = []string{"hostname", "model", "serial", "rack", "site", "u_start", "u_height", "management_ip", "mac", "network_role"}
		}
		if t.Kind == "physical_connection" {
			fields = []string{"endpoint_a_device", "endpoint_b_device", "endpoint_a_port", "endpoint_b_port", "model", "endpoint_a_optic", "endpoint_b_optic", "length", "media_type", "speed", "termination_a", "termination_b"}
		}
		for _, f := range fields {
			if t.Fields[f] == 0 && len(columns[f]) == 1 {
				t.Fields[f] = columns[f][0]
			}
		}
		// Repeated module headers belong to their explicit device/port group.
		// Use the already-bound side device, never assume A is the left side.
		if t.Kind == "physical_connection" && dualEndpointHeaders(columns) && len(columns["model"]) == 2 {
			devices, ports, models := columns["hostname"], columns["port_index"], columns["model"]
			if devices[0] < ports[0] && ports[0] < models[0] && models[0] < devices[1] && devices[1] < ports[1] && ports[1] < models[1] {
				for _, side := range []string{"a", "b"} {
					field := "endpoint_" + side + "_optic"
					for k, device := range devices {
						if t.Fields[field] == 0 && t.Fields["endpoint_"+side+"_device"] == device && t.Fields["endpoint_"+side+"_port"] == ports[k] {
							t.Fields[field] = models[k]
						}
					}
				}
			}
		}
		normalizeRowKeys(t)
	}
}

func inspectBindingSemantics(s bom.Sheet, p SheetPlan) error {
	heads := sourceHeaders(s, p)
	for _, t := range p.Tables {
		if t.Kind == "reference" {
			continue
		}
		if t.Type == "cable" && t.Kind != "physical_connection" && t.Kind != "material_summary" {
			return fmt.Errorf("ENTITY_KIND_CONFLICT: row=%d column=%d; cable must be physical_connection or material_summary, never a component", t.Start, firstBoundColumn(t.Fields))
		}
		if t.Kind == "physical_connection" && t.Type != "cable" {
			return fmt.Errorf("ENTITY_KIND_CONFLICT: row=%d column=%d; physical_connection requires cable", t.Start, firstBoundColumn(t.Fields))
		}
		if t.Kind == "physical_connection" {
			for _, side := range []string{"a", "b"} {
				device, port, uid := "endpoint_"+side+"_device", "endpoint_"+side+"_port", "endpoint_"+side+"_uid"
				dc, pc, uc := t.Fields[device], t.Fields[port], t.Fields[uid]
				portMeaning := bom.Field(heads[pc].Value)
				// A point/quantity column cannot become a physical port just because the
				// model binds it twice. Generic explicit port headers support multi-level tables.
				label := strings.ToLower(heads[pc].Value)
				explicitPort := portMeaning == port || portMeaning == "port_index" || strings.Contains(label, "port") || strings.Contains(label, "端口")
				if (dc == 0 || pc == 0 || dc == pc || !explicitPort) && (uc == 0 || bom.Field(heads[uc].Value) != uid) {
					return fmt.Errorf("PHYSICAL_ENDPOINT_BINDING_REQUIRED: row=%d column=%d; each side requires distinct device and explicit physical-port columns; point/quantity schedules remain reference/material_summary", t.Start, firstBoundColumn(t.Fields))
				}
			}
		}
		for f, col := range t.Fields {
			meaning := bom.Field(heads[col].Value)
			if meaning != "" && !compatibleBinding(f, meaning, t) || meaning == "" && guardedField(f) {
				return fmt.Errorf("SOURCE_BINDING_CONFLICT: row=%d column=%d field=%s; binding conflicts with explicit source meaning; preserve unsupported columns as unmapped evidence", t.Start, col, f)
			}
		}
	}
	return nil
}

// Unambiguous inventory headers do not need probabilistic column selection.
// Category remains a candidate for the same semantic classifier and review gates.
func explicitInventoryPlan(s bom.Sheet) (SheetPlan, bool) {
	p := SheetPlan{Purpose: "table", HeaderRows: []int{}, Tables: []TablePlan{}, Note: "明确设备清单表头由本地来源规则绑定；型号语义分类与人工审核仍独立执行"}
	heads := sourceHeaders(s, p)
	cols := map[string][]int{}
	row, end := 0, 0
	for col, c := range heads {
		if f := bom.Field(c.Value); f != "" {
			cols[f] = append(cols[f], col)
			if c.Row > row {
				row = c.Row
			}
		}
	}
	// Repeated device columns are a connection/matrix, not an inventory.
	if len(cols["hostname"]) != 1 || len(cols["model"]) != 1 || len(cols["endpoint_a_device"]) > 0 || len(cols["endpoint_b_device"]) > 0 {
		return p, false
	}
	for _, c := range s.Cells {
		if c.Row > end {
			end = c.Row
		}
	}
	if end <= row {
		return p, false
	}
	fields := map[string]int{}
	for _, f := range []string{"hostname", "model", "serial", "site", "rack", "u_start", "u_height", "rack_height", "management_ip", "mac", "brand", "manufacturer", "network_role", "role", "device_alias"} {
		if len(cols[f]) == 1 {
			fields[f] = cols[f][0]
		}
	}
	p.HeaderRows = []int{row}
	t := TablePlan{Key: "source-inventory", Kind: "planned_device", Type: "other", Start: row + 1, End: end, Fields: fields, Required: []string{"hostname"}, Types: map[string]string{}, Confidence: 1}
	p.Tables = append(p.Tables, t)
	if err := CheckExtraction(s, p); err != nil {
		return p, false
	}
	return p, true
}
