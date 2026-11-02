package engineering

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func standardPowerFixture(t *testing.T) (PhysicalModel, map[string]CalculationRule) {
	t.Helper()
	m := EmptyPhysical("power-standard-test")
	rules := map[string]CalculationRule{}
	for _, file := range []struct {
		name string
		step int
	}{{"01-equipment.xlsx", 1}, {"02-placement.xlsx", 2}, {"05-power.xlsx", 5}} {
		data, e := os.ReadFile("testdata/five-step/" + file.name)
		if e != nil {
			t.Fatal(e)
		}
		inputs, e := ReadStepWorkbook(bytes.NewReader(data), file.step)
		if e != nil {
			t.Fatal(e)
		}
		for _, q := range inputs {
			definitions, e := PowerRuleDefinitions(m.ProjectID, q)
			if e != nil {
				t.Fatal(e)
			}
			for _, r := range definitions {
				if e := ValidateRule(r); e != nil {
					t.Fatal(e)
				}
				r.ApprovalStatus = "approved"
				rules[r.ID+"@"+fmt.Sprint(r.Version)] = r
			}
			m, e = MergeStandardInput(m, q)
			if e != nil {
				t.Fatal(file.name, q.TableID, e)
			}
		}
	}
	return m, rules
}
func TestScopedPowerWorkbookIndependentExpectations(t *testing.T) {
	m, rules := standardPowerFixture(t)
	if len(m.PowerPlans) != 1 || m.PowerPlans[0].Scope == nil || len(m.PowerPlans[0].Circuits) != 2 || len(m.PowerPlans[0].Supplies) != 2 {
		t.Fatal("power scope or supply relationships missing")
	}
	run, e := ScopedPowerBalance(m, "revision", rules)
	if e != nil || run.Status != "succeeded" || run.ProviderVersion != "2" {
		t.Fatal(run, e)
	}
	values := map[string]TypedValue{}
	for _, v := range run.Outputs {
		values[v.Name] = v.Value
	}
	expectedLoad := map[string]string{"R1": "500", "R2": "300"}
	expectedRemaining := map[string]string{"R1": "1500", "R2": "1700"}
	for _, rack := range m.Facilities {
		if rack.Kind != "rack" {
			continue
		}
		load := values[rack.EntityID+"/load"]
		remaining := values[rack.EntityID+"/remaining"]
		if load.Decimal == nil || *load.Decimal != expectedLoad[rack.Code] || remaining.Decimal == nil || *remaining.Decimal != expectedRemaining[rack.Code] {
			t.Fatal("wrong rack balance", rack.Code, load, remaining)
		}
	}
	for _, c := range m.PowerPlans[0].Circuits {
		load := values[c.ID+"/load"]
		remaining := values[c.ID+"/remaining"]
		want := "500"
		rest := "1500"
		if c.Code == "CIRCUIT-R2-A" {
			want = "300"
			rest = "1700"
		}
		if load.Decimal == nil || *load.Decimal != want || remaining.Decimal == nil || *remaining.Decimal != rest {
			t.Fatal("wrong circuit balance", c.Code)
		}
	}
	for key, r := range rules {
		r.ApprovalStatus = "draft"
		rules[key] = r
	}
	if _, e := ScopedPowerBalance(m, "revision", rules); e == nil {
		t.Fatal("unapproved rule used")
	}
}
func TestScopedPowerBlocksCyclesAndUnprovenRedundancy(t *testing.T) {
	m, rules := standardPowerFixture(t)
	m.PowerPlans[0].Circuits[0].UpstreamCode = m.PowerPlans[0].Circuits[1].Code
	m.PowerPlans[0].Circuits[1].UpstreamCode = m.PowerPlans[0].Circuits[0].Code
	if _, e := ScopedPowerBalance(m, "revision", rules); e == nil {
		t.Fatal("power circuit cycle accepted")
	}
	m, rules = standardPowerFixture(t)
	m.PowerPlans[0].Scope.RedundancyMode = "dual_independent"
	if _, e := ScopedPowerBalance(m, "revision", rules); e == nil {
		t.Fatal("single feed declared redundant")
	}
	m, rules = standardPowerFixture(t)
	for key, r := range rules {
		factor := Quantity{"0.5", "1"}
		r.Expression.Arguments[1].Literal = &factor
		rules[key] = r
	}
	if _, e := ScopedPowerBalance(m, "revision", rules); e == nil {
		t.Fatal("lost power across PSU allocation accepted")
	}
}
