package ai

import "encoding/json"

// Reuse the existing workbook tools, and register the relationship actions as
// real provider functions. Text instructions alone are not tool availability.
func kickoffFunctions(finalOnly bool) []any {
	object := func(properties map[string]any, required ...string) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	str := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer"}
	function := func(name, description string, parameters map[string]any) any {
		return map[string]any{"type": "function", "function": map[string]any{"name": name, "description": description, "parameters": parameters, "strict": false}}
	}
	final := function("submit_result", "Submit the final JSON result for the current route, relationship investigation or extraction task. The result must match the task schema in the instructions.", object(map[string]any{"result": map[string]any{"type": "object"}}, "result"))
	if finalOnly {
		return []any{final}
	}
	raw, _ := prompts.ReadFile("prompts/engineering-tools.json")
	var existing []map[string]any
	_ = json.Unmarshal(raw, &existing)
	functions := []any{}
	allowed := map[string]bool{"inspect_sheet": true, "inspect_region": true, "read_range": true, "inspect_formula": true, "trace_formula_dependencies": true, "propose_calculation_rule": true, "run_calculation": true}
	for _, tool := range existing {
		f, _ := tool["function"].(map[string]any)
		name, _ := f["name"].(string)
		if allowed[name] {
			functions = append(functions, tool)
		}
	}
	ref := object(map[string]any{"sheet": str, "cell": str}, "sheet", "cell")
	refs := map[string]any{"type": "array", "items": ref}
	source := object(map[string]any{"sheet": str, "column": integer, "start_row": integer, "end_row": integer}, "sheet", "column", "start_row", "end_row")
	target := object(map[string]any{"sheet": str, "key_columns": map[string]any{"type": "array", "items": integer}, "value_column": integer, "start_row": integer, "end_row": integer}, "sheet", "key_columns", "value_column", "start_row", "end_row")
	alias := object(map[string]any{"from": ref, "to": ref, "reason": str, "evidence": refs}, "from", "to", "reason", "evidence")
	relation := object(map[string]any{"id": str, "kind": map[string]any{"type": "string", "enum": []string{"device_identity", "placement", "cable_spec", "model_spec"}}, "source": source, "targets": map[string]any{"type": "array", "items": target}, "reason": str, "evidence": refs, "aliases": map[string]any{"type": "array", "items": alias}}, "id", "kind", "source", "targets", "reason", "evidence")
	graphRange := object(map[string]any{"region": str, "sheet": str, "start_row": integer, "end_row": integer, "a_device": integer, "a_port": integer, "b_device": integer, "b_port": integer, "rate": integer, "a_model": integer, "b_model": integer}, "region", "sheet", "start_row", "end_row", "a_device", "a_port", "b_device", "b_port")
	functions = append(functions,
		function("inspect_connection_graph", "Inspect ALL termination rows across proposed regions: unique device nodes, port degrees, peer neighborhoods, rate/model evidence, duplicate links and conflicting endpoints. Names are opaque stable tokens; this tool does not classify roles.", object(map[string]any{"ranges": map[string]any{"type": "array", "items": graphRange, "minItems": 1, "maxItems": 128}, "offset": integer}, "ranges")),
		function("find_related_records", "Find related source records locally using the value of a specified source cell; results are candidates, not identity conclusions.", object(map[string]any{"sheet": str, "cell": str, "offset": integer}, "sheet", "cell")),
		function("evaluate_relationships", "Evaluate AI-proposed identity/placement/cable relationships over all source rows; returns matched, missing and conflicting counts and evidence.", object(map[string]any{"relations": map[string]any{"type": "array", "minItems": 1, "maxItems": 128, "items": relation}}, "relations")),
		function("verify_workbook_formulas", "Independently evaluate source formulas once locally. Formula-free workbooks return zero without calculation.", object(map[string]any{})), final)
	return functions
}
