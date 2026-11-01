package engineering

import (
	"keel.local/dev/internal/bom"
	"testing"
)

func TestSheetCompletionRequiresNonSampledAreas(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "moved", Cells: []bom.Cell{{Ref: "C4", Row: 4, Column: 3, Value: "header"}, {Ref: "C5", Row: 5, Column: 3, Value: "sample"}, {Ref: "C90", Row: 90, Column: 3, Value: "later input"}, {Ref: "F100", Row: 100, Column: 6, Formula: "1+1"}, {Ref: "J100", Row: 100, Column: 10, Value: "private", Sensitive: true}}}}}
	p := EngineeringProposal{Regions: []RegionMapping{{Sheet: "moved", Kind: "equipment", Bounds: Bounds{StartRow: 4, EndRow: 5, StartColumn: 3, EndColumn: 3}}}}
	if CheckSheetCoverage(wb, p, "moved") == nil {
		t.Fatal("sample accepted as whole sheet")
	}
	p.Regions[0].Bounds.EndRow = 90
	p.Regions = append(p.Regions, RegionMapping{Sheet: "moved", Kind: "other", Bounds: Bounds{StartRow: 100, EndRow: 100, StartColumn: 6, EndColumn: 6}})
	if err := CheckSheetCoverage(wb, p, "moved"); err != nil {
		t.Fatal(err)
	}
}

func TestRecognizedEndpointsCannotBeDowngradedToOther(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "customer arbitrary", Cells: []bom.Cell{{Ref: "A4", Row: 4, Column: 1, Value: "endpoint_b_port"}, {Ref: "B4", Row: 4, Column: 2, Value: "endpoint_a_device"}, {Ref: "C4", Row: 4, Column: 3, Value: "endpoint_b_device"}, {Ref: "D4", Row: 4, Column: 4, Value: "endpoint_a_port"}}}}}
	p := EngineeringProposal{Regions: []RegionMapping{{Sheet: "customer arbitrary", Kind: "other", Bounds: Bounds{StartRow: 4, EndRow: 4, StartColumn: 1, EndColumn: 4}}}}
	if CheckSheetCoverage(wb, p, "customer arbitrary") == nil {
		t.Fatal("recognized endpoints silently retained as unknown")
	}
	p.Regions[0].Kind = "connection"
	p.Regions[0].Columns = []ColumnBinding{{Field: "z_port", Column: 1}, {Field: "a_device", Column: 2}, {Field: "z_device", Column: 3}, {Field: "a_port", Column: 4}}
	if err := CheckSheetCoverage(wb, p, "customer arbitrary"); err != nil {
		t.Fatal(err)
	}
}
