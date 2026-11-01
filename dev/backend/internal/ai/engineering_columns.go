package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Native tool arguments use field-keyed columns to make one binding per field
// explicit. Canonical proposals/checkpoints retain the existing typed arrays.
func engineeringColumnMapSchema(definitions map[string]any, fields []string) {
	region, ok := definitions["Region"].(map[string]any)
	if !ok {
		return
	}
	definitions["ColumnPosition"] = map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"column": map[string]any{"type": "integer", "minimum": 1}, "unit": map[string]any{"type": "string"}},
		"required":   []string{"column"},
	}
	properties := map[string]any{}
	for _, field := range fields {
		properties[field] = map[string]any{"$ref": "#/$defs/ColumnPosition"}
	}
	region["properties"].(map[string]any)["columns"] = map[string]any{
		"type": "object", "properties": properties, "additionalProperties": false,
		"description": "Map each evidenced engineering field ONCE to {column: absolute_column_number, unit: optional_unit}. Omit unsupported fields. Region bounds still cover unbound source columns.",
	}
	delete(definitions, "Column")
}

func normalizeEngineeringColumns(raw []byte) ([]byte, error) {
	var action map[string]json.RawMessage
	if err := json.Unmarshal(raw, &action); err != nil {
		return raw, err
	}
	if len(action["proposal"]) == 0 {
		return raw, nil
	}
	var proposal map[string]json.RawMessage
	if err := json.Unmarshal(action["proposal"], &proposal); err != nil {
		return raw, err
	}
	var regions []map[string]json.RawMessage
	if err := json.Unmarshal(proposal["regions"], &regions); err != nil {
		return raw, err
	}
	changed := false
	for _, region := range regions {
		columns := bytes.TrimSpace(region["columns"])
		if len(columns) == 0 || columns[0] != '{' {
			continue
		} // Existing array protocol remains valid.
		decoder := json.NewDecoder(bytes.NewReader(columns))
		_, _ = decoder.Token()
		seen := map[string]bool{}
		bindings := []map[string]json.RawMessage{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return raw, err
			}
			field := token.(string)
			if seen[field] {
				return raw, fmt.Errorf("DUPLICATE_FIELD_BINDING: %s; a field key may appear only once", field)
			}
			seen[field] = true
			var binding map[string]json.RawMessage
			if err := decoder.Decode(&binding); err != nil || binding == nil {
				return raw, fmt.Errorf("INVALID_COLUMN_BINDING: %s requires an object with column and optional unit", field)
			}
			if _, exists := binding["field"]; exists {
				return raw, fmt.Errorf("INVALID_COLUMN_BINDING: field belongs in the object key")
			}
			binding["field"], _ = json.Marshal(field)
			bindings = append(bindings, binding)
		}
		region["columns"], _ = json.Marshal(bindings)
		changed = true
	}
	if !changed {
		return raw, nil
	}
	proposal["regions"], _ = json.Marshal(regions)
	action["proposal"], _ = json.Marshal(proposal)
	return json.Marshal(action)
}
