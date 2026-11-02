package kickoff

import (
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestNamedRoomAndSite(t *testing.T) {
	model := engineering.EmptyPhysical("project")
	inputs := []ModelInput{{TableID: "rooms", Rows: []map[string]string{{"name": "Room A", "site": "Site A"}}}, {TableID: "racks", Rows: []map[string]string{{"rack_id": "Rack A", "room_id": "Room A", "capacity_u": "42"}}}}
	if err := ApplyInputs(&model, ResolveReferences(model, inputs)); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]engineering.FacilityModel{}
	for _, f := range model.Facilities {
		kinds[f.Kind] = f
	}
	if len(kinds) != 3 || kinds["room"].ParentID == nil || *kinds["room"].ParentID != kinds["site"].EntityID || kinds["rack"].ParentID == nil || *kinds["rack"].ParentID != kinds["room"].EntityID {
		t.Fatalf("explicit hierarchy not modeled: %+v", model.Facilities)
	}
}

func TestUnassignedRackDoesNotBlockPlacement(t *testing.T) {
	m := engineering.EmptyPhysical("p")
	n, start := 42, 1
	h := 2
	v := func(id string) engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: engineering.StableID("p", "test", id), RevisionID: "revision", ProvenanceID: "provenance", EvidenceIDs: []string{"evidence"}, Maturity: engineering.Confirmed}
	}
	device := engineering.EquipmentPlan{EntityVersion: v("device"), EquipmentType: "server", Label: "known planned instance", IdentityKind: "planned_instance", Quantity: 1}
	rack := engineering.FacilityModel{EntityVersion: v("rack"), Kind: "rack", Label: "unassigned rack", CapacityU: &n}
	m.Equipment = []engineering.EquipmentPlan{device}
	m.Facilities = []engineering.FacilityModel{rack}
	m.Placements = []engineering.RackPlacement{{EntityVersion: v("placement"), EquipmentID: device.EntityID, RackID: rack.EntityID, StartU: &start, HeightU: &h}}
	_, p, e := Select(m, Project(m, nil, "", nil), []string{device.EntityID})
	if e != nil || len(p.PlacementIDs) != 1 {
		t.Fatalf("optional room blocked explicit placement: %v %+v", e, p)
	}
}
