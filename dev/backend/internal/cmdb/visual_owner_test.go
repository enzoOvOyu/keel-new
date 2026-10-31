package cmdb

import "testing"

func TestPhysicalPortKeepsComponentOwnerAndEnclosingDevicePrecedence(t *testing.T) {
	index := map[string]*VisualAsset{"p": {ID: "p", Type: "port"}, "g": {ID: "g", Type: "gpu"}, "s1": {ID: "s1", Type: "server"}, "s2": {ID: "s2", Type: "server"}}
	parents := map[string]map[string]bool{"p": {"g": true}}
	if owner, ambiguous := visualPortOwner("p", index, parents); owner != "g" || ambiguous {
		t.Fatal("standalone component port disappeared")
	}
	parents["g"] = map[string]bool{"s1": true}
	if owner, ambiguous := visualPortOwner("p", index, parents); owner != "s1" || ambiguous {
		t.Fatal("enclosing device must remain the aggregate owner")
	}
	parents["g"]["s2"] = true
	if owner, ambiguous := visualPortOwner("p", index, parents); owner != "" || !ambiguous {
		t.Fatal("ambiguous enclosing devices were hidden by component fallback")
	}
	delete(index, "g")
	if owner, _ := visualPortOwner("p", index, parents); owner != "" {
		t.Fatal("invisible component was exposed")
	}
}
