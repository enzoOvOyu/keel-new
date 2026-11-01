package engineering

import (
	"keel.local/dev/internal/bom"
	"testing"
)

func TestRecordBindingsAndIndependentModules(t *testing.T) {
	w := bom.Workbook{Sheets: []bom.Sheet{
		{Name: "任意参数区", Cells: []bom.Cell{{Ref: "C4", Sheet: "任意参数区", Row: 4, Column: 3, Value: "Synthetic-A"}, {Ref: "C8", Sheet: "任意参数区", Row: 8, Column: 3, Value: "server"}}},
		{Name: "数量", Cells: []bom.Cell{{Ref: "B2", Sheet: "数量", Row: 2, Column: 2, Value: "1"}}},
	}}
	p := EngineeringProposal{Summary: "transposed source", Regions: []RegionMapping{{Sheet: "任意参数区", Kind: "equipment", Bounds: Bounds{StartRow: 4, EndRow: 8, StartColumn: 3, EndColumn: 3}, Records: []RecordBinding{{Cells: []CellBinding{{Field: "label", Sheet: "任意参数区", Cell: "C4"}, {Field: "equipment_type", Sheet: "任意参数区", Cell: "C8"}, {Field: "quantity", Sheet: "数量", Cell: "B2", Unit: "count"}}}}}}, Modules: []ModuleDetection{{ModuleID: "equipment", Applicable: true, Reason: "设备参数"}, {ModuleID: "power_cooling", Applicable: true, Reason: "等待容量资料"}, {ModuleID: "network_design", Applicable: false, Reason: "未提供逻辑规划"}}}
	x, e := ExtractPhysical(w, p, "project", "source", "revision")
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Model.Equipment) != 1 || x.Model.Equipment[0].Label != "Synthetic-A" || x.Model.Equipment[0].Quantity != 1 || len(x.Facts) == 0 {
		t.Fatalf("record fields lost: %+v", x)
	}
	for _, f := range x.Facts {
		if e = f.Value.Validate(); e != nil {
			t.Fatal("invalid semantic type", e)
		}
	}
	for _, e := range x.Evidence {
		if e.SourceID != "source" {
			t.Fatal("source provenance lost")
		}
	}
	plan := BuildImportPlan("project", "source", "revision", 2, p, x.Model)
	view := EffectivePlan(plan, []TargetStatus{{Target: EquipmentTarget, State: Approved}}, nil)
	states := map[string]ModuleState{}
	for _, m := range view.Plan.Modules {
		states[m.DefinitionID] = m.State
	}
	if states["equipment"] != Approved || states["power_cooling"] != NeedsInput || states["network_design"] != NotApplicable {
		t.Fatal("module state leaked", states)
	}
	found := false
	for _, batch := range view.Batches {
		for _, id := range batch {
			if id == StableID("source", "module", "equipment") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("missing power input blocked equipment")
	}
	w.Sheets[1].Cells[0].Sensitive = true
	if _, e = ExtractPhysical(w, p, "project", "source", "revision"); e == nil {
		t.Fatal("sensitive record value extracted")
	}
}
