package main

import (
	"keel.local/dev/internal/ai"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"testing"
)

func TestKickoffEndpointRevisionReverse(t *testing.T) {
	a := map[string]string{"a_device_ref": "A", "a_port": "1", "b_device_ref": "B", "b_port": "2", "a_model": "model-A", "b_model": "model-B", "a_role": "compute", "b_role": "storage", "a_mac": "00:00:00:00:00:01"}
	b := map[string]string{}
	for key, value := range a {
		b[kickoffAgentSwapEndpointField(key)] = value
	}
	c := identityCandidate("connections", a, b)
	c.Evidence[1].Fields["b_model"] = ai.KickoffAgentCell{Sheet: "reverse", Cell: "B5"}
	c.Issues = []engineering.InputProblem{{Row: 2, Field: "b_serial", Code: "MISSING"}}
	before := engineering.Digest(c)
	out := kickoffAgentCombine([]ai.KickoffAgentCandidate{c})[0]
	if len(out.Rows) != 1 || out.Rows[0]["a_model"] != "model-A" || out.Rows[0]["b_role"] != "storage" {
		t.Fatalf("reverse not deduplicated correctly: %+v", out)
	}
	if out.Evidence[1].Fields["a_model"].Cell != "B5" || out.Issues[0].Field != "a_serial" || out.Issues[0].Row != 1 {
		t.Fatal("reverse provenance/issue not swapped")
	}
	if rows, _ := kickoffAgentAdoptionRows(out); len(rows) != 1 {
		t.Fatal("missing optional SN blocked")
	}
	if engineering.Digest(c) != before {
		t.Fatal("source artifact changed")
	}
}

func TestKickoffEndpointRevisionConflicts(t *testing.T) {
	for _, field := range []string{"serial", "model", "equipment_type", "label"} {
		c := identityCandidate("devices", map[string]string{"device_ref": "A", field: "one"}, map[string]string{"device_ref": "a", field: "two"})
		combined := kickoffAgentCombine([]ai.KickoffAgentCandidate{c})[0]
		if len(combined.Rows) != 1 {
			t.Fatal("normalized name duplicated")
		}
		if rows, _ := kickoffAgentAdoptionRows(combined); len(rows) != 0 {
			t.Fatalf("explicit %s conflict swallowed", field)
		}
	}
	a := map[string]string{"a_device_ref": "A", "a_port": "1", "b_device_ref": "B", "b_port": "2", "a_model": "one", "cable_length_m": "1"}
	b := map[string]string{"a_device_ref": "B", "a_port": "2", "b_device_ref": "A", "b_port": "1", "b_model": "two", "cable_length_m": "2"}
	out := kickoffAgentCombine([]ai.KickoffAgentCandidate{identityCandidate("connections", a, b)})[0]
	if rows, _ := kickoffAgentAdoptionRows(out); len(rows) != 0 {
		t.Fatal("model conflict hidden by optional length conflict")
	}
	b["b_model"] = "one"
	out = kickoffAgentCombine([]ai.KickoffAgentCandidate{identityCandidate("connections", a, b)})[0]
	rows, _ := kickoffAgentAdoptionRows(out)
	if len(rows) != 1 || rows[0]["cable_length_m"] != "" || len(out.Issues) != 1 {
		t.Fatal("length conflict must retain issue and omit only length")
	}
}

func TestKickoffEndpointRevisionNamesAndRoles(t *testing.T) {
	c := identityCandidate("connections",
		map[string]string{"a_device_ref": " Node-01  X ", "a_port": "1", "b_device_ref": "Peer", "b_port": "2", "a_role": "compute"},
		map[string]string{"a_device_ref": "node-01 x", "a_port": "3", "b_device_ref": "Peer", "b_port": "4", "a_role": "storage"},
	)
	candidates := kickoffAgentProjectEndpoints([]ai.KickoffAgentCandidate{c})
	m := engineering.EmptyPhysical("p")
	var ev []engineering.SourceEvidence
	oldID := engineering.StableID("p", "equipment_ref", "Node-01 X")
	m.Equipment = append(m.Equipment, engineering.EquipmentPlan{EntityVersion: engineering.EntityVersion{EntityID: oldID}, Label: "Node-01 X", EquipmentType: "device", IdentityKind: "planned_instance", Quantity: 1})
	for _, candidate := range candidates {
		if candidate.TableID == "devices" {
			if candidate.Rows[0]["role"] != "compute;storage" || len(candidate.Issues) != 0 {
				t.Fatalf("roles discarded or treated as conflict: %+v", candidate)
			}
		}
		kickoffAgentMergePhysical(&m, &ev, candidate, "source")
	}
	if len(m.Equipment) != 2 || m.Equipment[0].EntityID != oldID || m.Equipment[0].Label != "Node-01 X" || len(m.Ports) != 4 {
		t.Fatal("name normalization broke legacy identity/ports")
	}
	if m.Equipment[1].EntityID != engineering.StableID("p", "equipment_ref", "peer") {
		t.Fatal("new stable ID not normalized")
	}
	inputs := kickoff.ResolveReferences(m, []kickoff.ModelInput{{TableID: "placements", Rows: []map[string]string{{"device_id": " NODE-01\tX "}, {"device_id": "node01 x"}}}})
	if inputs[0].Rows[0]["device_id"] != oldID || inputs[0].Rows[1]["device_id"] != "node01 x" {
		t.Fatal("placement normalization changed punctuation or lost alias")
	}
}
