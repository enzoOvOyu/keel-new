package kickoff

import "testing"

func TestDeterministicCabling(t *testing.T) {
	inputs := []ModelInput{
		{TableID: "route_nodes", Rows: []map[string]string{{"code": "A"}, {"code": "B"}, {"code": "C"}}},
		{TableID: "route_edges", Rows: []map[string]string{{"from": "A", "to": "B", "distance_m": "6"}, {"from": "A", "to": "C", "distance_m": "20"}, {"from": "C", "to": "B", "distance_m": "3"}}},
		{TableID: "rack_access", Rows: []map[string]string{{"rack_id": "r1", "node": "A", "lead_m": "1"}, {"rack_id": "r2", "node": "B", "lead_m": "1"}}},
		{TableID: "cabling_rules", Rows: []map[string]string{{"code": "r", "version": "1", "strategy": "shortest_path", "source": "explicit test input", "cable_spec": "s", "slack_m": "2", "slack_percent": "0"}}},
		{TableID: "cable_specs", Rows: []map[string]string{{"code": "s", "medium": "DAC", "length_m": "5"}, {"code": "s", "medium": "DAC", "length_m": "10"}, {"code": "s", "medium": "optical", "length_m": "9"}}},
	}
	view := View{Devices: []Device{{ID: "a", RackID: "r1"}, {ID: "b", RackID: "r2"}, {ID: "c", RackID: "missing"}}, Ports: []Port{{ID: "p1", DeviceID: "a"}, {ID: "p2", DeviceID: "b"}, {ID: "p3", DeviceID: "c"}}, Connections: []Connection{{ID: "c1", A: "p1", B: "p2", Medium: "DAC"}, {ID: "c2", A: "p1", B: "p3", Medium: "DAC"}}}
	got := Calculate(view, inputs, "cabling")
	if got.Status != "partial" || len(got.Rows) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	r := got.Rows[0]
	if r.PathM != 6 || r.CalculatedM != 10 || r.StockM == nil || *r.StockM != 10 {
		t.Fatalf("incorrect independent reference result: %+v", r)
	}
	inputs[0].Version++
	if got.InputDigest == Digest(view, inputs, "cabling") {
		t.Fatal("changed input did not invalidate result")
	}
}
func TestPowerUnknownAndZero(t *testing.T) {
	view := View{Devices: []Device{{ID: "a", RackID: "r"}, {ID: "b", RackID: "r"}}, Racks: []Rack{{ID: "r"}}}
	inputs := []ModelInput{{TableID: "power_profiles", Rows: []map[string]string{{"device_id": "a", "basis": "planned", "power_kw": "0"}}}, {TableID: "rack_capacity", Rows: []map[string]string{{"rack_id": "r", "capacity_kw": "10"}}}}
	result := Calculate(view, inputs, "power")
	if result.Power[0].LoadKW != nil {
		t.Fatal("unknown device power was treated as zero")
	}
	inputs[0].Rows = append(inputs[0].Rows, map[string]string{"device_id": "b", "basis": "planned", "power_kw": "0"})
	result = Calculate(view, inputs, "power")
	if result.Power[0].LoadKW == nil || *result.Power[0].LoadKW != 0 || result.Status != "ready" {
		t.Fatalf("explicit zero not honored: %+v", result)
	}
}
