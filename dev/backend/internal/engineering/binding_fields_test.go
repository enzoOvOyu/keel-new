package engineering

import (
	"keel.local/dev/internal/bom"
	"strings"
	"testing"
)

func TestRejectDomainInappropriateBindings(t *testing.T) {
	wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "renamed", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "Quantity"}, {Ref: "A2", Row: 2, Column: 1, Value: "4"}}}}}
	for _, tc := range []struct{ kind, field string }{{"power", "cable_label"}, {"placement", "parent"}, {"connection", "quantity"}} {
		proposal := EngineeringProposal{Summary: "synthetic", Regions: []RegionMapping{{Sheet: "renamed", Kind: tc.kind, Bounds: Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 1}, Columns: []ColumnBinding{{Field: tc.field, Column: 1}}}}}
		if err := CheckProposal(wb, proposal); err == nil || !strings.Contains(err.Error(), "FIELD_NOT_SUPPORTED_FOR_REGION") {
			t.Fatalf("%s/%s accepted: %v", tc.kind, tc.field, err)
		}
	}
}
