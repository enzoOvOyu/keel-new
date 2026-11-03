package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"testing"
)

func cableAdoptionFixture() (engineering.PhysicalModel, ai.KickoffAgentCandidate) {
	a, b := "SYN-A", "SYN-B"
	m := engineering.PhysicalModel{ProjectID: "synthetic", Equipment: []engineering.EquipmentPlan{
		{EntityVersion: engineering.EntityVersion{EntityID: "a"}, Serial: &a},
		{EntityVersion: engineering.EntityVersion{EntityID: "b"}, Serial: &b},
	}, Ports: []engineering.PlannedPort{
		{EntityVersion: engineering.EntityVersion{EntityID: "pa"}, EquipmentID: "a", Name: "1"},
		{EntityVersion: engineering.EntityVersion{EntityID: "pb"}, EquipmentID: "b", Name: "2"},
	}}
	row := map[string]string{"a_serial": a, "b_serial": b, "a_port": "1", "b_port": "2", "cable_model": "Synthetic cable", "medium": "fiber", "cable_length_m": "3.5", "a_optic_model": "Synthetic optic A", "b_optic_model": "Synthetic optic B"}
	fields := map[string]ai.KickoffAgentCell{}
	for field := range row {
		fields[field] = ai.KickoffAgentCell{Sheet: "Synthetic", Cell: field + "1"}
	}
	return m, ai.KickoffAgentCandidate{TableID: "connections", Rows: []map[string]string{row}, Evidence: []ai.KickoffAgentEvidence{{Row: 1, Fields: fields}}}
}

func TestAdoptionCableNewAndExactPair(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "reversed existing"}[existing], func(t *testing.T) {
			m, c := cableAdoptionFixture()
			if existing {
				m.Connections = []engineering.PlannedConnection{{EntityVersion: engineering.EntityVersion{EntityID: "existing", Maturity: engineering.AsBuilt}, APortID: "pb", ZPortID: "pa"}}
			}
			var ev []engineering.SourceEvidence
			if !kickoffAgentMergePhysical(&m, &ev, c, "source") {
				t.Fatal("no change")
			}
			if len(m.Connections) != 1 || len(m.Cables) != 1 || len(m.Optics) != 0 {
				t.Fatal("unexpected entities")
			}
			cable := m.Cables[0]
			if cable.ConnectionID != m.Connections[0].EntityID || cable.EntityID != engineering.StableID(m.ProjectID, "cable-plan", cable.ConnectionID) || cable.Spec.ID == "" || cable.Maturity != engineering.Estimated || cable.AsBuilt != nil || cable.Confirmed != nil {
				t.Fatalf("bad association: %+v", cable)
			}
			if cable.Spec.SourceModelLabel != "Synthetic cable" || cable.Spec.Medium != "fiber" || cable.Estimated == nil || cable.Estimated.Length != (engineering.Quantity{Value: "3.5", Unit: "m"}) || len(cable.Estimated.EvidenceIDs) != 1 {
				t.Fatalf("missing source spec: %+v", cable)
			}
			for _, field := range []string{"a_optic_model", "b_optic_model"} {
				found := false
				for _, e := range ev {
					if e.EntityID == cable.ConnectionID && e.Field == field {
						found = true
					}
				}
				if !found {
					t.Fatal("optic source lost", field)
				}
			}
			before := engineering.Digest(m)
			evidenceBefore := engineering.Digest(ev)
			if kickoffAgentMergePhysical(&m, &ev, c, "source") || before != engineering.Digest(m) || evidenceBefore != engineering.Digest(ev) {
				t.Fatal("repeat not idempotent")
			}
		})
	}
}

func TestAdoptionCablePreservesExisting(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(map[bool]string{false: "blank enrichment", true: "observed"}[observed], func(t *testing.T) {
			m, c := cableAdoptionFixture()
			m.Connections = []engineering.PlannedConnection{{EntityVersion: engineering.EntityVersion{EntityID: "existing"}, APortID: "pa", ZPortID: "pb"}}
			m.Cables = []engineering.CablePlan{{EntityVersion: engineering.EntityVersion{EntityID: "cable", Maturity: engineering.Confirmed}, ConnectionID: "existing", Spec: engineering.CableSpec{ID: "spec", SourceModelLabel: "Keep model"}, Estimated: &engineering.LengthEvidence{Length: engineering.Quantity{Value: "7", Unit: "m"}}}}
			if observed {
				m.Cables[0].Maturity = engineering.AsBuilt
				m.Cables[0].AsBuilt = &engineering.LengthEvidence{Length: engineering.Quantity{Value: "8", Unit: "m"}}
			}
			var ev []engineering.SourceEvidence
			kickoffAgentMergePhysical(&m, &ev, c, "source")
			cable := m.Cables[0]
			if len(m.Cables) != 1 || cable.EntityID != "cable" || cable.Spec.ID != "spec" || cable.Spec.SourceModelLabel != "Keep model" || cable.Estimated.Length.Value != "7" {
				t.Fatal("existing values overwritten")
			}
			if observed {
				if cable.Spec.Medium != "" || cable.AsBuilt.Length.Value != "8" || cable.Maturity != engineering.AsBuilt {
					t.Fatal("observed changed")
				}
			} else if cable.Spec.Medium != "fiber" || cable.Maturity != engineering.Confirmed {
				t.Fatal("blank not enriched")
			}
			c.Rows[0]["medium"] = "copper"
			kickoffAgentMergePhysical(&m, &ev, c, "other-source")
			if m.Cables[0].Spec.Medium != cable.Spec.Medium {
				t.Fatal("medium conflict overwritten")
			}
		})
	}
}

func TestAdoptionCableOccupiedAndAmbiguous(t *testing.T) {
	for _, mode := range []string{"other", "exact-and-other", "patch", "multiple-cables"} {
		t.Run(mode, func(t *testing.T) {
			m, c := cableAdoptionFixture()
			exact := engineering.PlannedConnection{EntityVersion: engineering.EntityVersion{EntityID: "exact"}, APortID: "pa", ZPortID: "pb"}
			other := engineering.PlannedConnection{EntityVersion: engineering.EntityVersion{EntityID: "other"}, APortID: "pa", ZPortID: "pc"}
			switch mode {
			case "other":
				m.Connections = []engineering.PlannedConnection{other}
			case "exact-and-other":
				m.Connections = []engineering.PlannedConnection{exact, other}
			case "patch":
				other.APortID = "pd"
				other.PatchPortIDs = []string{"pa"}
				m.Connections = []engineering.PlannedConnection{other}
			case "multiple-cables":
				m.Connections = []engineering.PlannedConnection{exact}
				m.Cables = []engineering.CablePlan{{ConnectionID: "exact"}, {ConnectionID: "exact"}}
			}
			before := engineering.Digest(m)
			cables := engineering.Digest(m.Cables)
			var ev []engineering.SourceEvidence
			kickoffAgentMergePhysical(&m, &ev, c, "source")
			if engineering.Digest(m.Cables) != cables {
				t.Fatal("ambiguous cable modified")
			}
			if mode != "multiple-cables" && (engineering.Digest(m) != before || len(ev) != 0) {
				t.Fatal("occupied connection modified")
			}
		})
	}
}

func TestAdoptionCableInvalidLengthAndOpticsOnly(t *testing.T) {
	for _, length := range []string{"", "0", "-2", "NaN", "Inf", "3m"} {
		t.Run(length, func(t *testing.T) {
			m, c := cableAdoptionFixture()
			c.Rows[0]["cable_model"] = ""
			c.Rows[0]["medium"] = ""
			c.Rows[0]["cable_length_m"] = length
			var ev []engineering.SourceEvidence
			kickoffAgentMergePhysical(&m, &ev, c, "source")
			if len(m.Connections) != 1 || len(m.Cables) != 0 || len(m.Optics) != 0 || len(ev) == 0 {
				t.Fatal("invented cable/optic or lost evidence")
			}
		})
	}
}
