package engineering

import "testing"

func TestWorkloadSummaryUsesStockAndDoesNotInventLabor(t *testing.T) {
	m := EmptyPhysical("project")
	m.Equipment = []EquipmentPlan{{EntityVersion: EntityVersion{EntityID: "e"}, EquipmentType: "server", SourceModelLabel: "Synthetic", Quantity: 3}}
	m.Connections = []PlannedConnection{{EntityVersion: EntityVersion{EntityID: "c"}, APortID: "a", ZPortID: "z"}}
	m.Cables = []CablePlan{{EntityVersion: EntityVersion{EntityID: "plan"}, ConnectionID: "c", Spec: CableSpec{ID: "spec", SourceModelLabel: "Synthetic cable"}, Estimated: &LengthEvidence{Length: Quantity{"0.8", "m"}}, SelectedStock: &LengthEvidence{Length: Quantity{"100", "cm"}}}}
	items, errors := DerivedWorkloads(m)
	if len(errors) != 0 || len(items) != 3 {
		t.Fatalf("bad quantities %+v %+v", items, errors)
	}
	for _, v := range items {
		if v.LaborHours != nil {
			t.Fatal("labor invented")
		}
		switch v.WorkType {
		case "equipment_install":
			if v.Quantity.Value != "3" {
				t.Fatal("equipment group count lost")
			}
		case "cable_stock_distribution":
			if v.StandardLength.Value != "1" || v.StandardLength.Unit != "m" {
				t.Fatal("stock units not normalized")
			}
		}
	}
	m.Cables[0].SelectedStock = nil
	run := WorkloadSummary(m, "revision")
	if run.Status != "partial" || len(run.Outputs) != 2 {
		t.Fatal("estimated length used as purchased stock")
	}
}
func TestMaterialRuleAndUnitsRemainExplicit(t *testing.T) {
	m := EmptyPhysical("project")
	m.Materials = []MaterialEstimate{{EntityVersion: EntityVersion{EntityID: "m"}, SpecRef: "s", SourceModelLabel: "Synthetic", Required: Quantity{"10", "count"}, Purchased: &Quantity{"3", "count"}}}
	if len(ValidateEstimates(m)) != 0 || m.Materials[0].PurchaseGap != nil {
		t.Fatal("gap invented without project rule")
	}
	m.Materials[0].Spares = &Quantity{"1", "m"}
	if !hasIssue(ValidateEstimates(m), "MATERIAL_UNIT_MISMATCH") {
		t.Fatal("incompatible procurement unit accepted")
	}
	m.Workloads = []WorkloadEstimate{{EntityVersion: EntityVersion{EntityID: "w"}, WorkType: "equipment_install", Quantity: Quantity{"1", "count"}, LaborHours: &Quantity{"60", "min"}}}
	for _, i := range ValidateEstimates(m) {
		if i.Code == "WORKLOAD_TIME_INVALID" {
			t.Fatal("explicit minutes not a valid labor unit")
		}
	}
}
