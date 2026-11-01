package ai

import (
	"context"
	"encoding/json"
	"keel.local/dev/internal/bom"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEngineeringAgentUsesToolsAndRejectsApproval(t *testing.T) {
	if err := validateEngineeringSchema(context.Background(), []byte(`{"action":"inspect_workbook"}`)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"action":"approve"}`, `{"action":"inspect_workbook","database_write":true}`, `{"action":"read_range","sheet":"renamed","bounds":{"start_row":"1","end_row":2,"start_column":1,"end_column":2}}`} {
		if validateEngineeringSchema(context.Background(), []byte(raw)) == nil {
			t.Fatalf("accepted invalid tool request: %s", raw)
		}
	}
	responses := []string{`{"action":"inspect_workbook"}`, `{"action":"read_range","sheet":"renamed","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":2}}`, `{"action":"propose_engineering_model","proposal":{"summary":"Synthetic equipment evidence","regions":[{"sheet":"renamed","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":2},"kind":"equipment","semantic":"equipment inventory","header_rows":[1],"columns":[{"field":"label","column":1,"unit":""},{"field":"equipment_type","column":2,"unit":""}],"constants":[]}],"modules":[{"module_id":"equipment","applicable":true,"reason":"inventory evidence","sheets":["renamed"]}],"questions":[]}}`}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls >= len(responses) {
			t.Error("unbounded tool loop")
			http.Error(w, "excess", 500)
			return
		}
		content := responses[calls]
		calls++
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Model: "synthetic-protocol", Key: "synthetic-not-secret", TokenBudget: 2048, Timeout: time.Second}
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Sheet: "renamed", Ref: "A1", Row: 1, Column: 1, Value: "Label"}, {Sheet: "renamed", Ref: "B1", Row: 1, Column: 2, Value: "Type"}, {Sheet: "renamed", Ref: "A2", Row: 2, Column: 1, Value: "[LABEL]"}, {Sheet: "renamed", Ref: "B2", Row: 2, Column: 2, Value: "server"}}}}}
	steps := []string{}
	result, err := p.UnderstandEngineering(context.Background(), wb, func(tool, state, detail string) error { steps = append(steps, tool); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(steps) != 3 || len(result.Regions) != 1 {
		t.Fatalf("not a multiround proposal: calls=%d steps=%v", calls, steps)
	}
}
