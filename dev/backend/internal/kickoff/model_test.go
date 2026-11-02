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
