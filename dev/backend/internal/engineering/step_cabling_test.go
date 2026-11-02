package engineering

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCablingWorkbookGroundTruth(t *testing.T) {
	m := EmptyPhysical("fixture-project")
	for i, name := range []string{"01-equipment.xlsx", "02-placement.xlsx"} {
		b, e := os.ReadFile("testdata/five-step/" + name)
		if e != nil {
			t.Fatal(e)
		}
		inputs, e := ReadStepWorkbook(bytes.NewReader(b), i+1)
		if e != nil {
			t.Fatal(e)
		}
		for _, q := range inputs {
			m, e = MergeStandardInput(m, q)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	b, e := os.ReadFile("testdata/five-step/03-cabling.xlsx")
	if e != nil {
		t.Fatal(e)
	}
	inputs, e := ReadStepWorkbook(bytes.NewReader(b), 3)
	if e != nil {
		t.Fatal(e)
	}
	run := func(ref string, in []NamedValue) (Quantity, ID, VersionReference, error) {
		switch ref {
		case "fixture-lead-2m@1":
			return Quantity{"2", "m"}, "lead", VersionReference{"lead", 1}, nil
		case "fixture-slack-10pct@1":
			q, e := ApplyCableAllowance(Quantity{*in[0].Value.Decimal, *in[0].Value.Unit}, nil, "1.1")
			return q, "slack", VersionReference{"slack", 1}, e
		case "fixture-stock-ceiling@1":
			q, e := SelectCableStock(Quantity{*in[0].Value.Decimal, *in[0].Value.Unit}, []Quantity{{"20", "m"}, {"30", "m"}})
			return q, "stock", VersionReference{"stock", 1}, e
		}
		return Quantity{}, "", VersionReference{}, fmt.Errorf("unknown rule")
	}
	out, e := MergeCablingWorkbook(m, inputs, []ID{"source-evidence"}, run)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Cables) != 1 || len(out.Optics) != 2 || len(out.Materials) != 2 || len(out.Workloads) != 1 {
		t.Fatalf("wrong quantities: cables=%d optics=%d material=%d workload=%d", len(out.Cables), len(out.Optics), len(out.Materials), len(out.Workloads))
	}
	c := out.Cables[0]
	if !EqualQuantities(c.Calculated.Length, Quantity{"17.6", "m"}) || !EqualQuantities(c.SelectedStock.Length, Quantity{"20", "m"}) || c.AsBuilt != nil || len(c.Route) != 1 || !EqualQuantities(*c.Route[0].Distance, Quantity{"12", "m"}) {
		t.Fatalf("independent route/length truth differs: %+v", c)
	}
	for _, i := range ValidateConnectivity(out) {
		if i.Severity == "BLOCK" {
			t.Fatal(i)
		}
	}
	again, e := MergeCablingWorkbook(out, inputs, []ID{"source-evidence"}, run)
	if e != nil || len(again.Materials) != 2 || len(again.Cables) != 1 || len(again.Optics) != 2 {
		t.Fatal("repeat import duplicated entities", e)
	}
	for _, name := range []string{"route_edges", "cable_specs", "rack_access", "cabling_rules"} {
		bad := []InputPreviewRequest{}
		for _, q := range inputs {
			if q.TableID != name {
				bad = append(bad, q)
			}
		}
		if _, e = MergeCablingWorkbook(m, bad, []ID{"source-evidence"}, run); e == nil {
			t.Fatal("accepted missing", name)
		}
	}
	denied := func(string, []NamedValue) (Quantity, ID, VersionReference, error) {
		return Quantity{}, "", VersionReference{}, fmt.Errorf("unapproved")
	}
	if _, e = MergeCablingWorkbook(m, inputs, []ID{"source-evidence"}, denied); e == nil {
		t.Fatal("accepted unapproved rule")
	}
	noOptics := append([]InputPreviewRequest{}, inputs...)
	for i, q := range noOptics {
		if q.TableID == "cable_specs" {
			noOptics[i].CSV = strings.ReplaceAll(q.CSV, "separate-optics", "no-optics")
		}
	}
	revised, e := MergeCablingWorkbook(out, noOptics, []ID{"source-evidence"}, run)
	if e != nil || len(revised.Optics) != 0 || len(revised.Materials) != 1 {
		t.Fatal("stale optic demand survived plan change", e)
	}
	if len(out.Optics) != 2 || len(out.Materials) != 2 {
		t.Fatal("mutated earlier revision")
	}
	if rate, e := cablingRate("2.5", "Gbps"); e != nil || rate != 2500000000 {
		t.Fatal("fractional port rate", rate, e)
	}
	if len(m.Cables) != 0 || len(m.Optics) != 0 {
		t.Fatal("mutated input model")
	}
}
