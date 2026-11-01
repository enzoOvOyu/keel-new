package main

import (
	"testing"

	"keel.local/dev/internal/engineering"
)

func TestCableCalculationTracksRelevantInputs(t *testing.T) {
	m := engineering.EmptyPhysical("project")
	one, two := "10", "20"
	run := "run"
	m.Facilities = []engineering.FacilityModel{{EntityVersion: engineering.EntityVersion{EntityID: "rack-used"}, Kind: "rack", XMM: &one}, {EntityVersion: engineering.EntityVersion{EntityID: "rack-unrelated"}, Kind: "rack", XMM: &one}}
	c := engineering.CablePlan{EntityVersion: engineering.EntityVersion{EntityID: "cable"}, ConnectionID: "connection", Route: []engineering.RouteSegment{{FromID: "rack-used", ToID: "destination"}}, Calculated: &engineering.LengthEvidence{Length: engineering.Quantity{Value: "3", Unit: "m"}, CalculationRunID: &run}, SelectedStock: &engineering.LengthEvidence{Length: engineering.Quantity{Value: "5", Unit: "m"}, CalculationRunID: &run}, AsBuilt: &engineering.LengthEvidence{Length: engineering.Quantity{Value: "4", Unit: "m"}}}
	m.Cables = []engineering.CablePlan{c}
	changed := m
	changed.Facilities = append([]engineering.FacilityModel{}, m.Facilities...)
	changed.Cables = append([]engineering.CablePlan{}, m.Cables...)
	changed.Facilities[1].XMM = &two
	x := engineering.Extraction{Model: changed}
	invalidateCableInputs(m, &x)
	if x.Model.Cables[0].Calculated == nil {
		t.Fatal("unrelated rack invalidated cable")
	}
	x.Model.Facilities[0].XMM = &two
	invalidateCableInputs(m, &x)
	if x.Model.Cables[0].Calculated != nil || x.Model.Cables[0].SelectedStock != nil || x.Model.Cables[0].AsBuilt == nil {
		t.Fatal("changed input left stale results or erased actual evidence")
	}
	for _, field := range []string{"estimated", "calculated", "confirmed"} {
		t.Run(field, func(t *testing.T) {
			x := engineering.Extraction{Model: m}
			x.Model.Cables = append([]engineering.CablePlan{}, m.Cables...)
			length := &engineering.LengthEvidence{Length: engineering.Quantity{Value: "6", Unit: "m"}}
			switch field {
			case "estimated":
				x.Model.Cables[0].Estimated = length
			case "calculated":
				x.Model.Cables[0].Calculated = length
			case "confirmed":
				x.Model.Cables[0].Confirmed = length
			}
			x.Evidence = []engineering.SourceEvidence{{EntityID: "cable", Field: "standard_length", CalculationRunID: &run}, {EntityID: "cable", Field: "length"}}
			invalidateCableInputs(m, &x)
			if x.Model.Cables[0].SelectedStock != nil || x.Model.Cables[0].Calculated == nil || x.Model.Cables[0].AsBuilt == nil || len(x.Evidence) != 1 || x.Evidence[0].CalculationRunID != nil {
				t.Fatal("length changed without invalidating derived stock or lost independent evidence")
			}
		})
	}
	t.Run("source stock", func(t *testing.T) {
		x := engineering.Extraction{Model: m}
		x.Model.Cables = append([]engineering.CablePlan{}, m.Cables...)
		x.Model.Cables[0].SelectedStock = &engineering.LengthEvidence{Length: engineering.Quantity{Value: "5", Unit: "m"}}
		x.Model.Cables[0].Estimated = &engineering.LengthEvidence{Length: engineering.Quantity{Value: "6", Unit: "m"}}
		invalidateCableInputs(m, &x)
		if x.Model.Cables[0].SelectedStock == nil {
			t.Fatal("source stock removed instead of retaining it for validation")
		}
	})
}
