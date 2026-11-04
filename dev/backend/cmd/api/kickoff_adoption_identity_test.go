package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"testing"
)

func identityCandidate(table string, rows ...map[string]string) ai.KickoffAgentCandidate {
	c := ai.KickoffAgentCandidate{TableID: table, Rows: rows}
	for i, row := range rows {
		fields := map[string]ai.KickoffAgentCell{}
		for field := range row {
			fields[field] = ai.KickoffAgentCell{Sheet: "synthetic", Cell: field + "1"}
		}
		c.Evidence = append(c.Evidence, ai.KickoffAgentEvidence{Row: i + 1, Fields: fields})
	}
	return c
}
func TestKickoffAdoptionNameIdentityPublication(t *testing.T) {
	connections := identityCandidate("connections",
		map[string]string{"a_device_ref": "Panel A", "a_port": "1", "b_device_ref": "Switch B", "b_port": "2", "a_model": "Panel model", "a_equipment_type": "patch_panel", "b_equipment_type": "switch", "a_role": "compute", "b_role": "compute", "rate": "100", "rate_unit": "Gbps"},
		map[string]string{"a_device_ref": "Panel A", "a_port": "3", "b_device_ref": "Switch B", "b_port": "4", "a_model": "Panel model", "a_role": "storage", "b_role": "storage", "cable_length_m": "invalid"},
	)
	connections.Issues = []engineering.InputProblem{{Row: 2, Field: "cable_length_m", Code: "INVALID_LENGTH"}, {Row: 1, Field: "a_serial", Code: "SN_MISSING"}}
	inventory := identityCandidate("devices", map[string]string{"device_ref": "Panel A", "model": "Panel model"}, map[string]string{"device_ref": "unrelated inventory"})
	candidates := kickoffAgentProjectEndpoints(kickoffAgentCombine([]ai.KickoffAgentCandidate{connections, inventory}))
	m := engineering.EmptyPhysical("synthetic-project")
	var evidence []engineering.SourceEvidence
	for _, table := range []string{"devices", "ports", "connections"} {
		for _, c := range candidates {
			if c.TableID == table {
				rows, err := kickoffAgentAdoptionRows(c)
				if err != nil || len(rows) != len(c.Rows) {
					t.Fatalf("ineligible %s: %d/%d %v %+v", table, len(rows), len(c.Rows), err, c.Issues)
				}
				if !kickoffAgentMergePhysical(&m, &evidence, c, "source") {
					t.Fatalf("not adopted: %s", table)
				}
			}
		}
	}
	if len(m.Equipment) != 2 || len(m.Ports) != 4 || len(m.Connections) != 2 {
		t.Fatalf("incomplete or expanded identity set: %d/%d/%d", len(m.Equipment), len(m.Ports), len(m.Connections))
	}
	for _, d := range m.Equipment {
		if d.Serial != nil || d.IdentityKind != "planned_instance" || d.EntityID != engineering.StableID(m.ProjectID, "equipment_ref", kickoff.NormalizeDeviceRef(d.Label)) {
			t.Fatalf("fabricated or unstable identity: %+v", d)
		}
	}
	if m.Equipment[0].SourceModelLabel != "Panel model" || m.Ports[0].Role != "compute" || m.Ports[2].Role != "storage" || m.Ports[0].RateBPS == nil {
		t.Fatal("source attributes lost")
	}
	for _, field := range []string{"model", "role", "device_ref"} {
		found := false
		for _, ev := range evidence {
			if ev.Field == field {
				found = true
			}
		}
		if !found {
			t.Fatal("source evidence lost", field)
		}
	}
	before, beforeEvidence := engineering.Digest(m), engineering.Digest(evidence)
	for _, c := range candidates {
		kickoffAgentMergePhysical(&m, &evidence, c, "source")
	}
	if before != engineering.Digest(m) || beforeEvidence != engineering.Digest(evidence) {
		t.Fatal("repeat adoption not idempotent")
	}
	// Persistence binds these source-backed objects to an immutable revision.
	for _, object := range engineering.ModelObjects(&m) {
		object.Version.RevisionID = "revision"
		object.Version.ProvenanceID = "provenance"
		for _, ev := range evidence {
			if ev.EntityID == object.Version.EntityID {
				object.Version.EvidenceIDs = append(object.Version.EvidenceIDs, ev.ID)
			}
		}
	}
	view := kickoff.Project(m, evidence, "source", nil)
	if len(view.Devices) != 2 || view.Devices[0].Serial != "" {
		t.Fatal("review lost name-only identity")
	}
	_, publication, err := kickoff.Select(m, view, []string{m.Equipment[0].EntityID, m.Equipment[1].EntityID})
	if err != nil {
		t.Fatal(err)
	}
	if publication.Counts["devices"] != 2 || publication.Counts["ports"] != 4 || publication.Counts["connections"] != 2 {
		t.Fatalf("incomplete closure: %+v", publication.Counts)
	}
	frozen := kickoff.FrozenModel(m, publication)
	if len(frozen.Equipment) != 2 || frozen.Equipment[0].Serial != nil {
		t.Fatal("publication fabricated SN")
	}
	id := m.Equipment[0].EntityID
	kickoffAgentMergePhysical(&m, &evidence, identityCandidate("devices", map[string]string{"device_ref": "Panel A", "serial": "SYN-A"}), "source")
	kickoffAgentMergePhysical(&m, &evidence, identityCandidate("ports", map[string]string{"serial": "SYN-A", "port": "legacy"}), "source")
	if len(m.Equipment) != 2 || m.Equipment[0].EntityID != id || m.Equipment[0].Serial == nil || *m.Equipment[0].Serial != "SYN-A" || len(m.Ports) != 5 || m.Ports[4].EquipmentID != id {
		t.Fatal("SN enrichment broke identity or legacy references")
	}
}

func TestKickoffAdoptionIdentityConflicts(t *testing.T) {
	for _, scenario := range []string{"different identities", "different SN", "ambiguous name", "ambiguous SN"} {
		t.Run(scenario, func(t *testing.T) {
			a, b := "SYN-A", "SYN-B"
			m := engineering.EmptyPhysical("p")
			m.Equipment = []engineering.EquipmentPlan{{EntityVersion: engineering.EntityVersion{EntityID: "a"}, Label: "A", Serial: &a}, {EntityVersion: engineering.EntityVersion{EntityID: "b"}, Label: "B", Serial: &b}}
			row := map[string]string{"device_ref": "A", "serial": b, "port": "1"}
			switch scenario {
			case "different SN":
				row["serial"] = "SYN-OTHER"
			case "ambiguous name":
				m.Equipment[1].Label = "A"
				delete(row, "serial")
			case "ambiguous SN":
				m.Equipment[1].Serial = &a
				row["serial"] = a
			}
			var ev []engineering.SourceEvidence
			before := engineering.Digest(m)
			for _, table := range []string{"devices", "ports"} {
				kickoffAgentMergePhysical(&m, &ev, identityCandidate(table, row), "source")
			}
			if engineering.Digest(m) != before {
				t.Fatal("conflicting identity adopted")
			}
		})
	}
}

func TestKickoffAdoptionEndpointSourceAndLegacy(t *testing.T) {
	legacy := identityCandidate("connections", map[string]string{"a_serial": "SYN-A", "a_port": "1", "b_serial": "SYN-B", "b_port": "2"})
	for _, sourced := range []bool{true, false} {
		c := legacy
		if !sourced {
			c.Evidence = nil
		}
		candidates := kickoffAgentProjectEndpoints([]ai.KickoffAgentCandidate{c})
		m := engineering.EmptyPhysical("legacy")
		var ev []engineering.SourceEvidence
		for _, candidate := range candidates {
			if candidate.TableID == "devices" || candidate.TableID == "ports" {
				kickoffAgentMergePhysical(&m, &ev, candidate, "source")
			}
		}
		if sourced && (len(m.Equipment) != 2 || len(m.Ports) != 2 || m.Equipment[0].Serial == nil) {
			t.Fatal("legacy explicit SN broken")
		}
		if !sourced && len(m.Equipment) != 0 {
			t.Fatal("unsourced endpoints became devices")
		}
	}
}

func TestKickoffAdoptionCombineConflictsAndEligibility(t *testing.T) {
	for _, field := range []string{"serial", "model"} {
		c := identityCandidate("devices", map[string]string{"device_ref": "A", field: "one"}, map[string]string{"device_ref": "A", field: "two"})
		combined := kickoffAgentCombine([]ai.KickoffAgentCandidate{c})[0]
		if len(combined.Rows) != 1 || len(combined.Issues) != 1 || len(combined.Evidence) != 2 {
			t.Fatal("identity not deduplicated with conflict/source")
		}
		rows, _ := kickoffAgentAdoptionRows(combined)
		if len(rows) != 0 {
			t.Fatal("identity/core conflict bypassed")
		}

	}
	for _, c := range []ai.KickoffAgentCandidate{
		identityCandidate("devices", map[string]string{"label": "no identity"}),
		identityCandidate("ports", map[string]string{"port": "1"}),
		identityCandidate("connections", map[string]string{"a_device_ref": "A", "a_port": "1", "b_port": "2"}),
	} {
		if rows, _ := kickoffAgentAdoptionRows(c); len(rows) != 0 {
			t.Fatalf("missing identity selectable: %s", c.TableID)
		}
	}
}

func TestKickoffAdoptionGPUDeviceRemainsServer(t *testing.T) {
	m := engineering.EmptyPhysical("p")
	var ev []engineering.SourceEvidence
	kickoffAgentMergePhysical(&m, &ev, identityCandidate("devices", map[string]string{"device_ref": "GPU endpoint", "serial": "SYN-GPU", "equipment_type": "gpu"}), "source")
	kickoffAgentMergePhysical(&m, &ev, identityCandidate("components", map[string]string{"parent_serial": "SYN-GPU", "serial": "SYN-COMPONENT", "equipment_type": "gpu"}), "source")
	if len(m.Equipment) != 2 || m.Equipment[0].EquipmentType != "server" || m.Equipment[1].EquipmentType != "gpu" {
		t.Fatal("GPU device/component distinction lost")
	}
}
