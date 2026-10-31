package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"keel.local/dev/internal/bom"
	"reflect"
	"sort"
	"strings"
)

// A plan contains column references, never executable code or invented asset values.
type SheetPlan struct {
	Purpose    string          `json:"purpose"`
	HeaderRows []int           `json:"header_rows"`
	Tables     []TablePlan     `json:"tables"`
	Note       string          `json:"note"`
	Steps      []bom.AgentStep `json:"-"`
}
type TablePlan struct {
	Key        string            `json:"key"`
	Kind       string            `json:"record_kind"`
	Type       string            `json:"entity_type"`
	Start      int               `json:"start_row"`
	End        int               `json:"end_row"`
	Fields     map[string]int    `json:"fields"`
	TypeColumn int               `json:"type_column"`
	Types      map[string]string `json:"type_values"`
	Required   []string          `json:"row_keys"`
	Confidence float64           `json:"confidence"`
}

type PlanningContext struct {
	Summary     string               `json:"summary"`
	Sources     []PlanningSource     `json:"sources"`
	Identifiers []PlanningIdentifier `json:"identifiers"`
	Questions   []string             `json:"questions"`
}
type PlanningSource struct {
	Sheet       string `json:"sheet"`
	Role        string `json:"role"`
	Description string `json:"description"`
}
type PlanningIdentifier struct {
	Meaning string `json:"meaning"`
	Fields  string `json:"source_fields"`
	Use     string `json:"use"`
}

// Structural sampling keeps all headers and representative category values, not just an arbitrary first row.
func describeSheet(s bom.Sheet, brief bool) map[string]any {
	limit := 8
	if brief {
		limit = 3
	}
	rowSet := map[int]bool{}
	maxRow := 0
	for _, c := range s.Cells {
		if c.Row > maxRow {
			maxRow = c.Row
		}
		if !c.Sensitive && c.Value != "" {
			rowSet[c.Row] = true
		}
	}
	rowOrder := []int{}
	for row := range rowSet {
		rowOrder = append(rowOrder, row)
	}
	sort.Ints(rowOrder)
	if len(rowOrder) > limit {
		rowOrder = rowOrder[:limit]
	}
	sampled := map[int]bool{}
	for _, row := range rowOrder {
		sampled[row] = true
	}
	rows := map[int][]map[string]any{}
	distinct := map[int]map[string]int{}
	saturated := map[int]bool{}
	for _, c := range s.Cells {
		if c.Sensitive || c.Value == "" {
			continue
		}
		if sampled[c.Row] {
			rows[c.Row] = append(rows[c.Row], map[string]any{"column": c.Column, "cell": c.Ref, "value": c.Value, "is_formula": c.Formula != "", "has_cached_value": c.Value != ""})
		}
		if !brief && !saturated[c.Column] {
			if distinct[c.Column] == nil {
				distinct[c.Column] = map[string]int{}
			}
			distinct[c.Column][c.Value]++
			if len(distinct[c.Column]) > 64 {
				delete(distinct, c.Column)
				saturated[c.Column] = true
			}
		}
	}
	order := []int{}
	for row := range rows {
		order = append(order, row)
	}
	sort.Ints(order)
	sample := []any{}
	for n, row := range order {
		if n >= limit {
			break
		}
		sample = append(sample, map[string]any{"row": row, "cells": rows[row]})
	}
	values := map[int][]string{}
	if !brief {
		for col, entries := range distinct {
			if len(entries) <= 64 {
				for value := range entries {
					if len(value) < 180 {
						values[col] = append(values[col], value)
					}
				}
				sort.Strings(values[col])
			}
		}
	}
	return map[string]any{"sheet": s.Name, "last_row": maxRow, "merged_cell_count": len(s.Merges), "sample_rows": sample, "distinct_category_values": values}
}
func (p *Provider) UnderstandPlanning(ctx context.Context, project map[string]string, wb bom.Workbook) (PlanningContext, error) {
	sheets := []any{}
	for _, s := range wb.Sheets {
		view := describeSheet(s, true)
		view["regions"] = bom.NewWorkbookIR(bom.Workbook{Sheets: []bom.Sheet{s}}).InspectWorkbook()
		sheets = append(sheets, view)
	}
	prompt, _ := prompts.ReadFile("prompts/planning-v1.txt")
	raw, e := p.completeJSON(ctx, string(prompt), map[string]any{"project": project, "workbook": sheets, "allowed_types": bom.ImportTypes})
	if e != nil {
		return PlanningContext{}, e
	}
	var result PlanningContext
	if json.Unmarshal(raw, &result) != nil || result.Summary == "" {
		return result, errors.New("AI_PLANNING_SCHEMA_INVALID")
	}
	names := map[string]bool{}
	for _, s := range wb.Sheets {
		names[s.Name] = true
	}
	seen := map[string]bool{}
	for _, s := range result.Sources {
		if !names[s.Sheet] || seen[s.Sheet] {
			return result, errors.New("AI_PLANNING_SCHEMA_INVALID")
		}
		seen[s.Sheet] = true
	}
	// The global summary is context, not an extraction gate. Every actual
	// sheet is still independently inspected and audited by the orchestrator.
	// Missing summary entries are unknown, never implicitly excluded or approved.
	for _, sheet := range wb.Sheets {
		if !seen[sheet.Name] {
			result.Sources = append(result.Sources, PlanningSource{Sheet: sheet.Name, Role: "unknown", Description: "全局摘要未覆盖；必须独立分析，尚未确认用途或版本范围"})
			result.Questions = append(result.Questions, "需要逐表确认用途和项目范围："+sheet.Name)
		}
	}
	return result, nil
}
func (p *Provider) PlanSheet(ctx context.Context, s bom.Sheet, planning PlanningContext) (SheetPlan, error) {
	return p.PlanSheetWithWorkbook(ctx, bom.NewWorkbookIR(bom.Workbook{Sheets: []bom.Sheet{s}}), s.Name, planning, nil)
}

func decodeExtraction(content []byte, maxRow int) (SheetPlan, error) {
	var plan SheetPlan
	if e := json.Unmarshal(content, &plan); e != nil {
		return plan, errors.New("JSON field types do not match: column bindings and row indices must be integers; type_values must map text to strings")
	}
	for k := range plan.Tables {
		t := &plan.Tables[k]
		if t.Key == "" {
			t.Key = fmt.Sprintf("object-%d", k)
		}
		if !bom.KnownImportType(t.Type) {
			return plan, fmt.Errorf("entity_type %q is not in allowed_types", t.Type)
		}
		if t.Start < 1 || t.End < t.Start || t.End > maxRow {
			return plan, fmt.Errorf("table %s range %d:%d exceeds actual rows 1:%d", t.Key, t.Start, t.End, maxRow)
		}
		if t.Confidence < 0 || t.Confidence > 1 {
			return plan, errors.New("confidence must be between 0 and 1")
		}
		if t.TypeColumn < 0 || t.TypeColumn > 512 {
			return plan, errors.New("type_column must be 0 or an actual source column")
		}
		if t.Kind != "reference" && len(t.Fields) == 0 {
			return plan, errors.New("non-reference extractor requires source fields")
		}
		for field, col := range t.Fields {
			if bom.Field(field) != field || col < 1 || col > 512 {
				return plan, fmt.Errorf("field %q column %d is outside the allowed fields/columns", field, col)
			}
		}
		// Row eligibility is deterministic. Optional detail columns must never
		// filter out an otherwise identifiable incomplete source record.
		normalizeRowKeys(t)
		for _, key := range t.Required {
			if t.Fields[key] == 0 {
				return plan, fmt.Errorf("row_keys field %q is missing from fields", key)
			}
		}
		for value, typ := range t.Types {
			if strings.TrimSpace(value) == "" || !bom.KnownImportType(typ) {
				return plan, fmt.Errorf("type_values entry %q has unsupported entity_type %q", value, typ)
			}
		}
	}
	return plan, nil
}

type Description struct {
	ID          int    `json:"id"`
	Model       string `json:"model"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Classification struct {
	ID         int     `json:"id"`
	Type       string  `json:"entity_type"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func (p *Provider) Classify(ctx context.Context, input []Description, planning PlanningContext) ([]Classification, error) {
	prompt, _ := prompts.ReadFile("prompts/classification-v1.txt")
	local := append([]Description(nil), input...)
	for j := range local {
		local[j].ID = j
	}
	payload := map[string]any{"planning_context": planning.Summary, "items": local, "allowed_types": bom.ImportTypes}
	formats := []map[string]any{}
	if p.JSONMode {
		formats = append(formats, classificationResponseFormat(len(local)))
	}
	raw, e := p.completeJSON(ctx, string(prompt), payload, formats...)
	if e != nil && len(formats) > 0 && p.HTTPStatus == 400 {
		raw, e = p.completeJSON(ctx, string(prompt), payload)
	}
	if e != nil {
		return nil, e
	}
	var result struct {
		Items []Classification `json:"items"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Items) != len(input) {
		return nil, errors.New("AI_CLASSIFICATION_SCHEMA_INVALID")
	}
	ids := map[int]bool{}
	for _, i := range local {
		ids[i.ID] = true
	}
	for _, c := range result.Items {
		if !ids[c.ID] || !bom.KnownImportType(c.Type) || c.Confidence < 0 || c.Confidence > 1 {
			return nil, errors.New("AI_CLASSIFICATION_SCHEMA_INVALID")
		}
		delete(ids, c.ID)
	}
	for j := range result.Items {
		result.Items[j].ID = input[result.Items[j].ID].ID
	}
	return result.Items, nil
}

// The agent checks its recipe against the complete sheet, then asks for a correction
// with the actual uncovered category values. A sample-only plan cannot silently
// turn all remaining device roles into one category.
func inspectExtraction(s bom.Sheet, plan SheetPlan) error {
	if err := inspectBindingSemantics(s, plan); err != nil {
		return err
	}
	headers := map[int]bom.Cell{}
	canonical := map[string][]int{}
	for _, row := range plan.HeaderRows {
		for _, c := range s.Cells {
			if c.Row == row {
				headers[c.Column] = c
				if f := bom.Field(c.Value); f != "" {
					canonical[f] = append(canonical[f], c.Column)
				}
			}
		}
	}
	for _, t := range plan.Tables {
		if t.Kind == "reference" {
			continue
		}
		if t.Kind == "physical_connection" {
			// A point-to-point estimate or quantity schedule is not evidence of
			// individual physical ports. Keep it as reference/material planning.
			for _, side := range []string{"a", "b"} {
				device, port, uid := "endpoint_"+side+"_device", "endpoint_"+side+"_port", "endpoint_"+side+"_uid"
				if (t.Fields[device] == 0 || t.Fields[port] == 0) && (t.Fields[uid] == 0 || bom.Field(headers[t.Fields[uid]].Value) != uid) {
					return fmt.Errorf("PHYSICAL_ENDPOINT_BINDING_REQUIRED: row=%d column=%d; map explicit device AND port columns for each end; estimates/quantities without ports remain material_summary or reference", t.Start, firstBoundColumn(t.Fields))
				}
			}
		}
		for field, col := range t.Fields {
			sourceField := bom.Field(headers[col].Value)
			if (field == "endpoint_a_uid" && sourceField == "endpoint_a_device") || (field == "endpoint_b_uid" && sourceField == "endpoint_b_device") || (field == "wavelength" && (sourceField == "endpoint_a_optic" || sourceField == "endpoint_b_optic")) {
				return fmt.Errorf("SOURCE_BINDING_CONFLICT: row=%d column=%d field=%s; explicit header means %s, not a stable port ID or wavelength", t.Start, col, field, sourceField)
			}
			if field == "hostname" && len(canonical["hostname"]) > 0 && sourceField != "hostname" {
				return fmt.Errorf("%s binds hostname to %q; explicit hostname/label columns exist at %v. Preserve the other label as device_alias, not canonical identity", t.Key, headers[col].Value, canonical["hostname"])
			}
			if field == "speed" && sourceField == "media_type" || field == "site" && sourceField == "rack" || field == "rack_height" && sourceField == "u_start" {
				return fmt.Errorf("%s binds %s to header %q whose source meaning is %s; do not convert medium to speed, rack to site, position to capacity, or fabric counts to VLAN", t.Key, field, headers[col].Value, sourceField)
			}
		}
	}

	for i, t := range plan.Tables {
		for _, u := range plan.Tables[:i] {
			if t.Kind == "material_summary" && u.Kind == t.Kind && t.Start <= u.End && u.Start <= t.End && t.Fields["model"] > 0 && t.Fields["model"] == u.Fields["model"] && !disjointTypes(t, u) {
				return fmt.Errorf("duplicate summary extractors %s and %s classify the same source models several times; use ONE extractor with type_column/type_values or classify each unique model afterward", t.Key, u.Key)
			}
		}
	}
	return inspectSourceCoverage(s, plan)
}

func CheckExtraction(s bom.Sheet, plan SheetPlan) error { return inspectExtraction(s, plan) }

// Explicit source header meanings correct mechanical binding errors; they do not
// create values. Ambiguous aliases still remain candidates, never formal assets.
func normalizeSourceBindings(s bom.Sheet, p *SheetPlan) {
	normalizeExplicitBindings(s, p)
	heads := map[int]string{}
	sourceLabels := map[int]string{}
	columns := map[string][]int{}
	labels := []int{}
	for _, row := range p.HeaderRows {
		for _, c := range s.Cells {
			if c.Row == row {
				heads[c.Column] = bom.Field(c.Value)
				sourceLabels[c.Column] = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(c.Value), " ", ""))
				if heads[c.Column] != "" {
					columns[heads[c.Column]] = append(columns[heads[c.Column]], c.Column)
				}
				if heads[c.Column] == "hostname" {
					labels = append(labels, c.Column)
				}
			}
		}
	}
	for j := range p.Tables {
		t := &p.Tables[j]
		if t.Types == nil {
			t.Types = map[string]string{}
		}
		// A device label is never a CMDB port UID; an optic part number is
		// never a wavelength. Repair these observed mechanical key errors
		// from explicit headers, without changing source values or existing
		// canonical endpoint bindings. Alternate cells remain raw evidence.
		for _, wrong := range []string{"endpoint_a_uid", "endpoint_b_uid", "wavelength"} {
			col := t.Fields[wrong]
			actual := heads[col]
			if (wrong == "endpoint_a_uid" && actual == "endpoint_a_device") || (wrong == "endpoint_b_uid" && actual == "endpoint_b_device") || (wrong == "wavelength" && (actual == "endpoint_a_optic" || actual == "endpoint_b_optic")) {
				if t.Fields[actual] == 0 {
					t.Fields[actual] = col
				}
				delete(t.Fields, wrong)
			}
		}
		if t.Kind == "physical_connection" {
			// An A/Z-side device label is an endpoint, not a containing
			// parent of the cable. This otherwise creates a false dependency
			// on a display label that can never be selected for submission.
			if col := t.Fields["parent_uid"]; heads[col] == "endpoint_a_device" || heads[col] == "endpoint_b_device" {
				if t.Fields[heads[col]] == 0 {
					t.Fields[heads[col]] = col
				}
				delete(t.Fields, "parent_uid")
			}
			if col := t.Fields["parent_uid"]; sourceLabels[col] == "序号" || sourceLabels[col] == "no" || sourceLabels[col] == "no." || sourceLabels[col] == "rowid" {
				if t.Fields["sequence"] == 0 {
					t.Fields["sequence"] = col
				}
				delete(t.Fields, "parent_uid")
			}
		}
		if col := t.Fields["wavelength"]; sourceLabels[col] == "fibercorecount" || sourceLabels[col] == "纤芯数" {
			// A count has no wavelength unit. Keep the explicit count as a
			// specification (or unmapped evidence if that slot is occupied).
			if t.Fields["specification"] == 0 || t.Fields["specification"] == t.Fields["media_type"] {
				t.Fields["specification"] = col
			}
			delete(t.Fields, "wavelength")
		}
		// Explicit, unambiguous source headers are deterministic evidence. Fill
		// omitted bindings on an already identified object, never infer values,
		// invent an extractor or overwrite a conflicting model binding.
		fields := []string{}
		if t.Kind == "planned_device" && (t.Fields["hostname"] > 0 || t.Fields["model"] > 0) {
			fields = []string{"hostname", "serial", "model", "management_ip", "mac", "site", "rack", "u_start", "u_height"}
		}
		if t.Kind == "physical_connection" {
			fields = []string{"endpoint_a_device", "endpoint_a_port", "endpoint_b_device", "endpoint_b_port", "model", "endpoint_a_optic", "endpoint_b_optic", "length", "media_type", "speed"}
		}
		for _, field := range fields {
			if t.Fields[field] == 0 && len(columns[field]) == 1 {
				t.Fields[field] = columns[field][0]
			}
		}
		if col := t.Fields["hostname"]; col > 0 && len(labels) == 1 && heads[col] != "hostname" {
			if t.Fields["device_alias"] == 0 {
				t.Fields["device_alias"] = col
			}
			t.Fields["hostname"] = labels[0]
		}
		if col := t.Fields["speed"]; col > 0 && heads[col] == "media_type" {
			t.Fields["media_type"] = col
			delete(t.Fields, "speed")
		}
		if col := t.Fields["site"]; col > 0 && heads[col] == "rack" {
			if t.Fields["rack"] == 0 {
				t.Fields["rack"] = col
			}
			delete(t.Fields, "site")
		}
		if col := t.Fields["rack_height"]; col > 0 && heads[col] == "u_start" {
			if t.Fields["u_start"] == 0 {
				t.Fields["u_start"] = col
			}
			delete(t.Fields, "rack_height")
		}
		// Header repair may have added the actual identity column. Recompute
		// eligibility afterwards; a former model/serial key must not drop rows.
		normalizeRowKeys(t)
	}
	// A separate optic extractor and the cable-end model describe the same
	// module. Carry its explicit owner/port bindings across those recipes;
	// otherwise every row becomes an unowned extra module.
	for j := range p.Tables {
		t := &p.Tables[j]
		if t.Kind != "component" || (t.Type != "optic_module" && t.Type != "electrical_module") {
			continue
		}
		for _, field := range []string{"model", "name", "specification"} {
			col := t.Fields[field]
			for _, side := range []string{"a", "b"} {
				if heads[col] != "endpoint_"+side+"_optic" {
					continue
				}
				for _, cable := range p.Tables {
					if cable.Kind != "physical_connection" || cable.Start > t.Start || cable.End < t.End || cable.Fields["endpoint_"+side+"_optic"] != col {
						continue
					}
					for target, source := range map[string]string{"parent_uid": "endpoint_" + side + "_device", "port_index": "endpoint_" + side + "_port"} {
						if t.Fields[target] == 0 && cable.Fields[source] > 0 {
							t.Fields[target] = cable.Fields[source]
						}
					}
					if t.Fields["model"] == 0 {
						t.Fields["model"] = col
					}
					t.Required = []string{"model"}
				}
			}
		}
	}

	// A cable-end part number alone is not a separately identified physical
	// module. Its model/source remain on the cable; do not duplicate it as stock.
	retained := []TablePlan{}
	for _, t := range p.Tables {
		covered := false
		if t.Kind == "component" && (t.Type == "optic_module" || t.Type == "electrical_module") && t.Fields["serial"] == 0 {
			for _, cable := range p.Tables {
				if cable.Kind != "physical_connection" || cable.Start > t.Start || cable.End < t.End {
					continue
				}
				for _, side := range []string{"a", "b"} {
					if t.Fields["model"] > 0 && t.Fields["model"] == cable.Fields["endpoint_"+side+"_optic"] && t.Fields["parent_uid"] == cable.Fields["endpoint_"+side+"_device"] && t.Fields["port_index"] == cable.Fields["endpoint_"+side+"_port"] {
						covered = true
					}
				}
			}
		}
		if !covered {
			retained = append(retained, t)
		}
	}
	p.Tables = retained

	// A model may split attributes of one source object into two recipes.
	// Merge only compatible bindings; conflicting source interpretations still fail.
	merged := []TablePlan{}
	for _, table := range p.Tables {
		joined := false
		for j := range merged {
			old := &merged[j]
			if old.Kind != table.Kind || old.Type != table.Type || old.Start != table.Start || old.End != table.End || old.TypeColumn != table.TypeColumn || !reflect.DeepEqual(old.Types, table.Types) || SourceAnchor(old.Fields) != SourceAnchor(table.Fields) {
				continue
			}
			conflict := false
			for f, col := range table.Fields {
				if old.Fields[f] > 0 && old.Fields[f] != col {
					conflict = true
				}
			}
			if conflict {
				continue
			}
			for f, col := range table.Fields {
				old.Fields[f] = col
			}
			if table.Confidence < old.Confidence {
				old.Confidence = table.Confidence
			}
			joined = true
			break
		}
		if !joined {
			merged = append(merged, table)
		}
	}
	p.Tables = merged
}

func normalizeRowKeys(t *TablePlan) {
	t.Required = []string{}
	if t.Kind == "reference" {
		return
	}
	for _, key := range []string{"hostname", "endpoint_a_device", "endpoint_b_device", "parent_uid", "serial", "model", "name", "port_index"} {
		if t.Fields[key] > 0 {
			t.Required = []string{key}
			return
		}
	}
}

func firstBoundColumn(fields map[string]int) int {
	first := 512
	for _, col := range fields {
		if col > 0 && col < first {
			first = col
		}
	}
	return first
}

// Ignore narration/confidence/table names, but not changes to what gets read.
func SameExtraction(a, b SheetPlan) bool {
	signature := func(p SheetPlan) string {
		headers := append([]int{}, p.HeaderRows...)
		sort.Ints(headers)
		tables := []string{}
		for _, table := range p.Tables {
			table.Key, table.Confidence = "", 0
			raw, _ := json.Marshal(table)
			tables = append(tables, string(raw))
		}
		sort.Strings(tables)
		raw, _ := json.Marshal([]any{headers, tables})
		return string(raw)
	}
	return signature(a) == signature(b)
}

func disjointTypes(a, b TablePlan) bool {
	if a.TypeColumn == 0 || a.TypeColumn != b.TypeColumn || len(a.Types) == 0 || len(b.Types) == 0 {
		return false
	}
	for v := range a.Types {
		if _, ok := b.Types[v]; ok {
			return false
		}
	}
	return true
}

// Re-use a persisted model answer after fixing the local interpreter. This does
// not call the provider, and never reuses a transport failure as a valid plan.
func RestoreExtraction(s bom.Sheet, raw []byte) (SheetPlan, error) {
	p, e := RestoreExtractionBindings(s, raw)
	if e != nil {
		return p, e
	}
	return p, inspectExtraction(s, p)
}

// Legacy failed attempts could persist malformed answers. Only schema-valid,
// locally interpretable bindings can constrain a later recognition attempt.
func RestoreExtractionBindings(s bom.Sheet, raw []byte) (SheetPlan, error) {
	// Older Go saves encoded optional empty slices/maps as null although the
	// model contract uses []/{}. Repair only these persisted empty defaults;
	// malformed bindings and fresh model responses still fail strict schema.
	var stored map[string]json.RawMessage
	if json.Unmarshal(raw, &stored) == nil {
		var tables []map[string]json.RawMessage
		if json.Unmarshal(stored["tables"], &tables) == nil {
			for _, table := range tables {
				for _, key := range []string{"row_keys", "type_values"} {
					if string(table[key]) == "null" {
						delete(table, key)
					}
				}
			}
			stored["tables"], _ = json.Marshal(tables)
			raw, _ = json.Marshal(stored)
		}
	}
	if e := validateExtractionSchema(context.Background(), raw); e != nil {
		return SheetPlan{}, e
	}
	max := 0
	for _, c := range s.Cells {
		if c.Row > max {
			max = c.Row
		}
	}
	p, e := decodeExtraction(raw, max)
	if e != nil {
		return p, e
	}
	if p.Purpose == "" {
		return p, errors.New("NO_SAVED_EXTRACTION")
	}
	normalizeSourceBindings(s, &p)
	return p, nil
}
