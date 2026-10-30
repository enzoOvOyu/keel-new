package bom

import (
	"sort"
	"strconv"
	"strings"
)

// AI intake preserves every nonempty source row without guessing fields or types.
func PlanningSourceRows(w Workbook) Workbench {
	b := Workbench{Records: []ImportRecord{}, State: "pending_review"}
	for _, s := range w.Sheets {
		rows := map[int][]Cell{}
		order := []int{}
		for _, c := range s.Cells {
			if c.Sensitive || (strings.TrimSpace(c.Value) == "" && c.Formula == "") {
				continue
			}
			if _, ok := rows[c.Row]; !ok {
				order = append(order, c.Row)
			}
			rows[c.Row] = append(rows[c.Row], c)
		}
		sort.Ints(order)
		for _, row := range order {
			cells := rows[row]
			parts := []string{}
			for _, c := range cells {
				if c.Value == "" && c.Formula != "" {
					parts = append(parts, "[公式无缓存结果，待核查]")
				} else {
					parts = append(parts, c.Value)
				}
			}
			id := StableID("candidate:" + s.Name + ":" + strconv.Itoa(row))
			b.Records = append(b.Records, ImportRecord{ID: id, Sheet: s.Name, Row: row, Raw: cells, Original: strings.Join(parts, " · "), Item: Item{UID: id, Sheet: s.Name, Category: "other", Fields: map[string]string{}, Sources: map[string]Cell{}}, Recognition: "unrecognized", Method: "pending", Issues: []Issue{}})
		}
	}
	return b
}

// Link only exact source identities. The result remains a candidate graph until confirmation.
func LinkRecognizedRecords(b *Workbench) {
	merged := []ImportRecord{}
	indices := map[string]int{}
	for _, r := range b.Records {
		key := ""
		if !r.Imported && !r.Ignored && r.Item.Hostname != "" && (r.Item.Category == "server" || r.Item.Category == "switch" || r.Item.Category == "router") {
			key = r.Item.Category + ":" + r.Item.Hostname
		}
		if !r.Imported && !r.Ignored && (r.Item.Category == "optic_module" || r.Item.Category == "electrical_module") && r.Item.Fields["record_kind"] != "material_summary" && r.Item.Parent != "" && r.Item.Port != "" {
			key = r.Item.Category + ":" + r.Item.Parent + ":" + r.Item.Port
		}
		if key != "" {
			if n, ok := indices[key]; ok {
				old := &merged[n]
				conflict := false
				for _, field := range []string{"serial", "model", "rack", "site"} {
					a, c := old.Item.Fields[field], r.Item.Fields[field]
					if a != "" && c != "" && a != c {
						conflict = true
					}
				}
				if !conflict {
					for f, v := range r.Item.Fields {
						if old.Item.Fields[f] == "" {
							old.Item.Fields[f] = v
							old.Item.Sources[f] = r.Item.Sources[f]
						}
					}
					old.Raw = append(old.Raw, r.Raw...)
					continue
				}
				old.Ambiguity = true
				r.Ambiguity = true
			} else {
				indices[key] = len(merged)
			}
		}
		merged = append(merged, r)
	}
	b.Records = merged
	// Explicit module models at cable ends describe modules on those owner ports, not extra unknown rows.
	moduleKeys := map[string]bool{}
	for _, r := range b.Records {
		if r.Item.Category == "optic_module" || r.Item.Category == "electrical_module" {
			moduleKeys[r.Item.Fields["parent_uid"]+":"+r.Item.Fields["port_index"]+":"+r.Item.Model] = true
		}
	}
	add := []ImportRecord{}
	for _, r := range b.Records {
		if r.Item.Category != "cable" {
			continue
		}
		for _, side := range []string{"a", "b"} {
			f := "endpoint_" + side + "_optic"
			model := r.Item.Fields[f]
			owner := r.Item.Fields["endpoint_"+side+"_device"]
			port := r.Item.Fields["endpoint_"+side+"_port"]
			if model == "" || model == "/" || model == "-" || owner == "" || port == "" {
				continue
			}
			key := owner + ":" + port + ":" + model
			if moduleKeys[key] {
				continue
			}
			moduleKeys[key] = true
			id := StableID("module-candidate:" + key)
			fields := map[string]string{"category": "optic_module", "record_kind": "component", "name": model, "model": model, "parent_uid": owner, "port_index": port}
			sources := map[string]Cell{"name": r.Item.Sources[f], "model": r.Item.Sources[f], "parent_uid": r.Item.Sources["endpoint_"+side+"_device"], "port_index": r.Item.Sources["endpoint_"+side+"_port"]}
			add = append(add, ImportRecord{ID: id, Sheet: r.Sheet, Row: r.Row, Raw: r.Raw, Original: owner + " / " + port + " / " + model, Item: Item{UID: id, Sheet: r.Sheet, Category: "optic_module", Model: model, Parent: owner, Port: port, Fields: fields, Sources: sources}, Recognition: "recognized", Method: "provider", Confidence: r.Confidence, Reason: "来自布线明细的端口光模块型号；规划关系，尚未声明实物 SN", Issues: []Issue{}})
		}
	}
	b.Records = append(b.Records, add...)
	// Expose source-backed endpoint ports as candidates so the user can review the whole graph.
	items := []Item{}
	known := map[string]bool{}
	for _, r := range b.Records {
		if !r.Ignored {
			items = append(items, r.Item)
			known[r.ID] = true
		}
	}
	items = ResolveWorkbench(ExpandPorts(items))
	for _, i := range items {
		if known[i.UID] || i.Category != "port" {
			continue
		}
		i.Fields["name"] = i.Port
		i.Fields["parent_uid"] = i.Parent
		i.Fields["port_index"] = i.Port
		i.Fields["category"] = "port"
		i.Fields["record_kind"] = "physical_port"
		raw := []Cell{}
		row := 0
		for _, c := range i.Sources {
			raw = append(raw, c)
			row = c.Row
		}
		b.Records = append(b.Records, ImportRecord{ID: i.UID, Sheet: i.Sheet, Row: row, Raw: raw, Original: i.Port, Item: i, Method: "source_relation", Recognition: "recognized", Reason: "原始布线明确记录的设备端口；未推测 NIC 型号", Issues: []Issue{}})
	}
	// Resolve owner/endpoint references in the same candidate identity space.
	items = nil
	for _, r := range b.Records {
		if !r.Ignored {
			items = append(items, r.Item)
		}
	}
	items = ResolveWorkbench(items)
	byID := map[string]Item{}
	ports := map[string][]string{}
	for _, i := range items {
		byID[i.UID] = i
		if i.Category == "port" {
			ports[i.Parent+":"+i.Port] = append(ports[i.Parent+":"+i.Port], i.UID)
		}
	}
	for j := range b.Records {
		r := &b.Records[j]
		if i, ok := byID[r.ID]; ok {
			r.Item = i
		}
		if r.Item.Category == "optic_module" || r.Item.Category == "electrical_module" {
			p := ports[r.Item.Parent+":"+r.Item.Port]
			if len(p) == 1 {
				r.Item.Fields["port_uid"] = p[0]
			}
		}
	}
}

// Formula text belongs to the immutable workbook, not repeated in every candidate.
// Keep the cached value, provenance coordinates and an explicit formula flag.
func CompactWorkbench(b *Workbench) {
	compact := func(c Cell) Cell { c.FormulaBacked = c.FormulaBacked || c.Formula != ""; c.Formula = ""; return c }
	for j := range b.Records {
		r := &b.Records[j]
		r.HeaderContext = nil
		for k, c := range r.Raw {
			r.Raw[k] = compact(c)
		}
		for f, c := range r.Item.Sources {
			r.Item.Sources[f] = compact(c)
		}
		for k, c := range r.Unmapped {
			r.Unmapped[k] = compact(c)
		}
	}
}

// Read only a cached scalar; never execute or treat formula syntax/errors as identity.
func ExtractCellValue(c Cell, field string) string {
	v := strings.TrimSpace(c.Value)
	if (c.FormulaBacked || c.Formula != "") && v == "0" && (field == "model" || field == "serial" || field == "device_alias" || field == "endpoint_a_optic" || field == "endpoint_b_optic") {
		return ""
	}
	if v == "" || v == "-" || v == "/" || strings.HasPrefix(v, "=") {
		return ""
	}
	switch strings.ToUpper(v) {
	case "#REF!", "#VALUE!", "#N/A", "#DIV/0!", "#NAME?", "#NUM!", "#SPILL!":
		return ""
	}
	if (c.Formula != "" || c.FormulaBacked) && (field == "hostname" || field == "parent_uid" || field == "endpoint_a_device" || field == "endpoint_b_device" || field == "rack") {
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			return ""
		}
	}
	return v
}
