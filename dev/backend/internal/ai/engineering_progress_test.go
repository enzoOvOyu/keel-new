package ai

import (
	"context"
	"encoding/json"
	"errors"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEngineeringNativeToolsResumeSheetCheckpoint(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{}}
	for _, name := range []string{"previous", "renamed inventory"} {
		wb.Sheets = append(wb.Sheets, bom.Sheet{Name: name, Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Label"}, {Ref: "A2", Row: 2, Column: 1, Value: "synthetic"}}})
	}
	makeProposal := func(sheet string) engineering.EngineeringProposal {
		return engineering.EngineeringProposal{Summary: "Evidence-based inventory", Regions: []engineering.RegionMapping{{Sheet: sheet, Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 1}, Kind: "equipment", Semantic: "inventory", HeaderRows: []int{1}, Columns: []engineering.ColumnBinding{{Field: "label", Column: 1}}, Constants: []engineering.ConstantBinding{}}}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	}
	prior := EngineeringCheckpoint{Version: engineeringCheckpointVersion, Proposal: makeProposal("previous"), CompletedSheets: []string{"previous"}}
	calls, saved, artifacts := 0, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		var toolList []any
		_ = json.Unmarshal(body["tools"], &toolList)
		if calls == 0 && len(toolList) != 1 {
			t.Errorf("initial source inspection must have one focused tool, got %d", len(toolList))
		}
		if calls == 1 && len(toolList) != 1 {
			t.Errorf("a fully inspected two-cell sheet must progress to a proposal, got %d tools", len(toolList))
		}
		for _, tool := range toolList {
			function := tool.(map[string]any)["function"].(map[string]any)
			if function["name"] == "propose_engineering_model" {
				fields := function["parameters"].(map[string]any)["$defs"].(map[string]any)["Region"].(map[string]any)["properties"].(map[string]any)["columns"].(map[string]any)["propertyNames"].(map[string]any)["enum"].([]any)
				found := false
				for _, field := range fields {
					if field == "cable_model" {
						found = true
					}
					if field == "cable_type" {
						t.Error("unsupported cable_type advertised")
					}
				}
				if !found {
					t.Error("canonical cable_model missing from native tool contract")
				}
			}
		}
		if calls > 0 {
			var messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}
			_ = json.Unmarshal(body["messages"], &messages)
			if len(messages) < 4 || messages[len(messages)-1].Role != "tool" {
				t.Error("tool output not carried in native tool messages")
			}
		}
		if _, ok := body["response_format"]; ok {
			t.Error("JSON mode conflicts with native tools")
		}
		name := "inspect_sheet"
		args := map[string]any{"sheet": "renamed inventory"}
		if calls > 0 {
			name = "propose_engineering_model"
			args["proposal"] = makeProposal("renamed inventory")
		}
		if calls > 1 {
			t.Error("completed sheet replayed")
		}
		calls++
		raw, _ := json.Marshal(args)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]any{"name": name, "arguments": string(raw)}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second, TokenBudget: 2048, JSONMode: true}
	result, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil }, EngineeringToolHooks{Resume: &prior, Checkpoint: func(_ context.Context, cp EngineeringCheckpoint) error {
		saved++
		if saved == 2 && (len(cp.CompletedSheets) != 2 || len(cp.Proposal.Regions) != 2) {
			t.Errorf("lost prior checkpoint: %+v", cp)
		}
		return nil
	}, Artifact: func(context.Context, string, string, any) error { artifacts++; return nil }})
	if err != nil || calls != 2 || saved != 2 || artifacts != 2 || len(result.Regions) != 2 {
		t.Fatalf("progress failure: %v calls=%d saved=%d artifacts=%d", err, calls, saved, artifacts)
	}
}

func TestProviderErrorPreservesSafeTransientReason(t *testing.T) {
	for _, tc := range []struct {
		body      string
		temporary bool
	}{{`{"error":{"message":"secret upstream text","metadata":{"reason":"in_flight_budget_exhausted","headers":{"Retry-After":"120"}}}}`, true}, {`{"error":{"message":"insufficient credits"}}`, false}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(402); _, _ = w.Write([]byte(tc.body)) }))
		p := Provider{Endpoint: server.URL, Timeout: time.Second}
		_, err := p.completeJSON(context.Background(), "synthetic", map[string]string{})
		var upstream *ProviderError
		if !errors.As(err, &upstream) || upstream.Status != 402 || upstream.Temporary() != tc.temporary {
			t.Fatalf("incorrect error classification: %v", err)
		}
		if tc.temporary && upstream.RetryAfter != 120*time.Second {
			t.Fatal("lost retry-after")
		}
		if err.Error() == tc.body {
			t.Fatal("raw upstream error exposed")
		}
		server.Close()
	}
}

func TestEngineeringRestoresUnfinishedSheetToolHistory(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "arbitrary", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Label"}, {Ref: "A2", Row: 2, Column: 1, Value: "synthetic device"}}}}}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if calls == 2 {
			w.WriteHeader(401)
			return
		}
		content := `{"action":"inspect_sheet","sheet":"arbitrary"}`
		if calls == 3 {
			if len(body.Messages) < 4 || body.Messages[len(body.Messages)-1].Role != "tool" {
				t.Error("inspection result lost after restart")
			}
			content = `{"action":"propose_engineering_model","sheet":"arbitrary","proposal":{"summary":"source inventory","regions":[{"sheet":"arbitrary","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":1},"kind":"equipment","semantic":"inventory","header_rows":[1],"columns":[{"field":"label","column":1,"unit":""}],"constants":[]}],"modules":[],"questions":[]}}`
		}
		if calls > 3 {
			t.Error("unfinished sheet inspection repeated")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	var saved *EngineeringCheckpoint
	hook := func(_ context.Context, cp EngineeringCheckpoint) error {
		raw, _ := json.Marshal(cp)
		saved = new(EngineeringCheckpoint)
		return json.Unmarshal(raw, saved)
	}
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	trace := func(string, string, string) error { return nil }
	if _, err := p.UnderstandEngineering(context.Background(), wb, trace, EngineeringToolHooks{Checkpoint: hook}); err == nil {
		t.Fatal("provider failure falsely succeeded")
	}
	if saved == nil || len(saved.History) != 1 || len(saved.InspectedSheets) != 1 || len(saved.CompletedSheets) != 0 {
		t.Fatal("unfinished tool state not persisted")
	}
	resumed := Provider{Endpoint: server.URL, Timeout: time.Second}
	result, err := resumed.UnderstandEngineering(context.Background(), wb, trace, EngineeringToolHooks{Resume: saved, Checkpoint: hook})
	if err != nil || len(result.Regions) != 1 || calls != 3 {
		t.Fatalf("resume failed: %v calls=%d", err, calls)
	}
}

func TestProviderInterruptedBodyRemainsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(`{"choices":`))
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	_, err := p.completeJSON(context.Background(), "synthetic", map[string]string{})
	var upstream *ProviderError
	if !errors.As(err, &upstream) || upstream.Status != 200 || upstream.Reason != "response_interrupted" || !upstream.Temporary() {
		t.Fatalf("interrupted provider response not recoverable: %v", err)
	}
}

func TestEngineeringCorrectsMalformedToolOutputWithoutAcceptingIt(t *testing.T) {
	calls, archived := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		args := `{"sheet":`
		if calls == 2 {
			args = `{"sheet":"synthetic"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]any{"name": "inspect_sheet", "arguments": args}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	raw, err := p.engineeringCompletion(context.Background(), "synthetic", map[string]any{}, []any{map[string]any{"type": "function", "function": map[string]any{"name": "inspect_sheet"}}}, func(string, string, string) error { return nil }, func(_ context.Context, tool, state string, v any) error {
		archived++
		if tool != "provider_output" || state != "rejected" || v.(map[string]any)["response"] == "" {
			t.Error("invalid model response was not preserved")
		}
		return nil
	})
	if err != nil || calls != 2 || archived != 1 || !json.Valid(raw) {
		t.Fatalf("correction failed: %s %v calls=%d archived=%d", raw, err, calls, archived)
	}
}

func TestEngineeringRepeatedBadProposalStopsAndDoesNotPolluteContext(t *testing.T) {
	calls := 0
	bad := `{"action":"propose_engineering_model","sheet":"synthetic","proposal":{"summary":"bad mapping","regions":[{"sheet":"synthetic","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":1},"kind":"power","semantic":"synthetic power","header_rows":[1],"columns":[{"field":"cable_label","column":1}]}]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if calls > 1 {
			var input map[string]any
			_ = json.Unmarshal([]byte(body.Messages[1].Content), &input)
			if input["tool_errors"] == nil {
				t.Error("missing correction feedback")
			}
			feedback := input["tool_errors"].([]any)
			last := feedback[len(feedback)-1].(map[string]any)
			bindings, ok := last["binding_errors"].([]any)
			if !ok || len(bindings) != 1 || bindings[0].(map[string]any)["field"] != "cable_label" || last["next"] == nil {
				t.Error("specific binding correction lost before model request")
			}
			assistantCalls := 0
			for _, m := range body.Messages {
				if m.Role == "assistant" {
					assistantCalls++
				}
			}
			if assistantCalls != 1 {
				t.Error("rejected proposal reintroduced as an assistant example")
			}
		}
		content := bad
		if calls == 0 {
			content = `{"action":"inspect_sheet","sheet":"synthetic"}`
		}
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Power"}, {Ref: "A2", Row: 2, Column: 1, Value: "2"}}}}}
	_, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil })
	if err == nil || err.Error() != "ENGINEERING_AGENT_NO_PROGRESS" || calls != 4 {
		t.Fatalf("unbounded errors: %v calls=%d", err, calls)
	}
}

func TestEngineeringBindingErrorsIncludeAllUnsupportedAndDuplicateColumns(t *testing.T) {
	proposal := engineering.EngineeringProposal{Regions: []engineering.RegionMapping{{Sheet: "renamed", Kind: "connection", Columns: []engineering.ColumnBinding{
		{Field: "a_device", Column: 5}, {Field: "fabric", Column: 1}, {Field: "label", Column: 2}, {Field: "label", Column: 3}, {Field: "a_device", Column: 4},
	}}}}
	issues := engineeringBindingErrors(proposal, nil)
	if len(issues) != 4 || issues[0].(map[string]any)["column"] != 1 || issues[3].(map[string]any)["column"] != 4 {
		t.Fatalf("incomplete correction feedback: %+v", issues)
	}
	if len(proposal.Regions[0].Columns) != 5 {
		t.Fatal("feedback silently repaired the proposal")
	}
	history := []any{map[string]any{"action": engineeringAction{Action: "propose_engineering_model", Sheet: "renamed", Proposal: &proposal}, "result": map[string]string{"error": "FIELD_NOT_SUPPORTED_FOR_REGION: fabric"}}}
	if !engineeringProposalDue(bom.Sheet{Name: "renamed"}, history) {
		t.Fatal("schema correction allowed a detour into unrelated source reads")
	}
}

func TestEngineeringRangePagesWithoutLosingRequestedExtent(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Ref: "B100", Row: 100, Column: 2, Value: "sample"}, {Ref: "B200", Row: 200, Column: 2, Value: "later"}}}}}
	bounds := engineering.Bounds{StartRow: 100, EndRow: 200, StartColumn: 2, EndColumn: 2}
	result, err := engineeringReadRange(wb, "renamed", bounds)
	if err != nil {
		t.Fatal(err)
	}
	page := result.(map[string]any)
	if page["requested_bounds"] != bounds || page["returned_bounds"].(engineering.Bounds).EndRow != 131 || page["truncated"] != true || len(page["cells"].([]map[string]any)) != 1 {
		t.Fatalf("incorrect page: %+v", page)
	}
	history := []any{map[string]any{"action": engineeringAction{Action: "read_range", Sheet: "renamed", Bounds: &bounds}, "result": result}}
	if engineeringProposalDue(wb.Sheets[0], history) {
		t.Fatal("partial sample mistaken for complete read")
	}
	bounds.EndRow = 32
	if _, err := engineeringReadRange(wb, "renamed", bounds); err == nil {
		t.Fatal("reversed coordinates accepted")
	}
}

func TestEngineeringFormulaReadWindowRequiresInterpretation(t *testing.T) {
	history := []any{}
	for i := 0; i < 6; i++ {
		history = append(history, map[string]any{"action": engineeringAction{Action: "inspect_formula", Sheet: "renamed", Cell: "B4"}, "result": map[string]any{"formula": "1+1"}})
	}
	if !engineeringProposalDue(bom.Sheet{Name: "renamed"}, history) {
		t.Fatal("formula reads never progress to interpretation")
	}
	if engineeringProposalDue(bom.Sheet{Name: "different"}, history) {
		t.Fatal("another sheet's reads forced interpretation")
	}
}

func TestEngineeringHistoryKeepsOnlyActiveSheetEvidence(t *testing.T) {
	history := []any{
		map[string]any{"action": engineeringAction{Action: "inspect_sheet", Sheet: "completed"}},
		map[string]any{"action": engineeringAction{Action: "inspect_formula", Sheet: "active", Cell: "B4"}},
		map[string]any{"rejected_action": engineeringAction{Action: "read_range", Sheet: "active"}, "error": "INVALID_RANGE"},
	}
	filtered := engineeringSheetHistory(history, "active")
	if len(filtered) != 2 {
		t.Fatalf("lost active errors or retained unrelated context: %+v", filtered)
	}
}

func TestEngineeringMissingFormulaProvidesActualReferences(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		content := `{"action":"inspect_sheet","sheet":"synthetic"}`
		switch calls {
		case 1:
			content = `{"action":"inspect_formula","sheet":"synthetic","cell":"B2"}`
		case 2:
			var input map[string]any
			_ = json.Unmarshal([]byte(body.Messages[1].Content), &input)
			feedback := input["tool_errors"].([]any)[0].(map[string]any)
			refs := feedback["available_formula_cells"].([]any)
			if len(refs) != 1 || refs[0] != "B4" {
				t.Errorf("formula source references missing: %+v", refs)
			}
			content = `{"action":"inspect_formula","sheet":"synthetic","cell":"B4"}`
		default:
			if calls > 2 {
				content = `{"action":"propose_engineering_model","sheet":"synthetic","proposal":{"summary":"unmapped calculation retained","regions":[{"sheet":"synthetic","bounds":{"start_row":1,"end_row":4,"start_column":1,"end_column":2},"kind":"other","semantic":"calculation evidence, domain unknown","header_rows":[],"columns":[]}]}}`
			}
		}
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Calculation"}, {Ref: "B4", Row: 4, Column: 2, Value: "2", Formula: "1+1"}}}}}
	_, err := p.UnderstandEngineering(context.Background(), wb, func(string, string, string) error { return nil })
	if err != nil || calls != 4 {
		t.Fatalf("formula recovery failed: %v calls=%d", err, calls)
	}
}

func TestEngineeringBatchFailureReturnsFeedbackBeforeOtherCalls(t *testing.T) {
	calls, rejected := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := `{"action":"inspect_sheet","sheet":"synthetic"}`
		if calls == 2 {
			content = `[{"action":"inspect_formula","sheet":"synthetic","cell":"B2"},{"action":"inspect_formula","sheet":"synthetic","cell":"C2"},{"action":"inspect_formula","sheet":"synthetic","cell":"D2"}]`
		}
		if calls == 3 {
			var body struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			var input map[string]any
			_ = json.Unmarshal([]byte(body.Messages[1].Content), &input)
			if len(input["tool_errors"].([]any)) != 1 {
				t.Fatal("batch errors executed before model could correct")
			}
			content = `{"action":"propose_engineering_model","sheet":"synthetic","proposal":{"summary":"source retained","regions":[{"sheet":"synthetic","bounds":{"start_row":1,"end_row":1,"start_column":1,"end_column":1},"kind":"other","semantic":"unmapped source","header_rows":[],"columns":[]}]}}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "synthetic", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Unclassified evidence"}}}}}
	_, err := p.UnderstandEngineering(context.Background(), wb, func(_, state, _ string) error {
		if state == "rejected" {
			rejected++
		}
		return nil
	})
	if err != nil || calls != 3 || rejected != 1 {
		t.Fatalf("batch correction failed: %v calls=%d rejected=%d", err, calls, rejected)
	}
}

func TestSourceSpansHidePagingBoundariesButPreserveBlankSeparators(t *testing.T) {
	spans := engineeringSourceSpans([]bom.Region{{Sheet: "synthetic", Start: 4, End: 259, FirstColumn: 2, LastColumn: 8}, {Sheet: "synthetic", Start: 260, End: 500, FirstColumn: 3, LastColumn: 9}, {Sheet: "synthetic", Start: 502, End: 510, FirstColumn: 2, LastColumn: 8}})
	if len(spans) != 2 || spans[0].Start != 4 || spans[0].End != 500 || spans[0].Range != "B4:I500" || spans[1].Start != 502 {
		t.Fatalf("incorrect source spans: %+v", spans)
	}
}

func TestMappingToolsFollowAgentDomainWithoutRestrictingReclassification(t *testing.T) {
	raw, _ := prompts.ReadFile("prompts/engineering-tools.json")
	var functions []any
	_ = json.Unmarshal(raw, &functions)
	constrainEngineeringTools(functions, nil)
	history := []any{map[string]any{"action": engineeringAction{Action: "propose_engineering_model", Proposal: &engineering.EngineeringProposal{Regions: []engineering.RegionMapping{{Sheet: "renamed", Kind: "connection"}}}}}}
	kinds := engineeringProposedKinds(history, "renamed")
	scoped := scopeEngineeringProposalTools(engineeringSourceTools(functions, "renamed", true, true), functions, kinds, nil)
	foundClassification := false
	for _, tool := range scoped {
		f := tool.(map[string]any)["function"].(map[string]any)
		if f["name"] == "classify_information" {
			foundClassification = true
			continue
		}
		if f["name"] != "propose_engineering_model" {
			continue
		}
		defs := f["parameters"].(map[string]any)["$defs"].(map[string]any)
		fields := defs["mapping_Region"].(map[string]any)["properties"].(map[string]any)["columns"].(map[string]any)["propertyNames"].(map[string]any)["enum"].([]string)
		hasEndpoint := false
		for _, field := range fields {
			if field == "quantity" {
				t.Error("quantity leaked into connection mapping")
			}
			if field == "a_device" {
				hasEndpoint = true
			}
		}
		if !hasEndpoint {
			t.Error("endpoint field missing")
		}
	}
	if !foundClassification {
		t.Error("domain interpretation cannot be revised")
	}
	definitions := map[string]string{}
	for _, tool := range scoped {
		defs := tool.(map[string]any)["function"].(map[string]any)["parameters"].(map[string]any)["$defs"].(map[string]any)
		for key, value := range defs {
			raw, _ := json.Marshal(value)
			if previous, ok := definitions[key]; ok && previous != string(raw) {
				t.Errorf("conflicting provider definition: %s", key)
			}
			definitions[key] = string(raw)
		}
	}
}
