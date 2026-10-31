package main

import (
	"encoding/json"
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

// A derived record must never become a source row merely because it was first
// in that row after merging. The old code reused its ID and Link overwrote both.
func TestReanalysisDoesNotReuseDerivedModuleAsRawRow(t *testing.T) {
	s := bom.Sheet{Name: "inventory", Cells: []bom.Cell{{Sheet: "inventory", Ref: "A2", Row: 2, Column: 1, Value: "note only"}}}
	b := bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{s}})
	rawID := b.Records[0].ID
	b.Records[0].ID = "module-from-an-old-extractor"
	b.Records[0].Item.UID = b.Records[0].ID
	b.Records[0].Item.Category = "optic_module"
	b.Records[0].Method = "provider"
	plan := ai.SheetPlan{Purpose: "reference"}
	out := applySheetPlan(b.Records, s, plan, nil)
	if len(out) != 1 || out[0].ID != rawID || out[0].Item.Category != "other" {
		t.Fatal("derived identity reused for an unrecognized source row")
	}
}

func TestExtractionRetainsIncompleteCableAndProtectsReviewedPort(t *testing.T) {
	s := bom.Sheet{Name: "wires", Cells: []bom.Cell{
		{Sheet: "wires", Ref: "C2", Row: 2, Column: 3, Value: "synthetic-b"},
		{Sheet: "wires", Ref: "D2", Row: 2, Column: 4, Value: "eth1"},
	}}
	b := bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{s}})
	b.Records = append(b.Records, bom.ImportRecord{ID: "reviewed-port", Sheet: s.Name, Row: 2, Method: "source_relation", Confirmed: true, Item: bom.Item{UID: "reviewed-port", Category: "port"}})
	before, _ := json.Marshal(b.Records[1])
	p := ai.SheetPlan{Tables: []ai.TablePlan{{Kind: "physical_connection", Type: "cable", Start: 2, End: 2, Fields: map[string]int{"endpoint_a_device": 1, "endpoint_a_port": 2, "endpoint_b_device": 3, "endpoint_b_port": 4}, Required: []string{"endpoint_a_device"}}}}
	out := applySheetPlan(b.Records, s, p, nil)
	cables := 0
	for _, r := range out {
		if r.Item.Category == "cable" {
			cables++
			if r.Item.Fields["endpoint_a_device"] != "" {
				t.Fatal("missing source invented")
			}
		}
		if r.ID == "reviewed-port" {
			after, _ := json.Marshal(r)
			if string(before) != string(after) {
				t.Fatal("reviewed port changed")
			}
		}
	}
	if cables != 1 {
		t.Fatal("incomplete source connection disappeared")
	}
}

func TestSafeSheetPreservesMissingIdentity(t *testing.T) {
	s := bom.Sheet{Cells: []bom.Cell{{Row: 1, Column: 1, Value: "SN"}, {Row: 2, Column: 1, Value: ""}, {Row: 3, Column: 1, Value: "/"}, {Row: 4, Column: 1, Value: "synthetic-serial"}}}
	out := safeSheet(s)
	if out.Cells[1].Value != "" || out.Cells[2].Value != "/" || out.Cells[3].Value != "[IDENTITY]" {
		t.Fatal("masking fabricated identity presence")
	}
}

func TestFullResumeAppliesPreviouslyPartialSheetWithoutProvider(t *testing.T) {
	s := bom.Sheet{Name: "inventory", Cells: []bom.Cell{
		{Sheet: "inventory", Row: 1, Column: 1, Value: "Hostname"}, {Sheet: "inventory", Row: 1, Column: 2, Value: "Model"},
		{Sheet: "inventory", Row: 2, Column: 1, Value: "synthetic-a"}, {Sheet: "inventory", Row: 2, Column: 2, Value: "synthetic-model"},
		{Sheet: "inventory", Row: 3, Column: 1, Value: "synthetic-b"}, {Sheet: "inventory", Row: 3, Column: 2, Value: "synthetic-model"},
	}}
	p := ai.SheetPlan{Purpose: "table", HeaderRows: []int{1}, Tables: []ai.TablePlan{{Kind: "planned_device", Type: "switch", Start: 2, End: 3, Fields: map[string]int{"hostname": 1, "model": 2}, Required: []string{"hostname"}, Types: map[string]string{}, Confidence: .9}}, Note: "synthetic"}
	raw, _ := json.Marshal(p)
	b := bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{s}})
	b.Records = applySheetPlan(b.Records, s, p, map[string]bool{b.Records[1].ID: true})
	switches := func() int {
		n := 0
		for _, r := range b.Records {
			if r.Item.Category == "switch" {
				n++
			}
		}
		return n
	}
	if switches() != 1 {
		t.Fatal("partial extraction did not remain scoped")
	}
	info := bom.RecognitionSheet{State: "pending", Plan: raw}
	if !restoreWholeSheet(&b, s, &info) || info.State != "recognized" || switches() != 2 {
		t.Fatal("full resume retained partial candidates or pending sheet")
	}
	bad := bom.RecognitionSheet{State: "failed", Plan: []byte(`{"tables":[{"fields":{"model":0}}]}`)}
	before, _ := json.Marshal(b)
	if restoreWholeSheet(&b, s, &bad) {
		t.Fatal("malformed saved answer restored")
	}
	after, _ := json.Marshal(b)
	if string(before) != string(after) {
		t.Fatal("failed restore overwrote candidates")
	}
}

func TestReanalysisRemovesUnreviewedDerivedArtifacts(t *testing.T) {
	s := bom.Sheet{Name: "notes", Cells: []bom.Cell{{Sheet: "notes", Row: 2, Column: 1, Ref: "A2", Value: "synthetic reference"}}}
	b := bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{s}})
	b.Records = append(b.Records, bom.ImportRecord{ID: "stale-module", Sheet: "notes", Row: 2, Method: "source_relation", Item: bom.Item{UID: "stale-module", Category: "optic_module", Fields: map[string]string{}}})
	result := applySheetPlan(b.Records, s, ai.SheetPlan{Purpose: "reference"}, nil)
	if len(result) != 1 || result[0].ID == "stale-module" || result[0].Item.Category != "other" {
		t.Fatal("obsolete derived module survived source reanalysis")
	}
}

func TestExplicitSourceTypeCorrectsUnreviewedModelClassification(t *testing.T) {
	s := bom.Sheet{Name: "synthetic-inventory", Cells: []bom.Cell{{Ref: "A1", Row: 1, Column: 1, Value: "设备名称"}, {Ref: "B1", Row: 1, Column: 2, Value: "设备型号"}, {Ref: "C1", Row: 1, Column: 3, Value: "设备类型"}, {Ref: "A2", Row: 2, Column: 1, Value: "synthetic-switch"}, {Ref: "B2", Row: 2, Column: 2, Value: "synthetic-model"}, {Ref: "C2", Row: 2, Column: 3, Value: "Switch"}}}
	records := bom.PlanningSourceRows(bom.Workbook{Sheets: []bom.Sheet{s}}).Records
	p := ai.SheetPlan{HeaderRows: []int{1}, Tables: []ai.TablePlan{{Kind: "planned_device", Type: "server", Start: 2, End: 2, Fields: map[string]int{"hostname": 1, "model": 2}}}}
	records = applySheetPlan(records, s, p, nil)
	p.Tables[0].Fields["category"] = 3
	out := applySheetPlan(records, s, p, nil)
	found := false
	for _, r := range out {
		if r.Item.Hostname == "synthetic-switch" {
			found = true
			if r.Item.Category != "switch" || r.Item.Sources["category"].Ref != "C2" || r.Ambiguity {
				t.Fatal("unreviewed model guess overrode source type")
			}
		}
	}
	if !found {
		t.Fatal("source device lost")
	}
}
