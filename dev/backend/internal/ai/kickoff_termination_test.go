package ai

import (
	"context"
	"encoding/json"
	"keel.local/dev/internal/engineering"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKickoffTerminationWithoutSerialUsesGraphAndPreservesAttributes(t *testing.T) {
	wb, _ := relationWorkbook()
	wb.Sheets = wb.Sheets[1:]
	for i := range wb.Sheets[0].Cells {
		if wb.Sheets[0].Cells[i].Column == 5 && wb.Sheets[0].Cells[i].Row > 1 {
			wb.Sheets[0].Cells[i].Value = "25"
		}
	}
	region := KickoffRegion{ID: "links", Sheet: "links", Bounds: engineering.Bounds{StartRow: 1, EndRow: 3, StartColumn: 1, EndColumn: 5}, Steps: []int{1}, Reason: "explicit paired endpoints"}
	inspected := false
	srv := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, req *http.Request) {
		var body map[string]any
		json.NewDecoder(req.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		text := string(raw)
		name := "submit_result"
		var result any
		switch {
		case strings.Contains(text, "Task: route"):
			result = map[string]any{"regions": []KickoffRegion{region}}
		case strings.Contains(text, "Task: reason_relationships"):
			if !inspected {
				name = "inspect_connection_graph"
				result = map[string]any{"ranges": []KickoffGraphRange{{Region: "links", Sheet: "links", StartRow: 2, EndRow: 3, ADevice: 1, APort: 2, BDevice: 3, BPort: 4}}}
				inspected = true
			} else {
				result = KickoffRelationshipPlan{Entities: []KickoffEntityAssessment{{Region: "links", Kind: "connection", Reason: "explicit endpoints and peer topology", Evidence: []KickoffCellRef{{"links", "A2"}, {"links", "C2"}}}}, Relations: []KickoffRelation{}, Missing: []string{}}
			}
		default:
			if !inspected {
				t.Error("extracted before graph evidence")
			}
			result = KickoffTaskResult{Mappings: []KickoffMapping{{Region: "links", Table: "kickoff23/connections", Bounds: engineering.Bounds{StartRow: 2, EndRow: 3, StartColumn: 1, EndColumn: 5}, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "a_device_ref", Column: 1}, {Field: "a_port", Column: 2}, {Field: "b_device_ref", Column: 3}, {Field: "b_port", Column: 4}, {Field: "cable_length_m", Column: 5}}}}, Missing: []string{}}
		}
		args := result
		if name == "submit_result" {
			args = map[string]any{"result": result}
		}
		arguments, _ := json.Marshal(args)
		json.NewEncoder(out).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]string{"name": name, "arguments": string(arguments)}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer srv.Close()
	p := Provider{Endpoint: srv.URL, Model: "synthetic", Key: "synthetic", TokenBudget: 4096, Timeout: time.Second}
	result, err := p.ExtractKickoff(context.Background(), wb, wb, func(_, _, _ string) error { return nil }, EngineeringToolHooks{SourceHash: "synthetic-no-sn"}, KickoffAgentOptions{Domains: []string{"equipment"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.FlowVersion != "termination-v1" || len(result.Candidates) != 1 {
		t.Fatalf("wrong result: %v", result)
	}
	c := result.Candidates[0]
	if len(c.Rows) != 2 || len(c.Issues) != 0 {
		t.Fatalf("named links blocked: %v", c)
	}
	for _, row := range c.Rows {
		if row["a_device_ref"] == "" || row["b_device_ref"] == "" || row["a_serial"] != "" || row["b_serial"] != "" || row["cable_length_m"] != "25" {
			t.Fatalf("lost or fabricated field: %v", row)
		}
	}
}
func TestKickoffGraphReadsAllRowsAndDetectsConflictingPorts(t *testing.T) {
	wb, _ := relationWorkbook()
	wb.Sheets = wb.Sheets[1:]
	wb.Sheets[0].Cells[10].Value = wb.Sheets[0].Cells[5].Value
	wb.Sheets[0].Cells[11].Value = wb.Sheets[0].Cells[6].Value
	result, err := inspectKickoffConnectionGraph(wb, wb, []KickoffGraphRange{{Region: "links", Sheet: "links", StartRow: 2, EndRow: 3, ADevice: 1, APort: 2, BDevice: 3, BPort: 4}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	r := result.(map[string]any)
	if r["source_rows"] != 2 || r["device_count"] != 2 || r["conflicting_ports"] != 1 {
		t.Fatalf("graph evidence lost: %v", r)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "CLIENT-LEAF") || strings.Contains(string(encoded), "client-host") {
		t.Fatal("graph leaked source identities")
	}
}

func TestKickoffPartialTerminationIsRetainedAsException(t *testing.T) {
	wb, _ := relationWorkbook()
	wb.Sheets = wb.Sheets[1:]
	wb.Sheets[0].Cells[5].Value = ""
	b := engineering.Bounds{StartRow: 2, EndRow: 3, StartColumn: 1, EndColumn: 5}
	mapping := KickoffMapping{Region: "links", Table: "kickoff23/connections", Bounds: b, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "a_device_ref", Column: 1}, {Field: "a_port", Column: 2}, {Field: "b_device_ref", Column: 3}, {Field: "b_port", Column: 4}}}
	rows, evidence, err := expandKickoffRegion(wb, KickoffRegion{ID: "links", Sheet: "links", Bounds: b}, mapping)
	if err != nil {
		t.Fatal(err)
	}
	c := kickoffAgentCandidate(mapping.Table, rows, evidence)
	if len(c.Rows) != 2 || len(c.Issues) != 1 || c.Rows[0]["b_device_ref"] != "client-host" {
		t.Fatalf("incomplete source row silently dropped: %+v", c)
	}
}
