package engineering

import (
	"keel.local/dev/internal/bom"
	"testing"
)

func TestPowerBalanceSeparatesBasisAndUnknown(t *testing.T) {
	m := EmptyPhysical("project")
	m.Equipment = []EquipmentPlan{{EntityVersion: EntityVersion{EntityID: "a"}, Quantity: 1}, {EntityVersion: EntityVersion{EntityID: "b"}, Quantity: 1}}
	m.Placements = []RackPlacement{{EquipmentID: "a", RackID: "r"}, {EquipmentID: "b", RackID: "r"}}
	m.Facilities = []FacilityModel{{EntityVersion: EntityVersion{EntityID: "r"}, Kind: "rack"}}
	m.PowerPlans = []PowerPlan{{EntityVersion: EntityVersion{EntityID: "p"}, EquipmentProfiles: []EquipmentPowerProfile{{EquipmentID: "a", Basis: "planned", Power: Quantity{"1200", "W"}}, {EquipmentID: "b", Basis: "planned", Power: Quantity{"0.8", "kW"}}, {EquipmentID: "a", Basis: "measured", Power: Quantity{"9999", "W"}}}, Racks: []RackPowerPlan{{RackID: "r", Capacity: &Quantity{"3", "kW"}}}}}
	run := PowerBalance(m, "revision")
	if run.Status != "succeeded" || len(run.Outputs) != 2 || *run.Outputs[0].Value.Decimal != "2000" || *run.Outputs[1].Value.Decimal != "1000" {
		t.Fatalf("wrong independent balance: %+v", run)
	}
	m.PowerPlans[0].Racks[0].PlannedLoad = &Quantity{"2", "kW"}
	m.PowerPlans[0].Racks[0].CoolingCapacity = &Quantity{"1.5", "kW"}
	m.PowerPlans[0].Racks[0].CalculationRunID = &run.ID
	if !hasIssue(ValidatePower(m), "RACK_COOLING_EXCEEDED") {
		t.Fatal("cooling overload missed")
	}
	m.PowerPlans[0].EquipmentProfiles = m.PowerPlans[0].EquipmentProfiles[:1]
	run = PowerBalance(m, "revision")
	if run.Status != "failed" || len(run.Outputs) != 0 {
		t.Fatal("missing profile was silently treated as zero")
	}
}
func TestPowerExtractionUsesExplicitUnitsAndBasis(t *testing.T) {
	fields := []string{"device", "power", "power_basis", "rack", "power_capacity"}
	values := []string{"Synthetic-X", "2", "nameplate", "Synthetic-R", "8"}
	sheet := bom.Sheet{Name: "任意参数区域"}
	columns := []ColumnBinding{}
	for i, f := range fields {
		u := ""
		if f == "power" || f == "power_capacity" {
			u = "kW"
		}
		columns = append(columns, ColumnBinding{Field: f, Column: i + 1, Unit: u})
		sheet.Cells = append(sheet.Cells, bom.Cell{Sheet: sheet.Name, Ref: string(rune('A'+i)) + "1", Row: 1, Column: i + 1, Value: values[i]})
	}
	wb := bom.Workbook{Sheets: []bom.Sheet{sheet}}
	p := EngineeringProposal{Summary: "Power source", Regions: []RegionMapping{{Sheet: sheet.Name, Kind: "power", Bounds: Bounds{StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 5}, Columns: columns}}}
	x, e := ExtractPhysical(wb, p, "project", "source", "revision")
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Model.PowerPlans) != 1 || len(x.Model.PowerPlans[0].Racks) != 1 || x.Model.PowerPlans[0].EquipmentProfiles[0].Basis != "nameplate" || x.Model.PowerPlans[0].Racks[0].PlannedLoad != nil {
		t.Fatal("power semantics or unknown load lost")
	}
	if len(x.Model.Equipment) != 1 || len(x.Model.Facilities) != 1 || x.Model.PowerPlans[0].Racks[0].RackID != x.Model.Facilities[0].EntityID {
		t.Fatal("parallel physical identity")
	}
}
