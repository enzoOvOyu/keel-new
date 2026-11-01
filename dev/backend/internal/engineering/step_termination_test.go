package engineering

import (
	"strings"
	"testing"
)

const terminationHeader = "a端SN,a端MAC,a端端口,b端SN,b端机架位,b端MAC,b端端口,端接类型\n"
const terminationRow = "00001,00:11:22:33:44:55,eth1,00002,R1-U10,00:11:22:33:44:66,eth2,客户端接\n"

func TestMergeTerminationsAdditive(t *testing.T) {
	original := EmptyPhysical("project")
	original.Facilities = append(original.Facilities, FacilityModel{EntityVersion: EntityVersion{EntityID: "existing-rack"}, Kind: "rack", Label: "R1"})
	q := InputPreviewRequest{"terminations", terminationHeader + terminationRow}
	next, e := MergeTerminations(original, q)
	if e != nil {
		t.Fatal(e)
	}
	if len(original.Equipment) != 0 || len(next.Equipment) != 2 || len(next.Ports) != 2 || len(next.Connections) != 1 || len(next.Facilities) != 1 {
		t.Fatal("not additive or mutates input")
	}
	if *next.Equipment[0].Serial != "00001" || *next.Ports[0].MAC != "00:11:22:33:44:55" || next.Connections[0].BRackPosition != "R1-U10" {
		t.Fatal("lost endpoint identity")
	}
	again, e := MergeTerminations(next, q)
	if e != nil {
		t.Fatal(e)
	}
	if len(again.Equipment) != 2 || len(again.Ports) != 2 || len(again.Connections) != 1 || again.Connections[0].EntityID != next.Connections[0].EntityID {
		t.Fatal("not idempotent")
	}
	reverse := InputPreviewRequest{"terminations", terminationHeader + "00002,00:11:22:33:44:66,eth2,00001,R2-U20,00:11:22:33:44:55,eth1,客户端接\n"}
	reversed, err := MergeTerminations(next, reverse)
	if err != nil || len(reversed.Connections) != 1 || reversed.Connections[0].BRackPosition != "R1-U10" {
		t.Fatal("reverse duplicate changed orientation", err)
	}
	// Reuse an existing identity even if its ID was not derived from this importer.
	next.Equipment[0].EntityID = "stable-existing-id"
	next.Ports[0].EquipmentID = "stable-existing-id"
	again, e = MergeTerminations(next, q)
	if e != nil || again.Equipment[0].EntityID != "stable-existing-id" || len(again.Equipment) != 2 {
		t.Fatal("existing identity replaced", e)
	}
}
func TestMergeTerminationsRejectsConflictsWithoutMutation(t *testing.T) {
	q := InputPreviewRequest{"terminations", terminationHeader + terminationRow}
	next, e := MergeTerminations(EmptyPhysical("p"), q)
	if e != nil {
		t.Fatal(e)
	}
	for _, body := range []string{
		strings.Replace(terminationRow, "00:11:22:33:44:55", "00:11:22:33:44:99", 1),
		strings.Replace(terminationRow, "客户端接", "其他类型", 1),
		strings.Replace(terminationRow, "R1-U10", "R2-U20", 1),
		strings.Replace(terminationRow, "00002", "00003", 1),
	} {
		before := Digest(next)
		if _, err := MergeTerminations(next, InputPreviewRequest{"terminations", terminationHeader + body}); err == nil {
			t.Fatal("conflict accepted", body)
		}
		if Digest(next) != before {
			t.Fatal("input mutated on conflict")
		}
	}
}
