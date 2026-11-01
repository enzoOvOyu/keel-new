package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestEngineeringColumnMapNormalizesWithoutDroppingFields(t *testing.T) {
	raw := []byte(`{"action":"propose_engineering_model","sheet":"synthetic","proposal":{"summary":"mapped source","regions":[{"sheet":"synthetic","kind":"connection","semantic":"endpoints","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":8},"header_rows":[1],"columns":{"a_device":{"column":7},"length":{"column":2,"unit":"m"}}}]}}`)
	normalized, err := normalizeEngineeringColumns(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateEngineeringSchema(context.Background(), normalized); err != nil {
		t.Fatal(err)
	}
	var action engineeringAction
	_ = json.Unmarshal(normalized, &action)
	cols := action.Proposal.Regions[0].Columns
	if len(cols) != 2 || cols[0].Field != "a_device" || cols[0].Column != 7 || cols[1].Unit != "m" {
		t.Fatalf("binding changed: %+v", cols)
	}
	legacy, err := normalizeEngineeringColumns(normalized)
	if err != nil || string(legacy) != string(normalized) {
		t.Fatal("legacy canonical arrays changed")
	}
	duplicate := strings.Replace(string(raw), `"length":{"column":2,"unit":"m"}`, `"a_device":{"column":2}`, 1)
	if _, err := normalizeEngineeringColumns([]byte(duplicate)); err == nil || !strings.HasPrefix(err.Error(), "DUPLICATE_FIELD_BINDING") {
		t.Fatal("duplicate JSON property silently overwrote a source binding")
	}
	feedback := []any{map[string]any{"rejected_action": json.RawMessage(duplicate), "error": "DUPLICATE_FIELD_BINDING"}}
	if len(engineeringSheetHistory(feedback, "synthetic")) != 1 {
		t.Fatal("invalid native binding lost its corrective feedback")
	}
	unknown := strings.Replace(string(raw), `"column":7`, `"column":7,"invented":true`, 1)
	converted, err := normalizeEngineeringColumns([]byte(unknown))
	if err != nil {
		t.Fatal(err)
	}
	if validateEngineeringSchema(context.Background(), converted) == nil {
		t.Fatal("normalization dropped an unsupported property")
	}
}
