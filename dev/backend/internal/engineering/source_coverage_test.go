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
