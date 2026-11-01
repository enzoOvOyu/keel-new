package engineering

import "testing"

func TestStandardPortSpecificationsPreserveMACAndIdentity(t *testing.T) {
	m, e := MergeTerminations(EmptyPhysical("p"), InputPreviewRequest{"terminations", terminationHeader + terminationRow})
	if e != nil {
		t.Fatal(e)
	}
	id := m.Ports[0].EntityID
	mac := *m.Ports[0].MAC
	q := standardRequest("port_specs", []string{"00001", "eth1", "fiber", "LC", "100", "Gbps", "", ""})
	next, e := MergeStandardInput(m, q)
	if e != nil {
		t.Fatal(e)
	}
	p := next.Ports[0]
	if p.EntityID != id || p.MAC == nil || *p.MAC != mac || p.Connector != "LC" || p.Medium != "fiber" || p.RateBPS == nil || *p.RateBPS != 100000000000 {
		t.Fatal("port values lost or rate conversion wrong", p)
	}
	if m.Ports[0].RateBPS != nil {
		t.Fatal("modified previous revision")
	}
	bad := standardRequest("port_specs", []string{"00001", "eth1", "fiber", "LC", "100", "guessed-unit", "", ""})
	if _, e = MergeStandardInput(m, bad); e == nil {
		t.Fatal("unknown rate unit accepted")
	}
	breakout := standardRequest("port_specs", []string{"00001", "eth1", "fiber", "LC", "25", "Gbps", "physical1", "1"})
	next, e = MergeStandardInput(m, breakout)
	if e != nil {
		t.Fatal(e)
	}
	if next.Ports[0].ParentPortID == nil || next.Ports[0].BreakoutIndex == nil || len(next.Ports) != 3 {
		t.Fatal("breakout relation missing")
	}
}

func TestStandardPatchPathForwardReverseReferences(t *testing.T) {
	m, e := MergeTerminations(EmptyPhysical("p"), InputPreviewRequest{"terminations", terminationHeader + terminationRow})
	if e != nil {
		t.Fatal(e)
	}
	serial := "PATCH-003"
	m.Equipment = append(m.Equipment, EquipmentPlan{EntityVersion: EntityVersion{EntityID: StableID("p", "equipment_serial", serial), Maturity: Estimated}, EquipmentType: "patch_panel", Serial: &serial, Label: "Synthetic panel", Quantity: 1, IdentityKind: "planned_instance"})
	m, e = MergeStandardInput(m, standardRequest("port_specs", []string{serial, "in1", "fiber", "LC", "100", "Gbps", "", ""}, []string{serial, "out1", "fiber", "LC", "100", "Gbps", "", ""}))
	if e != nil {
		t.Fatal(e)
	}
	forward, e := MergeStandardInput(m, standardRequest("patch_paths", []string{"00001/eth1 -> 00002/eth2", "1", serial, "in1", "out1"}))
	if e != nil {
		t.Fatal(e)
	}
	reverse, e := MergeStandardInput(m, standardRequest("patch_paths", []string{"00002/eth2 -> 00001/eth1", "1", serial, "out1", "in1"}))
	if e != nil {
		t.Fatal(e)
	}
	if len(forward.Connections[0].PatchPortIDs) != 2 || Digest(forward.Connections[0].PatchPortIDs) != Digest(reverse.Connections[0].PatchPortIDs) {
		t.Fatal("reverse declaration flipped physical orientation")
	}
	if len(m.Connections[0].PatchPortIDs) != 0 {
		t.Fatal("modified input revision")
	}
}
