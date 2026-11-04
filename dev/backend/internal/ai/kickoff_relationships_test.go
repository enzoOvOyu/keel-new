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
	"keel.local/dev/internal/engineering"
)

func relationWorkbook() (bom.Workbook, KickoffRelationshipPlan) {
	sheet := func(name string, rows [][]string) bom.Sheet {
		s := bom.Sheet{Name: name}
		for r, row := range rows {
			for c, value := range row {
				s.Cells = append(s.Cells, bom.Cell{Sheet: name, Row: r + 1, Column: c + 1, Ref: fmt.Sprintf("%c%d", 'A'+c, r+1), Value: value})
			}
		}
		return s
	}
	w := bom.Workbook{Sheets: []bom.Sheet{
		sheet("inventory", [][]string{{"SN", "Label", "Customer alias", "Rack No", "U Posiotion"}, {"SYN-A", "leaf alpha", "client-leaf", "R1", "20"}, {"SYN-B", "server beta", "client-host", "R2", "9"}}),
		sheet("links", [][]string{{"Local device", "Local port", "Remote device", "Remote port", "Cable"}, {"CLIENT-LEAF", "eth1", "client-host", "eth2", "cable-x"}, {"unknown", "eth3", "client-host", "eth4", "cable-x"}}),
	}}
	relation := KickoffRelation{ID: "endpoint", Kind: "device_identity", Source: KickoffRelationSource{"links", 1, 2, 3}, Targets: []KickoffRelationTarget{{"inventory", []int{2, 3}, 1, 2, 3}}, Reason: "端接名称与设备清单同一行别名一致；型号不是实例", Evidence: []KickoffCellRef{{"links", "A2"}, {"inventory", "C2"}}}
	return w, KickoffRelationshipPlan{Entities: []KickoffEntityAssessment{{Region: "inventory", Kind: "device", Reason: "explicit device instance rows", Evidence: []KickoffCellRef{{"inventory", "A2"}}}}, Relations: []KickoffRelation{relation}}
}
func TestKickoffRelationshipEvidenceAndFullRange(t *testing.T) {
	w, p := relationWorkbook()
	index, reports, err := evaluateKickoffRelations(w, p)
	if err != nil {
		t.Fatal(err)
	}
	if reports[0].Total != 2 || reports[0].Matched != 1 || reports[0].Missing != 1 {
		t.Fatalf("incorrect closure counts: %+v", reports)
	}
	b := engineering.Bounds{StartRow: 2, EndRow: 3, StartColumn: 1, EndColumn: 5}
	r := KickoffRegion{ID: "links", Sheet: "links", Bounds: b}
	m := KickoffMapping{Region: "links", Table: "kickoff23/ports", Bounds: b, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "serial", Column: 1, Relationship: "endpoint"}, {Field: "port", Column: 2}}}
	rows, proof, err := expandKickoffRegionWithRelationships(w, r, m, index)
	if err != nil {
		t.Fatal(err)
	}
	candidate := kickoffAgentCandidate(m.Table, rows, proof)
	if candidate.Rows[0]["serial"] != "SYN-A" || candidate.Rows[1]["serial"] != "" {
		t.Fatal("identity invented or relation lost")
	}
	field := candidate.Evidence[0].Fields["serial"]
	if field.Sheet != "inventory" || field.Cell != "A2" || len(field.RelationshipEvidence) < 3 || field.RelationshipReason == "" {
		t.Fatalf("cross-sheet proof missing: %+v", field)
	}
}
func TestKickoffRelationshipRejectsConflictAndAllowsCorroboratedAlias(t *testing.T) {
	w, p := relationWorkbook()
	w.Sheets[0].Cells[12].Value = "client-leaf" // second row alias now points to a different serial
	_, r, e := evaluateKickoffRelations(w, p)
	if e != nil || r[0].Ambiguous != 1 {
		t.Fatalf("conflicting aliases accepted: %+v %v", r, e)
	}
	w, p = relationWorkbook()
	p.Relations[0].Aliases = []KickoffRelationAlias{{From: KickoffCellRef{"links", "A3"}, To: KickoffCellRef{"inventory", "B2"}, Reason: "reviewed source context", Evidence: []KickoffCellRef{{"links", "B3"}, {"inventory", "C2"}}}}
	index, r, e := evaluateKickoffRelations(w, p)
	if e != nil || r[0].Matched != 2 || index["endpoint"][KickoffCellRef{"links", "A3"}].Cell.Value != "SYN-A" {
		t.Fatalf("supported alias rejected: %+v %v", r, e)
	}
	p.Relations[0].Aliases[0].Evidence = nil
	if _, _, e = evaluateKickoffRelations(w, p); e == nil {
		t.Fatal("uncorroborated alias accepted")
	}
}
func TestKickoffRelationshipDeviceAssessmentGatesExtraction(t *testing.T) {
	w, p := relationWorkbook()
	bounds := engineering.Bounds{StartRow: 1, EndRow: 3, StartColumn: 1, EndColumn: 5}
	regions := []KickoffRegion{{ID: "inventory", Sheet: "inventory", Bounds: bounds}}
	task := KickoffTaskResult{Mappings: []KickoffMapping{{Region: "inventory", Table: "kickoff23/devices"}}}
	p.Entities[0].Kind = "material"
	if validateKickoffRelationshipBindings(w, regions, p, task) == nil {
		t.Fatal("material promoted into equipment")
	}
	p.Entities[0].Kind = "device"
	if err := validateKickoffRelationshipBindings(w, regions, p, task); err != nil {
		t.Fatal(err)
	}
	task.Mappings = []KickoffMapping{{Region: "links", Table: "kickoff23/connections", Bindings: []KickoffBinding{{Field: "a_serial", Column: 1}}}}
	if validateKickoffRelationshipBindings(w, regions, p, task) == nil {
		t.Fatal("uncorroborated endpoint accepted")
	}
}
func TestKickoffRelationshipSearchDoesNotExposeOriginalIdentities(t *testing.T) {
	w, _ := relationWorkbook()
	raw, _ := json.Marshal(w)
	var safe bom.Workbook
	json.Unmarshal(raw, &safe)
	for i := range safe.Sheets {
		for j := range safe.Sheets[i].Cells {
			safe.Sheets[i].Cells[j].Value = "[SAFE]"
		}
	}
	result, err := kickoffRelatedRecords(w, safe, KickoffCellRef{"links", "A2"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "SYN-A") || strings.Contains(string(encoded), "client-leaf") {
		t.Fatal("original identity escaped into tool response")
	}
}
func TestKickoffAgentInvestigatesBeforeExtracting(t *testing.T) {
	w, plan := relationWorkbook()
	regions := []KickoffRegion{{ID: "inventory", Sheet: "inventory", Bounds: engineering.Bounds{StartRow: 1, EndRow: 3, StartColumn: 1, EndColumn: 5}, Steps: []int{1}, Reason: "inventory instances"}, {ID: "links", Sheet: "links", Bounds: engineering.Bounds{StartRow: 1, EndRow: 3, StartColumn: 1, EndColumn: 5}, Steps: []int{1}, Reason: "connections"}}
	calls := 0
	investigations := 0
	extracted := false
	srv := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		encoded, _ := json.Marshal(body)
		text := string(encoded)
		var answer any
		switch {
		case strings.Contains(text, "Task: reason_relationships"):
			investigations++
			if investigations == 1 {
				answer = map[string]any{"action": "evaluate_relationships", "relations": plan.Relations}
			} else {
				answer = plan
			}
		case strings.Contains(text, "Task: route"):
			answer = map[string]any{"regions": regions, "missing_inputs": []string{}}
		default:
			extracted = true
			if investigations != 2 {
				t.Error("extraction preceded tested reasoning")
			}
			answer = KickoffTaskResult{Mappings: []KickoffMapping{}, Missing: []string{"source unresolved row retained"}}
		}
		if len(body["tools"].([]any)) == 0 || body["tool_choice"] != "required" {
			t.Error("workbook functions not registered with provider")
		}
		name := "submit_result"
		arguments := map[string]any{"result": answer}
		if action, ok := answer.(map[string]any); ok && action["action"] != nil {
			name = action["action"].(string)
			arguments = action
			delete(arguments, "action")
		}
		raw, _ := json.Marshal(arguments)
		json.NewEncoder(out).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"function": map[string]string{"name": name, "arguments": string(raw)}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer srv.Close()
	provider := Provider{Endpoint: srv.URL, Model: "synthetic", Key: "synthetic", TokenBudget: 4096, Timeout: time.Second}
	var saved EngineeringCheckpoint
	_, err := provider.ExtractKickoff(context.Background(), w, w, func(_, _, _ string) error { return nil }, EngineeringToolHooks{SourceHash: "synthetic", Checkpoint: func(_ context.Context, cp EngineeringCheckpoint) error { saved = cp; return nil }}, KickoffAgentOptions{Domains: []string{"equipment"}})
	if err != nil || !extracted || calls != 4 || saved.Kickoff.Relationships == nil {
		t.Fatalf("reasoning pipeline: calls=%d extracted=%v err=%v", calls, extracted, err)
	}
}

func TestKickoffVerifiedTextFeedsRelationsWithoutLeaking(t *testing.T) {
	w, plan := relationWorkbook()
	w.Sheets[1].Cells[5].Formula = "source_reference"
	w.Sheets[1].Cells[5].Value = "untrusted cache"
	values := map[string]KickoffVerifiedCell{"links!A2": {Value: "client-leaf", RunID: "verified-run"}}
	local := kickoffVerifiedWorkbook(w, values, false)
	_, reports, err := evaluateKickoffRelations(local, plan)
	if err != nil || reports[0].Matched != 1 {
		t.Fatalf("verified text unavailable to relation evaluator: %+v %v", reports, err)
	}
	safe := kickoffVerifiedWorkbook(w, values, true)
	if safe.Sheets[1].Cells[5].Value == "client-leaf" || !strings.HasPrefix(safe.Sheets[1].Cells[5].Value, "[LABEL-") {
		t.Fatal("verified identity leaked")
	}
	if w.Sheets[1].Cells[5].Formula == "" || w.Sheets[1].Cells[5].Value != "untrusted cache" {
		t.Fatal("original workbook mutated")
	}
}

func TestKickoffAssessedDeviceTypeIsSemanticNotIdentity(t *testing.T) {
	w, _ := relationWorkbook()
	bounds := engineering.Bounds{StartRow: 2, EndRow: 3, StartColumn: 1, EndColumn: 5}
	r := KickoffRegion{ID: "inventory", Sheet: "inventory", Bounds: bounds}
	m := KickoffMapping{Region: r.ID, Table: "kickoff23/devices", Bounds: bounds, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "equipment_type", Column: 2, Classification: "switch"}}}
	if err := validateKickoffMappings(w, []KickoffRegion{r}, 1, KickoffTaskResult{Mappings: []KickoffMapping{m}}); err != nil {
		t.Fatal(err)
	}
	m.Bindings[0].Field = "serial"
	if validateKickoffMappings(w, []KickoffRegion{r}, 1, KickoffTaskResult{Mappings: []KickoffMapping{m}}) == nil {
		t.Fatal("semantic classification fabricated identity")
	}
}
