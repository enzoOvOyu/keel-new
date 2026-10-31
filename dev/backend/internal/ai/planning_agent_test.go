package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keel.local/dev/internal/bom"
)

func syntheticCablePlan() (bom.Sheet, SheetPlan) {
	s := bom.Sheet{Name: "synthetic-cables"}
	for row, values := range [][]string{
		{"endpoint_a_device", "endpoint_a_port", "endpoint_b_device", "endpoint_b_port", "Model", "Cable Type"},
		{"synthetic-a", "eth0", "synthetic-b", "Ethernet1/1", "synthetic-wire", "100G"},
		{"synthetic-a", "eth1", "synthetic-b", "Ethernet1/2", "synthetic-wire", "400G"},
	} {
		for col, v := range values {
			s.Cells = append(s.Cells, bom.Cell{Sheet: s.Name, Row: row + 1, Column: col + 1, Ref: fmt.Sprintf("%c%d", 'A'+col, row+1), Value: v})
		}
	}
	p := SheetPlan{Purpose: "table", HeaderRows: []int{1}, Tables: []TablePlan{{Key: "wire", Kind: "physical_connection", Type: "cable", Start: 2, End: 3, Fields: map[string]int{"endpoint_a_device": 1, "endpoint_a_port": 2, "endpoint_b_device": 3, "endpoint_b_port": 4, "model": 5, "media_type": 6}, TypeColumn: 6, Types: map[string]string{"100G": "cable", "400G": "cable"}, Required: []string{"endpoint_a_device"}, Confidence: .9}}, Note: "synthetic only"}
	return s, p
}

func TestPlanCoverageRejectsLostTypeFieldAndRows(t *testing.T) {
	for _, mode := range []string{"type", "model", "range", "empty"} {
		t.Run(mode, func(t *testing.T) {
			s, p := syntheticCablePlan()
			if e := CheckExtraction(s, p); e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "type":
				delete(p.Tables[0].Types, "400G")
			case "model":
				delete(p.Tables[0].Fields, "model")
			case "range":
				p.Tables[0].End = 2
			case "empty":
				p.Tables = nil
			}
			if e := CheckExtraction(s, p); e == nil {
				t.Fatal("source omission accepted")
			}
		})
	}
}

func TestPlanRevisionPreservesNonstandardSourceBinding(t *testing.T) {
	s, p := syntheticCablePlan()
	for j := range s.Cells {
		if s.Cells[j].Ref == "E1" {
			s.Cells[j].Value = "nonstandard cable part label"
		}
	}
	_, next := syntheticCablePlan()
	delete(next.Tables[0].Fields, "model")
	if e := CheckPlanRevision(s, next, &p); e == nil || !strings.Contains(e.Error(), "EXTRACTION_REGRESSION") {
		t.Fatalf("lost prior model: %v", e)
	}
}

func TestAlternateEndpointLabelsAndIncompleteRows(t *testing.T) {
	s, p := syntheticCablePlan()
	s.Cells = append(s.Cells, bom.Cell{Sheet: s.Name, Row: 1, Column: 7, Ref: "G1", Value: "A Side Lable Name"}, bom.Cell{Sheet: s.Name, Row: 2, Column: 7, Ref: "G2", Value: "synthetic-alias"})
	if e := CheckExtraction(s, p); e != nil {
		t.Fatal("alternate label incorrectly requires a second cable", e)
	}
	p.Tables[0].Required = []string{"hostname", "model", "network_role"}
	raw, _ := json.Marshal(p)
	decoded, e := decodeExtraction(raw, 3)
	if e != nil || len(decoded.Tables[0].Required) != 1 || decoded.Tables[0].Required[0] != "endpoint_a_device" {
		t.Fatal("optional or unmapped row keys were not replaced by explicit source identity", e)
	}
}

func TestFailedLegacyPlanCannotBecomeBaseline(t *testing.T) {
	s, p := syntheticCablePlan()
	p.Tables[0].Fields["unsupported_legacy_field"] = 5
	raw, _ := json.Marshal(p)
	if _, e := RestoreExtractionBindings(s, raw); e == nil {
		t.Fatal("failed legacy recipe promoted to a valid baseline")
	}
}

func TestExplicitHeadersRepairOmissionsWithoutInventingValues(t *testing.T) {
	s, p := syntheticCablePlan()
	delete(p.Tables[0].Fields, "model")
	normalizeSourceBindings(s, &p)
	if p.Tables[0].Fields["model"] != 5 || p.Tables[0].Fields["speed"] != 0 {
		t.Fatal("explicit model omitted or absent speed invented")
	}
	duplicate := p.Tables[0]
	duplicate.Key = "another-model-name"
	p.Tables = append(p.Tables, duplicate)
	normalizeSourceBindings(s, &p)
	if len(p.Tables) != 1 {
		t.Fatal("compatible recipes for the same source object duplicated")
	}
	if e := CheckExtraction(s, p); e != nil {
		t.Fatal(e)
	}
}

func TestAgentInspectsRejectedRegionAndRepairsPlan(t *testing.T) {
	s, valid := syntheticCablePlan()
	_, invalid := syntheticCablePlan()
	delete(invalid.Tables[0].Types, "400G")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		var reply any
		switch calls {
		case 1:
			reply = map[string]any{"action": "submit_plan", "plan": invalid}
		case 2:
			if !strings.Contains(body.Messages[1].Content, "SOURCE_TYPE_UNCOVERED") || !strings.Contains(body.Messages[1].Content, "row=3") {
				t.Error("independent validator did not feed exact missing evidence back")
			}
			reply = map[string]any{"action": "inspect_region", "sheet_name": s.Name, "start_row": 3, "end_row": 3, "first_column": 1, "last_column": 6}
		case 3:
			var input struct {
				History []struct {
					Action string
					Result []bom.Cell
				} `json:"tool_history"`
			}
			if json.Unmarshal([]byte(body.Messages[1].Content), &input) != nil || len(input.History) < 2 {
				t.Error("IR history absent")
			} else {
				last := input.History[len(input.History)-1]
				found := false
				for _, cell := range last.Result {
					if cell.Row == 3 && cell.Column == 6 && cell.Value == "400G" {
						found = true
					}
				}
				if last.Action != "inspect_region" || !found {
					t.Error("requested IR result absent")
				}
			}
			reply = map[string]any{"action": "submit_plan", "plan": valid}
		default:
			t.Error("unbounded model calls")
		}
		raw, _ := json.Marshal(reply)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(raw)}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Model: "synthetic", Timeout: time.Second, TokenBudget: 2048, JSONMode: true}
	result, e := p.PlanSheet(context.Background(), s, PlanningContext{})
	if e != nil || calls != 3 || len(result.Tables) != 1 {
		t.Fatalf("repair failed calls=%d: %v", calls, e)
	}
	found := false
	for _, step := range result.Steps {
		if step.Tool == "inspect_region" && step.Region == "R3C1:R3C6" {
			found = true
		}
	}
	if !found {
		t.Fatal("actual requested tool execution missing from trace")
	}
}

func TestAgentStopsWithoutReturningRejectedPlan(t *testing.T) {
	s, invalid := syntheticCablePlan()
	invalid.Tables[0].End = 2
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		raw, _ := json.Marshal(invalid)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(raw)}}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Model: "synthetic", Timeout: time.Second}
	result, e := p.PlanSheet(context.Background(), s, PlanningContext{})
	if e == nil || calls != 8 || len(result.Tables) != 0 {
		t.Fatalf("invalid plan escaped loop: calls=%d error=%v", calls, e)
	}
}

func TestHeaderRepairRecomputesInventoryEligibility(t *testing.T) {
	s := bom.Sheet{Name: "inventory"}
	for row, values := range [][]string{{"Hostname", "Model", "SN", "Rack No"}, {"synthetic-leaf-a", "", "/", "R-A"}, {"synthetic-leaf-b", "synthetic-model", "/", "R-B"}} {
		for col, v := range values {
			s.Cells = append(s.Cells, bom.Cell{Sheet: s.Name, Row: row + 1, Column: col + 1, Value: v})
		}
	}
	p := SheetPlan{Purpose: "table", HeaderRows: []int{1}, Tables: []TablePlan{{Kind: "planned_device", Type: "switch", Start: 2, End: 3, Fields: map[string]int{"model": 2, "serial": 3}, Required: []string{"serial"}}}}
	normalizeSourceBindings(s, &p)
	if err := CheckExtraction(s, p); err != nil {
		t.Fatal(err)
	}
	if p.Tables[0].Required[0] != "hostname" {
		t.Fatal("obsolete serial eligibility retained")
	}
	for row, cells := range bom.MergedSourceCells(s) {
		if row > 1 && !tableRowPresent(p.Tables[0], cells) {
			t.Fatal("explicit device row disappeared")
		}
	}
}

func TestRejectFragmentedCableAndPlanningOnlyConnection(t *testing.T) {
	s, p := syntheticCablePlan()
	extra := TablePlan{Kind: "physical_connection", Type: "cable", Start: 2, End: 3, TypeColumn: 6, Types: map[string]string{"100G": "cable", "400G": "cable"}, Fields: map[string]int{"model": 5, "endpoint_a_uid": 1, "endpoint_b_uid": 3}}
	p.Tables = append(p.Tables, extra)
	normalizeSourceBindings(s, &p)
	if err := CheckExtraction(s, p); err != nil || len(p.Tables) != 1 {
		t.Fatal("fragmented source cable was not repaired into one object", err)
	}
	s = bom.Sheet{Name: "estimate", Cells: []bom.Cell{{Row: 1, Column: 1, Value: "A point"}, {Row: 2, Column: 1, Value: "synthetic-location"}}}
	p = SheetPlan{Purpose: "table", HeaderRows: []int{1}, Tables: []TablePlan{{Kind: "physical_connection", Type: "cable", Start: 2, End: 2, Fields: map[string]int{"endpoint_a_uid": 1, "endpoint_b_uid": 2}}}}
	if err := CheckExtraction(s, p); err == nil || !strings.Contains(err.Error(), "PHYSICAL_ENDPOINT_BINDING_REQUIRED") {
		t.Fatal("point estimates treated as physical port IDs", err)
	}
	p.Tables[0].Kind = "reference"
	if err := CheckExtraction(s, p); err != nil {
		t.Fatal("reference evidence rejected", err)
	}
}

func TestAgentPagesOversizedRegionRead(t *testing.T) {
	s, valid := syntheticCablePlan()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		var reply any
		if calls == 1 {
			reply = map[string]any{"action": "inspect_region", "sheet_name": s.Name, "start_row": 1, "end_row": 190, "first_column": 1, "last_column": 100}
		} else {
			var input struct {
				History []struct {
					Truncated    bool
					Result       []bom.Cell
					Continuation map[string]int
				} `json:"tool_history"`
			}
			json.Unmarshal([]byte(body.Messages[1].Content), &input)
			if len(input.History) != 1 || !input.History[0].Truncated || input.History[0].Continuation["returned_end_row"] != 16 || len(input.History[0].Result) == 0 {
				t.Error("bounded actual source page and continuation absent")
			}
			reply = map[string]any{"action": "submit_plan", "plan": valid}
		}
		raw, _ := json.Marshal(reply)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(raw)}}}})
	}))
	defer server.Close()
	provider := Provider{Endpoint: server.URL, Model: "synthetic", Timeout: time.Second}
	if _, err := provider.PlanSheet(context.Background(), s, PlanningContext{}); err != nil || calls != 2 {
		t.Fatalf("paged source inspection failed: calls=%d err=%v", calls, err)
	}
}

func TestOpticRecipesUseExplicitCableOwnerAndPort(t *testing.T) {
	s, p := syntheticCablePlan()
	s.Cells = append(s.Cells, bom.Cell{Row: 1, Column: 7, Value: "本端模块型号"}, bom.Cell{Row: 2, Column: 7, Value: "synthetic-optic"})
	p.Tables[0].Fields["endpoint_a_optic"] = 7
	p.Tables = append(p.Tables, TablePlan{Kind: "component", Type: "optic_module", Start: 2, End: 3, Fields: map[string]int{"name": 7, "model": 7}})
	normalizeSourceBindings(s, &p)
	component := p.Tables[1]
	if component.Fields["parent_uid"] != 1 || component.Fields["port_index"] != 2 || component.Fields["model"] != 7 {
		t.Fatal("explicit cable end module lost its owner or port")
	}
	if tableRowPresent(component, bom.MergedSourceCells(s)[3]) {
		t.Fatal("blank optic field created a module")
	}
}

func TestPersistedEmptyDefaultsRestoreWithoutRelaxingModelContract(t *testing.T) {
	s, p := syntheticCablePlan()
	p.Tables = []TablePlan{{Kind: "reference", Type: "other", Start: 1, End: 3, Fields: map[string]int{}, Types: nil, Required: nil, Confidence: .8}}
	p.HeaderRows = nil
	// Reference-only input has no inventory headers requiring extraction.
	s = bom.Sheet{Name: "notes", Cells: []bom.Cell{{Row: 1, Column: 1, Value: "synthetic note"}, {Row: 3, Column: 1, Value: "more notes"}}}
	p.HeaderRows = []int{}
	raw, _ := json.Marshal(p)
	if err := validateExtractionSchema(context.Background(), raw); err == nil {
		t.Fatal("fresh null defaults escaped strict model schema")
	}
	restored, err := RestoreExtraction(s, raw)
	if err != nil {
		t.Fatal("stored Go empty defaults could not recover", err)
	}
	raw, _ = json.Marshal(restored)
	if err := validateExtractionSchema(context.Background(), raw); err != nil {
		t.Fatal("saving restored plan would break next resume", err)
	}
}

func TestExtractionEquivalenceIgnoresNarrationButDetectsBindingChanges(t *testing.T) {
	_, a := syntheticCablePlan()
	_, b := syntheticCablePlan()
	b.Note = "another explanation"
	b.Tables[0].Key = "another-key"
	b.Tables[0].Confidence = .6
	if !SameExtraction(a, b) {
		t.Fatal("narration changed extraction scope")
	}
	b.Tables[0].Fields["model"] = 7
	if SameExtraction(a, b) {
		t.Fatal("changed source binding treated as fully applied")
	}
}

func TestCableEndpointLabelCannotCreatePhantomParent(t *testing.T) {
	s, p := syntheticCablePlan()
	p.Tables[0].Fields["parent_uid"] = 1
	normalizeSourceBindings(s, &p)
	if p.Tables[0].Fields["parent_uid"] != 0 || p.Tables[0].Fields["endpoint_a_device"] != 1 {
		t.Fatal("cable still depends on a device display label as its parent")
	}
}

func TestRowNumberAndFiberCountAreNotParentAndWavelength(t *testing.T) {
	s, p := syntheticCablePlan()
	s.Cells = append(s.Cells, bom.Cell{Row: 1, Column: 7, Value: "序号"}, bom.Cell{Row: 2, Column: 7, Value: "1"}, bom.Cell{Row: 1, Column: 8, Value: "Fiber core count"}, bom.Cell{Row: 2, Column: 8, Value: "8"})
	p.Tables[0].Fields["parent_uid"] = 7
	p.Tables[0].Fields["wavelength"] = 8
	normalizeSourceBindings(s, &p)
	f := p.Tables[0].Fields
	if f["parent_uid"] != 0 || f["wavelength"] != 0 || f["sequence"] != 7 || f["specification"] != 8 {
		t.Fatal("source counts created false dependencies or optical properties")
	}
}
