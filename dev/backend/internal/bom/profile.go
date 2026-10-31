package bom

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Column struct {
	Index     int    `json:"index"`
	Header    string `json:"header"`
	Field     string `json:"field"`
	Ignored   bool   `json:"ignored"`
	Reason    string `json:"reason,omitempty"`
	Transform string `json:"transform,omitempty"`
	Separator string `json:"separator,omitempty"`
	FillDown  bool   `json:"fill_down"`
}
type Profile struct {
	ExcludedRows map[int]string `json:"excluded_rows,omitempty"`
	Sheet        string         `json:"sheet_name"`
	Purpose      string         `json:"purpose"`
	Entity       string         `json:"candidate_entity"`
	HeaderRows   []int          `json:"header_regions"`
	Start        int            `json:"data_start"`
	End          int            `json:"data_end"`
	Columns      []Column       `json:"column_definitions"`
	Confirmed    bool           `json:"confirmed"`
	Reason       string         `json:"reason"`
	Parser       string         `json:"parser_version"`
}

var fieldReplacer = strings.NewReplacer(" ", "", "_", "", "-", "", "\n", "")

func canonical(s string) string { return strings.ToLower(fieldReplacer.Replace(s)) }

var canonicalFields = func() map[string]string {
	out := map[string]string{}
	for _, f := range []string{"device_alias", "manufacturer", "series", "sku", "device_number", "port_count", "port_type", "uplink_ports", "downlink_ports", "psu_count", "psu_type", "fan", "redundancy", "subnet", "network_domain", "roce", "infiniband", "wavelength", "distance", "fiber_mode", "cable_number", "patch_panel", "patch_port", "termination_a", "termination_b", "sequence", "direction", "purpose", "gpu_model", "nic_model", "disk_spec", "bmc_model", "power_spec", "record_kind", "quantity_required", "quantity_ordered", "quantity_spares", "quantity_total", "endpoint_a_optic", "endpoint_b_optic", "length", "rack_uid", "endpoint_a_device", "endpoint_b_device", "endpoint_a_port", "endpoint_b_port", "name", "description", "brand", "specification", "quantity", "unit", "category", "serial", "hostname", "model", "management_ip", "mac", "parent_uid", "port_index", "endpoint_a_uid", "endpoint_b_uid", "rack", "u_start", "u_height", "rack_height", "site", "pod", "speed", "mtu", "vlan", "vrf", "vni", "asn", "media_type", "baseline_ref", "model_version_ref", "endpoint_a_speed", "endpoint_b_speed", "port_uid", "optic_type", "breakout", "cpu_model", "cpu_count", "memory_value", "memory_unit", "gpu_count", "nic_count", "os_interface", "os_version", "kernel", "driver", "cuda", "nccl", "network_role", "rail", "role"} {
		out[canonical(f)] = f
	}
	return out
}()

// ImportFields is the authoritative vocabulary shared by AI planning and local validation.
func ImportFields() []string {
	out := []string{}
	for _, f := range canonicalFields {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func Field(s string) string {
	key := canonical(s)
	if f := canonicalFields[key]; f != "" {
		return f
	}
	if f := aliases[key]; f != "" {
		return f
	}
	extra := map[string]string{"线缆型号": "model", "cablemodel": "model", "本端模块型号": "endpoint_a_optic", "对端模块型号": "endpoint_b_optic", "线缆长度": "length", "物料名称": "name", "名称": "name", "描述": "description", "规格": "specification", "数量": "quantity", "单位": "unit", "品牌": "brand", "manufacturer": "brand", "qty": "quantity", "数量(台)": "quantity", "productname": "name", "物料描述": "description", "asidelablename": "endpoint_a_device", "zsidelablename": "endpoint_b_device", "本端设备名称": "endpoint_a_device", "对端设备名称": "endpoint_b_device", "本端端口名称": "endpoint_a_port", "对端端口名称": "endpoint_b_port", "a端口": "endpoint_a_port", "z端口": "endpoint_b_port", "cabletype": "media_type", "rackunit": "u_start", "uposition": "u_start", "uposiotion": "u_start", "racknumber": "rack", "rackno": "rack", "dcname": "site", "speed": "speed", "带宽": "speed", "u位": "u_start", "u数": "u_height", "parent": "parent_uid", "itemuid": "item_uid", "endpointauid": "endpoint_a_uid", "endpointbuid": "endpoint_b_uid", "modelversionref": "model_version_ref"}
	return extra[key]
}
func Profiles(w Workbook) []Profile {
	out := []Profile{}
	for _, s := range w.Sheets {
		scores := map[int]int{}
		byrow := map[int][]Cell{}
		max := 0
		for _, c := range s.Cells {
			if c.Row > max {
				max = c.Row
			}
			byrow[c.Row] = append(byrow[c.Row], c)
			if c.Row <= 50 && !c.Sensitive && Field(c.Value) != "" {
				scores[c.Row]++
			}
		}
		best, score := 0, 0
		for r, n := range scores {
			if n > score || (n == score && r < best) {
				best, score = r, n
			}
		}
		p := Profile{Sheet: s.Name, Purpose: "unclassified", HeaderRows: []int{}, Start: best + 1, End: max, Columns: []Column{}, Parser: "sheet-profile/v1"}
		if score >= 2 {
			p.HeaderRows = []int{best}
			p.Purpose = "assets"
			found := map[string]bool{}
			for _, c := range byrow[best] {
				f := Field(c.Value)
				if c.Sensitive {
					f = ""
				}
				found[f] = true
				p.Columns = append(p.Columns, Column{Index: c.Column, Header: c.Value, Field: f, Ignored: c.Sensitive})
			}
			if found["endpoint_a_device"] && found["endpoint_b_device"] {
				p.Purpose = "cabling"
				p.Entity = "cable"
			}
			if found["hostname"] || found["serial"] {
				p.Entity = "device"
			}
		}
		out = append(out, p)
	}
	return out
}
func Build(w Workbook, profiles []Profile) ([]Item, []Issue) {
	items := []Item{}
	issues := []Issue{}
	add := func(id, field, reason string) {
		issues = append(issues, Issue{fmt.Sprintf("profile-%d", len(issues)), "BLOCKER", "PROFILE-CONFIRM", "1.0", id, field, reason})
	}
	for _, p := range profiles {
		var s *Sheet
		for j := range w.Sheets {
			if w.Sheets[j].Name == p.Sheet {
				s = &w.Sheets[j]
			}
		}
		if s == nil {
			add("", p.Sheet, "Unknown Sheet reference")
			continue
		}
		if !p.Confirmed {
			add("", p.Sheet, "Sheet purpose and region require human confirmation")
			continue
		}
		if p.Purpose == "reference" || p.Purpose == "summary" || p.Purpose == "layout" {
			if strings.TrimSpace(p.Reason) == "" {
				add("", p.Sheet, "Non-material region requires explicit disposition reason")
			}
			continue
		}
		if p.Purpose != "assets" && p.Purpose != "cabling" && p.Purpose != "baseline" {
			add("", p.Sheet, "Unclassified data cannot be dropped")
			continue
		}
		if p.Start < 1 || p.End < p.Start {
			add("", p.Sheet, "Invalid data range")
			continue
		}
		cols := map[int]Column{}
		for _, c := range p.Columns {
			if c.Ignored {
				cols[c.Index] = c
				if c.Reason == "" {
					add("", p.Sheet, "Ignoring a field requires a reason")
				}
				continue
			}
			if c.Field == "" {
				add("", p.Sheet+":"+c.Header, "Unknown column requires mapping or reasoned exclusion")
			}
			cols[c.Index] = c
		}
		rows := map[int][]Cell{}
		for _, c := range s.Cells {
			if c.Row >= p.Start && c.Row <= p.End {
				rows[c.Row] = append(rows[c.Row], c)
			}
		}
		order := []int{}
		for r := range rows {
			order = append(order, r)
		}
		sort.Ints(order)
		last := map[int]Cell{}
		previousRow := 0
		for _, r := range order {
			if why, excluded := p.ExcludedRows[r]; excluded {
				if strings.TrimSpace(why) == "" {
					add("", p.Sheet+":"+strconv.Itoa(r), "Excluded source row requires an explicit reason")
				}
				continue
			}
			if previousRow > 0 && r > previousRow+1 {
				last = map[int]Cell{}
			}
			previousRow = r
			repeats := 0
			for _, c := range rows[r] {
				if col, ok := cols[c.Column]; ok && canonical(c.Value) == canonical(col.Header) {
					repeats++
				}
			}
			if repeats >= 2 {
				continue
			}
			present := map[int]bool{}
			for _, c := range rows[r] {
				present[c.Column] = true
			}
			for ci, col := range cols {
				if col.FillDown && !present[ci] {
					if prior, ok := last[ci]; ok {
						rows[r] = append(rows[r], prior)
					}
				}
			}
			i := Item{UID: StableID("candidate:" + s.Name + ":" + strconv.Itoa(r)), Category: p.Entity, Fields: map[string]string{}, Sources: map[string]Cell{}, Sheet: s.Name}
			if p.Purpose == "baseline" {
				i.Category = "network_baseline"
			}
			nonempty := false
			for _, c := range rows[r] {
				col, ok := cols[c.Column]
				if !ok && !c.Sensitive {
					add(i.UID, p.Sheet+":"+c.Ref, "Nonempty column not covered by mapping or explicit exclusion")
				}
				if !ok || c.Sensitive || col.Ignored || col.Field == "" {
					continue
				}
				if canonical(c.Value) == canonical(col.Header) {
					continue
				}
				value := c.Value
				if col.Transform == "trim" {
					value = strings.TrimSpace(value)
				}
				if old := i.Fields[col.Field]; old != "" && old != value {
					if col.Transform != "join" {
						add(i.UID, col.Field, "Multiple columns require explicit join transform")
					} else {
						value = old + col.Separator + value
					}
				}
				if col.Transform != "" && col.Transform != "identity" && col.Transform != "trim" && col.Transform != "join" {
					add(i.UID, col.Field, "Unsupported deterministic transform")
				}
				i.Fields[col.Field] = value
				i.Sources[col.Field+"@"+c.Ref] = c
				last[c.Column] = c
				nonempty = true
			}
			if !nonempty {
				continue
			}
			hydrate(&i)
			items = append(items, i)
		}
	}
	return items, issues
}
func hydrate(i *Item) {
	for k, v := range i.Fields {
		switch k {
		case "category":
			i.Category = strings.ToLower(v)
		case "serial":
			i.Serial = v
		case "hostname":
			i.Hostname = v
		case "model":
			i.Model = v
		case "parent_uid":
			i.Parent = v
		case "port_index":
			i.Port = v
		case "endpoint_a_uid":
			i.A = v
		case "endpoint_b_uid":
			i.B = v
		}
	}
}

// Explicit device aliases produce candidates only. No fuzzy name or port-number matching.
type RelationCandidate struct {
	ID         string   `json:"id"`
	Item       string   `json:"item_uid"`
	Field      string   `json:"field"`
	Candidates []string `json:"candidates"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason"`
}

func Relations(items []Item) []RelationCandidate {
	out := []RelationCandidate{}
	names := map[string][]string{}
	ports := map[string][]string{}
	for _, i := range items {
		if i.Hostname != "" {
			names[i.Hostname] = append(names[i.Hostname], i.UID)
		}
		if alias := i.Fields["device_alias"]; alias != "" && alias != i.Hostname {
			names[alias] = append(names[alias], i.UID)
		}
		if i.Category == "port" {
			ports[i.Parent+":"+i.Port] = append(ports[i.Parent+":"+i.Port], i.UID)
		}
	}
	for _, i := range items {
		if i.Category != "cable" {
			continue
		}
		for _, side := range []string{"a", "b"} {
			ds := names[i.Fields["endpoint_"+side+"_device"]]
			c := []string{}
			if len(ds) == 1 {
				for _, d := range ds {
					c = append(c, ports[d+":"+i.Fields["endpoint_"+side+"_port"]]...)
				}
			}
			out = append(out, RelationCandidate{i.UID + ":" + side, i.UID, "endpoint_" + side + "_uid", c, "needs_review", "Exact explicit device/port reference; confirm before writing physical relation"})
		}
	}
	return out
}

// Keep candidate IDs while editing; resolve the complete reference graph once at commit.
func Normalize(items []Item) ([]Item, error) {
	out := make([]Item, len(items))
	copy(out, items)
	ids := map[string]string{}
	for _, i := range items {
		if i.Category != "port" && i.Category != "cable" {
			ids[i.UID] = StableID(Identity(i))
		}
	}
	for _, i := range items {
		if i.Category == "port" {
			parent := ids[i.Parent]
			if parent == "" {
				return nil, fmt.Errorf("UNRESOLVED_PARENT")
			}
			i.Parent = parent
			ids[i.UID] = StableID(Identity(i))
		}
	}
	for _, i := range items {
		if i.Category == "cable" {
			a, b := ids[i.A], ids[i.B]
			if a == "" || b == "" {
				return nil, fmt.Errorf("UNRESOLVED_ENDPOINT")
			}
			i.A = a
			i.B = b
			ids[i.UID] = StableID(Identity(i))
		}
	}
	for j := range out {
		i := &out[j]
		i.Status = "imported"
		if i.Category == "rack" || i.Category == "network_baseline" || i.Category == "material_plan" || (KnownImportType(i.Category) && i.Category != "server" && i.Category != "switch" && i.Category != "gpu" && i.Category != "nic" && i.Category != "bmc" && i.Category != "psu" && i.Category != "optic_module" && i.Category != "port" && i.Category != "cable") {
			i.Subtype = i.Category
			i.Category = "other"
			if i.Subtype == "network_baseline" {
				i.Fields["planning_status"] = "draft"
			}
		}
		i.UID = ids[i.UID]
		if i.Parent != "" {
			i.Parent = ids[i.Parent]
		}
		if i.A != "" {
			i.A = ids[i.A]
		}
		if i.B != "" {
			i.B = ids[i.B]
		}
		if i.Fields == nil {
			i.Fields = map[string]string{}
		}
		for _, field := range []string{"rack_uid", "port_uid"} {
			if ref := i.Fields[field]; ref != "" {
				if ids[ref] == "" {
					return nil, fmt.Errorf("UNRESOLVED_REFERENCE")
				}
				i.Fields[field] = ids[ref]
			}
		}
		if i.Parent != "" {
			i.Fields["parent_uid"] = i.Parent
		}
		if i.A != "" {
			i.Fields["endpoint_a_uid"] = i.A
		}
		if i.B != "" {
			i.Fields["endpoint_b_uid"] = i.B
		}
	}
	return out, nil
}
func Reconcile(items []Item) ([]Item, []Issue) {
	out := []Item{}
	issues := []Issue{}
	index := map[string]int{}
	replaced := map[string]string{}
	for _, i := range items {
		key := ""
		if i.Serial != "" && i.Category != "other" && i.Category != "device" {
			key = Identity(i)
		}
		j, exists := index[key]
		if key == "" || !exists {
			if key != "" {
				index[key] = len(out)
			}
			out = append(out, i)
			continue
		}
		target := &out[j]
		conflict := false
		for k, v := range i.Fields {
			if old := target.Fields[k]; old != "" && old != v {
				issues = append(issues, Issue{fmt.Sprintf("merge-%d", len(issues)), "BLOCKER", "CROSS-SHEET-CONFLICT", "1.0", target.UID, k, "Same stable identity has conflicting values; manual reconciliation required"})
				conflict = true
			}
		}
		if conflict {
			out = append(out, i)
			continue
		}
		for k, v := range i.Fields {
			target.Fields[k] = v
		}
		for k, c := range i.Sources {
			target.Sources[k+"@"+c.Sheet+":"+c.Ref] = c
		}
		replaced[i.UID] = target.UID
	}
	for j := range out {
		if r := replaced[out[j].Parent]; r != "" {
			out[j].Parent = r
		}
		if r := replaced[out[j].A]; r != "" {
			out[j].A = r
		}
		if r := replaced[out[j].B]; r != "" {
			out[j].B = r
		}
	}
	return out, issues
}

// Expand only explicitly recorded endpoint ports into review candidates. The cable stays
// unbound until the reviewer selects a concrete physical port candidate.
func ExpandPorts(items []Item) []Item {
	out := append([]Item{}, items...)
	names := map[string][]Item{}
	ports := map[string]bool{}
	for _, i := range items {
		if i.Category == "server" || i.Category == "switch" {
			names[i.Hostname] = append(names[i.Hostname], i)
			if alias := i.Fields["device_alias"]; alias != "" && alias != i.Hostname {
				names[alias] = append(names[alias], i)
			}
		}
		if i.Category == "port" {
			ports[i.Parent+":"+i.Port] = true
		}
	}
	for _, cable := range items {
		if cable.Category != "cable" {
			continue
		}
		for _, side := range []string{"a", "b"} {
			device := cable.Fields["endpoint_"+side+"_device"]
			name := cable.Fields["endpoint_"+side+"_port"]
			owners := names[device]
			if name == "" || device == "" || len(owners) != 1 {
				continue
			}
			parent := owners[0]
			key := parent.UID + ":" + name
			if ports[key] {
				continue
			}
			sources := map[string]Cell{}
			for k, c := range cable.Sources {
				if strings.HasPrefix(k, "endpoint_"+side+"_") {
					sources[k] = c
				}
			}
			if len(sources) == 0 {
				continue
			}
			speed := cable.Fields["endpoint_"+side+"_speed"]
			out = append(out, Item{UID: StableID("port-candidate:" + key), Category: "port", Parent: parent.UID, Port: name, Fields: map[string]string{"speed": speed, "role": "pending_review"}, Sources: sources, Sheet: cable.Sheet})
			ports[key] = true
		}
	}
	return out
}
func EnsureRacks(items []Item) []Item {
	out := append([]Item{}, items...)
	racks := map[string]string{}
	for _, i := range items {
		if i.Category == "rack" || (i.Category == "other" && i.Subtype == "rack") {
			racks[i.Fields["site"]+":"+i.Fields["rack"]] = i.UID
		}
	}
	for j := range items {
		i := &out[j]
		site, rack := i.Fields["site"], i.Fields["rack"]
		if site == "" || rack == "" || i.Category == "rack" {
			continue
		}
		key := site + ":" + rack
		id := racks[key]
		if id == "" {
			id = StableID("rack-candidate:" + key)
			sources := map[string]Cell{}
			for k, c := range i.Sources {
				if strings.HasPrefix(k, "manual_position") || strings.HasPrefix(k, "rack") || strings.HasPrefix(k, "site") {
					sources[k] = c
				}
			}
			out = append(out, Item{UID: id, Category: "rack", Fields: map[string]string{"site": site, "rack": rack, "rack_height": i.Fields["rack_height"]}, Sources: sources, Sheet: i.Sheet})
			racks[key] = id
		}
		i.Fields["rack_uid"] = id
	}
	return out
}
func Generate(w Workbook, profiles []Profile) ([]Item, []Issue) {
	items, issues := Build(w, profiles)
	items = EnsureRacks(ExpandPorts(items))
	items, merged := Reconcile(items)
	return items, append(issues, merged...)
}
