package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"keel.local/dev/internal/engineering"
	"sort"
	"strings"
)

// A failed candidate remains a draft, not evidence. Local edits let the Agent
// correct one binding without regenerating unrelated, already inspected fields.
func engineeringCorrectionDraft(history []any, sheet string) (*engineering.EngineeringProposal, string) {
	for i := len(history) - 1; i >= 0; i-- {
		raw, _ := json.Marshal(history[i])
		var step struct {
			Action engineeringAction `json:"action"`
			Result struct {
				Error string `json:"error"`
			} `json:"result"`
		}
		if json.Unmarshal(raw, &step) != nil || step.Action.Proposal == nil || step.Action.Sheet != sheet {
			continue
		}
		for _, prefix := range []string{"FIELD_NOT_SUPPORTED_FOR_REGION", "HEADER_FIELD_CONFLICT", "FIELD_VALUE_CONFLICT", "FIELD_SOURCE_CONFLICT", "FIELD_BINDING_REQUIRED", "REGION_KIND_CONFLICT", "SUPPORTED_HEADER_UNMAPPED", "INCOMPLETE_SHEET_COVERAGE", "INVALID_CONSTANT_VALUE", "INVALID_CONSTANT_CELL", "UNSUPPORTED_CONSTANT_VALUE"} {
			if strings.HasPrefix(step.Result.Error, prefix) {
				return step.Action.Proposal, step.Result.Error
			}
		}
		return nil, "" // Structural errors still need the full proposal tool.
	}
	return nil, ""
}

func engineeringCorrectionTool(draft engineering.EngineeringProposal, sheet string, attributes []engineering.AttributeDefinition, validationError string) any {
	fields, removable := []string{}, []string{}
	kinds := []string{"other"}
	for kind := range engineeringRegionCapabilities() {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, region := range draft.Regions {
		fields = appendUnique(fields, engineering.RegionFields(region.Kind)...)
		for _, c := range region.Columns {
			removable = appendUnique(removable, c.Field)
		}
		for _, c := range region.Constants {
			removable = appendUnique(removable, c.Field)
		}
	}
	for _, a := range attributes {
		if a.ApprovalStatus == "approved" {
			fields = appendUnique(fields, fmt.Sprintf("attribute:%s@%d", a.ID, a.Version))
		}
	}
	columns := map[string]any{"type": "array", "description": "Sparse edits only; use [] when no column changes are needed. Each field appears once.", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"field", "column"}, "properties": map[string]any{"field": map[string]any{"type": "string", "enum": fields}, "column": map[string]any{"type": "integer", "minimum": 1}, "unit": map[string]any{"type": "string"}}}}
	constant := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"field", "value", "evidence_cells"}, "properties": map[string]any{
		"field": map[string]any{"type": "string", "enum": fields}, "value": map[string]any{"type": "string"}, "evidence_cells": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}},
	}}
	bounds := map[string]any{}
	for _, key := range []string{"start_row", "end_row", "start_column", "end_column"} {
		bounds[key] = map[string]any{"type": "integer", "minimum": 1}
	}
	tool := map[string]any{"type": "function", "function": map[string]any{
		"name": "map_semantic_fields", "description": "Correct draft_to_correct. Change only specified fields in one region; all other bindings remain unchanged. The entire resulting proposal is validated before saving. Region index is zero-based. Always include set_columns and remove_columns; use [] for an unused operation. Removing an unsupported field requires remove_columns, not an empty call.",
		"parameters": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"sheet", "region_index", "set_columns", "remove_columns"}, "properties": map[string]any{
			"sheet": map[string]any{"type": "string", "enum": []string{sheet}}, "region_index": map[string]any{"type": "integer", "minimum": 0, "maximum": len(draft.Regions) - 1},
			"set_kind":    map[string]any{"type": "string", "enum": kinds, "description": "Correct the region domain only when its source evidence supports that domain; keep existing bindings for validation."},
			"set_columns": columns, "remove_columns": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": removable}},
			"set_constants": map[string]any{"type": "array", "items": constant}, "remove_constants": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": removable}},
			"bounds":      map[string]any{"type": "object", "additionalProperties": false, "properties": bounds, "required": []string{"start_row", "end_row", "start_column", "end_column"}},
			"header_rows": map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 1}, "description": "All title, header, subtotal or footer rows to exclude from entity extraction."},
		}},
	}}
	fn := tool["function"].(map[string]any)
	focusedKeys := []string{}
	if strings.HasPrefix(validationError, "INCOMPLETE_SHEET_COVERAGE") {
		fn["description"] = "Include the missing source area by correcting bounds of the appropriate region. Preserve field bindings. Set header_rows to exclude titles, headers and totals from entity extraction; retain existing excluded rows."
		focusedKeys = []string{"sheet", "region_index", "bounds", "header_rows"}
	} else if strings.HasPrefix(validationError, "REGION_KIND_CONFLICT") {
		fn["description"] = "The source column is already correctly mapped. Correct only the region_index and set_kind identified by the validation error, choosing an evidenced allowed_region_kind. Preserve existing bindings for revalidation."
		focusedKeys = []string{"sheet", "region_index", "set_kind"}
	}
	if len(focusedKeys) > 0 {
		params := fn["parameters"].(map[string]any)
		props := params["properties"].(map[string]any)
		focused := map[string]any{}
		for _, key := range focusedKeys {
			focused[key] = props[key]
		}
		params["properties"] = focused
		params["required"] = focusedKeys
	}

	return tool
}

func materializeEngineeringCorrection(raw []byte, draft *engineering.EngineeringProposal, sheet string) ([]byte, bool, error) {
	var shape map[string]json.RawMessage
	if json.Unmarshal(raw, &shape) != nil || shape["region_index"] == nil {
		return raw, false, nil
	}
	var patch struct {
		Action          string                        `json:"action"`
		Sheet           string                        `json:"sheet"`
		Region          int                           `json:"region_index"`
		Kind            string                        `json:"set_kind"`
		Columns         json.RawMessage               `json:"set_columns"`
		RemoveColumns   []string                      `json:"remove_columns"`
		Constants       []engineering.ConstantBinding `json:"set_constants"`
		RemoveConstants []string                      `json:"remove_constants"`
		Bounds          *engineering.Bounds           `json:"bounds"`
		HeaderRows      *[]int                        `json:"header_rows"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&patch); err != nil {
		return raw, false, err
	}
	if patch.Action != "map_semantic_fields" || patch.Sheet != sheet || draft == nil || patch.Region < 0 || patch.Region >= len(draft.Regions) {
		return raw, false, fmt.Errorf("INVALID_DRAFT_CORRECTION")
	}
	if len(patch.Columns) == 0 && len(patch.RemoveColumns) == 0 && len(patch.Constants) == 0 && len(patch.RemoveConstants) == 0 && patch.Bounds == nil && patch.Kind == "" && patch.HeaderRows == nil {
		return raw, false, fmt.Errorf("EMPTY_DRAFT_CORRECTION: provide set_columns, remove_columns, set_constants, remove_constants or bounds; sheet and region_index alone make no change. Correct the outstanding draft validation error")
	}
	encoded, _ := json.Marshal(draft)
	var candidate engineering.EngineeringProposal
	_ = json.Unmarshal(encoded, &candidate)
	region := &candidate.Regions[patch.Region]
	if patch.Kind != "" {
		region.Kind = patch.Kind
	}
	remove := func(field string, names []string) bool {
		for _, name := range names {
			if field == name {
				return true
			}
		}
		return false
	}
	columns := []engineering.ColumnBinding{}
	if len(patch.Columns) > 0 {
		// Reuse the unique-key normalizer, then reject unknown nested properties.
		wrapper := map[string]any{"proposal": map[string]any{"regions": []any{map[string]any{"columns": patch.Columns}}}}
		data, _ := json.Marshal(wrapper)
		normalized, err := normalizeEngineeringColumns(data)
		if err != nil {
			return raw, false, err
		}
		var result struct {
			Proposal struct {
				Regions []struct {
					Columns []engineering.ColumnBinding `json:"columns"`
				} `json:"regions"`
			} `json:"proposal"`
		}
		decoder := json.NewDecoder(bytes.NewReader(normalized))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result); err != nil {
			return raw, false, err
		}
		columns = result.Proposal.Regions[0].Columns
	}
	for _, c := range columns {
		patch.RemoveColumns = append(patch.RemoveColumns, c.Field)
	}
	if len(patch.RemoveColumns) > 0 || len(columns) > 0 {
		kept := []engineering.ColumnBinding{}
		for _, c := range region.Columns {
			if !remove(c.Field, patch.RemoveColumns) {
				kept = append(kept, c)
			}
		}
		region.Columns = append(kept, columns...)
	}
	seen := map[string]bool{}
	for _, c := range patch.Constants {
		if seen[c.Field] {
			return raw, false, fmt.Errorf("DUPLICATE_CONSTANT_CORRECTION: %s", c.Field)
		}
		seen[c.Field] = true
		patch.RemoveConstants = append(patch.RemoveConstants, c.Field)
	}
	if len(patch.RemoveConstants) > 0 || len(patch.Constants) > 0 {
		constants := []engineering.ConstantBinding{}
		for _, c := range region.Constants {
			if !remove(c.Field, patch.RemoveConstants) {
				constants = append(constants, c)
			}
		}
		region.Constants = append(constants, patch.Constants...)
	}
	if patch.Bounds != nil {
		region.Bounds = *patch.Bounds
	}
	if patch.HeaderRows != nil {
		region.HeaderRows = *patch.HeaderRows
	}
	if engineering.Digest(candidate) == engineering.Digest(*draft) {
		return raw, false, fmt.Errorf("EMPTY_DRAFT_CORRECTION: change the field named in the validation error")
	}
	// Optional arrays omitted in the original tool call must serialize as arrays,
	// not Go nil/null, when the canonical draft re-enters strict validation.
	if candidate.Modules == nil {
		candidate.Modules = []engineering.ModuleDetection{}
	}
	if candidate.Questions == nil {
		candidate.Questions = []string{}
	}
	for i := range candidate.Regions {
		r := &candidate.Regions[i]
		if r.Constants == nil {
			r.Constants = []engineering.ConstantBinding{}
		}
		if r.Columns == nil {
			r.Columns = []engineering.ColumnBinding{}
		}
		if r.HeaderRows == nil {
			r.HeaderRows = []int{}
		}
	}
	normalized, err := json.Marshal(engineeringAction{Action: "map_semantic_fields", Sheet: sheet, Proposal: &candidate})
	return normalized, true, err
}
