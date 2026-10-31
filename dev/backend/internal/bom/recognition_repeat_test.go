package bom

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRecognizedGraphReplayPreservesModuleIdentity(t *testing.T) {
	b := Workbench{Records: []ImportRecord{
		{ID: "device-a", Item: Item{UID: "device-a", Category: "server", Hostname: "host-a", Fields: map[string]string{}}},
		{ID: "explicit-optic", Item: Item{UID: "explicit-optic", Category: "optic_module", Serial: "synthetic-module-serial", Parent: "device-a", Port: "eth0", Model: "synthetic-optic", Fields: map[string]string{}, Sources: map[string]Cell{}}},
		{ID: "device-b", Item: Item{UID: "device-b", Category: "switch", Hostname: "leaf-b", Fields: map[string]string{}}},
		{ID: "wire", Item: Item{UID: "wire", Category: "cable", Fields: map[string]string{"endpoint_a_device": "host-a", "endpoint_a_port": "eth0", "endpoint_b_device": "leaf-b", "endpoint_b_port": "Ethernet1/1", "endpoint_a_optic": "synthetic-optic"}, Sources: map[string]Cell{"endpoint_a_device": {Sheet: "cables", Row: 2, Column: 1, Ref: "A2", Value: "host-a"}, "endpoint_a_port": {Sheet: "cables", Row: 2, Column: 2, Ref: "B2", Value: "eth0"}, "endpoint_b_device": {Sheet: "cables", Row: 2, Column: 3, Ref: "C2", Value: "leaf-b"}, "endpoint_b_port": {Sheet: "cables", Row: 2, Column: 4, Ref: "D2", Value: "Ethernet1/1"}}}},
	}}
	LinkRecognizedRecords(&b)
	n := len(b.Records)
	if n != 6 {
		t.Fatalf("expected devices, cable, module and two ports; got %d", n)
	}
	for _, r := range b.Records {
		if r.Item.Category == "cable" && (r.Item.A == "" || r.Item.B == "" || r.Item.A == r.Item.B) {
			t.Fatal("cable missing distinct actual port references")
		}
	}
	var module string
	for _, r := range b.Records {
		if r.Item.Category == "optic_module" {
			module = r.ID
		}
	}
	if module == "" {
		t.Fatal("missing explicit source module")
	}
	// Reproduce a stored pre-fix candidate with a resolved parent and its duplicate.
	for _, r := range b.Records {
		if r.ID == module {
			b.Records = append(b.Records, r)
			break
		}
	}
	for pass := 0; pass < 3; pass++ {
		// A second inventory row names the same source device on every replay.
		b.Records = append(b.Records, ImportRecord{ID: "second-source-row", Raw: []Cell{{Sheet: "inventory", Row: 3, Column: 1, Ref: "A3", Value: "host-a"}}, Item: Item{UID: "second-source-row", Category: "server", Hostname: "host-a", Fields: map[string]string{}}})
		LinkRecognizedRecords(&b)
		if len(b.Records) != n {
			t.Fatalf("pass %d changed graph size: %d != %d", pass, len(b.Records), n)
		}
		seen := map[string]bool{}
		for _, r := range b.Records {
			if seen[r.ID] {
				t.Fatal("duplicate candidate ID")
			}
			seen[r.ID] = true
			if r.ID == "device-a" && len(r.Raw) != 1 {
				t.Fatal("replay duplicated immutable source evidence")
			}
		}
		if !seen[module] {
			t.Fatal("module identity changed after parent resolution")
		}
	}
}

func TestLinkDoesNotRewriteReviewedRecords(t *testing.T) {
	b := Workbench{Records: []ImportRecord{
		{ID: "owner", Item: Item{UID: "owner", Category: "server", Hostname: "synthetic-owner", Fields: map[string]string{}}},
		{ID: "reviewed", Confirmed: true, Item: Item{UID: "reviewed", Category: "optic_module", Parent: "synthetic-owner", Port: "eth0", Fields: map[string]string{"parent_uid": "synthetic-owner"}}},
	}}
	before, _ := json.Marshal(b.Records[1])
	LinkRecognizedRecords(&b)
	after, _ := json.Marshal(b.Records[1])
	if !bytes.Equal(before, after) {
		t.Fatal("reviewed source was changed through shared resolver maps")
	}
}

func TestMergedInventoryRepairsExistingComponentReferences(t *testing.T) {
	b := Workbench{Records: []ImportRecord{
		{ID: "first", Item: Item{UID: "first", Category: "switch", Hostname: "synthetic-leaf", Fields: map[string]string{}, Sources: map[string]Cell{}}},
		{ID: "second", Item: Item{UID: "second", Category: "switch", Hostname: "synthetic-leaf", Model: "synthetic-model", Fields: map[string]string{"model": "synthetic-model"}, Sources: map[string]Cell{}}},
		{ID: "optic-first", Item: Item{UID: "optic-first", Category: "optic_module", Parent: "first", Port: "eth1", Model: "synthetic-optic", Fields: map[string]string{"model": "synthetic-optic"}, Sources: map[string]Cell{}}},
		{ID: "optic-second", Item: Item{UID: "optic-second", Category: "optic_module", Parent: "second", Port: "eth1", Model: "synthetic-optic", Fields: map[string]string{"model": "synthetic-optic"}, Sources: map[string]Cell{}}},
		{ID: "existing-port", Item: Item{UID: "existing-port", Category: "port", Parent: "second", Port: "eth1", Fields: map[string]string{}, Sources: map[string]Cell{}}},
	}}
	LinkRecognizedRecords(&b)
	if len(b.Records) != 3 {
		t.Fatalf("components not merged in canonical owner space: %d", len(b.Records))
	}
	for _, r := range b.Records {
		if r.Item.Parent == "second" {
			t.Fatal("reference to removed inventory candidate")
		}
		if r.ID == "first" && r.Item.Model != "synthetic-model" {
			t.Fatal("merged model missing from typed item")
		}
		if r.Item.Category == "optic_module" && r.Item.Fields["port_uid"] != "existing-port" {
			t.Fatal("module lost port association")
		}
	}
}

func TestLegacyDuplicateIDDoesNotOverwriteConflictingEvidence(t *testing.T) {
	b := Workbench{Records: []ImportRecord{
		{ID: "duplicate", Item: Item{UID: "duplicate", Category: "other", Model: "model-a", Fields: map[string]string{}}},
		{ID: "duplicate", Item: Item{UID: "duplicate", Category: "other", Model: "model-b", Fields: map[string]string{}}},
	}}
	LinkRecognizedRecords(&b)
	if len(b.Records) != 2 || b.Records[0].Item.Model == b.Records[1].Item.Model || !b.Records[0].Ambiguity || !b.Records[1].Ambiguity {
		t.Fatal("conflicting legacy identities were silently overwritten")
	}
}

func TestCableModelEnrichesExistingOpticInFirstPass(t *testing.T) {
	b := Workbench{Records: []ImportRecord{
		{ID: "existing-optic", Item: Item{UID: "existing-optic", Category: "optic_module", Parent: "synthetic-owner", Port: "eth1", Fields: map[string]string{}, Sources: map[string]Cell{}}},
		{ID: "cable", Item: Item{UID: "cable", Category: "cable", Fields: map[string]string{"endpoint_a_device": "synthetic-owner", "endpoint_a_port": "eth1", "endpoint_a_optic": "synthetic-model"}, Sources: map[string]Cell{"endpoint_a_optic": {Sheet: "synthetic", Row: 2, Column: 1, Value: "synthetic-model"}}}},
	}}
	LinkRecognizedRecords(&b)
	if len(b.Records) != 2 || b.Records[0].Item.Model != "synthetic-model" || b.Records[0].Item.Sources["model"].Column != 1 {
		t.Fatal("first pass duplicated module instead of enriching source evidence")
	}
	LinkRecognizedRecords(&b)
	if len(b.Records) != 2 || b.Records[0].ID != "existing-optic" {
		t.Fatal("module merge required another pass")
	}
}

func TestEngineeringAcceptsExplicitOwnerAliasButRejectsForeignOwner(t *testing.T) {
	items := []Item{
		{UID: "device", Category: "router", Hostname: "synthetic-router", Fields: map[string]string{"device_alias": "synthetic-label"}},
		{UID: "port", Category: "port", Parent: "device", Port: "eth1", Fields: map[string]string{"speed": "100G"}},
		{UID: "cable", Category: "cable", A: "port", Fields: map[string]string{"endpoint_a_device": "synthetic-label", "endpoint_a_port": "eth1"}},
	}
	for _, i := range Engineering(items, nil) {
		if i.Rule == "ENDPOINT-SOURCE-CONFLICT" {
			t.Fatal("explicit alias rejected")
		}
	}
	items[2].Fields["endpoint_a_device"] = "unrelated-device"
	found := false
	for _, i := range Engineering(items, nil) {
		if i.Rule == "ENDPOINT-SOURCE-CONFLICT" {
			found = true
		}
	}
	if !found {
		t.Fatal("foreign owner accepted")
	}
}

func TestNamedComponentsOwnExplicitPortsWithoutChangingCategory(t *testing.T) {
	for _, kind := range []string{"gpu", "psu", "nic", "bmc"} {
		t.Run(kind, func(t *testing.T) {
			b := Workbench{Records: []ImportRecord{
				{ID: "component", Item: Item{UID: "component", Category: kind, Hostname: "synthetic-component", Fields: map[string]string{}}},
				{ID: "switch", Item: Item{UID: "switch", Category: "switch", Hostname: "synthetic-switch", Fields: map[string]string{}}},
				{ID: "wire", Item: Item{UID: "wire", Category: "cable", Fields: map[string]string{"endpoint_a_device": "synthetic-component", "endpoint_a_port": "eth1", "endpoint_b_device": "synthetic-switch", "endpoint_b_port": "eth2"}, Sources: map[string]Cell{"endpoint_a_device": {Value: "synthetic-component"}, "endpoint_b_device": {Value: "synthetic-switch"}}}},
			}}
			LinkRecognizedRecords(&b)
			if b.Records[0].Item.Category != kind {
				t.Fatal("component category promoted to server")
			}
			for _, r := range b.Records {
				if r.Item.Category == "cable" && (r.Item.A == "" || r.Item.B == "") {
					t.Fatal("explicit component ports absent from candidate graph")
				}
			}
		})
	}
}

func TestCablePartRequirementDoesNotInventSerializedOptic(t *testing.T) {
	b := Workbench{Records: []ImportRecord{{ID: "cable", Item: Item{UID: "cable", Category: "cable", Fields: map[string]string{"endpoint_a_device": "synthetic-owner", "endpoint_a_port": "eth1", "endpoint_a_optic": "synthetic-model"}, Sources: map[string]Cell{"endpoint_a_optic": {Sheet: "synthetic", Row: 2, Column: 3, Ref: "C2", Value: "synthetic-model"}}}}}}
	LinkRecognizedRecords(&b)
	if len(b.Records) != 1 || b.Records[0].Item.Fields["endpoint_a_optic"] != "synthetic-model" || b.Records[0].Item.Sources["endpoint_a_optic"].Ref != "C2" {
		t.Fatal("planned part became physical stock or lost provenance")
	}
}
