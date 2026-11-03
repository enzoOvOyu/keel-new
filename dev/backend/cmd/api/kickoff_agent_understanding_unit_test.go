package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestKickoffAgentUnderstandingNotApplicable(t *testing.T) {
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "notes", Cells: []bom.Cell{{Sheet: "notes", Ref: "A1", Row: 1, Column: 1, Value: "Notes"}, {Sheet: "notes", Ref: "A2", Row: 2, Column: 1, Value: "Supporting memo"}}}}}
	result := ai.KickoffResult{Regions: []ai.KickoffRegion{{ID: "memo", Sheet: "notes", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 1}, Steps: []int{}, Reason: "non-engineering memo"}}}
	report := kickoffAgentUnderstandingReport("source", "job", w, result)
	if report.State != "recognized" {
		t.Fatalf("non-applicable report stuck: %+v", report.Steps)
	}
	for _, i := range []int{2, 3, 4} {
		if report.Steps[i].State != "not_applicable" {
			t.Fatal("non-applicable stage counted as missing")
		}
	}
	result.Missing = []string{"Unresolved source information"}
	report = kickoffAgentUnderstandingReport("source", "job", w, result)
	if report.State != "partial" {
		t.Fatal("missing information silently passed")
	}
}
