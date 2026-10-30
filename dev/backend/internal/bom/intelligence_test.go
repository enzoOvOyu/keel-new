package bom

import "testing"

func TestFormulaIdentityAndMergedEvidence(t *testing.T) {
	for _, f := range []string{"hostname", "model", "endpoint_a_device", "endpoint_a_optic"} {
		if v := ExtractCellValue(Cell{Value: "0", Formula: "IFERROR(VLOOKUP(...),0)", FormulaBacked: true}, f); v != "" {
			t.Fatalf("cached absence became %s=%s", f, v)
		}
	}
	if got := ExtractCellValue(Cell{Value: "leaf-alpha", Formula: "VLOOKUP(...)", FormulaBacked: true}, "hostname"); got != "leaf-alpha" {
		t.Fatal("lost source-backed formula result")
	}
	s := Sheet{Name: "plan", Merges: []string{"A2:A3"}, Cells: []Cell{{Sheet: "plan", Ref: "A2", Row: 2, Column: 1, Value: "leaf-alpha"}, {Sheet: "plan", Ref: "B3", Row: 3, Column: 2, Value: "Ethernet1/1/1"}}}
	cells := MergedSourceCells(s)
	if cells[3][1].Ref != "A2" || cells[3][1].Value != "leaf-alpha" {
		t.Fatal("merged owner lost anchor provenance")
	}
	if cells[4][1].Value != "" {
		t.Fatal("filled past actual merge")
	}
}
func TestCoverageCannotHideUnmappedRegion(t *testing.T) {
	w := Workbook{Sheets: []Sheet{{Name: "planning", Cells: []Cell{{Sheet: "planning", Ref: "A1", Row: 1, Column: 1, Value: "switch"}, {Sheet: "planning", Ref: "A9", Row: 9, Column: 1, Value: "unknown useful note"}}}}}
	b := PlanningSourceRows(w)
	ir := NewWorkbookIR(w)
	a := ir.AuditCoverage(&b)
	if a.Complete || a.TotalCells != 2 || a.RetainedCells != 2 {
		t.Fatal("unanalyzed content falsely completed")
	}
	b.Sheets = []RecognitionSheet{{Name: "planning", State: "recognized"}}
	a = ir.AuditCoverage(&b)
	if !a.Complete || a.ExtractedCells != 0 || a.Regions[1].Unrecognized != 1 {
		t.Fatal("retention confused with extraction")
	}
	b.Records = b.Records[:1]
	a = ir.AuditCoverage(&b)
	if a.Complete {
		t.Fatal("lost source content passed coverage")
	}
}
