package ai

import (
	"context"
	"encoding/json"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKickoffAllowsBothEndpointsInSameRange(t *testing.T) {
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Row: 2, Column: 4}}}}}
	b := engineering.Bounds{StartRow: 2, EndRow: 2, StartColumn: 1, EndColumn: 4}
	r := KickoffRegion{ID: "links", Sheet: "renamed", Bounds: b}
	a := KickoffMapping{Region: r.ID, Table: "port_specs", Bounds: b, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "serial", Column: 1}, {Field: "port", Column: 2}}}
	z := a
	z.Bindings = []KickoffBinding{{Field: "serial", Column: 3}, {Field: "port", Column: 4}}
	if err := validateKickoffMappings(w, []KickoffRegion{r}, 3, KickoffTaskResult{Mappings: []KickoffMapping{a, z}}); err != nil {
		t.Fatal(err)
	}
	if err := validateKickoffMappings(w, []KickoffRegion{r}, 3, KickoffTaskResult{Mappings: []KickoffMapping{a, a}}); err == nil {
		t.Fatal("exact duplicate accepted")
	}
}

func TestKickoffProjectsKnownPortsWithoutInventingSpecs(t *testing.T) {
	tables := map[string][][]string{
		"terminations": {{"SN-A", "", "eth1", "SN-B", "", "", "eth2", ""}, {"SN-A", "", "eth1", "", "", "", "eth3", ""}},
		"port_specs":   {{"SN-A", "eth1", "fiber", "LC", "100", "Gbps", "", ""}},
	}
	if got := len(completeKickoffPortIdentities(tables)); got != 1 {
		t.Fatalf("projected %d identities", got)
	}
	if len(tables["port_specs"]) != 2 || tables["port_specs"][1][0] != "SN-B" || tables["port_specs"][1][1] != "eth2" || tables["port_specs"][0][2] != "fiber" {
		t.Fatal("missing/duplicated identity or overwritten source spec")
	}
	for _, v := range tables["port_specs"][1][2:] {
		if v != "" {
			t.Fatal("invented specification")
		}
	}
	if len(completeKickoffPortIdentities(tables)) != 0 {
		t.Fatal("projection is not idempotent")
	}
}

func TestKickoffReservesFinalCorrectionAndPreservesRouteMissing(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := `{"action":"read_range","sheet":"renamed","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":1}}`
		if calls == 7 {
			content = `{"regions":[],"unexpected":true}`
		} else if calls == 8 {
			content = `{"regions":[{"id":"source","sheet":"renamed","bounds":{"start_row":1,"end_row":2,"start_column":1,"end_column":1},"steps":[],"reason":"supporting information"}],"missing_inputs":["source: engineering confirmation required"]}`
		} else if calls > 8 {
			t.Error("unbounded requests")
			http.Error(w, "excess", 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	p := Provider{Endpoint: server.URL, Model: "synthetic-protocol", Key: "synthetic", TokenBudget: 2048, Timeout: time.Second}
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Sheet: "renamed", Ref: "A1", Row: 1, Column: 1, Value: "Notes"}, {Sheet: "renamed", Ref: "A2", Row: 2, Column: 1, Value: "Example"}}}}}
	result, err := p.ExtractKickoff(context.Background(), w, w, func(_, _, _ string) error { return nil }, EngineeringToolHooks{})
	if err != nil || calls != 8 || !strings.Contains(strings.Join(result.Missing, " "), "source: engineering confirmation required") {
		t.Fatalf("calls=%d missing=%v err=%v", calls, result.Missing, err)
	}
}

func TestKickoffExplicitRatesAndEmptySourceRows(t *testing.T) {
	for _, tc := range []struct{ source, number, unit string }{
		{"200G", "200", "Gbps"}, {"2.5 Gbit/s", "2.5", "Gbps"}, {"100Mbps", "100", "Mbps"},
		{"400", "", ""}, {"2x200G", "", ""}, {"200-400G", "", ""}, {"model-200G", "", ""},
		{"100MBps", "", ""},
	} {
		n, u := kickoffRate(tc.source)
		if n != tc.number || u != tc.unit {
			t.Fatalf("%q => %q %q", tc.source, n, u)
		}
	}
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Sheet: "renamed", Ref: "A2", Row: 2, Column: 1, Value: "planned-pair"}}}}}
	b := engineering.Bounds{StartRow: 2, EndRow: 2, StartColumn: 1, EndColumn: 2}
	r := KickoffRegion{ID: "source", Sheet: "renamed", Bounds: b}
	m := KickoffMapping{Region: r.ID, Table: "optic_specs", Bounds: b, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "model", Column: 2}}}
	rows, evidence, err := expandKickoffRegion(w, r, m)
	if err != nil || len(rows) != 0 || len(evidence.([]any)) != 1 {
		t.Fatalf("empty spec became a row or source evidence was lost: rows=%v evidence=%v err=%v", rows, evidence, err)
	}
}

func TestKickoffClassificationsAreSourceBackedAndRestricted(t *testing.T) {
	b := engineering.Bounds{StartRow: 2, EndRow: 2, StartColumn: 1, EndColumn: 1}
	r := KickoffRegion{ID: "equipment", Sheet: "renamed", Bounds: b}
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Sheet: "renamed", Ref: "A2", Row: 2, Column: 1, Value: "Synthetic server model"}}}}}
	m := KickoffMapping{Region: r.ID, Table: "equipment_specs", Bounds: b, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "equipment_type", Column: 1, Classification: "server"}}}
	if err := validateKickoffMappings(w, []KickoffRegion{r}, 2, KickoffTaskResult{Mappings: []KickoffMapping{m}}); err != nil {
		t.Fatal(err)
	}
	rows, evidence, err := expandKickoffRegion(w, r, m)
	if err != nil || len(rows) != 1 || rows[0][1] != "server" || !strings.Contains(string(mustKickoffJSON(t, evidence)), "ai_classification_needs_review") {
		t.Fatalf("classification lost source or review state: %v %v", rows, err)
	}
	m.Bindings[0].Field = "serial"
	if validateKickoffMappings(w, []KickoffRegion{r}, 2, KickoffTaskResult{Mappings: []KickoffMapping{m}}) == nil {
		t.Fatal("classification accepted for physical identity")
	}
}

func mustKickoffJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
