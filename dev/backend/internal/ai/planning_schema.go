package ai

import "keel.local/dev/internal/bom"

// Supply the same field/type vocabulary during generation, not just after a
// malformed answer. Providers without JSON Schema support still undergo exactly
// the same local validation and bounded correction loop.
func extractionResponseFormat(sheet bom.Sheet) map[string]any {
	object := func(props map[string]any, required ...string) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	}
	text := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer", "minimum": 1}
	fields := map[string]any{}
	scores := map[int]int{}
	for _, cell := range sheet.Cells {
		if cell.Row <= 8 && !cell.Sensitive && bom.Field(cell.Value) != "" {
			scores[cell.Row]++
		}
	}
	best := 0
	for row, score := range scores {
		if score > scores[best] || score == scores[best] && row < best {
			best = row
		}
	}
	sourceColumns := map[string][]int{}
	if scores[best] >= 2 {
		for _, cell := range sheet.Cells {
			if cell.Row == best && !cell.Sensitive {
				if field := bom.Field(cell.Value); field != "" {
					sourceColumns[field] = append(sourceColumns[field], cell.Column)
				}
			}
		}
	}
	for _, field := range bom.ImportFields() {
		fields[field] = integer
	}
	for _, field := range []string{"hostname", "serial", "management_ip", "mac", "rack", "site", "u_start", "u_height", "speed", "media_type"} {
		if cols := sourceColumns[field]; len(cols) > 0 {
			fields[field] = map[string]any{"type": "integer", "enum": cols}
		}
	}
	table := object(map[string]any{
		"key": text, "record_kind": map[string]any{"type": "string", "enum": []string{"planned_device", "material_summary", "component", "physical_connection", "physical_port", "logical_connection", "reference"}},
		"entity_type": map[string]any{"type": "string", "enum": bom.ImportTypes},
		"start_row":   integer, "end_row": integer, "fields": object(fields),
		"type_column": map[string]any{"type": "integer", "minimum": 0, "maximum": 512},
		"type_values": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string", "enum": bom.ImportTypes}},
		"row_keys":    map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": bom.ImportFields()}},
		"confidence":  map[string]any{"type": "number", "minimum": 0, "maximum": 1},
	}, "key", "record_kind", "entity_type", "start_row", "end_row", "fields", "type_column", "type_values", "row_keys", "confidence")
	plan := object(map[string]any{"purpose": map[string]any{"type": "string", "enum": []string{"table", "layout", "reference", "unknown"}}, "header_rows": map[string]any{"type": "array", "items": integer}, "tables": map[string]any{"type": "array", "items": table}, "note": text}, "purpose", "header_rows", "tables", "note")
	schema := object(map[string]any{
		"action":     map[string]any{"type": "string", "enum": []string{"submit_plan", "inspect_region", "analyze_sheet", "cross_sheet_resolve"}},
		"sheet_name": text, "start_row": integer, "end_row": integer, "first_column": integer, "last_column": integer, "plan": plan,
	}, "action")
	return map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "workbook_extraction_action", "strict": false, "schema": schema}}
}

func classificationResponseFormat(count int) map[string]any {
	item := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "entity_type", "confidence", "reason"}, "properties": map[string]any{
		"id":          map[string]any{"type": "integer", "minimum": 0, "maximum": count - 1},
		"entity_type": map[string]any{"type": "string", "enum": bom.ImportTypes},
		"confidence":  map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		"reason":      map[string]any{"type": "string"},
	}}
	schema := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"items"}, "properties": map[string]any{"items": map[string]any{"type": "array", "items": item, "minItems": count, "maxItems": count}}}
	return map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "bom_classification", "strict": false, "schema": schema}}
}
