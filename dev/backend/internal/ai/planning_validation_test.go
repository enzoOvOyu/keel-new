package ai

import (
	"keel.local/dev/internal/bom"
	"testing"
)

func TestExtractionRejectsSemanticBindingAndDuplicateSummary(t *testing.T) {
	s := bom.Sheet{Cells: []bom.Cell{{Row: 1, Column: 1, Value: "Cable Type"}, {Row: 1, Column: 2, Value: "Model"}}}
	p := SheetPlan{HeaderRows: []int{1}, Tables: []TablePlan{{Key: "wire", Fields: map[string]int{"speed": 1}}}}
	if CheckExtraction(s, p) == nil {
		t.Fatal("medium accepted as speed")
	}
	p.Tables = []TablePlan{{Key: "one", Kind: "material_summary", Start: 2, End: 10, Fields: map[string]int{"model": 2}}, {Key: "two", Kind: "material_summary", Start: 2, End: 10, Fields: map[string]int{"model": 2}}}
	if CheckExtraction(s, p) == nil {
		t.Fatal("same material rows extracted twice")
	}
}
