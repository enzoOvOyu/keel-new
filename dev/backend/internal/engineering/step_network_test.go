package engineering

import (
	"bytes"
	"os"
	"testing"
)

func TestStandardNetworkWorkbookExplicitIntent(t *testing.T) {
	m := EmptyPhysical("network-standard-test")
	for step, name := range []string{"01-equipment.xlsx", "02-placement.xlsx", "04-network.xlsx"} {
		actual := step + 1
		if step == 2 {
			actual = 4
		}
		data, e := os.ReadFile("testdata/five-step/" + name)
		if e != nil {
			t.Fatal(e)
		}
		inputs, e := ReadStepWorkbook(bytes.NewReader(data), actual)
		if e != nil {
			t.Fatal(e)
		}
		for _, q := range inputs {
			m, e = MergeStandardInput(m, q)
			if e != nil {
				t.Fatal(name, q.TableID, e)
			}
		}
	}
	if len(m.NetworkPlans) != 1 {
		t.Fatal("network plan missing")
	}
	p := m.NetworkPlans[0]
	if len(p.Fabrics) != 1 || len(p.Planes) != 1 || p.Planes[0].Profile != "ipv4-bgp" || len(p.InterfaceMappings) != 2 || len(p.BGP) != 1 || len(p.Allocations) != 4 {
		t.Fatal("explicit planning facts missing")
	}
	addresses := map[string]bool{}
	asns := map[int64]bool{}
	for _, v := range p.Allocations {
		if v.Address != nil {
			addresses[*v.Address] = true
		}
		if v.ASN != nil {
			asns[*v.ASN] = true
		}
	}
	if !addresses["192.0.2.0"] || !addresses["192.0.2.1"] || !asns[64512] || !asns[64513] {
		t.Fatal("changed explicit addresses or ASNs")
	}
	// Identity references must resolve to the pre-existing physical model.
	ports := map[string]bool{}
	for _, v := range m.Ports {
		ports[v.EntityID] = true
	}
	for _, v := range p.InterfaceMappings {
		if !ports[v.PhysicalPortID] {
			t.Fatal("parallel port identity")
		}
	}
	p.PhysicalRevisionID = "published-physical"
	p.SourceIDs = []string{"source"}
	p.EvidenceIDs = []string{"reviewed-source"}
	for i := range p.Fabrics {
		p.Fabrics[i].EvidenceIDs = p.EvidenceIDs
	}
	for i := range p.Planes {
		p.Planes[i].EvidenceIDs = p.EvidenceIDs
	}
	for i := range p.DeviceRoles {
		p.DeviceRoles[i].EvidenceIDs = p.EvidenceIDs
	}
	for i := range p.InterfaceMappings {
		p.InterfaceMappings[i].EvidenceIDs = p.EvidenceIDs
	}
	for i := range p.Allocations {
		p.Allocations[i].EvidenceIDs = p.EvidenceIDs
	}
	for i := range p.BGP {
		p.BGP[i].EvidenceIDs = p.EvidenceIDs
	}
	if issues := ValidateNetwork(p, m); len(issues) != 0 {
		t.Fatal(issues)
	}
	p.BGP = nil
	issues := ValidateNetwork(p, m)
	found := false
	for _, v := range issues {
		if v.Code == "NETWORK_PROFILE_NEIGHBOR_REQUIRED" {
			found = true
		}
	}
	if !found {
		t.Fatal("BGP profile without neighbors passed")
	}
	before := Digest(m)
	q := standardRequest("network_addresses", []string{"TEST-SRV-001", "ens1", "not-an-ip", "point-to-point", "source"})
	if _, e := MergeStandardInput(m, q); e == nil || Digest(m) != before {
		t.Fatal("bad IP accepted or input mutated")
	}
}
