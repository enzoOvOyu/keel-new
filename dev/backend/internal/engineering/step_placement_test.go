package engineering

import (
	"encoding/csv"
	"strings"
	"testing"
)

func standardRequest(table string, rows ...[]string) InputPreviewRequest {
	var b strings.Builder
	w := csv.NewWriter(&b)
	for _, v := range StepInputCatalog().Tables {
		if v.ID == table {
			headers := []string{}
			for _, f := range v.Fields {
				headers = append(headers, f.Label)
			}
			w.Write(headers)
			break
		}
	}
	for _, row := range rows {
		w.Write(row)
	}
	w.Flush()
	return InputPreviewRequest{table, b.String()}
}
func TestStandardFacilityPlacementChain(t *testing.T) {
	model, e := MergeTerminations(EmptyPhysical("project"), InputPreviewRequest{"terminations", terminationHeader + terminationRow})
	if e != nil {
		t.Fatal(e)
	}
	apply := func(q InputPreviewRequest) {
		t.Helper()
		var e error
		model, e = MergeStandardInput(model, q)
		if e != nil {
			t.Fatal(q.TableID, e)
		}
	}
	apply(standardRequest("rooms", []string{"S1", "场地一", "ROOM1", "机房一"}))
	apply(standardRequest("racks", []string{"ROOM1", "R1", "机柜一", "42", "600", "1200", "2200"}))
	apply(standardRequest("equipment_specs", []string{"00001", "server", "Shared model", "2"}))
	apply(standardRequest("placements", []string{"00001", "ROOM1", "R1", "u", "3", "2", "front", ""}))
	apply(standardRequest("placements", []string{"00002", "ROOM1", "R1", "non_u", "", "", "rear", "rear-side-1"}))
	if len(model.Equipment) != 2 || len(model.Ports) != 2 || len(model.Connections) != 1 || len(model.Facilities) != 3 || len(model.Placements) != 2 {
		t.Fatal("lost prior domain data")
	}
	for _, issue := range ValidatePhysical(model) {
		if issue.PublishTarget == PlacementTarget && strings.HasPrefix(issue.Code, "PLACEMENT_") {
			t.Fatal(issue.Code, issue.Message)
		}
	}
	oldID := model.Placements[0].EntityID
	apply(standardRequest("placements", []string{"00001", "ROOM1", "R1", "u", "5", "2", "front", ""}))
	if model.Placements[0].EntityID != oldID || len(model.Placements) != 2 {
		t.Fatal("placement edit changed identity")
	}
	before := Digest(model)
	_, e = MergeStandardInput(model, standardRequest("placements", []string{"00001", "ROOM1", "R1", "u", "5", "3", "front", ""}))
	if e == nil || Digest(model) != before {
		t.Fatal("height conflict accepted or mutated original")
	}
	_, e = MergeStandardInput(model, standardRequest("placements", []string{"00001", "ROOM1", "R1", "non_u", "", "", "rear", ""}))
	if e == nil {
		t.Fatal("non U missing position accepted")
	}
}
func TestStandardPlacementBlocksMissingDependencies(t *testing.T) {
	m := EmptyPhysical("p")
	for _, q := range []InputPreviewRequest{standardRequest("racks", []string{"missing", "R1", "Rack", "42", "600", "1200", "2200"})} {
		if _, e := MergeStandardInput(m, q); e == nil {
			t.Fatal("missing dependency accepted", q.TableID)
		}
	}
}

func TestInventoryWithoutTerminationsCreatesOnlySourceBackedInstances(t *testing.T) {
	m := EmptyPhysical("inventory")
	q := standardRequest("equipment_specs", []string{"source-SN", "server", "source-model", "2"}, []string{"", "server", "model-only", ""})
	next, err := MergeStandardInput(m, q)
	if err != nil || len(next.Equipment) != 1 || len(next.Ports) != 0 || next.Equipment[0].Serial == nil || *next.Equipment[0].Serial != "source-SN" || next.Equipment[0].IdentityKind != "planned_instance" {
		t.Fatal("inventory identity lost or fabricated", err)
	}
	again, err := MergeStandardInput(next, q)
	if err != nil || len(again.Equipment) != 1 || again.Equipment[0].EntityID != next.Equipment[0].EntityID {
		t.Fatal("inventory reimport duplicated identity", err)
	}
}

func TestOptionalPlacementConfiguration(t *testing.T) {
	m, err := MergeTerminations(EmptyPhysical("optional"), InputPreviewRequest{"terminations", terminationHeader + terminationRow})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []InputPreviewRequest{
		standardRequest("rooms", []string{"", "Site", "", "Room", "", ""}),
		standardRequest("racks", []string{"", "", "Rack", "42"}),
		standardRequest("placements", []string{"00001", "", "", "", "3", "2", "", ""}),
	} {
		if !PreviewStepInput(q).Valid {
			t.Fatal("optional configuration rejected", q.TableID)
		}
		m, err = MergeStandardInput(m, q)
		if err != nil {
			t.Fatal(q.TableID, err)
		}
	}
	if len(m.Facilities) != 3 || len(m.Placements) != 1 || m.Placements[0].InstallationType != "" || m.Placements[0].MountFace != "" {
		t.Fatal("inferred configuration values or lost relationship")
	}
	for _, f := range m.Facilities {
		if f.Code != "" {
			t.Fatal("fabricated facility code")
		}
	}
	for _, issue := range ValidatePhysical(m) {
		if issue.Code == "MOUNT_FACE_INVALID" || issue.Code == "INSTALLATION_TYPE_INVALID" || issue.Code == "FACILITY_PARENT_REQUIRED" {
			t.Fatal(issue.Code)
		}
	}
	q := standardRequest("placements", []string{"00002", "", "", "", "", "", "", "side-slot"})
	m, err = MergeStandardInput(m, q)
	if err != nil || !m.Placements[1].UsesNonUPosition() {
		t.Fatal("explicit position requires redundant installation configuration", err)
	}
	for _, issue := range ValidatePhysical(m) {
		if issue.EntityID == m.Placements[1].EntityID && issue.Code == "PLACEMENT_U_REQUIRED" {
			t.Fatal("non-U source forced into U placement")
		}
	}
	for _, id := range []string{"rooms", "racks"} {
		for _, table := range StepInputCatalog().Tables {
			if table.ID == id {
				for _, field := range table.Fields {
					if field.Required {
						t.Fatal("configuration remains required", id, field.Code)
					}
				}
			}
		}
	}
	ids := []ID{m.Facilities[0].EntityID, m.Facilities[1].EntityID, m.Facilities[2].EntityID}
	for _, q := range []InputPreviewRequest{standardRequest("rooms", []string{"SITE", "Site", "ROOM", "Room"}), standardRequest("racks", []string{"ROOM", "RACK", "Rack"})} {
		m, err = MergeStandardInput(m, q)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(m.Facilities) != 3 {
		t.Fatal("late configuration duplicated identities")
	}
	for i, id := range ids {
		if m.Facilities[i].EntityID != id {
			t.Fatal("late code replaced stable identity")
		}
	}
}
