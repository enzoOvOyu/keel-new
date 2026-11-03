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

func TestKnownRackDoesNotRequireCompleteOccupancy(t *testing.T) {
	m := engineering.EmptyPhysical("p")
	start := 12
	v := func(id string) engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: engineering.StableID("p", "test", id), RevisionID: "revision", ProvenanceID: "provenance", EvidenceIDs: []string{"evidence"}, Maturity: engineering.Confirmed}
	}
	d := engineering.EquipmentPlan{EntityVersion: v("device"), EquipmentType: "server", Label: "planned server", IdentityKind: "planned_instance", Quantity: 1}
	rack := engineering.FacilityModel{EntityVersion: v("rack"), Kind: "rack", Label: "R1"}
	m.Equipment = []engineering.EquipmentPlan{d}
	m.Facilities = []engineering.FacilityModel{rack}
	m.Placements = []engineering.RackPlacement{{EntityVersion: v("placement"), EquipmentID: d.EntityID, RackID: rack.EntityID, StartU: &start}}
	subset, p, err := Select(m, Project(m, nil, "", nil), []string{d.EntityID})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PlacementIDs) != 0 || len(subset.Placements) != 0 || len(p.LocationIDs) != 1 || len(p.FacilityIDs) != 1 {
		t.Fatalf("known rack dropped or incomplete occupancy promoted: %+v", p)
	}
	known := KnownLocations(m, p)
	if len(known.Placements) != 1 || known.Placements[0].StartU == nil || *known.Placements[0].StartU != start || known.Placements[0].HeightU != nil || known.Facilities[0].CapacityU != nil {
		t.Fatal("source-backed location was lost or missing geometry invented")
	}
	// Older publication closures with no approved locations must stay unchanged.
	p.LocationIDs, p.FacilityIDs = nil, nil
	old := FrozenModel(m, p)
	if len(old.Placements) != 0 || len(old.Facilities) != 0 {
		t.Fatal("old publication acquired unapproved locations")
	}
}

func TestProjectSummaryPreservesEvidenceAndIssuesWithoutRelations(t *testing.T) {
	m := engineering.EmptyPhysical("p")
	entity := engineering.EntityVersion{EntityID: "device", Maturity: engineering.Confirmed}
	sn := "synthetic-serial"
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: entity, EquipmentType: "server", IdentityKind: "planned_instance", Quantity: 1, Label: "server", Serial: &sn}}
	m.Ports = []engineering.PlannedPort{{EntityVersion: engineering.EntityVersion{EntityID: "port"}, EquipmentID: "device", Name: "p1"}}
	ev := []engineering.SourceEvidence{{EntityID: "device", Sheet: "first", Cell: "A1"}, {EntityID: "device", Sheet: "second", Cell: "B2"}}
	summary := ProjectSummary(m, ev, "source", nil)
	full := Project(m, ev, "source", nil)
	if len(summary.Ports) != 0 || len(summary.Connections) != 0 || len(summary.Devices) != 1 || len(full.Ports) != 1 {
		t.Fatal("summary projected relation details or lost devices")
	}
	if summary.Devices[0].Evidence.Sheet != "first" || summary.Devices[0].Evidence != full.Devices[0].Evidence {
		t.Fatal("indexed evidence changed first-match semantics")
	}
	if len(summary.Issues) != len(full.Issues) {
		t.Fatal("summary lost validation issues")
	}
}

func TestPlacementIdentityIsDistinctFromEquipment(t *testing.T) {
	m := engineering.EmptyPhysical("p")
	device := engineering.StableID("p", "equipment", "one")
	m.Equipment = append(m.Equipment, engineering.EquipmentPlan{EntityVersion: engineering.EntityVersion{EntityID: device}})
	inputs := []ModelInput{{TableID: "racks", Rows: []map[string]string{{"rack_id": "rack-one"}}}, {TableID: "placements", Rows: []map[string]string{{"device_id": device, "rack_id": "rack-one", "start_u": "1", "height_u": "2"}}}}
	if err := ApplyInputs(&m, inputs); err != nil {
		t.Fatal(err)
	}
	if len(m.Placements) != 1 || m.Placements[0].EntityID == device {
		t.Fatal("placement reused equipment identity")
	}
	id := m.Placements[0].EntityID
	if err := ApplyInputs(&m, inputs); err != nil {
		t.Fatal(err)
	}
	if len(m.Placements) != 1 || m.Placements[0].EntityID != id {
		t.Fatal("placement identity changed on reapply")
	}
}
