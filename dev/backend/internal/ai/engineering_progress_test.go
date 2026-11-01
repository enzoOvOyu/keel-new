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
		if len(toolList) != 14 {
			t.Errorf("native tools missing: %d", len(toolList))
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
		if len(cp.CompletedSheets) != 2 || len(cp.Proposal.Regions) != 2 {
			t.Errorf("lost prior checkpoint: %+v", cp)
		}
		return nil
	}, Artifact: func(context.Context, string, string, any) error { artifacts++; return nil }})
	if err != nil || calls != 2 || saved != 1 || artifacts != 2 || len(result.Regions) != 2 {
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
