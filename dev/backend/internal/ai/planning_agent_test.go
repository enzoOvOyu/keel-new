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
			if !strings.Contains(body.Messages[1].Content, "SOURCE_FIELD_UNCOVERED") || !strings.Contains(body.Messages[1].Content, "row=3") {
				t.Error("independent validator did not feed exact missing evidence back")
			}
			reply = map[string]any{"action": "inspect_region", "sheet_name": s.Name, "start_row": 3, "end_row": 3, "first_column": 1, "last_column": 6}
		case 3:
			if !strings.Contains(body.Messages[1].Content, "read") && !strings.Contains(body.Messages[1].Content, "400G") {
				t.Error("IR result absent")
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
	delete(invalid.Tables[0].Fields, "model")
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
