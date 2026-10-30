package bom

import (
	"strings"
	"testing"
)

func TestWorkbenchPreservesUnrecognizedRows(t *testing.T) {
	w := Workbook{Sheets: []Sheet{{Name: "Materials", Cells: []Cell{{Ref: "A1", Row: 1, Column: 1, Value: "category"}, {Ref: "B1", Row: 1, Column: 2, Value: "name"}, {Ref: "A2", Row: 2, Column: 1, Value: "nic"}, {Ref: "B2", Row: 2, Column: 2, Value: "ConnectX"}, {Ref: "A3", Row: 3, Column: 1, Value: "unknown information"}}}}}
	b := WorkbenchRows(w)
	if len(b.Records) != 2 || b.Records[1].Recognition != "unrecognized" {
		t.Fatal("source rows silently dropped", b)
	}
	i := WorkbenchItem(b.Records[0], "p")
	if i.Category != "material_plan" || i.Serial != "" || i.Fields["quantity"] != "" {
		t.Fatal("invented physical identity/quantity", i)
	}
	if len(MaterialPlanIssues(i)) < 2 {
		t.Fatal("missing quantity/unit must block")
	}
}
func TestWorkbenchIdentityDoesNotDependOnRow(t *testing.T) {
	r := ImportRecord{ID: "row-a", Item: Item{Category: "nic", Fields: map[string]string{"category": "nic", "name": "Adapter", "model": "CX", "quantity": "8", "unit": "张"}}}
	a := WorkbenchItem(r, "p")
	r.ID = "row-b"
	b := WorkbenchItem(r, "p")
	if Identity(a) != Identity(b) {
		t.Fatal("identity depends on source row")
	}
	if strings.Contains(Identity(a), "8") {
		t.Fatal("quantity must not change material identity")
	}
	if Identity(a) == Identity(WorkbenchItem(r, "other-project")) {
		t.Fatal("planning scope leaks")
	}
	if _, ok := r.Item.Fields["planning_identity"]; ok {
		t.Fatal("assembly mutated staged fields")
	}
}
func TestExplicitCableGraph(t *testing.T) {
	items := []Item{{UID: "server", Category: "server", Hostname: "srv", Fields: map[string]string{}}, {UID: "switch", Category: "switch", Hostname: "sw", Fields: map[string]string{}}, {UID: "p1", Category: "port", Parent: "srv", Port: "eth0", Fields: map[string]string{}}, {UID: "p2", Category: "port", Parent: "sw", Port: "swp1", Fields: map[string]string{}}, {UID: "c1", Category: "cable", Fields: map[string]string{"endpoint_a_device": "srv", "endpoint_a_port": "eth0", "endpoint_b_device": "sw", "endpoint_b_port": "swp1"}}}
	out := ResolveWorkbench(items)
	if out[2].Parent != "server" || out[4].A != "p1" || out[4].B != "p2" {
		t.Fatal("explicit graph not resolved", out)
	}
	items = append(items, Item{UID: "other", Hostname: "srv"})
	items[4].A = ""
	out = ResolveWorkbench(items)
	if out[4].A != "" {
		t.Fatal("ambiguous alias was guessed")
	}
}

func TestDescriptionIsNotAnIPAddress(t *testing.T) {
	i := Item{UID: "p", Category: "material_plan", Fields: map[string]string{"name": "AOC", "quantity": "2", "unit": "根", "description": "active optical cable"}}
	for _, e := range Validate([]Item{i}) {
		if e.Rule == "BOM-IP" {
			t.Fatal("description was mistakenly validated as an IP", e)
		}
	}
}
