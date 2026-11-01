package engineering

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"testing"
)

func SyntheticConnectivity(project, source, revision string) (bom.Workbook, EngineeringProposal, Extraction, error) {
	fields := []string{"a_device", "a_port", "a_parent_port", "z_device", "z_port", "a_optic", "z_optic", "medium", "length"}
	values := [][]string{{"Synthetic-A", "lane-1", "physical-1", "Synthetic-Z", "port-1", "Synthetic optic", "Synthetic optic", "fiber", "3"}, {"Synthetic-A", "lane-2", "physical-1", "Synthetic-Z", "port-2", "Synthetic optic", "Synthetic optic", "fiber", "4"}}
	sheet := bom.Sheet{Name: "Different customer connections", Cells: []bom.Cell{}}
	columns := []ColumnBinding{}
	for col, f := range fields {
		u := ""
		if f == "length" {
			u = "m"
		}
		columns = append(columns, ColumnBinding{Field: f, Column: col + 1, Unit: u})
	}
	for row, values := range values {
		for col, value := range values {
			sheet.Cells = append(sheet.Cells, bom.Cell{Sheet: sheet.Name, Ref: fmt.Sprintf("%c%d", 'A'+col, row+1), Row: row + 1, Column: col + 1, Value: value})
		}
	}
	wb := bom.Workbook{Sheets: []bom.Sheet{sheet}}
	p := EngineeringProposal{Summary: "Synthetic connections", Regions: []RegionMapping{{Sheet: sheet.Name, Kind: "cable", Bounds: Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: len(fields)}, Columns: columns}}, Modules: []ModuleDetection{{ModuleID: "connectivity_cabling", Applicable: true, Reason: "explicit endpoints", Sheets: []string{sheet.Name}}}, Questions: []string{}}
	x, e := ExtractPhysical(wb, p, project, source, revision)
	return wb, p, x, e
}
func TestConnectivityDedupAndStateSeparation(t *testing.T) {
	_, _, x, e := SyntheticConnectivity("project", "source", "revision")
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Model.Equipment) != 2 || len(x.Model.Ports) != 5 || len(x.Model.Connections) != 2 || len(x.Model.Cables) != 2 || len(x.Model.Optics) != 3 {
		t.Fatalf("incorrect expansion/physical-port optical dedup: %+v", x.Model)
	}
	for _, i := range x.Issues {
		if i.Severity == "BLOCK" {
			t.Fatal(i)
		}
	}
	if x.Model.Cables[0].AsBuilt != nil || x.Model.Cables[0].Calculated != nil || x.Model.Cables[0].Estimated == nil {
		t.Fatal("source estimate relabeled actual or calculated")
	}
	x.Model.Cables[0].SelectedStock = &LengthEvidence{Length: Quantity{"1", "m"}, EvidenceIDs: []string{"synthetic"}}
	if !hasIssue(ValidateConnectivity(x.Model), "CABLE_STOCK_TOO_SHORT") {
		t.Fatal("too short stock accepted")
	}
	x.Model.Connections[1].ZPortID = x.Model.Connections[0].ZPortID
	if !hasIssue(ValidateConnectivity(x.Model), "PORT_MULTIPLE_CONNECTIONS") {
		t.Fatal("duplicate physical termination accepted")
	}
}
func TestConnectivityRatesAndBreakoutCycle(t *testing.T) {
	if parseRate("400", "") != nil {
		t.Fatal("unitless source speed guessed")
	}
	if p := parseRate("2.5G", ""); p == nil || *p != 2500000000 {
		t.Fatal("explicit source speed not normalized")
	}
	_, _, x, e := SyntheticConnectivity("project", "source", "revision")
	if e != nil {
		t.Fatal(e)
	}
	x.Model.Ports[0].ParentPortID = &x.Model.Ports[1].EntityID
	if !hasIssue(ValidateConnectivity(x.Model), "BREAKOUT_CYCLE") {
		t.Fatal("cycle accepted")
	}
}

func TestConnectivityRequiresExplicitPatchPair(t *testing.T) {
	_, _, x, e := SyntheticConnectivity("project", "source", "revision")
	if e != nil {
		t.Fatal(e)
	}
	x.Model.Connections[0].PatchPortIDs = []string{x.Model.Ports[0].EntityID}
	if !hasIssue(ValidateConnectivity(x.Model), "PATCH_PASSTHROUGH_INCOMPLETE") {
		t.Fatal("single patch endpoint accepted as two cable ends")
	}
}

func TestCableStockDistributionNeedsEveryPhysicalSegment(t *testing.T) {
	m := EmptyPhysical("project")
	m.Connections = []PlannedConnection{{EntityVersion: EntityVersion{EntityID: "connection"}, APortID: "a", ZPortID: "z", PatchPortIDs: []string{"front", "rear"}}}
	first, second := 1, 2
	m.Cables = []CablePlan{{ConnectionID: "connection", SegmentIndex: &first, Spec: CableSpec{ID: "s", SourceModelLabel: "Synthetic"}, SelectedStock: &LengthEvidence{Length: Quantity{"3", "m"}}}, {ConnectionID: "connection", SegmentIndex: &second, Spec: CableSpec{ID: "s", SourceModelLabel: "Synthetic"}, SelectedStock: &LengthEvidence{Length: Quantity{"500", "cm"}}}}
	items, missing := DerivedWorkloads(m)
	if len(missing) != 0 || len(items) != 3 {
		t.Fatal("two external segment specs not counted independently", items, missing)
	}
	m.Cables[1].SelectedStock = nil
	_, missing = DerivedWorkloads(m)
	if len(missing) != 1 {
		t.Fatal("missing second-segment stock silently inferred", missing)
	}
	m.Cables[0].SegmentIndex = nil
	if !hasIssue(ValidateConnectivity(m), "CABLE_SEGMENT_REQUIRED") {
		t.Fatal("whole route assigned to arbitrary physical segment")
	}
}
