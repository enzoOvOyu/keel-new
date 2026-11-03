package main

import (
	"encoding/json"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"reflect"
	"testing"
)

func kickoffPublicationTestVersion(id string) engineering.EntityVersion {
	return engineering.EntityVersion{EntityID: id, RevisionID: "revision", ProvenanceID: "source", EvidenceIDs: []string{"evidence"}, Maturity: engineering.Confirmed}
}

func kickoffPublicationFacilityFixture() engineering.PhysicalModel {
	m := engineering.EmptyPhysical("publication-facilities")
	site, room, capacity := "site", "room", 42
	m.Facilities = []engineering.FacilityModel{
		{EntityVersion: kickoffPublicationTestVersion(site), Kind: "site", Label: "Site"},
		{EntityVersion: kickoffPublicationTestVersion(room), Kind: "room", Label: "Room", ParentID: &site},
		{EntityVersion: kickoffPublicationTestVersion("rack"), Kind: "rack", Label: "Empty rack", ParentID: &room, CapacityU: &capacity},
		{EntityVersion: kickoffPublicationTestVersion("other-room"), Kind: "room", Label: "Other room", ParentID: &site},
	}
	return m
}

func TestKickoffPublicationFacilityOnlyClosure(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"site", []string{"site"}},
		{"room", []string{"site", "room"}},
		{"rack", []string{"site", "room", "rack"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			m := kickoffPublicationFacilityFixture()
			subset, p, err := kickoff.SelectScope(m, kickoff.Project(m, nil, "", nil), nil, []string{tc.id}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(p.FacilityIDs, tc.want) || len(subset.Facilities) != len(tc.want) || len(subset.Equipment) != 0 {
				t.Fatalf("facility-only closure: %+v", p)
			}
			if p.Counts["facilities"] != len(tc.want) || p.Counts["devices"] != 0 || p.Counts["locations"] != 0 || p.Counts["placements"] != 0 {
				t.Fatalf("facility-only counts: %+v", p.Counts)
			}
			frozen := kickoff.FrozenModel(m, p)
			if !reflect.DeepEqual(frozen.Facilities, subset.Facilities) {
				t.Fatal("frozen closure changed the selected facilities")
			}
		})
	}
	m := kickoffPublicationFacilityFixture()
	for _, ids := range [][]string{nil, {"missing"}, {"rack", "rack"}} {
		if _, _, err := kickoff.SelectScope(m, kickoff.Project(m, nil, "", nil), nil, ids, nil); err == nil {
			t.Fatalf("invalid facility scope accepted: %v", ids)
		}
	}
}

func TestKickoffPublicationFacilityFactsCompareContent(t *testing.T) {
	inputs := []kickoff.ModelInput{{TableID: "rooms", Version: 1, Rows: []map[string]string{{"room_id": "room", "address": "A"}, {"room_id": "other-room", "address": "B"}}}}
	base := kickoff.FacilityPublicationFacts(kickoffPublicationFacilityFixture(), inputs)
	m := kickoffPublicationFacilityFixture()
	for i := range m.Facilities {
		m.Facilities[i].RevisionID = "new-revision"
		m.Facilities[i].EvidenceIDs = []string{"new-evidence"}
		m.Facilities[i].ProvenanceID = "new-source"
	}
	reordered := []kickoff.ModelInput{{TableID: "rooms", Version: 99, Rows: []map[string]string{inputs[0].Rows[1], inputs[0].Rows[0]}}}
	for id, facts := range kickoff.FacilityPublicationFacts(m, reordered) {
		if kickoff.PublicationChanged(facts, base[id]) {
			t.Fatalf("revision, evidence or input ordering changed facility %s", id)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*engineering.FacilityModel)
	}{
		{"coordinates", func(f *engineering.FacilityModel) {
			x, frame := "1200", "room-frame"
			f.XMM, f.CoordinateFrameID = &x, &frame
		}},
		{"capacity", func(f *engineering.FacilityModel) { capacity := 48; f.CapacityU = &capacity }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := kickoffPublicationFacilityFixture()
			tc.change(&changed.Facilities[2])
			facts := kickoff.FacilityPublicationFacts(changed, inputs)
			if !kickoff.PublicationChanged(facts["rack"], base["rack"]) || kickoff.PublicationChanged(facts["room"], base["room"]) || kickoff.PublicationChanged(facts["other-room"], base["other-room"]) {
				t.Fatal("empty-rack change did not remain independently publishable")
			}
		})
	}
	changedInputs := []kickoff.ModelInput{{TableID: "rooms", Version: 2, Rows: []map[string]string{{"room_id": "room", "address": "C"}, inputs[0].Rows[1]}}}
	facts := kickoff.FacilityPublicationFacts(m, changedInputs)
	if !kickoff.PublicationChanged(facts["room"], base["room"]) || !kickoff.PublicationChanged(facts["rack"], base["rack"]) || kickoff.PublicationChanged(facts["other-room"], base["other-room"]) {
		t.Fatal("room input changes must affect that room and its selected descendants")
	}
}

func TestKickoffPublishedAncestorOnlyAdvancesExistingFacilityFacts(t *testing.T) {
	for _, change := range []string{"none", "device", "port", "placement", "rack", "input"} {
		t.Run(change, func(t *testing.T) {
			m := kickoffPublicationFacilityFixture()
			m.Equipment = []engineering.EquipmentPlan{{EntityVersion: kickoffPublicationTestVersion("device"), EquipmentType: "server", Label: "Device", IdentityKind: "planned_instance", Quantity: 1}}
			m.Ports = []engineering.PlannedPort{{EntityVersion: kickoffPublicationTestVersion("port"), EquipmentID: "device", Name: "eth1"}}
			start := 1
			m.Placements = []engineering.RackPlacement{{EntityVersion: kickoffPublicationTestVersion("placement"), EquipmentID: "device", RackID: "rack", StartU: &start}}
			inputs := []kickoff.ModelInput{{TableID: "racks", Rows: []map[string]string{{"rack_id": "rack", "name": "Original"}}}}
			baseline := kickoff.PublicationFacts(m, inputs)
			for id, values := range kickoff.FacilityPublicationFacts(m, inputs) {
				baseline[id] = values
			}
			baseline["unrelated"] = map[string]string{"equipment:unrelated": "original"}
			delete(baseline, "other-room") // Unpublished objects must stay new.
			before, err := json.Marshal(baseline)
			if err != nil {
				t.Fatal(err)
			}
			m.Facilities[1].Label = "Published room rename"
			switch change {
			case "device":
				m.Equipment[0].Label = "Unpublished device rename"
			case "port":
				m.Ports[0].Name = "eth2"
			case "placement":
				start = 3
			case "rack":
				capacity := 48
				m.Facilities[2].CapacityU = &capacity
			case "input":
				inputs[0].Rows[0]["name"] = "Unpublished input"
			}
			frozen, _, err := kickoff.SelectScope(m, kickoff.Project(m, nil, "", nil), nil, []string{"room"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			published := kickoff.FacilityPublicationFacts(frozen, inputs)
			syncKickoffPublishedFacilityFacts(baseline, frozen, published)
			actual := kickoff.PublicationFacts(m, inputs)
			for id, values := range kickoff.FacilityPublicationFacts(m, inputs) {
				actual[id] = values
			}
			if got := kickoff.PublicationChanged(actual["device"], baseline["device"]); got != (change != "none") {
				t.Fatalf("device changed=%v after room publication, unpublished change=%s", got, change)
			}
			if got := kickoff.PublicationChanged(actual["rack"], baseline["rack"]); got != (change == "rack" || change == "input") {
				t.Fatalf("rack changed=%v after room publication, unpublished change=%s", got, change)
			}
			var expected map[string]map[string]string
			if err := json.Unmarshal(before, &expected); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"device", "rack", "room"} {
				expected[id]["facility:room"] = published["room"]["facility:room"]
			}
			if !reflect.DeepEqual(baseline, expected) {
				t.Fatal("ancestor publication changed unrelated facts, added keys or created a baseline")
			}
		})
	}
}

func TestKickoffPublicationKnownLocationKeepsAncestors(t *testing.T) {
	m := kickoffPublicationFacilityFixture()
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: kickoffPublicationTestVersion("device"), EquipmentType: "server", Label: "Device", IdentityKind: "planned_instance", Quantity: 1}}
	start, height := 1, 2
	m.Placements = []engineering.RackPlacement{{EntityVersion: kickoffPublicationTestVersion("placement"), EquipmentID: "device", RackID: "rack", StartU: &start}}
	_, p, err := kickoff.Select(m, kickoff.Project(m, nil, "", nil), []string{"device"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.LocationIDs, []string{"device"}) || !reflect.DeepEqual(p.FacilityIDs, []string{"site", "room", "rack"}) || p.Counts["locations"] != 1 || p.Counts["placements"] != 0 {
		t.Fatalf("partial location closure: %+v", p)
	}
	known, frozen := kickoff.KnownLocations(m, p), kickoff.FrozenModel(m, p)
	if len(known.Placements) != 1 || len(frozen.Placements) != 0 || len(known.Facilities) != 3 || len(frozen.Facilities) != 3 {
		t.Fatal("partial location lost ancestors or became complete occupancy")
	}
	m.Placements[0].HeightU = &height
	_, p, err = kickoff.Select(m, kickoff.Project(m, nil, "", nil), []string{"device"})
	if err != nil || p.Counts["placements"] != 1 || p.Counts["locations"] != 0 {
		t.Fatalf("complete occupancy counts: %+v, %v", p.Counts, err)
	}
}

func TestKickoffPublicationExistingPeerPort(t *testing.T) {
	m := engineering.EmptyPhysical("publication-existing-peer")
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: kickoffPublicationTestVersion("device"), EquipmentType: "server", Label: "Device", IdentityKind: "planned_instance", Quantity: 1}}
	m.Ports = []engineering.PlannedPort{{EntityVersion: kickoffPublicationTestVersion("local"), EquipmentID: "device", Name: "eth1"}}
	m.Connections = []engineering.PlannedConnection{{EntityVersion: kickoffPublicationTestVersion("link"), APortID: "local", ZPortID: "peer", PatchPortIDs: []string{"patch-in", "patch-out"}}}
	refs := map[string]engineering.PlannedPort{
		"peer":      {EntityVersion: engineering.EntityVersion{EntityID: "peer"}, EquipmentID: "existing-switch", Name: "eth2"},
		"patch-in":  {EntityVersion: engineering.EntityVersion{EntityID: "patch-in"}, EquipmentID: "existing-panel", Name: "in"},
		"patch-out": {EntityVersion: engineering.EntityVersion{EntityID: "patch-out"}, EquipmentID: "existing-panel", Name: "out"},
	}
	view := kickoff.Project(m, nil, "", nil)
	_, p, err := kickoff.SelectScope(m, view, []string{"device"}, nil, refs)
	if err != nil {
		t.Fatal(err)
	}
	frozen := kickoff.FrozenModel(m, p)
	if !reflect.DeepEqual(p.ConnectionIDs, []string{"link"}) || !reflect.DeepEqual(p.PortIDs, []string{"local"}) || len(frozen.Connections) != 1 || len(frozen.Equipment) != 1 || len(frozen.Ports) != 1 {
		t.Fatalf("reference peer was lost or republished: %+v", p)
	}
	view.Issues = append(view.Issues, kickoff.Issue{ID: "identity-conflict", Scope: "identity", EntityID: "existing-switch", Title: "Identity conflict"})
	if _, _, err = kickoff.SelectScope(m, view, []string{"device"}, nil, refs); err == nil {
		t.Fatal("CMDB reference bypassed an unresolved identity issue")
	}
	if _, _, err = kickoff.SelectScope(m, kickoff.Project(m, nil, "", nil), []string{"device"}, nil, nil); err != nil {
		t.Fatalf("old device-only scope was broken: %v", err)
	}
}

func TestKickoffPublicationFacilityRequestDigestAndJSON(t *testing.T) {
	digest := kickoffPublicationRequestDigest([]string{"b", "a"}, []string{"rack", "room"})
	if digest != kickoffPublicationRequestDigest([]string{"a", "b"}, []string{"room", "rack"}) || digest == kickoffPublicationRequestDigest([]string{"a", "b"}, []string{"room"}) || digest == kickoffPublicationRequestDigest([]string{"a", "b"}, nil) {
		t.Fatal("idempotency must bind both canonical request scopes")
	}
	if kickoffPublicationRequestDigest(nil, nil) != kickoffPublicationRequestDigest([]string{}, []string{}) {
		t.Fatal("omitted and empty optional scopes differ")
	}
	raw, err := json.Marshal(kickoff.Publication{FacilityIDs: []string{"rack"}})
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		FacilityIDs []string `json:"facilityIds"`
	}
	if json.Unmarshal(raw, &receipt) != nil || !reflect.DeepEqual(receipt.FacilityIDs, []string{"rack"}) {
		t.Fatal("publication omitted facilityIds")
	}
	raw, err = json.Marshal(kickoffPublicationOptions{Facilities: []kickoffPublicationFacilityOption{{kickoffPublicationOption: kickoffPublicationOption{ID: "rack", State: "changed", Reason: "Coordinates changed"}, Label: "Empty rack", Kind: "rack"}}})
	if err != nil {
		t.Fatal(err)
	}
	var options struct {
		Facilities []map[string]string `json:"facilities"`
	}
	if json.Unmarshal(raw, &options) != nil || len(options.Facilities) != 1 || options.Facilities[0]["id"] != "rack" || options.Facilities[0]["state"] != "changed" || options.Facilities[0]["label"] != "Empty rack" || options.Facilities[0]["kind"] != "rack" || options.Facilities[0]["reason"] == "" {
		t.Fatal("facility options lost UI fields")
	}
}
