package bom

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRecognizedGraphReplayPreservesModuleIdentity(t *testing.T) {
	b := Workbench{Records: []ImportRecord{
		{ID: "device-a", Item: Item{UID: "device-a", Category: "server", Hostname: "host-a", Fields: map[string]string{}}},
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
