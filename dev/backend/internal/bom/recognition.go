package bom

import (
	"encoding/json"
	"fmt"
	"reflect"
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
	// Resolvers update field maps. Keep reviewed records isolated and byte-stable.
	protected := map[string]ImportRecord{}
	for j, r := range b.Records {
		if r.Imported || r.Ignored || r.Confirmed || r.Method == "manual" {
			protected[r.ID] = r
			raw, _ := json.Marshal(r)
			var copy ImportRecord
			json.Unmarshal(raw, &copy)
			b.Records[j] = copy
		}
	}
	defer func() {
		for j, r := range b.Records {
			if old, ok := protected[r.ID]; ok {
				b.Records[j] = old
			}
		}
	}()
	// Recover identical persisted duplicates before using candidate IDs as map
	// keys. Never resolve a conflicting duplicate by last-writer-wins.
	unique := []ImportRecord{}
	positions := map[string]int{}
	for _, r := range b.Records {
		if n, exists := positions[r.ID]; exists {
			old := &unique[n]
			if reflect.DeepEqual(old.Item, r.Item) && !r.Imported && !r.Confirmed && !r.Ignored && r.Method != "manual" {
				mergeSourceEvidence(old, r)
				continue
			}
			old.Ambiguity, r.Ambiguity = true, true
		} else {
			positions[r.ID] = len(unique)
		}
		unique = append(unique, r)
	}
	b.Records = unique
	// Compare both source labels and resolved references in the same identity space.
	aliases := map[string]map[string]bool{}
	ids := map[string]bool{}
	rebuildAliases := func() {
		aliases = map[string]map[string]bool{}
		ids = map[string]bool{}
		for _, r := range b.Records {
			ids[r.ID] = true
			for _, name := range []string{r.Item.Hostname, r.Item.Serial, r.Item.Fields["device_alias"]} {
				if name == "" {
					continue
				}
				if aliases[name] == nil {
					aliases[name] = map[string]bool{}
				}
				aliases[name][r.ID] = true
			}
		}
	}
	rebuildAliases()

	ownerID := func(name string) string {
		if ids[name] {
			return name
		}
		if len(aliases[name]) == 1 {
			for id := range aliases[name] {
				return id
			}
		}
		return name
	}

	redirects := map[string]string{}
	// Resolve inventory identities first. Component keys must see the final
	// owner identity rather than an inventory row removed in this same pass.
	for _, phase := range []string{"devices", "components"} {
		merged := []ImportRecord{}
		indices := map[string]int{}
		for _, r := range b.Records {
			key := ""
			if phase == "devices" && !r.Imported && !r.Ignored && !r.Confirmed && r.Method != "manual" && r.Item.Hostname != "" && (r.Item.Category == "server" || r.Item.Category == "switch" || r.Item.Category == "router") {
				key = r.Item.Category + ":" + r.Item.Hostname
			}
			if phase == "components" && !r.Imported && !r.Ignored && !r.Confirmed && r.Method != "manual" && (r.Item.Category == "optic_module" || r.Item.Category == "electrical_module") && r.Item.Fields["record_kind"] != "material_summary" && r.Item.Parent != "" && r.Item.Port != "" {
				key = r.Item.Category + ":" + ownerID(r.Item.Parent) + ":" + r.Item.Port
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
						if old.Item.Fields == nil {
							old.Item.Fields = map[string]string{}
						}
						if old.Item.Sources == nil {
							old.Item.Sources = map[string]Cell{}
						}
						for f, v := range r.Item.Fields {
							if old.Item.Fields[f] == "" {
								old.Item.Fields[f] = v
								old.Item.Sources[f] = r.Item.Sources[f]
							}
						}
						if old.Item.Model == "" {
							old.Item.Model = r.Item.Model
						}
						if old.Item.Serial == "" {
							old.Item.Serial = r.Item.Serial
						}
						mergeSourceEvidence(old, r)
						redirects[r.ID] = old.ID
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
		for j := range b.Records {
			r := &b.Records[j]
			if r.Imported || r.Confirmed || r.Ignored || r.Method == "manual" {
				continue
			}
			for _, ref := range []struct {
				value *string
				field string
			}{{&r.Item.Parent, "parent_uid"}, {&r.Item.A, "endpoint_a_uid"}, {&r.Item.B, "endpoint_b_uid"}} {
				if to := redirects[*ref.value]; to != "" && to != *ref.value {
					*ref.value = to
					r.Item.Fields[ref.field] = to
				}
			}
			if to := redirects[r.Item.Fields["port_uid"]]; to != "" {
				r.Item.Fields["port_uid"] = to
			}
		}
		rebuildAliases()
	}
	// Explicit module models at cable ends describe modules on those owner ports, not extra unknown rows.
	moduleKeys := map[string]bool{}
	modulesByPort := map[string][]int{}
	for j, r := range b.Records {
		if r.Item.Category == "optic_module" || r.Item.Category == "electrical_module" {
			moduleKeys[ownerID(r.Item.Parent)+":"+r.Item.Port+":"+r.Item.Model] = true
			modulesByPort[ownerID(r.Item.Parent)+":"+r.Item.Port] = append(modulesByPort[ownerID(r.Item.Parent)+":"+r.Item.Port], j)
		}
	}
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
			key := ownerID(owner) + ":" + port + ":" + model
			if moduleKeys[key] {
				continue
			}
			moduleKeys[key] = true
			if existing := modulesByPort[ownerID(owner)+":"+port]; len(existing) == 1 {
				old := &b.Records[existing[0]]
				if old.Item.Model == "" && !old.Imported && !old.Confirmed && !old.Ignored && old.Method != "manual" {
					// The AI may extract an explicit port module without its
					// model. Enrich that same source-backed object immediately;
					// don't create another module for the next pass to merge.
					old.Item.Model = model
					if old.Item.Fields == nil {
						old.Item.Fields = map[string]string{}
					}
					if old.Item.Sources == nil {
						old.Item.Sources = map[string]Cell{}
					}
					old.Item.Fields["model"], old.Item.Sources["model"] = model, r.Item.Sources[f]
					mergeSourceEvidence(old, r)
					continue
				}
			}
			// An endpoint model is a planning requirement, not a serialized installed
			// module. Keep it on the source cable. Only explicit inventory candidates
			// may create the physical module; existing ones can be enriched above.

		}
	}
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
	idCounts := map[string]int{}
	ports := map[string][]string{}
	for _, i := range items {
		byID[i.UID] = i
		idCounts[i.UID]++
		if i.Category == "port" {
			ports[i.Parent+":"+i.Port] = append(ports[i.Parent+":"+i.Port], i.UID)
		}
	}
	for j := range b.Records {
		r := &b.Records[j]
		if i, ok := byID[r.ID]; ok && idCounts[r.ID] == 1 {
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

func mergeSourceEvidence(dst *ImportRecord, src ImportRecord) {
	seen := map[string]bool{}
	for _, c := range dst.Raw {
		seen[fmt.Sprint(c.Sheet, ":", c.Row, ":", c.Column)] = true
	}
	for _, c := range src.Raw {
		key := fmt.Sprint(c.Sheet, ":", c.Row, ":", c.Column)
		if !seen[key] {
			dst.Raw = append(dst.Raw, c)
			seen[key] = true
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
	v := RackUnitValue(field, strings.TrimSpace(c.Value))
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
