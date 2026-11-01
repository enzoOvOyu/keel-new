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
	prior := EngineeringCheckpoint{Proposal: makeProposal("previous"), CompletedSheets: []string{"previous"}}
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
				fields := function["parameters"].(map[string]any)["$defs"].(map[string]any)["Column"].(map[string]any)["properties"].(map[string]any)["field"].(map[string]any)["enum"].([]any)
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
