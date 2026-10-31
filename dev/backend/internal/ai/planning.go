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
	rows := map[int][]map[string]any{}
	maxRow := 0
	distinct := map[int]map[string]int{}
	for _, c := range s.Cells {
		if c.Row > maxRow {
			maxRow = c.Row
		}
		if c.Sensitive || c.Value == "" {
			continue
		}
		rows[c.Row] = append(rows[c.Row], map[string]any{"column": c.Column, "cell": c.Ref, "value": c.Value, "is_formula": c.Formula != "", "has_cached_value": c.Value != ""})
		if distinct[c.Column] == nil {
			distinct[c.Column] = map[string]int{}
		}
		distinct[c.Column][c.Value]++
	}
	order := []int{}
	for row := range rows {
		order = append(order, row)
	}
	sort.Ints(order)
	sample := []any{}
	limit := 8
	if brief {
		limit = 3
	}
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
	if len(seen) != len(names) {
		return result, errors.New("AI_PLANNING_SOURCE_COVERAGE")
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
		if t.Kind == "reference" {
			t.Required = nil
		}
		for _, key := range []string{"hostname", "endpoint_a_device", "endpoint_b_device", "parent_uid", "serial", "model", "name", "port_index"} {
			if t.Fields[key] > 0 {
				t.Required = []string{key}
				break
			}
		}
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
	raw, e := p.completeJSON(ctx, string(prompt), map[string]any{"planning_context": planning.Summary, "items": input, "allowed_types": bom.ImportTypes})
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
	for _, i := range input {
		ids[i.ID] = true
	}
	for _, c := range result.Items {
		if !ids[c.ID] || !bom.KnownImportType(c.Type) || c.Confidence < 0 || c.Confidence > 1 {
			return nil, errors.New("AI_CLASSIFICATION_SCHEMA_INVALID")
		}
		delete(ids, c.ID)
	}
	return result.Items, nil
}

// The agent checks its recipe against the complete sheet, then asks for a correction
// with the actual uncovered category values. A sample-only plan cannot silently
// turn all remaining device roles into one category.
func inspectExtraction(s bom.Sheet, plan SheetPlan) error {
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
		for field, col := range t.Fields {
			sourceField := bom.Field(headers[col].Value)
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
	heads := map[int]string{}
	columns := map[string][]int{}
	labels := []int{}
	for _, row := range p.HeaderRows {
		for _, c := range s.Cells {
			if c.Row == row {
				heads[c.Column] = bom.Field(c.Value)
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
		// Explicit, unambiguous source headers are deterministic evidence. Fill
		// omitted bindings on an already identified object, never infer values,
		// invent an extractor or overwrite a conflicting model binding.
		fields := []string{}
		if t.Kind == "planned_device" && (t.Fields["hostname"] > 0 || t.Fields["model"] > 0) {
			fields = []string{"hostname", "serial", "model", "management_ip", "mac", "site", "rack", "u_start", "u_height"}
		}
		if t.Kind == "physical_connection" && (t.Fields["endpoint_a_device"] > 0 || t.Fields["endpoint_b_device"] > 0) {
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
	}

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
