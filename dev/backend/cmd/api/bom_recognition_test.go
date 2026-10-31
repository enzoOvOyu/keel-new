package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/bom"
	"testing"
)

func TestReanalysisKeepsCandidateIdentityAndHumanReview(t *testing.T) {
	s := bom.Sheet{Name: "inventory", Cells: []bom.Cell{{Sheet: "inventory", Ref: "A1", Row: 1, Column: 1, Value: "Hostname"}, {Sheet: "inventory", Ref: "B1", Row: 1, Column: 2, Value: "Model"}, {Sheet: "inventory", Ref: "A2", Row: 2, Column: 1, Value: "synthetic-host"}, {Sheet: "inventory", Ref: "B2", Row: 2, Column: 2, Value: "synthetic-model"}}}
	p := ai.SheetPlan{HeaderRows: []int{1}, Tables: []ai.TablePlan{{Key: "model-chosen-key", Kind: "planned_device", Type: "server", Start: 2, End: 2, Fields: map[string]int{"hostname": 1, "model": 2}}}}
	rows := applySheetPlan(bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{s}}).Records, s, p, nil)
	id := rows[1].ID
	p.Tables[0].Key = "different-model-chosen-key"
	again := applySheetPlan(rows, s, p, nil)
	if again[1].ID != id {
		t.Fatal("model table name changed candidate identity")
	}
	for _, mode := range []string{"confirmed", "manual", "imported"} {
		saved := append([]bom.ImportRecord{}, rows...)
		saved[1].Confirmed = mode == "confirmed"
		saved[1].Imported = mode == "imported"
		if mode == "manual" {
			saved[1].Method = "manual"
		}
		p.Tables[0].Type = "disk"
		result := applySheetPlan(saved, s, p, nil)
		if result[1].Item.Category != "server" || result[1].Confirmed != saved[1].Confirmed || result[1].Method != saved[1].Method {
			t.Fatalf("overwrote %s candidate", mode)
		}
	}
}
