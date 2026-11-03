package kickoff

import (
	"keel.local/dev/internal/engineering"
	"testing"
)

func publicationFixture() engineering.PhysicalModel {
	m := engineering.EmptyPhysical("test")
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: engineering.EntityVersion{EntityID: "a", RevisionID: "old"}, EquipmentType: "server", Quantity: 1, IdentityKind: "serial", Label: "A", SourceModelLabel: "model"}, {EntityVersion: engineering.EntityVersion{EntityID: "b"}, EquipmentType: "switch", Quantity: 1, IdentityKind: "serial", Label: "B"}}
	m.Ports = []engineering.PlannedPort{{EntityVersion: engineering.EntityVersion{EntityID: "pa"}, EquipmentID: "a", Name: "eth1"}, {EntityVersion: engineering.EntityVersion{EntityID: "pb"}, EquipmentID: "b", Name: "eth2"}}
	return m
}
func TestPublicationFactsActualChanges(t *testing.T) {
	base := PublicationFacts(publicationFixture(), nil)
	m := publicationFixture()
	m.Equipment[0].RevisionID = "new"
	m.Equipment[0].EvidenceIDs = []string{"different-proof"}
	if PublicationChanged(PublicationFacts(m, nil)["a"], base["a"]) {
		t.Fatal("revision/evidence is not an engineering change")
	}
	mac := "02:00:00:00:00:01"
	m.Ports[0].MAC = &mac
	facts := PublicationFacts(m, nil)
	if !PublicationChanged(facts["a"], base["a"]) || PublicationChanged(facts["b"], base["b"]) {
		t.Fatal("port change must affect only its device")
	}
	m = publicationFixture()
	m.Connections = []engineering.PlannedConnection{{EntityVersion: engineering.EntityVersion{EntityID: "link"}, APortID: "pa", ZPortID: "pb"}}
	facts = PublicationFacts(m, nil)
	if !PublicationChanged(facts["a"], base["a"]) || !PublicationChanged(facts["b"], base["b"]) {
		t.Fatal("new connection must affect both endpoints")
	}
}
func TestPublicationFactsScopedInputs(t *testing.T) {
	m := publicationFixture()
	before := []ModelInput{{TableID: "management", Version: 1, Rows: []map[string]string{{"device_id": "a", "management_ip": "192.0.2.1"}, {"device_id": "b", "management_ip": "192.0.2.2"}}}}
	base := PublicationFacts(m, before)
	reordered := []ModelInput{{TableID: "management", Version: 99, Rows: []map[string]string{before[0].Rows[1], before[0].Rows[0]}}}
	if PublicationChanged(PublicationFacts(m, reordered)["a"], base["a"]) {
		t.Fatal("row order and input version alone are not changes")
	}
	after := []ModelInput{{TableID: "management", Rows: []map[string]string{{"device_id": "a", "management_ip": "192.0.2.3"}, before[0].Rows[1]}}}
	facts := PublicationFacts(m, after)
	if !PublicationChanged(facts["a"], base["a"]) || PublicationChanged(facts["b"], base["b"]) {
		t.Fatal("another device's input must not enable selection")
	}
	fresh := PublicationFacts(m, []ModelInput{{TableID: "boot_interfaces", Rows: []map[string]string{{"device_id": "a", "mac": "02:00:00:00:00:01"}}}})
	empty := PublicationFacts(m, nil)
	if PublicationChanged(fresh["b"], empty["b"]) {
		t.Fatal("a new table scoped to another device must not enable selection")
	}
}
