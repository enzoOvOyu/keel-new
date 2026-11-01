package engineering

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"math/big"
	"testing"
)

// These are synthetic structural variants, not cross-customer acceptance.
func TestPhysicalExtractionStructuralVariants(t *testing.T) {
	for _, variant := range []struct {
		name        string
		offset      int
		reverse     bool
		unit, width string
	}{
		{"Customer inventory", 0, false, "mm", "600"},
		{"任意名称", 5, true, "cm", "60"},
		{"Rearranged", 13, false, "m", "0.6"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			sheet := bom.Sheet{Name: variant.name, Cells: []bom.Cell{}}
			fields := []string{"label", "equipment_type", "rack", "room", "site", "start_u", "height_u"}
			values := []string{"Synthetic-A", "server", "R", "Room", "Site", "U3", "2U"}
			cols := []ColumnBinding{}
			put := func(row, col int, value string) {
				sheet.Cells = append(sheet.Cells, bom.Cell{Sheet: sheet.Name, Row: row, Column: col, Ref: fmt.Sprintf("%c%d", 'A'+col-1, row), Value: value})
			}
			for i, f := range fields {
				col := i + 1
				if variant.reverse {
					col = len(fields) - i
				}
				cols = append(cols, ColumnBinding{Field: f, Column: col})
				put(variant.offset+1, col, "Different header "+f)
				put(variant.offset+2, col, values[i])
			}
			put(variant.offset+5, 1, "R")
			put(variant.offset+5, 2, "32U")
			put(variant.offset+5, 3, variant.width)
			put(variant.offset+5, 4, "Room")
			put(variant.offset+5, 5, "Site")
			wb := bom.Workbook{Sheets: []bom.Sheet{sheet, {Name: "Unrelated", Cells: []bom.Cell{{Sheet: "Unrelated", Ref: "A1", Row: 1, Column: 1, Value: "retained note"}}}}}
			p := EngineeringProposal{Summary: "Synthetic regions", Regions: []RegionMapping{
				{Sheet: sheet.Name, Kind: "equipment", Bounds: Bounds{StartRow: variant.offset + 1, EndRow: variant.offset + 2, StartColumn: 1, EndColumn: 7}, HeaderRows: []int{variant.offset + 1}, Columns: cols},
				{Sheet: sheet.Name, Kind: "rack", Bounds: Bounds{StartRow: variant.offset + 5, EndRow: variant.offset + 5, StartColumn: 1, EndColumn: 5}, Columns: []ColumnBinding{{Field: "label", Column: 1}, {Field: "capacity_u", Column: 2}, {Field: "width_mm", Column: 3, Unit: variant.unit}, {Field: "room", Column: 4}, {Field: "site", Column: 5}}},
				{Sheet: "Unrelated", Kind: "other", Bounds: Bounds{StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 1}},
			}}
			x, err := ExtractPhysical(wb, p, "project", "source", "revision")
			if err != nil {
				t.Fatal(err)
			}
			if len(x.Model.Equipment) != 1 || len(x.Model.Placements) != 1 || len(x.Model.Facilities) != 3 {
				t.Fatalf("lost entities: %+v", x.Model)
			}
			for _, i := range x.Issues {
				if i.Severity == "BLOCK" {
					t.Fatal(i)
				}
			}
			if x.Model.Equipment[0].EntityID != StableID("project", "equipment", "Synthetic-A") || *x.Model.Placements[0].StartU != 3 {
				t.Fatal("unstable identity or lost U position")
			}
			rack := x.Model.Facilities[2]
			if rack.WidthMM == nil || !positive(*rack.WidthMM) || new(big.Rat).SetFloat64(600).RatString() != ratString(*rack.WidthMM) {
				t.Fatalf("unit conversion: %v", rack.WidthMM)
			}
			if !hasIssue(x.Issues, "SOURCE_INFORMATION_RETAINED") {
				t.Fatal("unrelated content silently dropped")
			}
		})
	}
}

func TestPhysicalUnknownDimensionAndConstantEvidence(t *testing.T) {
	w := bom.Workbook{Sheets: []bom.Sheet{{Name: "Source", Cells: []bom.Cell{{Sheet: "Source", Ref: "A1", Row: 1, Column: 1, Value: "Rack"}, {Sheet: "Source", Ref: "B1", Row: 1, Column: 2, Value: "TBD"}}}}}
	p := EngineeringProposal{Summary: "unknown dimension", Regions: []RegionMapping{{Sheet: "Source", Kind: "rack", Bounds: Bounds{StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 2}, Columns: []ColumnBinding{{Field: "label", Column: 1}, {Field: "width_mm", Column: 2, Unit: "mm"}}}}}
	x, err := ExtractPhysical(w, p, "project", "source", "revision")
	if err != nil {
		t.Fatal(err)
	}
	if x.Model.Facilities[0].WidthMM != nil || !hasIssue(x.Issues, "NUMERIC_VALUE_UNRESOLVED") {
		t.Fatal("unknown became a fabricated dimension")
	}
	p.Regions[0].Constants = []ConstantBinding{{Field: "capacity_u", Value: "42", EvidenceCells: []string{"B1"}}}
	if CheckProposal(w, p) == nil {
		t.Fatal("invented capacity accepted without evidence")
	}
}

func ratString(s string) string {
	n, ok := new(big.Rat).SetString(s)
	if !ok {
		return "invalid"
	}
	return n.RatString()
}

func TestSourceConstantsResolveOnlyCitedSanitizedValues(t *testing.T) {
	raw := bom.Workbook{Sheets: []bom.Sheet{{Name: "Source", Cells: []bom.Cell{{Ref: "A1", Value: "Synthetic site", Row: 1, Column: 1}, {Ref: "A2", Value: "Different synthetic site", Row: 2, Column: 1}}}}}
	safe := bom.Workbook{Sheets: []bom.Sheet{{Name: "Source", Cells: []bom.Cell{{Ref: "A1", Value: "[LABEL]", Row: 1, Column: 1}, {Ref: "A2", Value: "[LABEL]", Row: 2, Column: 1}}}}}
	p := EngineeringProposal{Regions: []RegionMapping{{Sheet: "Source", Constants: []ConstantBinding{{Field: "site", Value: "[LABEL]", EvidenceCells: []string{"A1"}}}}}}
	resolved := ResolveSourceConstants(raw, safe, p)
	if resolved.Regions[0].Constants[0].Value != "Synthetic site" || p.Regions[0].Constants[0].Value != "[LABEL]" {
		t.Fatal("cited constant not restored locally or proposal mutated")
	}
	p.Regions[0].Constants[0].EvidenceCells = []string{"A1", "A2"}
	if ResolveSourceConstants(raw, safe, p).Regions[0].Constants[0].Value != "[LABEL]" {
		t.Fatal("ambiguous anonymized identities guessed")
	}
}
