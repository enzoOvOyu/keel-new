package engineering

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"testing"
)

func TestNetworkExtractionCrossRegionOrderAndExplicitFacts(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		rows := []struct {
			kind           string
			fields, values []string
		}{
			{"network_evpn", []string{"fabric", "vni", "vtep_device", "vtep_interface", "route_distinguisher", "import_targets", "export_targets"}, []string{"fabric-a", "1000", "switch-a", "loopback0", "65001:1", "65001:1, 65001:2", "65001:1"}},
			{"network_allocation", []string{"fabric", "plane", "device", "interface", "allocation_kind", "address"}, []string{"fabric-a", "plane-a", "switch-a", "loopback0", "ip", "192.0.2.1/32"}},
			{"network_bgp", []string{"local_device", "local_interface", "peer_device", "peer_interface", "local_asn", "peer_asn", "afi", "safi"}, []string{"switch-a", "eth0", "switch-b", "eth1", "65001", "65002", "ipv4", "unicast"}},
		}
		if reverse {
			rows[0], rows[2] = rows[2], rows[0]
		}
		wb := bom.Workbook{Sheets: []bom.Sheet{}}
		proposal := EngineeringProposal{Summary: "synthetic network facts", Regions: []RegionMapping{}}
		for n, r := range rows {
			sheet := bom.Sheet{Name: fmt.Sprintf("arbitrary-%d", n), Cells: []bom.Cell{}}
			region := RegionMapping{Sheet: sheet.Name, Kind: r.kind, Bounds: Bounds{StartRow: 7, EndRow: 7, StartColumn: 1, EndColumn: len(r.fields)}, Columns: []ColumnBinding{}}
			for i, f := range r.fields {
				region.Columns = append(region.Columns, ColumnBinding{Field: f, Column: i + 1})
				sheet.Cells = append(sheet.Cells, bom.Cell{Sheet: sheet.Name, Row: 7, Column: i + 1, Ref: fmt.Sprintf("%c7", 'A'+i), Value: r.values[i]})
			}
			wb.Sheets = append(wb.Sheets, sheet)
			proposal.Regions = append(proposal.Regions, region)
		}
		x, e := ExtractPhysical(wb, proposal, "project", "source", "revision")
		if e != nil {
			t.Fatal(e)
		}
		if len(x.Model.Equipment) != 0 || len(x.Model.NetworkPlans) != 1 {
			t.Fatal("network facts fabricated physical devices")
		}
		p := x.Model.NetworkPlans[0]
		if len(p.Allocations) != 3 || len(p.EVPNVXLAN[0].VTEPAllocationIDs) != 1 || p.EVPNVXLAN[0].ImportTargets[1] != "65001:2" {
			t.Fatalf("cross-region references lost: %+v", p)
		}
		if p.PhysicalRevisionID != "" || len(p.InterfaceMappings) != 0 {
			t.Fatal("missing physical version/interfaces invented")
		}
		if !hasIssue(ValidateNetwork(p, x.Model), "NETWORK_PHYSICAL_REVISION_REQUIRED") {
			t.Fatal("missing physical revision accepted")
		}
	}
}
