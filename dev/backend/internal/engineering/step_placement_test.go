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
	for _, q := range []InputPreviewRequest{standardRequest("racks", []string{"missing", "R1", "Rack", "42", "600", "1200", "2200"}), standardRequest("equipment_specs", []string{"missing", "server", "M", "2"})} {
		if _, e := MergeStandardInput(m, q); e == nil {
			t.Fatal("missing dependency accepted", q.TableID)
		}
	}
}
