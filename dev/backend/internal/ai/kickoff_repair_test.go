package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func rackDrawing() (bom.Workbook, KickoffRegion, KickoffMapping) {
	s := bom.Sheet{Name: "arbitrary drawing", Merges: []string{"B2:B3"}}
	add := func(row, col int, ref, value string) {
		s.Cells = append(s.Cells, bom.Cell{Sheet: s.Name, Row: row, Column: col, Ref: ref, Value: value})
	}
	add(1, 2, "B1", "rack-example")
	add(2, 2, "B2", "device-example")
	for i := 0; i < 4; i++ {
		add(i+2, 1, fmt.Sprintf("A%d", i+2), fmt.Sprint(4-i))
		add(i+2, 3, fmt.Sprintf("C%d", i+2), fmt.Sprint(4-i))
	}
	r := KickoffRegion{ID: "drawing", Sheet: s.Name, Reason: "rack elevation", Steps: []int{2}, Bounds: engineering.Bounds{StartRow: 1, EndRow: 5, StartColumn: 1, EndColumn: 3}}
	m := KickoffMapping{Region: r.ID, Table: "kickoff23/placements", Layout: "cells", Bounds: engineering.Bounds{StartRow: 2, EndRow: 5, StartColumn: 2, EndColumn: 2}, Bindings: []KickoffBinding{{Field: "device_id"}, {Field: "rack_id", Row: 1}, {Field: "start_u", Column: 1, Normalize: "u_grid_start", Classification: "rack_u_axis"}, {Field: "height_u", Column: 1, Normalize: "u_grid_height", Classification: "rack_u_axis"}}}
	return bom.Workbook{Sheets: []bom.Sheet{s}}, r, m
}
func TestKickoffRackDrawingUsesPairedAxesAndOccupiedBlock(t *testing.T) {
	w, r, m := rackDrawing()
	if e := validateKickoffMappings(w, []KickoffRegion{r}, 2, KickoffTaskResult{Mappings: []KickoffMapping{m}}); e != nil {
		t.Fatal(e)
	}
	rows, refs, e := expandKickoffRegion(w, r, m)
	if e != nil {
		t.Fatal(e)
	}
	c := kickoffAgentCandidate(m.Table, rows, refs)
	if len(c.Rows) != 1 || c.Rows[0]["start_u"] != "3" || c.Rows[0]["height_u"] != "2" || c.Rows[0]["rack_id"] != "rack-example" {
		t.Fatalf("wrong expansion: %+v", c.Rows)
	}
	proof := c.Evidence[0].Fields["height_u"].NormalizationEvidence
	if proof == nil || (proof.MappingBounds == nil || proof.MappingBounds.StartRow != 2 || proof.MappingBounds.EndRow != 3) || len(proof.Ticks) != 2 {
		t.Fatalf("missing evidence: %+v", proof)
	}
	cap := kickoffUScaleCapacity(w.Sheets[0], r, KickoffMapping{Bounds: r.Bounds}, KickoffBinding{Column: 1, Classification: "rack_u_axis"})
	if cap.Value != "4" {
		t.Fatalf("capacity: %+v", cap)
	}
	w.Sheets[0].Cells = w.Sheets[0].Cells[:len(w.Sheets[0].Cells)-1]
	if v := kickoffUScaleCapacity(w.Sheets[0], r, KickoffMapping{Bounds: r.Bounds}, KickoffBinding{Column: 1, Classification: "rack_u_axis"}); v.Value != "" {
		t.Fatal("broken opposite scale accepted")
	}
	w, r, m = rackDrawing()
	if v := kickoffUScaleCapacity(w.Sheets[0], r, KickoffMapping{Bounds: r.Bounds}, KickoffBinding{Column: 1}); v.Value != "" {
		t.Fatal("bare numbering accepted")
	}
	w.Sheets[0].Merges = nil
	if v := kickoffUGridPlacement(w.Sheets[0], r, m, m.Bindings[3], w.Sheets[0].Cells[1]); v.Value != "" {
		t.Fatal("unmerged label height invented")
	}
}
func TestKickoffPartialProviderFailureAndDomainReuse(t *testing.T) {
	calls := 0
	w, region, mapping := rackDrawing()
	region.Steps = []int{2, 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var content any
		switch calls {
		case 1:
			content = map[string]any{"regions": []KickoffRegion{region}, "missing_inputs": []string{}}
		case 2:
			content = KickoffTaskResult{Mappings: []KickoffMapping{mapping}, Missing: []string{}}
		default:
			http.Error(w, "synthetic unavailable", 401)
			return
		}
		encoded, _ := json.Marshal(content)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(encoded)}, "finish_reason": "stop"}}})
	}))
	defer srv.Close()
	p := Provider{Endpoint: srv.URL, Model: "synthetic-protocol", Key: "synthetic", TokenBudget: 2048, Timeout: time.Second}
	var cp EngineeringCheckpoint
	hooks := EngineeringToolHooks{SourceHash: "synthetic-drawing", Checkpoint: func(_ context.Context, c EngineeringCheckpoint) error { cp = c; return nil }}
	result, e := p.ExtractKickoff(context.Background(), w, w, func(_, _, _ string) error { return nil }, hooks, KickoffAgentOptions{Domains: []string{"facility", "cabling", "network"}})
	if e != nil || len(result.Candidates) != 1 || len(result.Missing) == 0 || cp.Kickoff.Tasks[3].Failure == "" {
		t.Fatalf("partial lost: candidates=%d missing=%v err=%v", len(result.Candidates), result.Missing, e)
	}
	before := calls
	hooks.Resume = &cp
	result, e = p.ExtractKickoff(context.Background(), w, w, func(_, _, _ string) error { return nil }, hooks, KickoffAgentOptions{Domains: []string{"network"}})
	if e != nil || len(result.Candidates) != 1 || calls != before {
		t.Fatalf("cache lost: candidates=%d calls=%d err=%v", len(result.Candidates), calls, e)
	}
}

// Optional local golden workbook; no source data is committed or logged.
func TestKickoffRackDrawingLocalFixture(t *testing.T) {
	path := os.Getenv("KEEL_KICKOFF_LAYOUT_FIXTURE")
	if path == "" {
		t.Skip("local workbook not requested")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	wb, e := bom.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	sheet, ok := kickoffSheet(wb, "落位图")
	if !ok {
		t.Fatal("fixture drawing not found")
	}
	region := KickoffRegion{ID: "fixture-layout", Sheet: sheet.Name, Bounds: engineering.Bounds{StartRow: 1, EndRow: 53, StartColumn: 2, EndColumn: 4}}
	cap := kickoffUScaleCapacity(sheet, region, KickoffMapping{Bounds: region.Bounds}, KickoffBinding{Column: 2, Classification: "rack_u_axis"})
	if cap.Value != "52" {
		t.Fatalf("fixture capacity not recognized: %s", cap.Missing)
	}
	measured := 0
	for _, c := range sheet.Cells {
		if c.Column != 3 || c.Row < 2 || c.Row > 53 {
			continue
		}
		v := kickoffUGridPlacement(sheet, region, KickoffMapping{Bounds: region.Bounds}, KickoffBinding{Column: 2, Normalize: "u_grid_height", Classification: "rack_u_axis"}, c)
		if v.Value != "" {
			measured++
		}
	}
	if measured == 0 {
		t.Fatal("no occupied merged block resolved against fixture U scale")
	}
	t.Logf("local drawing: capacity 52U; %d occupied blocks resolved; no provider call", measured)
}
