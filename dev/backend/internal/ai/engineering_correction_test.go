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
	raw, corrected, err := materializeEngineeringCorrection([]byte(`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"set_columns":[{"field":"label","column":3}]}`), draft, "synthetic")
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

func TestEngineeringCorrectionCoversTitleWithoutCreatingFakeEquipment(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Equipment project title"}, {Ref: "A2", Row: 2, Column: 1, Value: "Name"}, {Ref: "B2", Row: 2, Column: 2, Value: "Quantity"}, {Ref: "A3", Row: 3, Column: 1, Value: "TEST-DEVICE"}, {Ref: "B3", Row: 3, Column: 2, Value: "1"}}}}}
	draft := engineering.EngineeringProposal{Summary: "source", Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "equipment", Semantic: "inventory", Bounds: engineering.Bounds{StartRow: 2, EndRow: 3, StartColumn: 1, EndColumn: 2}, HeaderRows: []int{2}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 1}, {Field: "quantity", Column: 2}}}}}
	issue := engineering.CheckSheetCoverage(wb, draft, "synthetic")
	if issue == nil {
		t.Fatal("missing title was not retained")
	}
	tool := engineeringCorrectionTool(draft, "synthetic", nil, issue.Error()).(map[string]any)["function"].(map[string]any)
	props := tool["parameters"].(map[string]any)["properties"].(map[string]any)
	if props["set_columns"] != nil || props["bounds"] == nil || props["header_rows"] == nil {
		t.Fatal("coverage correction still exposes unrelated fields")
	}
	raw, corrected, err := materializeEngineeringCorrection([]byte(`{"action":"map_semantic_fields","sheet":"synthetic","region_index":0,"bounds":{"start_row":1,"end_row":3,"start_column":1,"end_column":2},"header_rows":[1,2]}`), &draft, "synthetic")
	if err != nil || !corrected {
		t.Fatal(err)
	}
	if err = validateEngineeringSchema(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	var action engineeringAction
	_ = json.Unmarshal(raw, &action)
	if err = engineering.CheckSheetCoverage(wb, *action.Proposal, "synthetic"); err != nil {
		t.Fatal(err)
	}
	result, err := engineering.ExtractPhysical(wb, *action.Proposal, "project", "source", "revision")
	if err != nil || len(result.Model.Equipment) != 1 || result.Model.Equipment[0].Label != "TEST-DEVICE" {
		t.Fatalf("title became an asset: %+v %v", result.Model.Equipment, err)
	}
}

func TestEngineeringCorrectsDomainBeforeAddingMissingFields(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls++
		args := map[string]any{"sheet": "synthetic", "region_index": 0}
		if calls == 1 {
			var declared []map[string]any
			_ = json.Unmarshal(body["tools"], &declared)
			props := declared[0]["function"].(map[string]any)["parameters"].(map[string]any)["properties"].(map[string]any)
			if props["set_kind"] == nil || props["set_columns"] != nil {
				t.Error("domain error still invites edits to correctly mapped columns")
			}
			args["set_kind"] = "power"
		} else {
			args["set_columns"] = []any{map[string]any{"field": "power", "column": 2, "unit": "kW"}, map[string]any{"field": "power_capacity", "column": 3, "unit": "kW"}}
			args["remove_columns"] = []string{}
		}
		if calls > 2 {
			t.Error("domain correction did not resolve its actual cause")
		}
		encoded, _ := json.Marshal(args)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]any{"name": "map_semantic_fields", "arguments": string(encoded)}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer server.Close()
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Rack"}, {Ref: "B1", Row: 1, Column: 2, Value: "Max Load(kW)"}, {Ref: "C1", Row: 1, Column: 3, Value: "Rack Power Capacity(kW)"}, {Ref: "A2", Row: 2, Column: 1, Value: "TEST-RACK"}, {Ref: "B2", Row: 2, Column: 2, Value: "12"}, {Ref: "C2", Row: 2, Column: 3, Value: "24"}}}}}
	cp := EngineeringCheckpoint{Version: engineeringCheckpointVersion, InspectedSheets: []string{"synthetic"}, CompletedSheets: []string{"synthetic"}, Proposal: engineering.EngineeringProposal{Summary: "source", Regions: []engineering.RegionMapping{{Sheet: "synthetic", Kind: "placement", Semantic: "rack power", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 3}, HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "rack", Column: 1}}}}}}
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	result, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil }, EngineeringToolHooks{Resume: &cp})
	if err != nil || calls != 2 || result.Regions[0].Kind != "power" || len(result.Regions[0].Columns) != 3 {
		t.Fatalf("domain correction failed: %v calls=%d", err, calls)
	}
}

func TestEngineeringRetainsOtherRejectedDraftsAcrossSheetSaveAndRestart(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		var input map[string]any
		_ = json.Unmarshal([]byte(body.Messages[1].Content), &input)
		calls++
		expected := "alpha"
		if calls == 2 {
			expected = "beta"
		}
		if input["active_sheet"] != expected || input["draft_to_correct"] == nil || len(body.Tools) != 1 || body.Tools[0].Function.Name != "map_semantic_fields" {
			t.Errorf("pending draft lost or mixed across sheets: active=%v calls=%d", input["active_sheet"], calls)
		}
		args, _ := json.Marshal(map[string]any{"sheet": expected, "region_index": 0, "set_columns": []any{}, "remove_columns": []string{"quantity"}})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]any{"name": "map_semantic_fields", "arguments": string(args)}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer server.Close()
	wb := bom.Workbook{}
	prior := EngineeringCheckpoint{Version: engineeringCheckpointVersion, CompletedSheets: []string{"alpha", "beta"}, InspectedSheets: []string{"alpha", "beta"}, Proposal: engineering.EngineeringProposal{Summary: "source"}}
	for _, sheet := range []string{"alpha", "beta"} {
		wb.Sheets = append(wb.Sheets, bom.Sheet{Name: sheet, Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "No."}, {Ref: "B1", Row: 1, Column: 2, Value: "Device Name"}, {Ref: "A2", Row: 2, Column: 1, Value: "1"}, {Ref: "B2", Row: 2, Column: 2, Value: "TEST-" + sheet}}})
		prior.Proposal.Regions = append(prior.Proposal.Regions, engineering.RegionMapping{Sheet: sheet, Kind: "equipment", Semantic: "inventory", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 2}, HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 2}, {Field: "quantity", Column: 1}}})
	}
	var saved EngineeringCheckpoint
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	_, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil }, EngineeringToolHooks{Resume: &prior, Checkpoint: func(_ context.Context, cp EngineeringCheckpoint) error {
		if len(cp.CompletedSheets) == 1 {
			raw, _ := json.Marshal(cp)
			_ = json.Unmarshal(raw, &saved)
			return context.Canceled
		}
		return nil
	}})
	if err != context.Canceled || len(saved.PendingDrafts) != 1 || len(saved.PendingDrafts["beta"].Regions) != 1 {
		t.Fatalf("unprocessed sheet draft not durable: %v %+v", err, saved.PendingDrafts)
	}
	restarted := Provider{Endpoint: server.URL, Timeout: time.Second}
	result, err := restarted.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil }, EngineeringToolHooks{Resume: &saved})
	if err != nil || calls != 2 || len(result.Regions) != 2 {
		t.Fatalf("resume regenerated already-inspected sheets: %v calls=%d", err, calls)
	}
	for _, region := range result.Regions {
		if len(region.Columns) != 1 || region.Columns[0].Field != "label" || region.Columns[0].Column != 2 {
			t.Fatal("unrelated valid identity binding changed")
		}
	}
}
