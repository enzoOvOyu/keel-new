package bom

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ImportRecord is a disposition of a source row, never a second asset identity.
type ImportRecord struct {
	Ambiguity     bool              `json:"ambiguity"`
	Unmapped      []Cell            `json:"unmapped_cells,omitempty"`
	Dispositions  map[string]string `json:"cell_dispositions,omitempty"`
	HeaderContext []Cell            `json:"header_context,omitempty"`
	ID            string            `json:"id"`
	Sheet         string            `json:"sheet_name"`
	Row           int               `json:"source_row"`
	Raw           []Cell            `json:"source_cells"`
	Original      string            `json:"original_description"`
	Item          Item              `json:"item"`
	Recognition   string            `json:"recognition_status"`
	Confidence    *float64          `json:"confidence"`
	Method        string            `json:"method"`
	Confirmed     bool              `json:"confirmed"`
	Ignored       bool              `json:"ignored"`
	Reason        string            `json:"reason,omitempty"`
	Imported      bool              `json:"imported"`
	AssetID       string            `json:"asset_id,omitempty"`
	Before        map[string]string `json:"before,omitempty"`
	Issues        []Issue           `json:"issues"`
}
type RecognitionSheet struct {
	Name       string          `json:"sheet_name"`
	State      string          `json:"state"`
	Purpose    string          `json:"purpose"`
	Note       string          `json:"note"`
	Error      string          `json:"error,omitempty"`
	Rows       int             `json:"rows"`
	Recognized int             `json:"recognized"`
	Plan       json.RawMessage `json:"plan,omitempty"`
}
type Workbench struct {
	Classifications map[string]json.RawMessage `json:"classifications,omitempty"`
	Steps           []AgentStep                `json:"steps,omitempty"`
	Coverage        *CoverageAudit             `json:"coverage,omitempty"`
	Phase           string                     `json:"phase,omitempty"`
	Analysis        json.RawMessage            `json:"analysis,omitempty"`
	Sheets          []RecognitionSheet         `json:"sheets,omitempty"`
	Records         []ImportRecord             `json:"records"`
	State           string                     `json:"state"`
	Error           string                     `json:"error,omitempty"`
	Provider        string                     `json:"provider,omitempty"`
}

var ImportTypes = []string{"server", "switch", "router", "gpu", "nic", "cpu", "memory", "disk", "bmc", "psu", "optic_module", "electrical_module", "port", "cable", "pdu", "power_cable", "rack", "patch_panel", "connector", "network_baseline", "material", "other"}

func KnownImportType(t string) bool {
	for _, v := range ImportTypes {
		if t == v {
			return true
		}
	}
	return false
}
func WorkbenchRows(w Workbook) Workbench {
	profiles := Profiles(w)
	out := Workbench{Records: []ImportRecord{}, State: "pending_review"}
	for si, s := range w.Sheets {
		p := profiles[si]
		cols := map[int]string{}
		headers := map[int]bool{}
		for _, h := range p.HeaderRows {
			headers[h] = true
		}
		for _, c := range p.Columns {
			if !c.Ignored {
				cols[c.Index] = c.Field
			}
		}
		rows := map[int][]Cell{}
		order := []int{}
		for _, c := range s.Cells {
			if c.Sensitive || strings.TrimSpace(c.Value) == "" {
				continue
			}
			if _, ok := rows[c.Row]; !ok {
				order = append(order, c.Row)
			}
			rows[c.Row] = append(rows[c.Row], c)
		}
		sort.Ints(order)
		contextCells := []Cell{}
		for _, c := range s.Cells {
			if headers[c.Row] && !c.Sensitive {
				contextCells = append(contextCells, c)
			}
		}
		for _, r := range order {
			if headers[r] {
				continue
			}
			cells := rows[r]
			repeated := 0
			for _, c := range cells {
				if Field(c.Value) != "" && Field(c.Value) == cols[c.Column] {
					repeated++
				}
			}
			if repeated >= 2 {
				continue
			}
			id := StableID("candidate:" + s.Name + ":" + strconv.Itoa(r))
			item := Item{UID: id, Sheet: s.Name, Status: "planned", Fields: map[string]string{}, Sources: map[string]Cell{}}
			parts := []string{}
			for _, c := range cells {
				parts = append(parts, c.Value)
				item.Sources["raw@"+c.Ref] = c
				if f := cols[c.Column]; f != "" && r >= p.Start {
					item.Fields[f] = strings.TrimSpace(c.Value)
					item.Sources[f] = c
				}
			}
			hydrate(&item)
			rec := ImportRecord{HeaderContext: contextCells, ID: id, Sheet: s.Name, Row: r, Raw: cells, Original: strings.Join(parts, " · "), Item: item, Recognition: "unrecognized", Method: "rules", Issues: []Issue{}}
			if len(p.HeaderRows) > 0 && r >= p.Start {
				for _, c := range cells {
					if cols[c.Column] == "" {
						rec.Unmapped = append(rec.Unmapped, c)
					}
				}
			}
			if KnownImportType(item.Category) && item.Category != "other" {
				rec.Recognition = "recognized"
			}
			out.Records = append(out.Records, rec)
		}
	}
	return out
}
func WorkbenchItem(r ImportRecord, project string) Item {
	i := r.Item
	fields := map[string]string{}
	for k, v := range i.Fields {
		fields[k] = v
	}
	i.Fields = fields
	hydrate(&i)
	i.UID = r.ID
	i.Sheet = r.Sheet
	i.Status = "planned"
	if i.Fields == nil {
		i.Fields = map[string]string{}
	}
	// A quantity plan has no manufactured serial; it remains a planning resource.
	if i.Serial == "" && i.Hostname != "" && (i.Category == "server" || i.Category == "switch" || i.Category == "router") {
		i.Fields["planning_identity"] = project + ":" + i.Category + ":" + i.Hostname
		i.Fields["record_kind"] = "planned_device"
	}
	if i.Fields["record_kind"] == "material_summary" || (i.Serial == "" || i.Category == "material") && i.Fields["record_kind"] != "planned_device" && i.Fields["record_kind"] != "component" && i.Category != "port" && i.Category != "cable" && i.Category != "rack" && i.Category != "network_baseline" && KnownImportType(i.Category) && i.Category != "other" {
		if i.Fields["name"] == "" && i.Hostname != "" {
			i.Fields["name"] = i.Hostname
		}
		i.Fields["intended_type"] = i.Category
		if i.Category == "material" {
			i.Serial = ""
			delete(i.Fields, "serial")
		}
		i.Category = "material_plan"
		i.Fields["planning_identity"] = project + ":" + i.Fields["intended_type"] + ":" + strings.ToLower(strings.Join([]string{i.Fields["name"], i.Model, i.Fields["brand"], i.Fields["specification"], i.Fields["unit"]}, "|"))
	}
	return i
}
func WorkbenchItems(b Workbench, project string) []Item {
	out := []Item{}
	for _, r := range b.Records {
		if !r.Ignored {
			out = append(out, WorkbenchItem(r, project))
		}
	}
	return ResolveWorkbench(EnsureRacks(ExpandPorts(ResolveWorkbench(out))))
}
func ValidateWorkbench(b *Workbench, issues []Issue) {
	byID := map[string][]Issue{}
	for _, i := range issues {
		byID[i.Item] = append(byID[i.Item], i)
	}
	for j := range b.Records {
		r := &b.Records[j]
		r.Issues = []Issue{}
		if r.Ignored {
			continue
		}
		r.Issues = append(r.Issues, byID[r.ID]...)
		r.Issues = append(r.Issues, byID[""]...)
		if r.Ambiguity {
			r.Issues = append(r.Issues, RecordIssue(r.ID, "category", "AI 标记了歧义，请人工修正并说明依据，或重新识别，不能以置信度替代核查"))
		}
		if r.Recognition == "unrecognized" {
			r.Issues = append(r.Issues, Issue{ID: "unknown-" + r.ID, Severity: "BLOCKER", Rule: "SOURCE-CLASSIFICATION", Version: "2", Item: r.ID, Field: "category", Reason: "无法识别的来源行：请指定类型或注明非入库内容"})
		}
		if len(r.Issues) > 0 && r.Recognition != "unrecognized" {
			r.Recognition = "needs_review"
		} else if r.Recognition != "unrecognized" {
			r.Recognition = "recognized"
		}
	}
}
func (r ImportRecord) Blocked() bool {
	for _, i := range r.Issues {
		if i.Severity == "BLOCKER" {
			return true
		}
	}
	return false
}
func MaterialPlanIssues(i Item) []Issue {
	out := []Issue{}
	add := func(f, m string) {
		out = append(out, Issue{ID: "plan-" + i.UID + f, Severity: "BLOCKER", Rule: "MATERIAL-PLAN", Version: "1", Item: i.UID, Field: f, Reason: m})
	}
	if i.Fields["name"] == "" {
		add("name", "规划物料名称不能为空")
	}
	q, e := strconv.ParseFloat(i.Fields["quantity"], 64)
	if e != nil || q <= 0 || math.IsNaN(q) || math.IsInf(q, 0) {
		add("quantity", "数量必须为来源明确的正数，不能自动补 1")
	}
	if i.Fields["unit"] == "" {
		add("unit", "数量单位需要明确")
	}
	return out
}
func RecordIssue(id, field, reason string) Issue {
	return Issue{ID: fmt.Sprint(id, "-", field), Severity: "BLOCKER", Rule: "SOURCE-EVIDENCE", Version: "1", Item: id, Field: field, Reason: reason}
}

func AssetType(i Item) string {
	if i.Category == "other" && i.Subtype != "" {
		return i.Subtype
	}
	return i.Category
}

// Resolve explicit, unique identifiers only. Human confirmation approves the resulting relation.
func ResolveWorkbench(items []Item) []Item {
	names := map[string][]string{}
	ids := map[string]bool{}
	for _, i := range items {
		ids[i.UID] = true
		for _, name := range []string{i.Hostname, i.Serial, i.Fields["device_alias"], i.Fields["item_uid"]} {
			if name != "" {
				found := false
				for _, id := range names[name] {
					if id == i.UID {
						found = true
					}
				}
				if !found {
					names[name] = append(names[name], i.UID)
				}
			}
		}
	}
	for j := range items {
		i := &items[j]
		if !ids[i.Parent] && len(names[i.Parent]) == 1 {
			i.Parent = names[i.Parent][0]
			i.Fields["parent_uid"] = i.Parent
		}
	}
	index := map[string]int{}
	for j, i := range items {
		index[i.UID] = j
	}
	for _, r := range Relations(items) {
		if len(r.Candidates) != 1 {
			continue
		}
		j, ok := index[r.Item]
		if !ok {
			continue
		}
		i := &items[j]
		if r.Field == "endpoint_a_uid" && i.A == "" {
			i.A = r.Candidates[0]
			i.Fields[r.Field] = i.A
		}
		if r.Field == "endpoint_b_uid" && i.B == "" {
			i.B = r.Candidates[0]
			i.Fields[r.Field] = i.B
		}
	}

	return items
}

func RequiresModel(kind string) bool {
	if !KnownImportType(kind) {
		return false
	}
	switch kind {
	case "port", "cable", "rack", "network_baseline", "other", "material":
		return false
	}
	return true
}
