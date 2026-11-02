package engineering

import (
	"strings"
	"testing"
)

func TestEndpointPlacementReferencesReuseExactSerials(t *testing.T) {
	terms := standardRequest("terminations", []string{"S1", "02:00:00:00:00:01", "eth1", "S2", "", "02:00:00:00:00:02", "eth2"})
	places := standardRequest("placements", []string{"S1", "", "Rack-A", "", "12", "", "", ""})
	inputs := []ModelingInput{{TableID: "terminations", CSV: terms.CSV}, {TableID: "placements", Version: 3, CSV: places.CSV}}
	refs := ResolveEndpointPlacements(EmptyPhysical("p"), inputs)
	if len(refs) != 2 || refs[0].State != "candidate" || refs[0].RackCode != "Rack-A" || *refs[0].StartU != 12 || refs[0].InputVersion != 3 || refs[0].InputRows[0] != 2 || refs[1].State != "missing" {
		t.Fatalf("source location relation lost: %+v", refs)
	}
	before := Digest(inputs)
	inputs[1].CSV += "S1,,Rack-B,,20,,,\n"
	if got := ResolveEndpointPlacements(EmptyPhysical("p"), inputs); got[0].State != "conflict" || len(got[0].InputRows) != 2 {
		t.Fatal("conflicting source positions not surfaced")
	}
	inputs[1].CSV = places.CSV
	sn := "S1"
	u := 30
	m := EmptyPhysical("p")
	m.Equipment = append(m.Equipment, EquipmentPlan{EntityVersion: EntityVersion{EntityID: "existing-equipment"}, Serial: &sn})
	m.Facilities = append(m.Facilities, FacilityModel{EntityVersion: EntityVersion{EntityID: "existing-rack"}, Kind: "rack", Code: "Confirmed-Rack"})
	m.Placements = append(m.Placements, RackPlacement{EntityVersion: EntityVersion{EntityID: "existing-placement", Maturity: Confirmed}, EquipmentID: "existing-equipment", RackID: "existing-rack", StartU: &u})
	modelBefore := Digest(m)
	refs = ResolveEndpointPlacements(m, inputs)
	if refs[0].State != "conflict" || refs[0].EquipmentID != "existing-equipment" || refs[0].PlacementID != "existing-placement" || refs[0].RackCode != "Confirmed-Rack" || *refs[0].StartU != 30 || Digest(m) != modelBefore || Digest(inputs) != before {
		t.Fatal("projection overwrote a confirmed location or source")
	}
}

func TestEndpointIdentityAdoptionDoesNotRequirePlacement(t *testing.T) {
	q := standardRequest("terminations", []string{"S1", "02:00:00:00:00:01", "eth1", "S2", "", "02:00:00:00:00:02", "eth2"})
	m, err := MergeTerminations(EmptyPhysical("p"), q)
	if err != nil || len(m.Connections) != 1 || len(m.Equipment) != 2 || len(m.Placements) != 0 {
		t.Fatalf("identity is still gated by placement: %v", err)
	}
	partial, err := MergeStandardInput(m, standardRequest("placements", []string{"S1", "", "Rack-A", "u", "12", "1", "front", ""}))
	if err != nil {
		t.Fatal("missing room configuration blocked planning input", err)
	}
	missingParent := false
	for _, issue := range ValidatePhysical(partial) {
		missingParent = missingParent || issue.Code == "FACILITY_PARENT_REQUIRED"
	}
	if !missingParent {
		t.Fatal("incomplete facility relationship was marked release-ready")
	}
	if _, err = MergeCablingWorkbook(m, nil, nil, nil); err == nil || !strings.HasPrefix(err.Error(), "CABLE_INPUT_REQUIRED") {
		t.Fatal("cabling prerequisites bypassed", err)
	}
	withDeclaration := standardRequest("terminations", []string{"S1", "02:00:00:00:00:01", "eth1", "S2", "source-location", "02:00:00:00:00:02", "eth2"})
	m, err = MergeTerminations(m, withDeclaration)
	if err != nil {
		t.Fatal(err)
	}
	m, err = MergeTerminations(m, q)
	if err != nil || m.Connections[0].BRackPosition != "source-location" {
		t.Fatal("empty declaration overwrote existing evidence", err)
	}
}
