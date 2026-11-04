package kickoff

import (
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestResolveIdentitySourceBackedName(t *testing.T) {
	for _, scenario := range []string{"name", "SN enrichment", "legacy SN", "different SN", "model only", "missing evidence", "placement conflict"} {
		t.Run(scenario, func(t *testing.T) {
			m := engineering.EmptyPhysical("p")
			version := func(id string) engineering.EntityVersion {
				return engineering.EntityVersion{EntityID: id, RevisionID: "revision", ProvenanceID: "source", EvidenceIDs: []string{"source-cell"}, Maturity: engineering.Confirmed}
			}
			m.Equipment = []engineering.EquipmentPlan{
				{EntityVersion: version("a"), EquipmentType: "server", Label: "Endpoint", SourceModelLabel: "model", IdentityKind: "planned_instance", Quantity: 1},
				{EntityVersion: version("b"), EquipmentType: "server", Label: "Endpoint", SourceModelLabel: "model", IdentityKind: "planned_instance", Quantity: 1},
			}
			sn, other := "SYN-A", "SYN-B"
			rejected := false
			switch scenario {
			case "SN enrichment":
				m.Equipment[0].Serial = &sn
			case "legacy SN":
				m.Equipment[0].Serial = &sn
				m.Equipment[1].Serial = &sn
				m.Equipment[1].Label = "Alias"
			case "different SN":
				m.Equipment[0].Serial = &sn
				m.Equipment[1].Serial = &other
				rejected = true
			case "model only":
				m.Equipment[1].Label = "Other"
				rejected = true
			case "missing evidence":
				m.Equipment[0].EvidenceIDs = nil
				rejected = true
			case "placement conflict":
				m.Placements = []engineering.RackPlacement{{EquipmentID: "a", RackID: "rack-a"}, {EquipmentID: "b", RackID: "rack-b"}}
				rejected = true
			}
			m.Ports = []engineering.PlannedPort{{EntityVersion: version("port"), EquipmentID: "a", Name: "1"}}
			before := engineering.Digest(m)
			err := ResolveIdentity(&m, "a", "b")
			if rejected {
				if err == nil {
					t.Fatal("unsupported identity association accepted")
				}
				if before != engineering.Digest(m) {
					t.Fatal("rejected association changed model")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Equipment) != 1 || m.Equipment[0].EntityID != "b" || m.Ports[0].EquipmentID != "b" {
				t.Fatal("references not associated")
			}
			if scenario == "SN enrichment" && (m.Equipment[0].Serial == nil || *m.Equipment[0].Serial != sn) {
				t.Fatal("explicit SN lost")
			}
			if scenario == "name" && m.Equipment[0].Serial != nil {
				t.Fatal("fabricated SN")
			}
			if _, _, err = Select(m, Project(m, nil, "", nil), []string{"b"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNameIdentityConflictBlocksPublication(t *testing.T) {
	m := engineering.EmptyPhysical("p")
	for _, id := range []string{"a", "b"} {
		m.Equipment = append(m.Equipment, engineering.EquipmentPlan{EntityVersion: engineering.EntityVersion{EntityID: id, RevisionID: "r", ProvenanceID: "p", EvidenceIDs: []string{"e"}, Maturity: engineering.Confirmed}, EquipmentType: "patch_panel", Label: "Panel", Quantity: 1, IdentityKind: "planned_instance"})
	}
	view := Project(m, nil, "", nil)
	if len(view.Devices) != 2 {
		t.Fatal("patch panel missing from review")
	}
	if _, _, err := Select(m, view, []string{"a"}); err == nil {
		t.Fatal("ambiguous name published")
	}
	if err := ResolveIdentity(&m, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Select(m, Project(m, nil, "", nil), []string{"b"}); err != nil {
		t.Fatal(err)
	}
}
