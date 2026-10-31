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

func TestPhysicalPointAndQuantityBindingsAreRejected(t *testing.T) {
	for _, portColumn := range []int{1, 3} {
		s := bom.Sheet{Cells: []bom.Cell{{Row: 1, Column: 1, Value: "A point"}, {Row: 1, Column: 2, Value: "Z point"}, {Row: 1, Column: 3, Value: "数量"}}}
		p := SheetPlan{HeaderRows: []int{1}, Tables: []TablePlan{{Kind: "physical_connection", Type: "cable", Start: 2, End: 2, Fields: map[string]int{"endpoint_a_device": 1, "endpoint_b_device": 2, "endpoint_a_port": portColumn, "endpoint_b_port": portColumn}}}}
		if CheckExtraction(s, p) == nil {
			t.Fatal("point or count promoted to a physical port")
		}
	}
}
func TestKnownColumnSemanticsCannotBecomeNetworkOrCableIdentity(t *testing.T) {
	s, p := syntheticCablePlan()
	s.Cells = append(s.Cells, bom.Cell{Row: 1, Column: 7, Value: "A Side Lable Name"}, bom.Cell{Row: 2, Column: 7, Value: "synthetic cable label"}, bom.Cell{Row: 1, Column: 8, Value: "Rack No"})
	p.Tables[0].Fields["endpoint_a_device"] = 7
	p.Tables[0].Fields["vlan"] = 8
	normalizeSourceBindings(s, &p)
	if p.Tables[0].Fields["endpoint_a_device"] != 1 || p.Tables[0].Fields["vlan"] != 0 {
		t.Fatal("semantic source conflict survived normalization")
	}
	if err := CheckExtraction(s, p); err != nil {
		t.Fatal(err)
	}
	p.Tables[0].Kind = "component"
	if CheckExtraction(s, p) == nil {
		t.Fatal("optic recipe emitted cable")
	}
}
func TestExplicitInventoryDoesNotDependOnAIHeaderChoice(t *testing.T) {
	s := bom.Sheet{Name: "synthetic", Cells: []bom.Cell{
		{Row: 1, Column: 1, Value: "设备名称"}, {Row: 1, Column: 2, Value: "设备型号"}, {Row: 1, Column: 3, Value: "设备U数"}, {Row: 1, Column: 4, Value: "设备角色"},
		{Row: 2, Column: 1, Value: "synthetic-switch"}, {Row: 2, Column: 2, Value: "synthetic-model"}, {Row: 2, Column: 3, Value: "2"}, {Row: 2, Column: 4, Value: "leaf"},
	}}
	p, ok := explicitInventoryPlan(s)
	if !ok || len(p.Tables) != 1 || p.Tables[0].Fields["u_height"] != 3 || p.Tables[0].Fields["network_role"] != 4 {
		t.Fatal("explicit inventory not preserved")
	}
	if p.Tables[0].Type != "other" {
		t.Fatal("inventory category guessed without classification")
	}
	s.Cells = append(s.Cells, bom.Cell{Row: 1, Column: 5, Value: "设备名称"})
	if _, ok := explicitInventoryPlan(s); ok {
		t.Fatal("two-sided connection mistaken for inventory")
	}
}
func TestClassificationMapsLocalBatchIDsBackToSourceGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Messages []struct{ Content string } }
		json.NewDecoder(r.Body).Decode(&req)
		var input struct{ Items []Description }
		json.Unmarshal([]byte(req.Messages[1].Content), &input)
		if len(input.Items) != 2 || input.Items[0].ID != 0 || input.Items[1].ID != 1 {
			t.Error("nonlocal IDs sent")
		}
		raw := `{"items":[{"id":1,"entity_type":"switch","confidence":0.8,"reason":"synthetic"},{"id":0,"entity_type":"server","confidence":0.8,"reason":"synthetic"}]}`
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": raw}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	out, err := p.Classify(context.Background(), []Description{{ID: 29, Model: "synthetic-a"}, {ID: 81, Model: "synthetic-b"}}, PlanningContext{})
	if err != nil || out[0].ID != 81 || out[1].ID != 29 {
		t.Fatal("classification lost source group identity", err)
	}
}

func TestIncompleteGlobalSummaryDoesNotExcludeRemainingSheets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := `{"summary":"synthetic context","sources":[{"sheet":"first","role":"inventory","description":"synthetic"}],"identifiers":[],"questions":[]}`
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": raw}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Timeout: time.Second}
	result, err := p.UnderstandPlanning(context.Background(), nil, bom.Workbook{Sheets: []bom.Sheet{{Name: "first"}, {Name: "second"}}})
	if err != nil || len(result.Sources) != 2 || result.Sources[1].Sheet != "second" || result.Sources[1].Role != "unknown" || len(result.Questions) != 1 {
		t.Fatal("omitted global summary entry aborted extraction or invented a disposition", err)
	}
}

func TestTwoSidedGenericHeadersAreConnectionsNotInventory(t *testing.T) {
	s := bom.Sheet{Name: "synthetic-dual"}
	for row, values := range [][]string{{"设备名称", "端口号", "模块型号", "设备名称", "端口号", "模块型号"}, {"synthetic-a", "eth1", "synthetic-optic-a", "synthetic-b", "eth2", "synthetic-optic-b"}} {
		for col, v := range values {
			s.Cells = append(s.Cells, bom.Cell{Row: row + 1, Column: col + 1, Value: v})
		}
	}
	p := SheetPlan{HeaderRows: []int{1}, Tables: []TablePlan{{Kind: "physical_connection", Type: "cable", Start: 2, End: 2, Fields: map[string]int{"endpoint_a_device": 1, "endpoint_a_port": 2, "endpoint_a_optic": 3, "endpoint_b_device": 4, "endpoint_b_port": 5, "endpoint_b_optic": 6}}}}
	if err := CheckExtraction(s, p); err != nil {
		t.Fatal("two-sided source mistaken for a single inventory", err)
	}
	p.Tables[0].Fields["endpoint_b_device"] = 1
	if CheckExtraction(s, p) == nil {
		t.Fatal("second device identity silently dropped")
	}
}
func TestPointSummaryWithoutPortHeadersMayRemainReference(t *testing.T) {
	s := bom.Sheet{Cells: []bom.Cell{{Row: 1, Column: 1, Value: "Aside dev"}, {Row: 1, Column: 2, Value: "Bside dev"}, {Row: 1, Column: 3, Value: "Cable Model"}, {Row: 2, Column: 1, Value: "synthetic-a"}, {Row: 2, Column: 2, Value: "synthetic-b"}, {Row: 2, Column: 3, Value: "synthetic-model"}}}
	p := SheetPlan{HeaderRows: []int{1}, Purpose: "reference", Tables: []TablePlan{{Kind: "reference", Type: "other", Start: 2, End: 2, Fields: map[string]int{}}}}
	if err := CheckExtraction(s, p); err != nil {
		t.Fatal("summary forced to invent a physical cable", err)
	}
}
