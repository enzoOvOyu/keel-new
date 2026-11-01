package ai

import (
	"context"
	"encoding/json"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEngineeringCorrectsOneBindingWithoutRegeneratingOtherFields(t *testing.T) {
	calls, originalPatch := 0, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls++
		name, args := "inspect_sheet", map[string]any{"sheet": "synthetic"}
		if calls == 2 {
			name = "propose_engineering_model"
			args["proposal"] = engineering.EngineeringProposal{Summary: "candidate", Modules: []engineering.ModuleDetection{}, Questions: []string{}, Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "equipment", Semantic: "inventory", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 2}, HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 2}, {Field: "quantity", Column: 1}}, Constants: []engineering.ConstantBinding{}}}}
		}
		if calls == 3 {
			var messages []struct {
				Content string `json:"content"`
			}
			_ = json.Unmarshal(body["messages"], &messages)
			var focus map[string]any
			if len(messages) != 2 || json.Unmarshal([]byte(messages[1].Content), &focus) != nil || focus["validation_error"] == nil || focus["draft_to_correct"] == nil || focus["workbook"] != nil {
				t.Errorf("correction lost the actual validation error or retained unrelated tasks: %s", body["messages"])
			}
			var tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			_ = json.Unmarshal(body["tools"], &tools)
			if len(tools) != 1 || tools[0].Function.Name != "map_semantic_fields" {
				t.Errorf("expected a focused correction tool: %+v", tools)
			}
			name = "map_semantic_fields"
			args = map[string]any{"sheet": "synthetic", "region_index": 0, "remove_columns": []string{"quantity"}}
		}
		if calls > 3 {
			t.Error("corrected sheet did not finish")
		}
		encoded, _ := json.Marshal(args)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]any{"name": name, "arguments": string(encoded)}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer server.Close()
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "No."}, {Ref: "B1", Row: 1, Column: 2, Value: "Name"}, {Ref: "A2", Row: 2, Column: 1, Value: "1"}, {Ref: "B2", Row: 2, Column: 2, Value: "TEST-DEVICE"}}}}}
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	result, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil }, EngineeringToolHooks{Artifact: func(_ context.Context, tool, state string, value any) error {
		if tool == "map_semantic_fields" && state == "succeeded" {
			request := value.(map[string]any)["request"].(json.RawMessage)
			var patch map[string]json.RawMessage
			_ = json.Unmarshal(request, &patch)
			originalPatch = patch["remove_columns"] != nil && patch["proposal"] == nil
		}
		return nil
	}})
	if err != nil || calls != 3 || !originalPatch {
		t.Fatalf("correction failed: %v calls=%d originalPatch=%v", err, calls, originalPatch)
	}
	cols := result.Regions[0].Columns
	if len(cols) != 1 || cols[0].Field != "label" || cols[0].Column != 2 {
		t.Fatalf("unrelated source binding changed: %+v", cols)
	}
}

func TestEngineeringCorrectionRejectsWrongSheetUnknownFieldsAndNoop(t *testing.T) {
	draft := &engineering.EngineeringProposal{Summary: "draft", Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "equipment", Columns: []engineering.ColumnBinding{{Field: "label", Column: 2}}}}}
	for _, raw := range []string{
		`{"action":"map_semantic_fields","sheet":"another","region_index":0,"remove_columns":["label"]}`,
		`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0}`,
		`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"set_columns":{"label":{"column":3,"invented":true}}}`,
		`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"set_columns":{"label":{"column":3},"label":{"column":4}}}`,
	} {
		if _, _, err := materializeEngineeringCorrection([]byte(raw), draft, "synthetic"); err == nil {
			t.Fatalf("invalid correction accepted: %s", raw)
		}
	}
	if draft.Regions[0].Columns[0].Column != 2 {
		t.Fatal("rejected correction mutated draft")
	}
}

func TestEngineeringCorrectionRestoresOmittedArrayDefaults(t *testing.T) {
	draft := &engineering.EngineeringProposal{Summary: "draft", Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "equipment", Semantic: "inventory", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 3}, HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 2}}}}}
	raw, corrected, err := materializeEngineeringCorrection([]byte(`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"set_columns":{"label":{"column":3}}}`), draft, "synthetic")
	if err != nil || !corrected {
		t.Fatalf("correction failed: %v", err)
	}
	if err := validateEngineeringSchema(context.Background(), raw); err != nil {
		t.Fatalf("omitted optional arrays became invalid nulls: %v", err)
	}
}

func TestEngineeringRevalidatesSavedCandidateWithTargetedCorrection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Tools) != 1 || body.Tools[0].Function.Name != "map_semantic_fields" {
			t.Errorf("restored candidate discarded instead of corrected: %+v", body.Tools)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"remove_columns":["quantity"]}`}}}})
	}))
	defer server.Close()
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "No."}, {Ref: "B1", Row: 1, Column: 2, Value: "Name"}, {Ref: "A2", Row: 2, Column: 1, Value: "1"}, {Ref: "B2", Row: 2, Column: 2, Value: "TEST"}}}}}
	checkpoint := EngineeringCheckpoint{Version: engineeringCheckpointVersion, CompletedSheets: []string{"synthetic"}, InspectedSheets: []string{"synthetic"}, Proposal: engineering.EngineeringProposal{Summary: "old candidate", Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "equipment", Semantic: "inventory", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 2}, HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 2}, {Field: "quantity", Column: 1}}}}}}
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	result, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil }, EngineeringToolHooks{Resume: &checkpoint})
	if err != nil || p.Calls != 1 || len(result.Regions[0].Columns) != 1 || result.Regions[0].Columns[0].Column != 2 {
		t.Fatalf("resume correction failed: %v calls=%d", err, p.Calls)
	}
}

func TestEngineeringCorrectionReclassifiesRegionWithoutLosingSourceBindings(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Rack"}, {Ref: "B1", Row: 1, Column: 2, Value: "Power"}, {Ref: "A2", Row: 2, Column: 1, Value: "TEST-RACK"}, {Ref: "B2", Row: 2, Column: 2, Value: "12"}}}}}
	draft := &engineering.EngineeringProposal{Summary: "source", Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "placement", Semantic: "rack power", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 2}, HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "rack", Column: 1}, {Field: "power", Column: 2, Unit: "kW"}}}}}
	if engineering.CheckProposal(wb, *draft) == nil {
		t.Fatal("wrong domain accepted")
	}
	raw, corrected, err := materializeEngineeringCorrection([]byte(`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"set_kind":"power","set_columns":{},"remove_columns":[]}`), draft, "synthetic")
	if err != nil || !corrected {
		t.Fatalf("classification correction: %v", err)
	}
	if err = validateEngineeringSchema(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	var action engineeringAction
	_ = json.Unmarshal(raw, &action)
	if err = engineering.CheckProposal(wb, *action.Proposal); err != nil {
		t.Fatal(err)
	}
	if action.Proposal.Regions[0].Kind != "power" || len(action.Proposal.Regions[0].Columns) != 2 || action.Proposal.Regions[0].Columns[1].Unit != "kW" || draft.Regions[0].Kind != "placement" {
		t.Fatal("source bindings lost or original draft mutated")
	}
}
