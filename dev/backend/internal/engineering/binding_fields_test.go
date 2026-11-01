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

func TestRejectAuxiliaryColumnsMasqueradingAsCableFacts(t *testing.T) {
	for _, tc := range []struct{ header, value, field, errorCode string }{
		{"Manufacturer", "Synthetic Vendor", "cable_label", "HEADER_FIELD_CONFLICT"},
		{"Brand", "Synthetic Vendor", "connector_a", "HEADER_FIELD_CONFLICT"},
		{"Fiber core count", "24", "length_state", "FIELD_VALUE_CONFLICT"},
		{"Length status", "calculated", "length_state", ""},
		{"Cable tag", "TEST-CABLE", "cable_label", ""},
	} {
		wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "any customer sheet", Cells: []bom.Cell{{Ref: "C4", Row: 4, Column: 3, Value: tc.header}, {Ref: "C5", Row: 5, Column: 3, Value: tc.value}}}}}
		p := EngineeringProposal{Summary: "source binding", Regions: []RegionMapping{{Sheet: wb.Sheets[0].Name, Kind: "connection", Bounds: Bounds{StartRow: 4, EndRow: 5, StartColumn: 3, EndColumn: 3}, HeaderRows: []int{4}, Columns: []ColumnBinding{{Field: tc.field, Column: 3}}}}}
		err := CheckProposal(wb, p)
		if tc.errorCode == "" && err != nil {
			t.Fatalf("valid %s rejected: %v", tc.field, err)
		}
		if tc.errorCode != "" && (err == nil || !strings.HasPrefix(err.Error(), tc.errorCode)) {
			t.Fatalf("incorrect %s mapping accepted: %v", tc.header, err)
		}
	}
}

func TestRejectEquipmentMetadataMasqueradingAsTypedFacts(t *testing.T) {
	for _, tc := range []struct{ header, value, field, errorCode string }{
		{"Asset ID", "TEST-1", "depth_mm", "HEADER_FIELD_CONFLICT"},
		{"No.", "42", "quantity", "HEADER_FIELD_CONFLICT"},
		{"Rack Unit", "36", "height_u", "HEADER_FIELD_CONFLICT"},
		{"Position label", "TEST-RACK-36", "mount_face", "FIELD_VALUE_CONFLICT"},
		{"Planning class", "42", "identity_kind", "FIELD_VALUE_CONFLICT"},
		{"Configuration combo", "synthetic-option", "equipment_type", "FIELD_VALUE_CONFLICT"},
		{"Device Type", "switch", "equipment_type", ""},
		{"Rack Unit", "36", "start_u", ""},
		{"Asset ID", "TEST-1", "label", ""},
	} {
		wb := bom.Workbook{Sheets: []bom.Sheet{{Name: "arbitrary", Cells: []bom.Cell{{Ref: "D7", Row: 7, Column: 4, Value: tc.header}, {Ref: "D8", Row: 8, Column: 4, Value: tc.value}}}}}
		p := EngineeringProposal{Summary: "source mapping", Regions: []RegionMapping{{Sheet: "arbitrary", Kind: "equipment", Bounds: Bounds{StartRow: 7, EndRow: 8, StartColumn: 4, EndColumn: 4}, HeaderRows: []int{7}, Columns: []ColumnBinding{{Field: tc.field, Column: 4}}}}}
		err := CheckProposal(wb, p)
		if tc.errorCode == "" && err != nil {
			t.Fatalf("valid %s rejected: %v", tc.field, err)
		}
		if tc.errorCode != "" && (err == nil || !strings.HasPrefix(err.Error(), tc.errorCode)) {
			t.Fatalf("incorrect %s accepted: %v", tc.field, err)
		}
	}
}
