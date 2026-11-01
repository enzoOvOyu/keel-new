// Generates synthetic standard-input workbooks. It never reads customer data.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"keel.local/dev/internal/engineering"
)

func main() {
	destination := "internal/engineering/testdata/five-step"
	if len(os.Args) > 1 {
		destination = os.Args[1]
	}
	if e := os.MkdirAll(destination, 0755); e != nil {
		panic(e)
	}
	data := map[int][]engineering.StandardTableData{
		1: {{TableID: "terminations", Rows: [][]string{{"TEST-SRV-001", "02:00:00:00:01:01", "eth1", "TEST-SW-002", "R2/U10", "02:00:00:00:02:01", "Ethernet1", "optical-duplex"}}}},
		2: {
			{TableID: "rooms", Rows: [][]string{{"SITE-A", "Synthetic site", "ROOM-A", "Synthetic room"}}},
			{TableID: "racks", Rows: [][]string{{"ROOM-A", "R1", "Synthetic rack one", "42", "600", "1200", "2200"}, {"ROOM-A", "R2", "Synthetic rack two", "42", "600", "1200", "2200"}}},
			{TableID: "equipment_specs", Rows: [][]string{{"TEST-SRV-001", "server", "Synthetic server", "2"}, {"TEST-SW-002", "switch", "Synthetic switch", "1"}}},
			{TableID: "placements", Rows: [][]string{{"TEST-SRV-001", "ROOM-A", "R1", "u", "3", "2", "front", ""}, {"TEST-SW-002", "ROOM-A", "R2", "u", "10", "1", "front", ""}}},
		},
		3: {
			{TableID: "port_specs", Rows: [][]string{{"TEST-SRV-001", "eth1", "fiber", "LC", "100", "Gbps", "", ""}, {"TEST-SW-002", "Ethernet1", "fiber", "LC", "100", "Gbps", "", ""}}},
			{TableID: "route_nodes", Rows: [][]string{{"NODE-1", "ROOM-A", "R1", "ROOM-A-mm", "0", "0", "2200", "mm"}, {"NODE-2", "ROOM-A", "R2", "ROOM-A-mm", "12000", "0", "2200", "mm"}}},
			{TableID: "route_edges", Rows: [][]string{{"EDGE-1", "NODE-1", "NODE-2", "12", "m", "fiber"}}},
			{TableID: "rack_access", Rows: [][]string{{"R1", "NODE-1", "fixture-lead-2m-v1"}, {"R2", "NODE-2", "fixture-lead-2m-v1"}}},
			{TableID: "cable_specs", Rows: [][]string{{"FIBER-LC-100G", "fiber", "LC", "LC", "100", "Gbps", "20", "m", "separate-optics"}, {"FIBER-LC-100G", "fiber", "LC", "LC", "100", "Gbps", "30", "m", "separate-optics"}}},
			{TableID: "optic_specs", Rows: [][]string{{"SYNTHETIC-100G-LC", "LC", "fiber", "100", "Gbps", "100", "m"}}},
			{TableID: "cabling_rules", Rows: [][]string{{"fixture-cabling", "1", "shortest_path", "fixture-lead-2m-v1", "fixture-slack-10pct-v1", "fixture-stock-ceiling-v1", "SYNTHETIC_ONLY: rules must be explicitly created and approved in the test project"}}},
		},
		4: {
			{TableID: "network_planes", Rows: [][]string{{"FABRIC-A", "ethernet", "PLANE-A", "ipv4-bgp"}}},
			{TableID: "network_roles", Rows: [][]string{{"TEST-SRV-001", "FABRIC-A", "PLANE-A", "host"}, {"TEST-SW-002", "FABRIC-A", "PLANE-A", "leaf"}}},
			{TableID: "network_interfaces", Rows: [][]string{{"TEST-SRV-001", "eth1", "ens1", "PLANE-A", "", ""}, {"TEST-SW-002", "Ethernet1", "Ethernet1", "PLANE-A", "", ""}}},
			{TableID: "network_addresses", Rows: [][]string{{"TEST-SRV-001", "ens1", "192.0.2.0/31", "point-to-point", "SYNTHETIC_ONLY documentation address"}, {"TEST-SW-002", "Ethernet1", "192.0.2.1/31", "point-to-point", "SYNTHETIC_ONLY documentation address"}}},
			{TableID: "network_asns", Rows: [][]string{{"TEST-SRV-001", "64512", "SYNTHETIC_ONLY private ASN"}, {"TEST-SW-002", "64513", "SYNTHETIC_ONLY private ASN"}}},
			{TableID: "network_bgp", Rows: [][]string{{"TEST-SRV-001", "ens1", "TEST-SW-002/Ethernet1", "64512", "64513", "ipv4-unicast", "fixture-network-policy-v1"}}},
		},
		5: {
			{TableID: "power_profiles", Rows: [][]string{{"TEST-SRV-001", "500", "W", "planned", "SYNTHETIC_ONLY engineering input"}, {"TEST-SW-002", "300", "W", "planned", "SYNTHETIC_ONLY engineering input"}}},
			{TableID: "rack_capacity", Rows: [][]string{{"R1", "2", "kW", "SYNTHETIC_ONLY capacity", "air", ""}, {"R2", "2", "kW", "SYNTHETIC_ONLY capacity", "air", ""}}},
			{TableID: "power_circuits", Rows: [][]string{{"PDU-R1-A", "R1", "", "CIRCUIT-R1-A", "2", "kW"}, {"PDU-R2-A", "R2", "", "CIRCUIT-R2-A", "2", "kW"}}},
			{TableID: "power_connections", Rows: [][]string{{"TEST-SRV-001", "PSU1", "CIRCUIT-R1-A", "A", "fixture-full-load-v1"}, {"TEST-SW-002", "PSU1", "CIRCUIT-R2-A", "A", "fixture-full-load-v1"}}},
			{TableID: "power_rules", Rows: [][]string{{"fixture-power", "1", "normal-single-feed", "planned", "1", "SYNTHETIC_ONLY; not a redundant power design"}}},
		},
	}
	names := []string{"", "01-equipment.xlsx", "02-placement.xlsx", "03-cabling.xlsx", "04-network.xlsx", "05-power.xlsx"}
	for step := 1; step <= 5; step++ {
		b, e := engineering.WriteStepWorkbook(step, data[step])
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(destination, names[step]), b, 0644); e != nil {
			panic(e)
		}
		fmt.Println(names[step])
	}
	// Expected results are independently specified, not generated from model output.
	expected := map[string]any{"classification": "SYNTHETIC_ONLY", "acceptance": "NOT_RUN", "equipment_count": 2, "port_count": 2, "connection_count": 1, "site_count": 1, "room_count": 1, "rack_count": 2, "placements": map[string]any{"TEST-SRV-001": map[string]any{"rack": "R1", "start_u": 3, "height_u": 2}, "TEST-SW-002": map[string]any{"rack": "R2", "start_u": 10, "height_u": 1}}, "cabling": map[string]any{"route_m": "12", "lead_per_end_m": "2", "slack_factor": "1.1", "required_m": "17.6", "selected_stock_m": "20", "cable_count": 1, "separate_optic_count": 2}, "power": map[string]any{"R1_load_W": "500", "R1_remaining_W": "1500", "R2_load_W": "300", "R2_remaining_W": "1700", "redundancy": "NOT_APPLICABLE: explicit single-feed fixture"}, "network": map[string]any{"addresses": []string{"192.0.2.0/31", "192.0.2.1/31"}, "asns": []int{64512, 64513}, "bgp_sessions": 1}, "limits": []string{"Generated files and expected results do not imply business or Agent acceptance", "Fixture rules and policy references require explicit test-project approval", "No real device configuration or external AI calls"}}
	b, e := json.MarshalIndent(expected, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(filepath.Join(destination, "expected.json"), append(b, '\n'), 0644); e != nil {
		panic(e)
	}
}
